import { ChatStreamPatchMode, type CorrelationInfo } from '../chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb';
import { appSlice } from '../store/appSlice';
import type { AppDispatch } from '../store/store';
import { type TimelineEntity, timelineSlice } from '../store/timelineSlice';
import { decodeKnownUIEvent } from './chatappPayloads';
import { asRecord, asString, type CanonicalFrame } from './protocol';
import { recordUIEventDebug } from './streamDebug';
import { agentModeEntity, agentModePreviewEntityId, messageEntity } from './timelineSnapshot';

type TimelineMutation = {
  upsert?: TimelineEntity;
  upsertIfExists?: TimelineEntity;
  deleteId?: string;
  status?: string;
};

function visibleText(payload: { content?: string; text?: string }): string {
  return payload.content || payload.text || '';
}

function patchModeName(mode: unknown): string {
  if (typeof mode === 'string' && mode.trim()) {
    return mode;
  }
  switch (mode) {
    case ChatStreamPatchMode.SNAPSHOT:
      return 'CHAT_STREAM_PATCH_MODE_SNAPSHOT';
    case ChatStreamPatchMode.REPLACE:
      return 'CHAT_STREAM_PATCH_MODE_REPLACE';
    case ChatStreamPatchMode.UNSPECIFIED:
      return 'CHAT_STREAM_PATCH_MODE_UNSPECIFIED';
    default:
      return 'CHAT_STREAM_PATCH_MODE_APPEND';
  }
}

function definedProps(props: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(props).filter(([, value]) => value !== undefined && !(typeof value === 'string' && value === '')),
  );
}

