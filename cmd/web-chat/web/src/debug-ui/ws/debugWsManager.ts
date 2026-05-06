import type { AppDispatch } from '../../store/store';
import { setFollowStatus } from '../store/uiSlice';

type ConnectArgs = {
  sessionId: string;
  basePrefix: string;
  dispatch: AppDispatch;
};

type CanonicalFrame = Record<string, unknown>;

function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function normalizeServerFrame(frame: CanonicalFrame): CanonicalFrame {
  if ('type' in frame) return frame;
  if (frame.hello) return { type: 'hello', ...(asRecord(frame.hello)) };
  if (frame.snapshot) {
    const snapshot = asRecord(frame.snapshot);
    return {
      type: 'snapshot',
      sessionId: asString(snapshot.sessionId),
      ordinal: snapshot.snapshotOrdinal,
      entities: Array.isArray(snapshot.entities) ? snapshot.entities : [],
    };
  }
  if (frame.subscribed) return { type: 'subscribed', ...(asRecord(frame.subscribed)) };
  if (frame.uiEvent) {
    const uiEvent = asRecord(frame.uiEvent);
    return {
      type: 'ui-event',
      sessionId: asString(uiEvent.sessionId),
      ordinal: uiEvent.eventOrdinal,
      name: asString(uiEvent.name),
      payload: asRecord(uiEvent.payload),
    };
  }
  if (frame.error) {
    const error = asRecord(frame.error);
    return {
      type: 'error',
      sessionId: asString(error.sessionId),
      error: asString(error.message),
      code: asString(error.code),
      detail: asString(error.detail),
    };
  }
  return frame;
}

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
      ws.send(JSON.stringify({ subscribe: { sessionId: args.sessionId, sinceSnapshotOrdinal: '0' } }));
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
        frame = normalizeServerFrame(JSON.parse(String(message.data)) as CanonicalFrame);
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
