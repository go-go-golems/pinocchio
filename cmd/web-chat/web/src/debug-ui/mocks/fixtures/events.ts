import type { MwTrace, SemEvent } from '../../types';

// Mock events
export const mockEvents: SemEvent[] = [
  {
    type: 'llm.start',
    id: 'msg-a1b2c3d4',
    seq: 1707053365100000000,
    stream_id: 'main',
    data: { role: 'assistant' },
    received_at: '2026-02-06T14:32:08.100Z',
  },
  {
    type: 'tool.start',
    id: 'tc_001',
    seq: 1707053365200000000,
    stream_id: 'main',
    data: { id: 'tc_001', name: 'get_weather', input: '{"location":"Paris"}' },
    received_at: '2026-02-06T14:32:10.500Z',
  },
  {
    type: 'tool.result',
    id: 'tc_001',
    seq: 1707053365300000000,
    stream_id: 'main',
    data: { result: '{"temperature":18,"condition":"cloudy"}' },
    received_at: '2026-02-06T14:32:11.200Z',
  },
  {
    type: 'llm.delta',
    id: 'msg-a1b2c3d4',
    seq: 1707053365400000000,
    stream_id: 'main',
    data: { cumulative: 'The weather in Paris' },
    received_at: '2026-02-06T14:32:15.000Z',
  },
  {
    type: 'llm.delta',
    id: 'msg-a1b2c3d4',
    seq: 1707053365500000000,
    stream_id: 'main',
    data: { cumulative: 'The weather in Paris is currently 18°C and cloudy.' },
    received_at: '2026-02-06T14:32:17.500Z',
  },
  {
    type: 'llm.final',
    id: 'msg-a1b2c3d4',
    seq: 1707053365600000000,
    stream_id: 'main',
    data: { text: 'The weather in Paris is currently 18°C and cloudy.' },
    received_at: '2026-02-06T14:32:18.000Z',
  },
];

// Mock middleware trace
export const mockMwTrace: MwTrace = {
  conv_id: 'conv_8a3f',
  inference_id: 'inf_abc',
  chain: [
    {
      layer: 0,
      name: 'logging-mw',
      pre_blocks: 2,
      post_blocks: 2,
      blocks_added: 0,
      blocks_removed: 0,
      blocks_changed: 0,
      metadata_changes: [],
      duration_ms: 1,
    },
    {
      layer: 1,
      name: 'system-prompt-mw',
      pre_blocks: 2,
      post_blocks: 2,
      blocks_added: 0,
      blocks_removed: 0,
      blocks_changed: 1,
      changed_blocks: [{ index: 0, kind: 'system', change: 'content_modified' }],
      metadata_changes: [],
      duration_ms: 2,
    },
    {
      layer: 2,
      name: 'tool-reorder-mw',
      pre_blocks: 2,
      post_blocks: 2,
      blocks_added: 0,
      blocks_removed: 0,
      blocks_changed: 0,
      metadata_changes: [],
      duration_ms: 1,
    },
  ],
  engine: {
    model: 'claude-3.5-sonnet',
    input_blocks: 2,
    output_blocks: 3,
    latency_ms: 2412,
    tokens_in: 847,
    tokens_out: 312,
    stop_reason: 'tool_use',
  },
};