function correlationProps(correlation?: CorrelationInfo): Record<string, unknown> {
  if (!correlation) return {};
  return definedProps({
    correlation,
    sessionId: correlation.sessionId,
    runId: correlation.runId,
    turnId: correlation.turnId,
    providerCallId: correlation.providerCallId,
    segmentId: correlation.segmentId,
    toolCallId: correlation.toolCallId,
  });
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

function widgetEntity(id: string, props: Record<string, unknown>): TimelineEntity {
  return {
    id,
    kind: 'ChatWidgetInstance',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    props,
  };
}

function frontendToolEntity(id: string, props: Record<string, unknown>): TimelineEntity {
  return {
    id,
    kind: 'ChatFrontendToolCall',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    props,
  };
}

function rawTimelineMutationFromUIEvent(frame: CanonicalFrame, sessionId = ''): TimelineMutation | null {
  const name = asString(frame.name);
  const payload = asRecord(frame.payload);
  switch (name) {
    case 'ChatWidgetInstanceStarted': {
      const instanceId = asString(payload.instanceId);
      if (!instanceId) return null;
      return {
        upsert: widgetEntity(instanceId, {
          instanceId,
          widgetName: asString(payload.widgetName),
          parentMessageId: asString(payload.parentMessageId),
          status: payload.status || 'WIDGET_STATUS_STREAMING',
          props: asRecord(payload.props),
        }),
      };
    }
    case 'ChatWidgetInstancePatched': {
      const instanceId = asString(payload.instanceId);
      if (!instanceId) return null;
      return {
        upsert: widgetEntity(instanceId, definedProps({
          instanceId,
          widgetName: asString(payload.widgetName),
          status: payload.status || 'WIDGET_STATUS_STREAMING',
          props: asRecord(payload.patch),
        })),
      };
    }
    case 'ChatWidgetInstanceCompleted': {
      const instanceId = asString(payload.instanceId);
      if (!instanceId) return null;
      return { upsert: widgetEntity(instanceId, { instanceId, status: payload.status || 'WIDGET_STATUS_READY' }) };
    }
    case 'ChatWidgetInstanceRemoved': {
      const instanceId = asString(payload.instanceId);
      return instanceId ? { deleteId: instanceId } : null;
    }
    case 'ChatFrontendToolCallRequested': {
      const toolCallId = asString(payload.toolCallId);
      if (!toolCallId) return null;
      return {
        upsert: frontendToolEntity(toolCallId, {
          sessionId,
          toolCallId,
          toolName: asString(payload.toolName),
          name: asString(payload.toolName),
          parentMessageId: asString(payload.messageId),
          mode: payload.mode,
          status: asString(payload.status) || 'requested',
          input: asRecord(payload.input),
        }),
      };
    }
    case 'ChatFrontendToolResultReceived': {
      const toolCallId = asString(payload.toolCallId);
      if (!toolCallId) return null;
      return {
        upsert: frontendToolEntity(toolCallId, {
          sessionId,
          toolCallId,
          toolName: asString(payload.toolName),
          name: asString(payload.toolName),
          parentMessageId: asString(payload.messageId),
          status: asString(payload.status) || 'success',
          result: asRecord(payload.result),
          error: asString(payload.error),
          done: true,
        }),
      };
    }
    default:
      return null;
  }
}

export function timelineMutationFromUIEvent(frame: CanonicalFrame, sessionId = ''): TimelineMutation | null {
  const rawMutation = rawTimelineMutationFromUIEvent(frame, sessionId);
  if (rawMutation) return rawMutation;

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
    case 'ChatTextPatch': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        upsert: messageEntity(messageId, {
          role: payload.role || 'assistant',
          prompt: payload.prompt,
          contentPatch: visibleText(payload),
          patchMode: patchModeName(payload.mode),
          status: payload.status || 'streaming',
          streaming: !payload.final,
          parentMessageId: parentMessageId(messageId, ':text:'),
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
      const upsert = messageEntity(
        messageId,
        definedProps({
          role: payload.role || 'assistant',
          prompt: payload.prompt,
          ...(content ? { content } : {}),
          status: payload.status || 'finished',
          streaming: false,
          parentMessageId: parentMessageId(messageId, ':text:'),
          ...(payload.final ? { final: payload.final } : {}),
          finishReason: payload.finishReason,
          ...correlationProps(payload.correlation),
        }),
      );
      return {
        ...(content ? { upsert } : { upsertIfExists: upsert }),
        status: payload.status || 'finished',
      };
    }
    case 'ChatReasoningSegmentStarted': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return { status: 'streaming' };
    }
    case 'ChatReasoningPatch': {
      const payload = event.payload;
      const messageId = payload.messageId;
      if (!messageId) return null;
      return {
        upsert: messageEntity(messageId, {
          role: 'thinking',
          contentPatch: visibleText(payload),
          patchMode: patchModeName(payload.mode),
          status: payload.status || 'streaming',
          streaming: !payload.final,
          parentMessageId: payload.parentMessageId,
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
      const upsert = messageEntity(
        messageId,
        definedProps({
          role: 'thinking',
          ...(content ? { content } : {}),
          status: payload.status || 'finished',
          streaming: false,
          parentMessageId: payload.parentMessageId,
          source: payload.source,
          finishReason: payload.finishReason,
          ...correlationProps(payload.correlation),
        }),
      );
      return content ? { upsert } : { upsertIfExists: upsert };
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
    case 'ChatToolArgumentsPatch':
    case 'ChatToolCallRequested':
    case 'ChatToolExecutionStarted':
    case 'ChatToolCallFinished': {
      const payload = event.payload;
      if (!payload.toolCallId) return null;
      const isPatch = event.name === 'ChatToolArgumentsPatch';
      const hasInput = isPatch ? 'arguments' in payload && payload.arguments !== '' : 'input' in payload && payload.input !== '';
      const input = hasInput ? (isPatch && 'arguments' in payload ? payload.arguments : 'input' in payload ? payload.input : '') : '';
      const patchMode = isPatch && 'mode' in payload ? payload.mode : undefined;
      const executing = 'executing' in payload ? payload.executing : false;
      return {
        upsert: toolCallEntity(
          payload.toolCallId,
          definedProps({
            messageId: payload.messageId,
            toolCallId: payload.toolCallId,
            name: payload.toolName,
            toolName: payload.toolName,
            ...(hasInput ? (isPatch ? { inputRawPatch: input, patchMode: patchModeName(patchMode) } : { input: parseToolInput(input), inputRaw: input }) : {}),
            executing,
            status: payload.status,
            arguments: 'arguments' in payload ? payload.arguments : undefined,
            ...correlationProps(payload.correlation),
            done: event.name === 'ChatToolCallFinished' || payload.status === 'completed',
          }),
        ),
      };
    }
    case 'ChatToolResultReady': {
      const payload = event.payload;
      if (!payload.toolCallId) return null;
      return {
        upsert: toolResultEntity(
          `${payload.toolCallId}:result`,
          definedProps({
            messageId: payload.messageId,
            toolCallId: payload.toolCallId,
            name: payload.toolName,
            toolName: payload.toolName,
            customKind: payload.toolName,
            result: payload.result,
            resultRaw: payload.result,
            status: payload.status,
            ...correlationProps(payload.correlation),
          }),
        ),
      };
    }
    default:
      return null;
  }
}

export function applyUIEvent(frame: CanonicalFrame, dispatch: AppDispatch, sessionId = '') {
  const mutation = timelineMutationFromUIEvent(frame, sessionId);
  recordUIEventDebug(sessionId, frame, mutation);
  if (!mutation) return;
  if (mutation.deleteId) {
    dispatch(timelineSlice.actions.deleteEntity(mutation.deleteId));
  }
  if (mutation.upsert) {
    dispatch(timelineSlice.actions.upsertEntity(mutation.upsert));
  }
  if (mutation.upsertIfExists) {
    dispatch(timelineSlice.actions.upsertEntityIfExists(mutation.upsertIfExists));
  }
  if (mutation.status) {
    dispatch(appSlice.actions.setStatus(mutation.status));
  }
}
