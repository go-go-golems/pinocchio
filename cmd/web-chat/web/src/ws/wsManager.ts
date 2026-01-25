import type { AppDispatch } from '../store/store';
import { timelineSlice } from '../store/timelineSlice';
import { handleSem, registerDefaultSemHandlers } from '../sem/registry';
import { appSlice } from '../store/appSlice';
import { fromJson } from '@bufbuild/protobuf';
import { TimelineSnapshotV1Schema, type TimelineSnapshotV1 } from '../sem/pb/proto/sem/timeline/transport_pb';
import { timelineEntityFromProto } from '../sem/timelineMapper';

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

function applyTimelineSnapshot(snapshot: TimelineSnapshotV1, dispatch: AppDispatch) {
  if (!snapshot?.entities) return;
  for (const e of snapshot.entities) {
    const mapped = timelineEntityFromProto(e, snapshot.version);
    if (!mapped) continue;
    dispatch(timelineSlice.actions.upsertEntity(mapped));
  }
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

    // Hydrate via GET /timeline (canonical path).
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

    if (nonce !== this.connectNonce) return;

    let lastSeq = 0;

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
