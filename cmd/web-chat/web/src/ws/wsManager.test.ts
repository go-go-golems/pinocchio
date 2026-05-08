import { describe, expect, it } from 'vitest';
import { decodeKnownUIEvent } from './chatappPayloads';
import { timelineEntityFromSnapshotEntity, timelineMutationFromUIEvent } from './wsManager';

describe('timelineEntityFromSnapshotEntity', () => {
  it('maps thinking ChatMessage snapshot entities to message render entities', () => {
    const entity = timelineEntityFromSnapshotEntity({
      kind: 'ChatMessage',
      id: 'chat-msg-1:thinking',
      payload: {
        messageId: 'chat-msg-1:thinking',
        role: 'thinking',
        content: 'high level plan',
        status: 'finished',
        streaming: false,
      },
    });

    expect(entity).not.toBeNull();
    expect(entity?.kind).toBe('message');
    expect(entity?.id).toBe('chat-msg-1:thinking');
    expect(entity?.props.role).toBe('thinking');
    expect(entity?.props.content).toBe('high level plan');
  });

  it('maps committed AgentMode snapshot entities to agent_mode render entities', () => {
    const entity = timelineEntityFromSnapshotEntity({
      kind: 'AgentMode',
      id: 'session',
      payload: {
        title: 'agentmode: mode switched',
        data: { from: 'analyst', to: 'reviewer', analysis: 'hello' },
        preview: false,
        messageId: 'chat-msg-1',
      },
    });

    expect(entity).not.toBeNull();
    expect(entity?.kind).toBe('agent_mode');
    expect(entity?.id).toBe('session');
    expect(entity?.props.preview).toBe(false);
    expect(entity?.props.data).toEqual({ from: 'analyst', to: 'reviewer', analysis: 'hello' });
  });

  it('unwraps Struct payloads for hydrated AgentMode snapshot entities', () => {
    const entity = timelineEntityFromSnapshotEntity({
      kind: 'AgentMode',
      id: 'session',
      payload: {
        '@type': 'type.googleapis.com/google.protobuf.Struct',
        value: {
          title: 'agentmode: mode switched',
          data: { from: 'financial_analyst', to: 'financial_analyst', analysis: 'No switch needed.' },
          preview: false,
          messageId: 'chat-msg-5',
        },
      },
    });

    expect(entity).not.toBeNull();
    expect(entity?.kind).toBe('agent_mode');
    expect(entity?.props.title).toBe('agentmode: mode switched');
    expect(entity?.props.messageId).toBe('chat-msg-5');
    expect(entity?.props.data).toEqual({
      from: 'financial_analyst',
      to: 'financial_analyst',
      analysis: 'No switch needed.',
    });
  });
});

