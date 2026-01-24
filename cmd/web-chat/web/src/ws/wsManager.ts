import type { AppDispatch } from '../store/store';
import { timelineSlice } from '../store/timelineSlice';
import { handleSem, registerDefaultSemHandlers } from '../sem/registry';

type ConnectArgs = {
  convId: string;
  basePrefix: string;
  dispatch: AppDispatch;
  onStatus?: (s: string) => void;
};

type RawSemEnvelope = any;

function seqFromEnvelope(envelope: RawSemEnvelope): number | null {
  const seq = envelope?.event?.seq;
  if (typeof seq === 'number' && Number.isFinite(seq)) return seq;
  return null;
}

class WsManager {
  private ws: WebSocket | null = null;
  private convId: string = '';
  private connectNonce = 0;
  private hydrated: boolean = false;
  private buffered: RawSemEnvelope[] = [];

  async connect(args: ConnectArgs) {
    if (this.ws && this.convId === args.convId) return;
    this.disconnect();

    this.connectNonce++;
    const nonce = this.connectNonce;

    this.convId = args.convId;
    this.hydrated = false;
    this.buffered = [];

    registerDefaultSemHandlers();

    args.onStatus?.('connecting ws...');
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${window.location.host}${args.basePrefix}/ws?conv_id=${encodeURIComponent(args.convId)}`;
    const ws = new WebSocket(url);
    this.ws = ws;

    ws.onopen = () => {
      if (nonce !== this.connectNonce) return;
      args.onStatus?.('ws connected');
    };
    ws.onclose = () => {
      if (nonce !== this.connectNonce) return;
      args.onStatus?.('ws closed');
    };
    ws.onerror = () => {
      if (nonce !== this.connectNonce) return;
      args.onStatus?.('ws error');
    };
    ws.onmessage = (m) => {
      if (nonce !== this.connectNonce) return;
      try {
        const payload = JSON.parse(String(m.data));
        if (!this.hydrated) {
          this.buffered.push(payload);
          return;
        }
        handleSem(payload, args.dispatch);
      } catch {
        // ignore
      }
    };

    args.onStatus?.('hydrating...');
    await this.hydrate(args, nonce);
  }

  disconnect() {
    this.connectNonce++;
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

  private async hydrate(args: ConnectArgs, nonce: number) {
    if (this.hydrated) return;
    args.dispatch(timelineSlice.actions.clear());

    const res = await fetch(`${args.basePrefix}/hydrate?conv_id=${encodeURIComponent(args.convId)}`);
    const j = await res.json();
    const frames = ((j && j.frames) || []) as RawSemEnvelope[];

    if (nonce !== this.connectNonce) return;

    const orderedFrames = [...frames].sort((a, b) => (seqFromEnvelope(a) ?? 0) - (seqFromEnvelope(b) ?? 0));
    let lastSeq = 0;
    for (const fr of orderedFrames) {
      const seq = seqFromEnvelope(fr);
      if (seq && seq > lastSeq) lastSeq = seq;
      handleSem(fr, args.dispatch);
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
