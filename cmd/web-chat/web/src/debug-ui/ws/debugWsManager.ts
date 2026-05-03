import type { AppDispatch } from '../../store/store';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { setFollowStatus } from '../store/uiSlice';

type ConnectArgs = {
  sessionId: string;
  basePrefix: string;
  dispatch: AppDispatch;
};

type CanonicalFrame = Record<string, unknown>;

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

    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${window.location.host}${args.basePrefix}/api/chat/ws`;
    const ws = new WebSocket(url);
    this.ws = ws;

    args.dispatch(setFollowStatus('connecting'));

    ws.onopen = () => {
      if (nonce !== this.nonce) return;
      ws.send(JSON.stringify({ type: 'subscribe', sessionId: args.sessionId, sinceOrdinal: '0' }));
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
        frame = JSON.parse(String(message.data)) as CanonicalFrame;
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

export function useDebugTimelineFollow() {
  const dispatch = useAppDispatch();
  const selectedSessionId = useAppSelector((state) => state.ui.selectedSessionId);
  const follow = useAppSelector((state) => state.ui.follow);
  const basePrefix = '';

  // This is intentionally simple — the real connect/disconnect lifecycle
  // is managed by the component that reads follow.enabled and sessionId.
  return { dispatch, selectedSessionId, follow, basePrefix };
}