describe('timelineMutationFromUIEvent', () => {
  it('updates status without creating an empty assistant placeholder for ChatMessageStarted', () => {
    const mutation = timelineMutationFromUIEvent({
      name: 'ChatMessageStarted',
      payload: {
        messageId: 'chat-msg-2',
        prompt: 'Explain ordinals',
        status: 'streaming',
        streaming: true,
      },
    });

    expect(mutation).toEqual({ status: 'streaming', upsert: undefined });
  });

  it('creates a stopped message mutation when ChatMessageStopped carries only an error', () => {
    const mutation = timelineMutationFromUIEvent({
      name: 'ChatMessageStopped',
      payload: {
        messageId: 'chat-msg-5',
        role: 'assistant',
        prompt: 'ok what is next',
        status: 'stopped',
        error: "responses api error: invalid input id",
      },
    });

    expect(mutation).not.toBeNull();
    expect(mutation?.status).toBe('stopped');
    expect(mutation?.upsert?.id).toBe('chat-msg-5');
    expect(mutation?.upsert?.kind).toBe('message');
    expect(mutation?.upsert?.props.role).toBe('assistant');
    expect(mutation?.upsert?.props.content).toBe('');
    expect(mutation?.upsert?.props.error).toBe('responses api error: invalid input id');
    expect(mutation?.upsert?.props.streaming).toBe(false);
  });

  it('does not create an empty placeholder mutation for ChatReasoningStarted without visible content', () => {
    const mutation = timelineMutationFromUIEvent({
      name: 'ChatReasoningStarted',
      payload: {
        messageId: 'chat-msg-2:thinking',
        status: 'streaming',
        streaming: true,
      },
    });

    expect(mutation).toBeNull();
  });

  it('creates a thinking message mutation for ChatReasoningAppended', () => {
    const mutation = timelineMutationFromUIEvent({
      name: 'ChatReasoningAppended',
      payload: {
        messageId: 'chat-msg-2:thinking',
        content: 'draft plan',
        status: 'streaming',
        streaming: true,
      },
    });

    expect(mutation).not.toBeNull();
    expect(mutation?.upsert?.id).toBe('chat-msg-2:thinking');
    expect(mutation?.upsert?.kind).toBe('message');
    expect(mutation?.upsert?.props.role).toBe('thinking');
    expect(mutation?.upsert?.props.content).toBe('draft plan');
    expect(mutation?.status).toBe('streaming');
  });

  it('does not create an empty placeholder mutation for ChatReasoningFinished without visible content', () => {
    const mutation = timelineMutationFromUIEvent({
      name: 'ChatReasoningFinished',
      payload: {
        messageId: 'chat-msg-2:thinking',
        status: 'finished',
        streaming: false,
      },
    });

    expect(mutation).toBeNull();
  });

  it('creates a finished thinking message mutation for ChatReasoningFinished', () => {
    const mutation = timelineMutationFromUIEvent({
      name: 'ChatReasoningFinished',
      payload: {
        messageId: 'chat-msg-2:thinking',
        content: 'high level plan',
        status: 'finished',
        streaming: false,
      },
    });

    expect(mutation).not.toBeNull();
    expect(mutation?.upsert?.id).toBe('chat-msg-2:thinking');
    expect(mutation?.upsert?.props.role).toBe('thinking');
    expect(mutation?.upsert?.props.content).toBe('high level plan');
    expect(mutation?.upsert?.props.streaming).toBe(false);
  });

  it('preserves typed reasoning provider IDs and optional zero indexes', () => {
    const event = decodeKnownUIEvent({
      name: 'ChatReasoningAppended',
      payload: {
        '@type': 'type.googleapis.com/pinocchio.chatapp.v1.ReasoningUpdate',
        messageId: 'chat-msg-2:thinking:1',
        parentMessageId: 'chat-msg-2',
        segment: 1,
        segmentType: 'thinking',
        chunk: 'plan',
        content: 'plan',
        status: 'streaming',
        streaming: true,
        provider: 'openai-responses',
        responseId: 'resp_123',
        itemId: 'rs_123',
        outputIndex: 0,
        summaryIndex: 0,
        choiceIndex: 0,
        streamKind: 'reasoning',
        correlationKey: 'openai-chat:resp_123:choice:0:reasoning',
      },
    });

    expect(event?.name).toBe('ChatReasoningAppended');
    if (event?.name !== 'ChatReasoningAppended') throw new Error('expected typed reasoning event');
    expect(event.payload.outputIndex).toBe(0);
    expect(event.payload.summaryIndex).toBe(0);
    expect(event.payload.choiceIndex).toBe(0);
    expect(event.payload.correlationKey).toBe('openai-chat:resp_123:choice:0:reasoning');

    const mutation = timelineMutationFromUIEvent({ name: event.name, payload: event.payload });
    expect(mutation?.upsert?.props.provider).toBe('openai-responses');
    expect(mutation?.upsert?.props.responseId).toBe('resp_123');
    expect(mutation?.upsert?.props.itemId).toBe('rs_123');
    expect(mutation?.upsert?.props.outputIndex).toBe(0);
    expect(mutation?.upsert?.props.summaryIndex).toBe(0);
    expect(mutation?.upsert?.props.choiceIndex).toBe(0);
    expect(mutation?.upsert?.props.streamKind).toBe('reasoning');
    expect(mutation?.upsert?.props.correlationKey).toBe('openai-chat:resp_123:choice:0:reasoning');
  });

  it('creates a preview entity mutation for ChatAgentModePreviewUpdated', () => {
    const mutation = timelineMutationFromUIEvent({
      name: 'ChatAgentModePreviewUpdated',
      payload: {
        messageId: 'chat-msg-2',
        candidateMode: 'reviewer',
        analysis: 'hello',
        parseState: 'candidate',
      },
    });

    expect(mutation).not.toBeNull();
    expect(mutation?.deleteId).toBeUndefined();
    expect(mutation?.upsert?.id).toBe('agent-mode-preview:chat-msg-2');
    expect(mutation?.upsert?.kind).toBe('agent_mode_preview');
    expect(mutation?.upsert?.props.preview).toBe(true);
    expect(mutation?.upsert?.props.data).toEqual({
      from: '',
      to: 'reviewer',
      analysis: 'hello',
      parseState: 'candidate',
    });
  });

  it('creates a delete mutation for ChatAgentModePreviewCleared', () => {
    const mutation = timelineMutationFromUIEvent({
      name: 'ChatAgentModePreviewCleared',
      payload: { messageId: 'chat-msg-2' },
    });

    expect(mutation).toEqual({ deleteId: 'agent-mode-preview:chat-msg-2' });
  });

  it('creates typed tool call and tool result mutations', () => {
    const toolCall = timelineMutationFromUIEvent({
      name: 'ChatToolCallUpdated',
      payload: {
        messageId: 'chat-msg-7',
        toolCallId: 'call_1',
        toolName: 'search',
        input: '{"query":"gold"}',
        executing: true,
        status: 'executing',
        provider: 'openai',
        responseId: 'resp_tool',
        choiceIndex: 0,
        streamKind: 'tool_call',
        correlationKey: 'openai-chat:resp_tool:choice:0:tool:call_1',
        toolCallIndex: 0,
      },
    });

    expect(toolCall?.upsert?.id).toBe('call_1');
    expect(toolCall?.upsert?.kind).toBe('tool_call');
    expect(toolCall?.upsert?.props.name).toBe('search');
    expect(toolCall?.upsert?.props.input).toEqual({ query: 'gold' });
    expect(toolCall?.upsert?.props.executing).toBe(true);
    expect(toolCall?.upsert?.props.correlationKey).toBe('openai-chat:resp_tool:choice:0:tool:call_1');
    expect(toolCall?.upsert?.props.toolCallIndex).toBe(0);

    const result = timelineMutationFromUIEvent({
      name: 'ChatToolResultReady',
      payload: {
        messageId: 'chat-msg-7',
        toolCallId: 'call_1',
        toolName: 'search',
        result: 'found it',
        status: 'success',
        provider: 'openai',
        responseId: 'resp_tool',
        choiceIndex: 0,
        streamKind: 'tool_call',
        correlationKey: 'openai-chat:resp_tool:choice:0:tool:call_1',
        toolCallIndex: 0,
      },
    });

    expect(result?.upsert?.id).toBe('call_1:result');
    expect(result?.upsert?.kind).toBe('tool_result');
    expect(result?.upsert?.props.customKind).toBe('search');
    expect(result?.upsert?.props.result).toBe('found it');
    expect(result?.upsert?.props.correlationKey).toBe('openai-chat:resp_tool:choice:0:tool:call_1');
    expect(result?.upsert?.props.toolCallIndex).toBe(0);
  });
});
