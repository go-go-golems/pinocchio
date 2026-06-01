import type { DebugEntity, DebugEvent } from '../store/debugSlice';

export const debugEvents: DebugEvent[] = [
  {
    name: 'ChatRunStarted',
    ordinal: 1,
    sessionId: 'story-session',
    payload: { runId: 'run-1' },
    receivedAt: '2026-05-31T12:00:00Z',
  },
  {
    name: 'ChatTextPatch',
    ordinal: 2,
    sessionId: 'story-session',
    payload: { messageId: 'm1', text: 'hello' },
    receivedAt: '2026-05-31T12:00:01Z',
  },
  {
    name: 'ChatToolCallStarted',
    ordinal: 3,
    sessionId: 'story-session',
    payload: { toolCallId: 'tool-1', toolName: 'mock.search' },
    receivedAt: '2026-05-31T12:00:02Z',
  },
];

export const debugEntities: DebugEntity[] = [
  {
    id: 'm1',
    kind: 'message',
    props: { role: 'assistant', content: 'hello' },
  },
  {
    id: 'tool-1',
    kind: 'tool_call',
    props: { toolName: 'mock.search', status: 'completed' },
  },
  {
    id: 'agent-mode',
    kind: 'agent_mode',
    props: { title: 'Agent mode switch', to: 'research' },
  },
];
