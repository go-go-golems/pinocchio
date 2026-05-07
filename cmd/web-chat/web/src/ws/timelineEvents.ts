import { appSlice } from '../store/appSlice';
import type { AppDispatch } from '../store/store';
import { type TimelineEntity, timelineSlice } from '../store/timelineSlice';
import { decodeKnownUIEvent } from './chatappPayloads';
import type { CanonicalFrame } from './protocol';
import { recordUIEventDebug } from './streamDebug';
import { agentModeEntity, agentModePreviewEntityId, messageEntity } from './timelineSnapshot';

type TimelineMutation = {
  upsert?: TimelineEntity;
  deleteId?: string;
  status?: string;
};

function visibleText(payload: { content?: string; text?: string; chunk?: string }): string {
  return payload.content || payload.text || payload.chunk || '';
}

function toolCallEntity(id: string, props: Record<string, unknown>): TimelineEntity {
  return {
    id,
    kind: 'tool_call',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    props,
  };
}

function toolResultEntity(id: string, props: Record<string, unknown>): TimelineEntity {
  return {
    id,
    kind: 'tool_result',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    props,
  };
}

function parseToolInput(input: string): unknown {
  const trimmed = input.trim();
  if (!trimmed) return {};
  try {
    return JSON.parse(trimmed) as unknown;
  } catch {
    return input;
  }
}

export function timelineMutationFromUIEvent(frame: CanonicalFrame): TimelineMutation | null {
  const event = decodeKnownUIEvent(frame);
  if (!event) return null;

  switch (event.name) {
    case 'ChatMessageAccepted': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        upsert: messageEntity(messageId, {
          role: payload.role || 'user',
          content: payload.content || payload.text,
          status: payload.status || 'submitted',
          streaming: payload.streaming === true,
          parentMessageId: payload.parentMessageId,
          segment: payload.segment,
          segmentType: payload.segmentType,
          final: payload.final,
        }),
      };
    }
    case 'ChatMessageStarted': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      const content = payload.content || payload.text;
      return {
        upsert: content
          ? messageEntity(messageId, {
              role: payload.role || 'assistant',
              prompt: payload.prompt,
              content,
              status: payload.status || 'streaming',
              streaming: true,
              parentMessageId: payload.parentMessageId,
              segment: payload.segment,
              segmentType: payload.segmentType,
              final: payload.final,
            })
          : undefined,
        status: 'streaming',
      };
    }
    case 'ChatMessageAppended': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        upsert: messageEntity(messageId, {
          role: payload.role || 'assistant',
          content: visibleText(payload),
          status: payload.status || 'streaming',
          streaming: true,
          parentMessageId: payload.parentMessageId,
          segment: payload.segment,
          segmentType: payload.segmentType,
          final: payload.final,
        }),
        status: 'streaming',
      };
    }
    case 'ChatMessageFinished': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      const content = payload.content || payload.text;
      return {
        upsert: content
          ? messageEntity(messageId, {
              role: payload.role || 'assistant',
              prompt: payload.prompt,
              content,
              status: payload.status || 'finished',
              streaming: false,
              parentMessageId: payload.parentMessageId,
              segment: payload.segment,
              segmentType: payload.segmentType,
              final: payload.final,
            })
          : undefined,
        status: 'finished',
      };
    }
    case 'ChatMessageStopped': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      const content = payload.content || payload.text;
      return {
        upsert: content || payload.error
          ? messageEntity(messageId, {
              role: payload.role || 'assistant',
              prompt: payload.prompt,
              content,
              status: payload.status || 'stopped',
              streaming: false,
              error: payload.error,
              parentMessageId: payload.parentMessageId,
              segment: payload.segment,
              segmentType: payload.segmentType,
              final: payload.final,
            })
          : undefined,
        status: 'stopped',
      };
    }
    case 'ChatReasoningStarted': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      const content = payload.content || payload.text;
      if (!content) return null;
      return {
        upsert: messageEntity(messageId, {
          role: 'thinking',
          content,
          status: payload.status || 'streaming',
          streaming: payload.streaming !== false,
          parentMessageId: payload.parentMessageId,
          segment: payload.segment,
          segmentType: payload.segmentType,
          source: payload.source,
          provider: payload.provider,
          responseId: payload.responseId,
          itemId: payload.itemId,
          outputIndex: payload.outputIndex,
          summaryIndex: payload.summaryIndex,
        }),
        status: 'streaming',
      };
    }
    case 'ChatReasoningAppended': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        upsert: messageEntity(messageId, {
          role: 'thinking',
          content: visibleText(payload),
          status: payload.status || 'streaming',
          streaming: payload.streaming !== false,
          parentMessageId: payload.parentMessageId,
          segment: payload.segment,
          segmentType: payload.segmentType,
          source: payload.source,
          provider: payload.provider,
          responseId: payload.responseId,
          itemId: payload.itemId,
          outputIndex: payload.outputIndex,
          summaryIndex: payload.summaryIndex,
        }),
        status: 'streaming',
      };
    }
    case 'ChatReasoningFinished': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      const content = payload.content || payload.text;
      if (!content) return null;
      return {
        upsert: messageEntity(messageId, {
          role: 'thinking',
          content,
          status: payload.status || 'finished',
          streaming: false,
          parentMessageId: payload.parentMessageId,
          segment: payload.segment,
          segmentType: payload.segmentType,
          source: payload.source,
          provider: payload.provider,
          responseId: payload.responseId,
          itemId: payload.itemId,
          outputIndex: payload.outputIndex,
          summaryIndex: payload.summaryIndex,
        }),
      };
    }
    case 'ChatAgentModePreviewUpdated': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        upsert: agentModeEntity(agentModePreviewEntityId(messageId), 'agent_mode_preview', {
          title: 'Agent mode preview',
          data: {
            from: '',
            to: payload.candidateMode,
            analysis: payload.analysis,
            parseState: payload.parseState,
          },
          preview: true,
          messageId,
        }),
      };
    }
    case 'ChatAgentModeCommitted': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        upsert: agentModeEntity('agent-mode', 'agent_mode', {
          title: payload.title || 'Agent mode switch',
          data: {
            from: payload.from,
            to: payload.to,
            analysis: payload.analysis,
          },
          preview: false,
          messageId,
        }),
      };
    }
    case 'ChatAgentModePreviewCleared': {
      const messageId = event.payload.messageId;
      if (!messageId) return null;
      return { deleteId: agentModePreviewEntityId(messageId) };
    }
    case 'ChatToolCallStarted':
    case 'ChatToolCallUpdated':
    case 'ChatToolCallFinished': {
      const payload = event.payload;
      if (!payload.toolCallId) return null;
      return {
        upsert: toolCallEntity(payload.toolCallId, {
          messageId: payload.messageId,
          toolCallId: payload.toolCallId,
          name: payload.toolName || 'tool',
          toolName: payload.toolName,
          input: parseToolInput(payload.input),
          inputRaw: payload.input,
          executing: payload.executing,
          status: payload.status,
          done: event.name === 'ChatToolCallFinished' || payload.status === 'completed',
        }),
      };
    }
    case 'ChatToolResultReady': {
      const payload = event.payload;
      if (!payload.toolCallId) return null;
      return {
        upsert: toolResultEntity(`${payload.toolCallId}:result`, {
          messageId: payload.messageId,
          toolCallId: payload.toolCallId,
          name: payload.toolName || 'tool',
          toolName: payload.toolName,
          customKind: payload.toolName,
          result: payload.result,
          resultRaw: payload.result,
          status: payload.status,
        }),
      };
    }
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
