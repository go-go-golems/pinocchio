import { appSlice } from '../store/appSlice';
import { errorsSlice, makeAppError } from '../store/errorsSlice';
import type { AppDispatch } from '../store/store';
import { type TimelineEntity, timelineSlice } from '../store/timelineSlice';
import { logWarn } from '../utils/logger';

type ConnectArgs = {
  sessionId: string;
  basePrefix: string;
  dispatch: AppDispatch;
  onStatus?: (s: string) => void;
  hydrate?: boolean;
};

type CanonicalFrame = Record<string, unknown>;

type SnapshotEntityFrame = {
  kind?: unknown;
  id?: unknown;
  tombstone?: unknown;
  payload?: unknown;
};

function reportError(
  dispatch: AppDispatch,
  message: string,
  scope: string,
  err?: unknown,
  extra?: Record<string, unknown>,
) {
  dispatch(errorsSlice.actions.reportError(makeAppError(message, scope, err, extra)));
}

function safeOrdinal(raw: unknown): number | null {
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    return Number.isSafeInteger(raw) ? raw : null;
  }
  if (typeof raw === 'string' && raw.trim()) {
    const n = Number(raw);
    if (Number.isFinite(n) && Number.isSafeInteger(n)) return n;
  }
  return null;
}

function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function messageEntity(id: string, props: Record<string, unknown>): TimelineEntity {
  return {
    id,
    kind: 'message',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    props: {
      role: 'assistant',
      ...props,
    },
  };
}

function timelineEntityFromSnapshotEntity(entity: SnapshotEntityFrame): TimelineEntity | null {
  const kind = asString(entity?.kind);
  const id = asString(entity?.id);
  const payload = asRecord(entity?.payload);
  if (!id) return null;

  if (kind === 'ChatMessage') {
    const messageId = asString(payload.messageId) || id;
    return messageEntity(messageId, {
      prompt: asString(payload.prompt),
      content: asString(payload.text),
      status: asString(payload.status) || 'idle',
      streaming: payload.streaming === true,
    });
  }

  return {
    id,
    kind: kind || 'system',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    props: payload,
  };
}

function applySnapshot(frame: CanonicalFrame, dispatch: AppDispatch) {
  dispatch(timelineSlice.actions.clear());
  const entities = Array.isArray(frame.entities) ? (frame.entities as SnapshotEntityFrame[]) : [];
  let status = 'idle';
  for (const entity of entities) {
    const mapped = timelineEntityFromSnapshotEntity(entity);
    if (!mapped) continue;
    dispatch(timelineSlice.actions.upsertEntity(mapped));
    if (mapped.kind === 'message') {
      const nextStatus = asString(mapped.props?.status);
      if (nextStatus) status = nextStatus;
    }
  }
  dispatch(appSlice.actions.setStatus(status));
}

function applyUIEvent(frame: CanonicalFrame, dispatch: AppDispatch) {
  const payload = asRecord(frame.payload);
  const messageId = asString(payload.messageId);
  if (!messageId) return;

  switch (asString(frame.name)) {
    case 'ChatMessageStarted':
      dispatch(timelineSlice.actions.upsertEntity(messageEntity(messageId, {
        prompt: asString(payload.prompt),
        content: '',
        status: 'streaming',
        streaming: true,
      })));
      dispatch(appSlice.actions.setStatus('streaming'));
      return;
    case 'ChatMessageAppended':
      dispatch(timelineSlice.actions.upsertEntity(messageEntity(messageId, {
        content: asString(payload.text) || asString(payload.chunk),
        status: 'streaming',
        streaming: true,
      })));
      dispatch(appSlice.actions.setStatus('streaming'));
      return;
    case 'ChatMessageFinished':
      dispatch(timelineSlice.actions.upsertEntity(messageEntity(messageId, {
        content: asString(payload.text),
        status: 'finished',
        streaming: false,
      })));
      dispatch(appSlice.actions.setStatus('finished'));
      return;
    case 'ChatMessageStopped':
      dispatch(timelineSlice.actions.upsertEntity(messageEntity(messageId, {
        content: asString(payload.text),
        status: 'stopped',
        streaming: false,
      })));
      dispatch(appSlice.actions.setStatus('stopped'));
      return;
    default:
      return;
  }
}

