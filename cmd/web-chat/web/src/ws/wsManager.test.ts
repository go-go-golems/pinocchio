import { describe, expect, it } from 'vitest';
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
});
