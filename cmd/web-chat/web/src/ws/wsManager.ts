import type { AppDispatch } from '../store/store';
import { timelineSlice } from '../store/timelineSlice';
import { handleSem, registerDefaultSemHandlers } from '../sem/registry';
import { appSlice } from '../store/appSlice';
import { fromJson } from '@bufbuild/protobuf';
import { TimelineSnapshotV1Schema, type TimelineEntityV1, type TimelineSnapshotV1 } from '../sem/pb/proto/sem/timeline/transport_pb';

type ConnectArgs = {
  convId: string;
  basePrefix: string;
  dispatch: AppDispatch;
  onStatus?: (s: string) => void;
  hydrate?: boolean;
};

type RawSemEnvelope = any;

function seqFromEnvelope(envelope: RawSemEnvelope): number | null {
  const seq = envelope?.event?.seq;
  if (typeof seq === 'number' && Number.isFinite(seq)) return seq;
  return null;
}

function isObject(v: unknown): v is Record<string, any> {
  return !!v && typeof v === "object";
}

function applyTimelineSnapshot(snapshot: TimelineSnapshotV1, dispatch: AppDispatch) {
  if (!snapshot?.entities) return;
  for (const e of snapshot.entities) {
    if (!e?.id || !e?.kind) continue;
    dispatch(
      timelineSlice.actions.upsertEntity({
        id: e.id,
        kind: e.kind,
        createdAt: Number((e as any).createdAtMs ?? 0) || Date.now(),
        updatedAt: Number((e as any).updatedAtMs ?? 0) || Date.now(),
        props: propsFromTimelineEntity(e),
      }),
    );
  }
}

function propsFromTimelineEntity(e: TimelineEntityV1): any {
  const kind = e.kind;
  const snap = (e as any).snapshot;
  if (!snap || !isObject(snap)) return {};

  // Note: bufbuild/es represents oneofs as `{ case, value }`.
  const oneof = snap as any;
  const val = oneof.value;

  if (kind === 'message' && oneof.case === 'message') {
    return { role: val?.role, content: val?.content, streaming: !!val?.streaming };
  }
  if (kind === 'tool_call' && oneof.case === 'toolCall') {
    return { name: val?.name, input: val?.input ?? {}, status: val?.status, progress: val?.progress, done: !!val?.done };
  }
  if (kind === 'tool_result' && oneof.case === 'toolResult') {
    return { result: val?.resultRaw ?? '', customKind: val?.customKind ?? '' };
  }
  if (kind === 'thinking_mode' && oneof.case === 'thinkingMode') {
    const status = val?.status ?? '';
    const success = status === 'completed' ? true : status === 'error' ? false : undefined;
    return {
      status,
      mode: val?.mode,
      phase: val?.phase,
      reasoning: val?.reasoning,
      success: typeof val?.success === 'boolean' ? val.success : success,
      error: val?.error ?? '',
    };
  }
  if (kind === 'planning' && oneof.case === 'planning') {
    const iterations = Array.isArray(val?.iterations)
      ? val.iterations.map((it: any) => ({
          index: Number(it?.iterationIndex ?? 0),
          action: String(it?.action ?? ''),
          reasoning: String(it?.reasoning ?? ''),
          strategy: String(it?.strategy ?? ''),
          progress: String(it?.progress ?? ''),
          toolName: String(it?.toolName ?? ''),
          reflectionText: String(it?.reflectionText ?? ''),
          emittedAt: Number(it?.emittedAtUnixMs ?? 0),
        }))
      : [];
    const execution = val?.execution
      ? {
          executorModel: val.execution.executorModel,
          directive: val.execution.directive,
          startedAt: Number(val.execution.startedAtUnixMs ?? 0),
          status: val.execution.status,
          errorMessage: val.execution.errorMessage,
        }
      : null;
    const completed =
      val?.finalDecision || val?.statusReason || val?.finalDirective
        ? {
            finalDecision: val?.finalDecision ?? '',
            statusReason: val?.statusReason ?? '',
            finalDirective: val?.finalDirective ?? '',
          }
        : null;
    return {
      runId: val?.runId,
      provider: val?.provider,
      plannerModel: val?.plannerModel,
      maxIterations: Number(val?.maxIterations ?? 0) || undefined,
      iterations,
      completed,
      execution,
    };
  }
  if (kind === 'status' && oneof.case === 'status') {
    return { text: val?.text, type: val?.type };
  }
  if (kind === 'team_analysis' && oneof.case === 'teamAnalysis') {
    return val ?? {};
  }

  return val ?? {};
}

class WsManager {
  private ws: WebSocket | null = null;
  private convId: string = '';
  private connectNonce = 0;
  private hydrated: boolean = false;
  private buffered: RawSemEnvelope[] = [];
  private lastDispatch: AppDispatch | null = null;
  private lastOnStatus: ((s: string) => void) | null = null;

