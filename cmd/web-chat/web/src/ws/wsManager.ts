import { fromJson } from '@bufbuild/protobuf';
import { type TimelineSnapshotV2, TimelineSnapshotV2Schema } from '../sem/pb/proto/sem/timeline/transport_pb';
import { handleSem, registerDefaultSemHandlers } from '../sem/registry';
import { timelineEntityFromProto } from '../sem/timelineMapper';
import { appSlice } from '../store/appSlice';
import { errorsSlice, makeAppError } from '../store/errorsSlice';
import type { AppDispatch } from '../store/store';
import { timelineSlice } from '../store/timelineSlice';
import { isRecord } from '../utils/guards';
import { logError, logWarn } from '../utils/logger';

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

function applyTimelineSnapshot(snapshot: TimelineSnapshotV2, dispatch: AppDispatch) {
  if (!snapshot?.entities || !Array.isArray(snapshot.entities)) return;
  for (const e of snapshot.entities) {
    const mapped = timelineEntityFromProto(e, snapshot.version);
    if (!mapped) continue;
    dispatch(timelineSlice.actions.upsertEntity(mapped));
  }
}

function reportError(
  dispatch: AppDispatch,
  message: string,
  scope: string,
  err?: unknown,
  extra?: Record<string, unknown>,
) {
  dispatch(errorsSlice.actions.reportError(makeAppError(message, scope, err, extra)));
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
      logWarn('websocket error', { scope: 'ws.onerror', convId: args.convId });
      reportError(args.dispatch, 'websocket error', 'ws.onerror', undefined, { convId: args.convId });
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
      } catch (err) {
        logWarn('ws message parse failed', { scope: 'ws.onmessage', extra: { data: String(m.data).slice(0, 200) } }, err);
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
    } catch (err) {
      logWarn('ws close failed', { scope: 'ws.close', convId: this.convId }, err);
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

    // Hydrate via core timeline API endpoint; this is not debug-only.
    try {
      const res = await fetch(`${args.basePrefix}/api/timeline?conv_id=${encodeURIComponent(args.convId)}`);
      if (res.ok) {
        let j: unknown = null;
        try {
          j = await res.json();
        } catch (err) {
          logError('hydrate json parse failed', err, { scope: 'hydrate', convId: args.convId });
          reportError(args.dispatch, 'hydrate json parse failed', 'hydrate', err, { convId: args.convId });
          j = null;
        }
        if (nonce !== this.connectNonce) return;
        if (isRecord(j)) {
          const snap = fromJson(TimelineSnapshotV2Schema as any, j as any, { ignoreUnknownFields: true }) as any;
          if (snap) {
            if (nonce !== this.connectNonce) return;
            applyTimelineSnapshot(snap, args.dispatch);
          }
        } else if (j !== null) {
          logWarn('hydrate payload invalid', { scope: 'hydrate', convId: args.convId });
          reportError(args.dispatch, 'hydrate payload invalid', 'hydrate', undefined, { convId: args.convId });
        }
      } else {
        logWarn('hydrate http error', { scope: 'hydrate', convId: args.convId, extra: { status: res.status } });
        reportError(args.dispatch, 'hydrate http error', 'hydrate', undefined, { convId: args.convId, status: res.status });
      }
    } catch (err) {
      logError('hydrate failed', err, { scope: 'hydrate', convId: args.convId });
      reportError(args.dispatch, 'hydrate failed', 'hydrate', err, { convId: args.convId });
    }

    if (nonce !== this.connectNonce) return;

    const lastSeq = 0;

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