class WsManager {
  private ws: WebSocket | null = null;
  private sessionId = '';
  private connectNonce = 0;
  private hydrated = false;
  private buffered: CanonicalFrame[] = [];
  private lastDispatch: AppDispatch | null = null;
  private lastOnStatus: ((s: string) => void) | null = null;

  async connect(args: ConnectArgs) {
    if (this.ws && this.sessionId === args.sessionId) {
      return;
    }
    this.disconnect();

    this.connectNonce++;
    const nonce = this.connectNonce;

    this.sessionId = args.sessionId;
    this.hydrated = false;
    this.buffered = [];
    this.lastDispatch = args.dispatch;
    this.lastOnStatus = args.onStatus ?? null;

    args.onStatus?.('connecting ws...');
    args.dispatch(appSlice.actions.setWsStatus('connecting'));
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${window.location.host}${args.basePrefix}/api/chat/ws`;
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
      setTimeout(() => settleOpen?.(), 1500);
    });

    ws.onopen = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) return;
      args.onStatus?.('ws connected');
      args.dispatch(appSlice.actions.setWsStatus('connected'));
      try {
        ws.send(JSON.stringify({ type: 'subscribe', sessionId: args.sessionId, sinceOrdinal: '0' }));
      } catch (err) {
        reportError(args.dispatch, 'ws subscribe failed', 'ws.subscribe', err, { sessionId: args.sessionId });
      }
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
      logWarn('websocket error', { scope: 'ws.onerror', sessionId: args.sessionId });
      reportError(args.dispatch, 'websocket error', 'ws.onerror', undefined, { sessionId: args.sessionId });
      args.onStatus?.('ws error');
      args.dispatch(appSlice.actions.setWsStatus('error'));
    };
    ws.onmessage = (m) => {
      if (nonce !== this.connectNonce) return;
      try {
        const frame = JSON.parse(String(m.data)) as CanonicalFrame;
        const ord = safeOrdinal(frame.ordinal);
        if (ord !== null) {
          args.dispatch(appSlice.actions.setLastSeq(ord));
        }
        this.handleFrame(frame, args, nonce);
      } catch (err) {
        logWarn('ws message parse failed', { scope: 'ws.onmessage', extra: { data: String(m.data).slice(0, 200) } }, err);
      }
    };

    await openPromise;
  }

  disconnect() {
    this.connectNonce++;
    this.lastOnStatus?.('ws disconnected');
    this.lastDispatch?.(appSlice.actions.setWsStatus('disconnected'));
    try {
      this.ws?.close();
    } catch (err) {
      logWarn('ws close failed', { scope: 'ws.close', sessionId: this.sessionId }, err);
    }
    this.ws = null;
    this.sessionId = '';
    this.hydrated = false;
    this.buffered = [];
  }

  private handleFrame(frame: CanonicalFrame, args: ConnectArgs, nonce: number) {
    const type = asString(frame.type);
    if (type === 'hello') {
      return;
    }
    if (type === 'error') {
      reportError(args.dispatch, asString(frame.error) || 'ws error', 'ws.frame', undefined, { frame });
      return;
    }
    if (type === 'snapshot') {
      if (nonce !== this.connectNonce) return;
      applySnapshot(frame, args.dispatch);
      this.hydrated = true;
      args.onStatus?.('hydrated');
      const buffered = this.buffered;
      this.buffered = [];
      for (const next of buffered) {
        applyUIEvent(next, args.dispatch);
      }
      return;
    }
    if (type === 'subscribed') {
      args.onStatus?.('subscribed');
      return;
    }
    if (type === 'ui-event') {
      if (!this.hydrated) {
        this.buffered.push(frame);
        return;
      }
      applyUIEvent(frame, args.dispatch);
    }
  }
}

export const wsManager = new WsManager();
