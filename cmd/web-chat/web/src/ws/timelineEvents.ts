import type { CorrelationInfo } from '../chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb';
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

function correlationProps(correlation?: CorrelationInfo): Record<string, unknown> {
  if (!correlation) return {};
  return {
    correlation,
    provider: correlation.provider,
    model: correlation.model,
    responseId: correlation.responseId,
    itemId: correlation.itemId,
    outputIndex: correlation.outputIndex,
    summaryIndex: correlation.summaryIndex,
    choiceIndex: correlation.choiceIndex,
    contentBlockIndex: correlation.contentBlockIndex,
    streamKind: correlation.streamKind,
    correlationKey: correlation.correlationKey,
    parentCorrelationKey: correlation.parentCorrelationKey,
    providerCallId: correlation.providerCallId,
    providerCallIndex: correlation.providerCallIndex,
    segmentId: correlation.segmentId,
    segmentIndex: correlation.segmentIndex,
    segmentType: correlation.segmentType,
    toolCallId: correlation.toolCallId,
    toolCallIndex: correlation.toolCallIndex,
  };
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

function parentMessageId(messageId: string, marker: string): string | undefined {
  const idx = messageId.lastIndexOf(marker);
  return idx > 0 ? messageId.slice(0, idx) : undefined;
}

export function timelineMutationFromUIEvent(frame: CanonicalFrame): TimelineMutation | null {
  const event = decodeKnownUIEvent(frame);
  if (!event) return null;

  switch (event.name) {
    case 'ChatUserMessageAccepted': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        upsert: messageEntity(messageId, {
          role: payload.role || 'user',
          prompt: payload.prompt,
          content: payload.content || payload.text,
          status: payload.status || 'accepted',
          streaming: false,
        }),
      };
    }
    case 'ChatRunStarted':
      return { status: 'streaming' };
    case 'ChatRunFinished':
      return { status: event.payload.status || 'finished' };
    case 'ChatRunStopped':
      return { status: event.payload.status || 'stopped' };
    case 'ChatRunFailed':
      return { status: event.payload.status || 'failed' };
    case 'ChatProviderCallStarted':
    case 'ChatProviderCallMetadataUpdated':
    case 'ChatProviderCallFinished':
      return null;
    case 'ChatTextSegmentStarted': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        status: 'streaming',
        upsert: undefined,
      };
    }
    case 'ChatTextDelta': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        upsert: messageEntity(messageId, {
          role: payload.role || 'assistant',
          prompt: payload.prompt,
          content: visibleText(payload),
          status: payload.status || 'streaming',
          streaming: true,
          parentMessageId: parentMessageId(messageId, ':text:'),
          segment: payload.correlation?.segmentIndex,
          segmentType: payload.correlation?.segmentType,
          final: false,
          ...correlationProps(payload.correlation),
        }),
        status: 'streaming',
      };
    }
    case 'ChatTextSegmentFinished': {
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
              parentMessageId: parentMessageId(messageId, ':text:'),
              segment: payload.correlation?.segmentIndex,
              segmentType: payload.correlation?.segmentType,
              final: payload.final,
              finishReason: payload.finishReason,
              ...correlationProps(payload.correlation),
            })
          : undefined,
        status: payload.status || 'finished',
      };
    }
    case 'ChatReasoningSegmentStarted': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return { status: 'streaming' };
    }
    case 'ChatReasoningDelta': {
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
          segment: payload.correlation?.segmentIndex,
          segmentType: payload.correlation?.segmentType,
          source: payload.source,
          ...correlationProps(payload.correlation),
        }),
        status: 'streaming',
      };
    }
    case 'ChatReasoningSegmentFinished': {
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
          segment: payload.correlation?.segmentIndex,
          segmentType: payload.correlation?.segmentType,
          source: payload.source,
          finishReason: payload.finishReason,
          ...correlationProps(payload.correlation),
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
    case 'ChatToolCallArgumentsDelta':
    case 'ChatToolCallRequested':
    case 'ChatToolExecutionStarted':
    case 'ChatToolCallFinished': {
      const payload = event.payload;
      if (!payload.toolCallId) return null;
      const input = 'input' in payload ? payload.input : '';
      const executing = 'executing' in payload ? payload.executing : false;
      return {
        upsert: toolCallEntity(payload.toolCallId, {
          messageId: payload.messageId,
          toolCallId: payload.toolCallId,
          name: payload.toolName || 'tool',
          toolName: payload.toolName,
          input: parseToolInput(input),
          inputRaw: input,
          executing,
          status: payload.status,
          argumentsDelta: 'argumentsDelta' in payload ? payload.argumentsDelta : undefined,
          ...correlationProps(payload.correlation),
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
          ...correlationProps(payload.correlation),
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
