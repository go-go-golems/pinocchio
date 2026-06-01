import { buildWebSocketURL, type CanonicalFrame, encodeSubscribeFrame, parseServerFrame } from '../../ws/protocol';
import type { AppDispatch } from '../store/store';
import { setFollowStatus } from '../store/uiSlice';

type ConnectArgs = {
  sessionId: string;
  basePrefix: string;
  dispatch: AppDispatch;
};

let onFrame: ((frame: CanonicalFrame) => void) | null = null;

export function setOnFrame(fn: ((frame: CanonicalFrame) => void) | null) {
  onFrame = fn;
}

class DebugWsManager {
  private ws: WebSocket | null = null;
  private nonce = 0;

  disconnect() {
    this.nonce++;
    try { this.ws?.close(); } catch { /* no-op */ }
    this.ws = null;
  }

  async connect(args: ConnectArgs) {
    this.disconnect();
    const nonce = ++this.nonce;

    const ws = new WebSocket(buildWebSocketURL({ basePrefix: args.basePrefix }));
    this.ws = ws;

    args.dispatch(setFollowStatus('connecting'));

    ws.onopen = () => {
      if (nonce !== this.nonce) return;
      ws.send(encodeSubscribeFrame(args.sessionId));
    };

    ws.onclose = () => {
      if (nonce !== this.nonce) return;
      args.dispatch(setFollowStatus('closed'));
    };

    ws.onerror = () => {
      if (nonce !== this.nonce) return;
      args.dispatch(setFollowStatus('closed'));
    };

    ws.onmessage = (message) => {
      if (nonce !== this.nonce) return;
      let frame: CanonicalFrame;
      try {
        frame = parseServerFrame(String(message.data));
      } catch {
        return;
      }
      const type = String(frame.type ?? '');

      if (type === 'snapshot') {
        args.dispatch(setFollowStatus('connected'));
      }

      if (onFrame) {
        onFrame(frame);
      }
    };
  }
}

export const debugWsManager = new DebugWsManager();
