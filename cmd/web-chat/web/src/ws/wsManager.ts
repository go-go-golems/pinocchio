import { appSlice } from '../store/appSlice';
import { errorsSlice, makeAppError } from '../store/errorsSlice';
import type { AppDispatch } from '../store/store';
import { type TimelineEntity, timelineSlice } from '../store/timelineSlice';
import { logWarn } from '../utils/logger';
import {
  asRecord,
  asString,
  buildWebSocketURL,
  type CanonicalFrame,
  encodeSubscribeFrame,
  parseServerFrame,
  safeOrdinal,
} from './protocol';
import {
  recordLifecycle,
  recordParsedFrame,
  recordRawWS,
  recordUIEventDebug,
} from './streamDebug';
import { applySnapshot, agentModeEntity, agentModePreviewEntityId, messageEntity } from './timelineSnapshot';

export { timelineEntityFromSnapshotEntity } from './timelineSnapshot';

type ConnectArgs = {
  sessionId: string;
  basePrefix: string;
  dispatch: AppDispatch;
  onStatus?: (s: string) => void;
  hydrate?: boolean;
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

type TimelineMutation = {
  upsert?: TimelineEntity;
  deleteId?: string;
  status?: string;
};

export function timelineMutationFromUIEvent(frame: CanonicalFrame): TimelineMutation | null {
  const payload = asRecord(frame.payload);
  const messageId = asString(payload.messageId);
  if (!messageId) return null;

  switch (asString(frame.name)) {
    case 'ChatMessageAccepted':
      return {
        upsert: messageEntity(messageId, {
          role: asString(payload.role) || 'user',
          content: asString(payload.content) || asString(payload.text),
          status: asString(payload.status) || 'submitted',
          streaming: payload.streaming === true,
        }),
      };
    case 'ChatMessageStarted': {
      const content = asString(payload.content) || asString(payload.text);
      return {
        upsert: content
          ? messageEntity(messageId, {
              role: asString(payload.role) || 'assistant',
              prompt: asString(payload.prompt),
              content,
              status: asString(payload.status) || 'streaming',
              streaming: true,
            })
          : undefined,
        status: 'streaming',
      };
    }
    case 'ChatMessageAppended':
      return {
        upsert: messageEntity(messageId, {
          role: asString(payload.role) || 'assistant',
          content: asString(payload.content) || asString(payload.text) || asString(payload.chunk),
          status: asString(payload.status) || 'streaming',
          streaming: true,
        }),
        status: 'streaming',
      };
    case 'ChatMessageFinished': {
      const content = asString(payload.content) || asString(payload.text);
      return {
        upsert: content
          ? messageEntity(messageId, {
              role: asString(payload.role) || 'assistant',
              prompt: asString(payload.prompt),
              content,
              status: asString(payload.status) || 'finished',
              streaming: false,
            })
          : undefined,
        status: 'finished',
      };
    }
    case 'ChatMessageStopped': {
      const content = asString(payload.content) || asString(payload.text);
      const error = asString(payload.error);
      return {
        upsert: content || error
          ? messageEntity(messageId, {
              role: asString(payload.role) || 'assistant',
              prompt: asString(payload.prompt),
              content,
              status: asString(payload.status) || 'stopped',
              streaming: false,
              error,
            })
          : undefined,
        status: 'stopped',
      };
    }
    case 'ChatReasoningStarted': {
      const content = asString(payload.content) || asString(payload.text);
      if (!content) return null;
      return {
        upsert: messageEntity(messageId, {
          role: 'thinking',
          content,
          status: asString(payload.status) || 'streaming',
          streaming: payload.streaming !== false,
        }),
        status: 'streaming',
      };
    }
    case 'ChatReasoningAppended':
      return {
        upsert: messageEntity(messageId, {
          role: 'thinking',
          content: asString(payload.content) || asString(payload.text) || asString(payload.chunk),
          status: asString(payload.status) || 'streaming',
          streaming: payload.streaming !== false,
        }),
        status: 'streaming',
      };
    case 'ChatReasoningFinished': {
      const content = asString(payload.content) || asString(payload.text);
      if (!content) return null;
      return {
        upsert: messageEntity(messageId, {
          role: 'thinking',
          content,
          status: asString(payload.status) || 'finished',
          streaming: false,
        }),
      };
    }
    case 'ChatAgentModePreviewUpdated':
      return {
        upsert: agentModeEntity(agentModePreviewEntityId(messageId), 'agent_mode_preview', {
          title: 'Agent mode preview',
          data: {
            from: '',
            to: asString(payload.candidateMode),
            analysis: asString(payload.analysis),
            parseState: asString(payload.parseState),
          },
          preview: true,
          messageId,
        }),
      };
    case 'ChatAgentModeCommitted':
      return {
        upsert: agentModeEntity('agent-mode', 'agent_mode', {
          title: asString(payload.title) || 'Agent mode switch',
          data: {
            from: asString(payload.from),
            to: asString(payload.to),
            analysis: asString(payload.analysis),
          },
          preview: false,
          messageId,
        }),
      };
    case 'ChatAgentModePreviewCleared':
      return { deleteId: agentModePreviewEntityId(messageId) };
    default:
      return null;
  }
}

function applyUIEvent(frame: CanonicalFrame, dispatch: AppDispatch, sessionId = '') {
  const mutation = timelineMutationFromUIEvent(frame);
  recordUIEventDebug(sessionId, frame, mutation);
  if (!mutation) return;
  if (mutation.deleteId) {
    dispatch(timelineSlice.actions.deleteEntity(mutation.deleteId));
  }
  if (mutation.upsert) {
    dispatch(timelineSlice.actions.upsertEntity(mutation.upsert));
  }
  if (mutation.status) {
    dispatch(appSlice.actions.setStatus(mutation.status));
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
    const ws = new WebSocket(buildWebSocketURL({ basePrefix: args.basePrefix }));
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

    recordLifecycle(args.sessionId, 'connect-start', { basePrefix: args.basePrefix });

    ws.onopen = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) return;
      recordLifecycle(args.sessionId, 'open');
      args.onStatus?.('ws connected');
      args.dispatch(appSlice.actions.setWsStatus('connected'));
      try {
        ws.send(encodeSubscribeFrame(args.sessionId));
      } catch (err) {
        reportError(args.dispatch, 'ws subscribe failed', 'ws.subscribe', err, { sessionId: args.sessionId });
      }
    };
    ws.onclose = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) return;
      recordLifecycle(args.sessionId, 'close');
      args.onStatus?.('ws closed');
      args.dispatch(appSlice.actions.setWsStatus('closed'));
    };
    ws.onerror = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) return;
      recordLifecycle(args.sessionId, 'error');
      logWarn('websocket error', { scope: 'ws.onerror', sessionId: args.sessionId });
      reportError(args.dispatch, 'websocket error', 'ws.onerror', undefined, { sessionId: args.sessionId });
      args.onStatus?.('ws error');
      args.dispatch(appSlice.actions.setWsStatus('error'));
    };
    ws.onmessage = (m) => {
      if (nonce !== this.connectNonce) return;
      try {
        const raw = String(m.data);
        recordRawWS(args.sessionId, raw);
        const frame = parseServerFrame(raw);
        recordParsedFrame(args.sessionId, frame);
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
      applySnapshot(frame, args.dispatch, args.sessionId);
      this.hydrated = true;
      args.onStatus?.('hydrated');
      const buffered = this.buffered;
      this.buffered = [];
      for (const next of buffered) {
        applyUIEvent(next, args.dispatch, args.sessionId);
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
      applyUIEvent(frame, args.dispatch, args.sessionId);
    }
  }
}

export const wsManager = new WsManager();
