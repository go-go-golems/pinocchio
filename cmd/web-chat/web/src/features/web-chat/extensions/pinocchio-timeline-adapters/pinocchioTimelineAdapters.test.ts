import { createTimelineAdapterRegistry } from '@go-go-golems/chat-provider';
import { describe, expect, it } from 'vitest';
import { pinocchioAgentModeAdapter, pinocchioBackendToolAdapter } from './pinocchioTimelineAdapters';

describe('pinocchio timeline adapters baseline parity', () => {
  it('projects live and hydrated AgentMode entities to agent_mode cards', () => {
    const registry = createTimelineAdapterRegistry();
    registry.register(pinocchioAgentModeAdapter);

    const live = registry.projectLive(
      {
        name: 'ChatAgentModeCommitted',
        payload: {
          messageId: 'm-agent',
          from: 'chat',
          to: 'research',
          analysis: 'needs tools',
        },
      },
      { sessionId: 's1' },
    );
    const snapshot = registry.projectSnapshot(
      {
        id: 'agent-mode',
        kind: 'AgentMode',
        payload: {
          messageId: 'm-agent',
          from: 'chat',
          to: 'research',
          analysis: 'needs tools',
        },
      },
      { sessionId: 's1' },
    );

    expect(live?.adapterName).toBe('pinocchio.agent-mode');
    expect(snapshot?.adapterName).toBe('pinocchio.agent-mode');
    expect(live?.mutation.upsert?.kind).toBe('agent_mode');
    expect(snapshot?.mutation.upsert?.kind).toBe('agent_mode');
    expect(live?.mutation.upsert?.props.data).toMatchObject({ to: 'research' });
    expect(snapshot?.mutation.upsert?.props.data).toMatchObject({ to: 'research' });
  });

  it('hydrates backend tool call/result snapshots to card-compatible kinds', () => {
    const registry = createTimelineAdapterRegistry();
    registry.register(pinocchioBackendToolAdapter);

    const call = registry.projectSnapshot(
      {
        id: 'tool-1',
        kind: 'ChatToolCall',
        payload: {
          messageId: 'm-tool',
          toolCallId: 'tool-1',
          toolName: 'mock.search',
          input: '{"query":"timeline adapter"}',
          status: 'completed',
        },
      },
      { sessionId: 's1' },
    );
    const result = registry.projectSnapshot(
      {
        id: 'tool-1:result',
        kind: 'ChatToolResult',
        payload: {
          messageId: 'm-tool',
          toolCallId: 'tool-1',
          toolName: 'mock.search',
          result: '{"items":["ok"]}',
          status: 'completed',
        },
      },
      { sessionId: 's1' },
    );

    expect(call?.adapterName).toBe('pinocchio.backend-tools');
    expect(result?.adapterName).toBe('pinocchio.backend-tools');
    expect(call?.mutation.upsert?.kind).toBe('tool_call');
    expect(result?.mutation.upsert?.kind).toBe('tool_result');
    expect(call?.mutation.upsert?.props.toolName).toBe('mock.search');
    expect(result?.mutation.upsert?.props.toolName).toBe('mock.search');
  });
});
