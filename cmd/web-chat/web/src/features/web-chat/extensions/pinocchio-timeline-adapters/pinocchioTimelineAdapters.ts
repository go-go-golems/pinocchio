import type { TimelineMutation } from '@go-go-golems/chat-provider';
import { defineChatExtensions, defineLiveAndHydrateAdapter, defineLiveOnlyAdapter } from '@go-go-golems/chat-provider';
import { asRecord, asString } from '@go-go-golems/chat-provider/ws';

function now() {
  return Date.now();
}

function definedProps(props: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(props).filter(([, value]) => value !== undefined && !(typeof value === 'string' && value === '')),
  );
}

function messageEntity(id: string, props: Record<string, unknown>) {
  return { id, kind: 'message', createdAt: now(), updatedAt: now(), props };
}

function agentModeEntity(id: string, kind: 'agent_mode' | 'agent_mode_preview', props: Record<string, unknown>) {
  return { id, kind, createdAt: now(), updatedAt: now(), props };
}

function toolCallEntity(id: string, props: Record<string, unknown>) {
  return { id, kind: 'tool_call', createdAt: now(), updatedAt: now(), props };
}

function toolResultEntity(id: string, props: Record<string, unknown>) {
  return { id, kind: 'tool_result', createdAt: now(), updatedAt: now(), props };
}

function agentModePreviewEntityId(messageId: string): string {
  return `agent-mode-preview:${messageId}`;
}

function patchModeName(mode: unknown): string {
  if (typeof mode === 'string' && mode.trim()) return mode;
  if (mode === 2) return 'CHAT_STREAM_PATCH_MODE_SNAPSHOT';
  if (mode === 3) return 'CHAT_STREAM_PATCH_MODE_REPLACE';
  return 'CHAT_STREAM_PATCH_MODE_APPEND';
}

function correlationProps(correlation: unknown): Record<string, unknown> {
  const c = asRecord(correlation);
  if (Object.keys(c).length === 0) return {};
  return definedProps({
    correlation,
    sessionId: c.sessionId,
    runId: c.runId,
    turnId: c.turnId,
    providerCallId: c.providerCallId,
    segmentId: c.segmentId,
    toolCallId: c.toolCallId,
  });
}

function parseToolInput(input: string): unknown {
  const trimmed = input.trim();
  if (!trimmed) return {};
  try {
    return JSON.parse(trimmed);
  } catch {
    return input;
  }
}

function payloadRecord(value: unknown): Record<string, unknown> {
  return asRecord(value);
}

