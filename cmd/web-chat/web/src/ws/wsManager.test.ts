import { describe, expect, it } from 'vitest';
import { timelineEntityFromSnapshotEntity, timelineMutationFromUIEvent } from './wsManager';

describe('timelineEntityFromSnapshotEntity', () => {
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