  async connect(args: ConnectArgs) {
    if (this.ws && this.convId === args.convId) {
      if (args.hydrate !== false) {
        await this.ensureHydrated(args);
      }
      return;
    }
    this.disconnect();

    this.connectNonce++;
    const nonce = this.connectNonce;

    this.convId = args.convId;
    this.hydrated = false;
    this.buffered = [];
    this.lastDispatch = args.dispatch;
    this.lastOnStatus = args.onStatus ?? null;

    registerDefaultSemHandlers();

    args.onStatus?.('connecting ws...');
    args.dispatch(appSlice.actions.setWsStatus('connecting'));
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${window.location.host}${args.basePrefix}/ws?conv_id=${encodeURIComponent(args.convId)}`;
    const ws = new WebSocket(url);
    this.ws = ws;

    let settleOpen: (() => void) | null = null;
    const openPromise = new Promise<void>((resolve) => {
      let settled = false;
      settleOpen = () => {
        if (settled) return;
        settled = true;
        resolve();
      };
      // Don't hang forever on first-message send; best-effort timeout.
      setTimeout(() => settleOpen?.(), 1500);
    });

    ws.onopen = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) return;
      args.onStatus?.('ws connected');
      args.dispatch(appSlice.actions.setWsStatus('connected'));
    };
    ws.onclose = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) return;
      args.onStatus?.('ws closed');
      args.dispatch(appSlice.actions.setWsStatus('closed'));
    };
    ws.onerror = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) return;
      args.onStatus?.('ws error');
      args.dispatch(appSlice.actions.setWsStatus('error'));
    };
    ws.onmessage = (m) => {
      if (nonce !== this.connectNonce) return;
      try {
        const payload = JSON.parse(String(m.data));
        const seq = seqFromEnvelope(payload);
        if (seq !== null) args.dispatch(appSlice.actions.setLastSeq(seq));
        if (!this.hydrated) {
          this.buffered.push(payload);
          return;
        }
        handleSem(payload, args.dispatch);
      } catch {
        // ignore
      }
    };

    await openPromise;
    if (nonce !== this.connectNonce) return;

    if (args.hydrate === false) return;

    args.onStatus?.('hydrating...');
    await this.hydrate(args, nonce);
  }

  disconnect() {
    this.connectNonce++;
    this.lastOnStatus?.('ws disconnected');
    this.lastDispatch?.(appSlice.actions.setWsStatus('disconnected'));
    try {
      this.ws?.close();
    } catch {
      // ignore
    }
    this.ws = null;
    this.convId = '';
    this.hydrated = false;
    this.buffered = [];
  }

  async ensureHydrated(args: ConnectArgs) {
    if (!args?.convId) return;
    if (!this.ws || this.convId !== args.convId) return;
    if (this.hydrated) return;
    const nonce = this.connectNonce;
    args.onStatus?.('hydrating...');
    await this.hydrate(args, nonce);
  }

  private async hydrate(args: ConnectArgs, nonce: number) {
    if (this.hydrated) return;
    if (nonce !== this.connectNonce) return;
    args.dispatch(timelineSlice.actions.clear());

    // Prefer durable hydration via GET /timeline (PI-004).
    // If the server has no timeline store enabled, fall back to legacy GET /hydrate.
    let hydratedViaTimeline = false;
    try {
      const res = await fetch(`${args.basePrefix}/timeline?conv_id=${encodeURIComponent(args.convId)}`);
      if (res.ok) {
        const j = await res.json();
        if (nonce !== this.connectNonce) return;
        if (isObject(j)) {
          const snap = fromJson(TimelineSnapshotV1Schema as any, j as any, { ignoreUnknownFields: true }) as any;
          if (snap) {
            if (nonce !== this.connectNonce) return;
            applyTimelineSnapshot(snap, args.dispatch);
            hydratedViaTimeline = true;
          }
        }
      }
    } catch {
      // ignore
    }

    let frames: RawSemEnvelope[] = [];
    let lastSeqFromServer: number | null = null;
    let queueDepth: number | null = null;
    if (!hydratedViaTimeline) {
      try {
        const res = await fetch(`${args.basePrefix}/hydrate?conv_id=${encodeURIComponent(args.convId)}`);
        const j = await res.json();
        frames = ((j && j.frames) || []) as RawSemEnvelope[];
        lastSeqFromServer = j && typeof j.last_seq === 'number' ? j.last_seq : null;
        queueDepth = j && typeof j.queue_depth === 'number' ? j.queue_depth : null;
      } catch {
        // If hydration fails (e.g. dev proxy misconfigured and we get HTML), don't block streaming.
        frames = [];
        lastSeqFromServer = null;
        queueDepth = null;
      }
    }

    if (nonce !== this.connectNonce) return;

    if (queueDepth !== null) args.dispatch(appSlice.actions.setQueueDepth(queueDepth));
    if (lastSeqFromServer !== null) args.dispatch(appSlice.actions.setLastSeq(lastSeqFromServer));

    let lastSeq = 0;
    if (!hydratedViaTimeline) {
      const orderedFrames = [...frames].sort((a, b) => (seqFromEnvelope(a) ?? 0) - (seqFromEnvelope(b) ?? 0));
      for (const fr of orderedFrames) {
        const seq = seqFromEnvelope(fr);
        if (seq && seq > lastSeq) lastSeq = seq;
        handleSem(fr, args.dispatch);
      }
    }

    if (nonce !== this.connectNonce) return;

    this.hydrated = true;
    args.onStatus?.('hydrated');

    const buffered = this.buffered;
    this.buffered = [];
    buffered.sort((a, b) => (seqFromEnvelope(a) ?? 0) - (seqFromEnvelope(b) ?? 0));
    for (const fr of buffered) {
      const seq = seqFromEnvelope(fr);
      if (seq && lastSeq && seq <= lastSeq) continue;
      handleSem(fr, args.dispatch);
    }
  }
}

export const wsManager = new WsManager();