export const pinocchioReasoningAdapter = defineLiveOnlyAdapter({
  name: 'pinocchio.reasoning',
  priority: -10,
  hydrationUnsupportedReason: 'Reasoning snapshots are durable ChatMessage entities with role=thinking and hydrate through chat-provider.message.',
  live: {
    accepts: (frame) => ['ChatReasoningSegmentStarted', 'ChatReasoningPatch', 'ChatReasoningSegmentFinished'].includes(asString(frame.name)),
    project(frame): TimelineMutation | null {
      const name = asString(frame.name);
      const payload = payloadRecord(frame.payload);
      const messageId = asString(payload.messageId);
      if (!messageId) return null;

      switch (name) {
        case 'ChatReasoningSegmentStarted':
          return { status: 'streaming' };
        case 'ChatReasoningPatch':
          return {
            upsert: messageEntity(messageId, {
              role: 'thinking',
              contentPatch: payload.text || payload.content || '',
              patchMode: patchModeName(payload.mode),
              status: payload.status || 'streaming',
              streaming: payload.final !== true,
              parentMessageId: payload.parentMessageId,
              source: payload.source,
              ...correlationProps(payload.correlation),
            }),
            status: 'streaming',
          };
        case 'ChatReasoningSegmentFinished': {
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
        default:
          return null;
      }
    },
  },
});

export const pinocchioAgentModeAdapter = defineLiveAndHydrateAdapter({
  name: 'pinocchio.agent-mode',
  priority: -10,
  live: {
    accepts: (frame) => ['ChatAgentModePreviewUpdated', 'ChatAgentModeCommitted', 'ChatAgentModePreviewCleared'].includes(asString(frame.name)),
    project(frame): TimelineMutation | null {
      const name = asString(frame.name);
      const payload = payloadRecord(frame.payload);
      const messageId = asString(payload.messageId);

      switch (name) {
        case 'ChatAgentModePreviewUpdated':
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
        case 'ChatAgentModeCommitted':
          if (!messageId) return null;
          return {
            upsert: agentModeEntity('agent-mode', 'agent_mode', {
              title: payload.title || 'Agent mode switch',
              data: { from: payload.from, to: payload.to, analysis: payload.analysis },
              preview: false,
              messageId,
            }),
          };
        case 'ChatAgentModePreviewCleared':
          return messageId ? { deleteId: agentModePreviewEntityId(messageId) } : null;
        default:
          return null;
      }
    },
  },
  hydrate: {
    kind: 'supported',
    project(entity) {
      if (asString(entity.kind) !== 'AgentMode') return null;
      const payload = payloadRecord(entity.payload);
      return agentModeEntity(asString(entity.id) || 'agent-mode', 'agent_mode', {
        title: asString(payload.title) || 'Agent mode switch',
        data: {
          from: asString(payload.from),
          to: asString(payload.to),
          analysis: asString(payload.analysis),
        },
        preview: payload.preview === true,
        messageId: asString(payload.messageId),
      });
    },
  },
});

export const pinocchioBackendToolAdapter = defineLiveAndHydrateAdapter({
  name: 'pinocchio.backend-tools',
  priority: -10,
  live: {
    accepts: (frame) => ['ChatToolCallStarted', 'ChatToolArgumentsPatch', 'ChatToolExecutionStarted', 'ChatToolCallFinished', 'ChatToolResultReady'].includes(asString(frame.name)),
    project(frame): TimelineMutation | null {
      const name = asString(frame.name);
      const payload = payloadRecord(frame.payload);
      const toolCallId = asString(payload.toolCallId);
      if (!toolCallId) return null;

      switch (name) {
        case 'ChatToolCallStarted':
        case 'ChatToolArgumentsPatch':
        case 'ChatToolExecutionStarted':
        case 'ChatToolCallFinished': {
          const isPatch = name === 'ChatToolArgumentsPatch';
          const hasInput = isPatch ? 'arguments' in payload && payload.arguments !== '' : 'input' in payload && payload.input !== '';
          const input = hasInput ? (isPatch && 'arguments' in payload ? payload.arguments : payload.input) : '';
          const patchMode = isPatch && 'mode' in payload ? payload.mode : undefined;
          return {
            upsert: toolCallEntity(
              toolCallId,
              definedProps({
                messageId: payload.messageId,
                toolCallId,
                name: payload.toolName,
                toolName: payload.toolName,
                ...(hasInput ? (isPatch ? { inputRawPatch: input, patchMode: patchModeName(patchMode) } : { input: parseToolInput(String(input)), inputRaw: input }) : {}),
                executing: 'executing' in payload ? payload.executing : false,
                status: payload.status,
                arguments: 'arguments' in payload ? payload.arguments : undefined,
                ...correlationProps(payload.correlation),
                done: name === 'ChatToolCallFinished' || payload.status === 'completed',
              }),
            ),
          };
        }
        case 'ChatToolResultReady':
          return {
            upsert: toolResultEntity(
              `${toolCallId}:result`,
              definedProps({
                messageId: payload.messageId,
                toolCallId,
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
        default:
          return null;
      }
    },
  },
  hydrate: {
    kind: 'supported',
    project(entity) {
      const kind = asString(entity.kind);
      const id = asString(entity.id);
      const payload = payloadRecord(entity.payload);
      if (kind === 'ChatToolCall') {
        const inputRaw = asString(payload.input);
        const toolCallId = asString(payload.toolCallId) || id;
        if (!toolCallId) return null;
        return toolCallEntity(toolCallId, {
          messageId: payload.messageId,
          toolCallId,
          name: payload.toolName,
          toolName: payload.toolName,
          input: parseToolInput(inputRaw),
          inputRaw,
          executing: payload.executing === true,
          status: payload.status,
          ...correlationProps(payload.correlation),
          done: payload.status === 'completed',
        });
      }
      if (kind === 'ChatToolResult') {
        const toolCallId = asString(payload.toolCallId);
        if (!id) return null;
        return toolResultEntity(id, {
          messageId: payload.messageId,
          toolCallId,
          name: payload.toolName,
          toolName: payload.toolName,
          customKind: payload.toolName,
          result: asString(payload.result),
          resultRaw: asString(payload.result),
          status: payload.status,
          ...correlationProps(payload.correlation),
        });
      }
      return null;
    },
  },
});

export const pinocchioWebChatTimelineAdapters = defineChatExtensions({
  name: 'pinocchio.web-chat.timeline-adapters',
  timelineAdapters: [pinocchioReasoningAdapter, pinocchioAgentModeAdapter, pinocchioBackendToolAdapter],
});
