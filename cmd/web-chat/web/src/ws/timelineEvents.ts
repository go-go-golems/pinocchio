import { appSlice } from '../store/appSlice';
import type { AppDispatch } from '../store/store';
import { type TimelineEntity, timelineSlice } from '../store/timelineSlice';
import { asRecord, asString, type CanonicalFrame } from './protocol';
import { recordUIEventDebug } from './streamDebug';
import { agentModeEntity, agentModePreviewEntityId, messageEntity } from './timelineSnapshot';

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

export function applyUIEvent(frame: CanonicalFrame, dispatch: AppDispatch, sessionId = '') {
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
