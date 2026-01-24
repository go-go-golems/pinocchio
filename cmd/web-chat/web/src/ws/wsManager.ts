import type { AppDispatch } from '../store/store';
import { timelineSlice } from '../store/timelineSlice';
import { handleSem, registerDefaultSemHandlers } from '../sem/registry';

type ConnectArgs = {
  convId: string;
  basePrefix: string;
  dispatch: AppDispatch;
  onStatus?: (s: string) => void;
};

class WsManager {
  private ws: WebSocket | null = null;
  private convId: string = '';
  private hydrated: boolean = false;

  async connect(args: ConnectArgs) {
    if (this.ws && this.convId === args.convId) return;
    this.disconnect();

    this.convId = args.convId;
    this.hydrated = false;

    registerDefaultSemHandlers();

    args.onStatus?.('hydrating...');
    await this.hydrate(args);

    args.onStatus?.('connecting ws...');
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${window.location.host}${args.basePrefix}/ws?conv_id=${encodeURIComponent(args.convId)}`;
    const ws = new WebSocket(url);
    this.ws = ws;

    ws.onopen = () => args.onStatus?.('ws connected');
    ws.onclose = () => args.onStatus?.('ws closed');
    ws.onerror = () => args.onStatus?.('ws error');
    ws.onmessage = (m) => {
      try {
        const payload = JSON.parse(String(m.data));
        handleSem(payload, args.dispatch);
      } catch {
        // ignore
      }
    };
  }

  disconnect() {
    try {
      this.ws?.close();
    } catch {
      // ignore
    }
    this.ws = null;
    this.convId = '';
    this.hydrated = false;
  }

  private async hydrate(args: ConnectArgs) {
    if (this.hydrated) return;
    args.dispatch(timelineSlice.actions.clear());

    const res = await fetch(`${args.basePrefix}/hydrate?conv_id=${encodeURIComponent(args.convId)}`);
    const j = await res.json();
    const frames = (j && j.frames) || [];
    for (const fr of frames) {
      handleSem(fr, args.dispatch);
    }
    this.hydrated = true;
    args.onStatus?.('hydrated');
  }
}

export const wsManager = new WsManager();

