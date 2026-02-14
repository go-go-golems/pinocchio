import type { ConversationDetail, ConversationSummary, SessionSummary } from '../../types';

// Mock conversations
export const mockConversations: ConversationSummary[] = [
  {
    id: 'conv_8a3f',
    profile_slug: 'general',
    session_id: 'sess_01',
    engine_config_sig: 'abc123',
    is_running: false,
    ws_connections: 2,
    last_activity: '2026-02-06T14:32:18Z',
    turn_count: 3,
    has_timeline: true,
  },
  {
    id: 'conv_9b4g',
    profile_slug: 'agent',
    session_id: 'sess_02',
    engine_config_sig: 'def456',
    is_running: true,
    ws_connections: 1,
    last_activity: '2026-02-06T14:35:22Z',
    turn_count: 5,
    has_timeline: true,
  },
  {
    id: 'conv_0c5h',
    profile_slug: 'default',
    session_id: 'sess_03',
    engine_config_sig: 'ghi789',
    is_running: false,
    ws_connections: 0,
    last_activity: '2026-02-06T12:10:05Z',
    turn_count: 1,
    has_timeline: false,
  },
];

export const mockConversationDetail: ConversationDetail = {
  ...mockConversations[0],
  engine_config: {
    profile_slug: 'general',
    system_prompt: 'You are a helpful assistant. Be concise and accurate.',
    middlewares: [
      { name: 'logging-mw' },
      { name: 'system-prompt-mw' },
      { name: 'tool-reorder-mw' },
    ],
    tools: ['get_weather', 'calculator', 'search'],
  },
};

// Mock sessions
export const mockSessions: SessionSummary[] = [
  {
    session_id: 'sess_01',
    turn_count: 2,
    first_turn_at: '2026-02-06T14:30:01Z',
    last_turn_at: '2026-02-06T14:32:18Z',
  },
  {
    session_id: 'sess_02',
    turn_count: 1,
    first_turn_at: '2026-02-06T14:28:00Z',
    last_turn_at: '2026-02-06T14:28:45Z',
  },
];
