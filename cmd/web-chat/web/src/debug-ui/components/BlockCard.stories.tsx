import type { Meta, StoryObj } from '@storybook/react';
import type { ParsedBlock } from '../types';
import { BlockCard } from './BlockCard';

const meta: Meta<typeof BlockCard> = {
  title: 'Debug UI/BlockCard',
  component: BlockCard,
  parameters: {
    layout: 'padded',
  },
};

export default meta;
type Story = StoryObj<typeof BlockCard>;

// Standard metadata helper
const stdMeta = (blockId: string) => ({
  'geppetto.block_id@v1': blockId,
  'geppetto.turn_id@v1': 'turn_01',
  'geppetto.session_id@v1': 'sess_01234567',
  'geppetto.created_at@v1': '2026-02-06T14:32:08.000Z',
});

const systemBlock: ParsedBlock = {
  index: 0,
  kind: 'system',
  role: 'system',
  payload: { text: 'You are a helpful assistant. Be concise and accurate.' },
  metadata: {
    ...stdMeta('blk_sys_001'),
    'geppetto.middleware@v1': 'system-prompt-mw',
    'geppetto.source@v1': 'profile.yaml',
    'geppetto.profile@v1': 'general',
  },
};

const userBlock: ParsedBlock = {
  index: 1,
  kind: 'user',
  role: 'user',
  payload: { text: 'What is the weather in Paris?' },
  metadata: {
    ...stdMeta('blk_usr_001'),
    'webchat.client_id@v1': 'client_abc123',
    'webchat.input_method@v1': 'keyboard',
    'webchat.timestamp@v1': 1707229920000,
  },
};

const toolCallBlock: ParsedBlock = {
  index: 2,
  id: 'tc_001',
  kind: 'tool_call',
  payload: {
    id: 'tc_001',
    name: 'get_weather',
    args: { location: 'Paris', units: 'celsius' },
  },
  metadata: {
    ...stdMeta('blk_tc_001'),
    'geppetto.inference_id@v1': 'inf_abc123',
    'geppetto.model@v1': 'claude-3.5-sonnet',
    'geppetto.tool_config@v1': { timeout_ms: 5000, retries: 2 },
  },
};

const toolUseBlock: ParsedBlock = {
  index: 3,
  kind: 'tool_use',
  payload: {
    id: 'tc_001',
    result: { temperature: 18, condition: 'cloudy', humidity: 65 },
  },
  metadata: {
    ...stdMeta('blk_tu_001'),
    'geppetto.tool_call_id@v1': 'tc_001',
    'geppetto.tool_name@v1': 'get_weather',
    'geppetto.execution@v1': { duration_ms: 234, status: 'success', retries: 0 },
    'geppetto.cache@v1': { hit: false, ttl_s: 300 },
  },
};

const llmTextBlock: ParsedBlock = {
  index: 4,
  kind: 'llm_text',
  role: 'assistant',
  payload: { text: 'The weather in Paris is currently 18°C and cloudy with 65% humidity.' },
  metadata: {
    ...stdMeta('blk_llm_001'),
    'geppetto.inference_id@v1': 'inf_abc123',
    'geppetto.model@v1': 'claude-3.5-sonnet',
    'geppetto.usage@v1': { prompt_tokens: 847, completion_tokens: 42, total_tokens: 889 },
    'geppetto.latency_ms@v1': 1823,
    'geppetto.stop_reason@v1': 'end_turn',
  },
};

const reasoningBlock: ParsedBlock = {
  index: 5,
  kind: 'reasoning',
  payload: { encrypted_content: '<encrypted>' },
  metadata: {
    ...stdMeta('blk_reason_001'),
    'geppetto.inference_id@v1': 'inf_abc123',
    'geppetto.model@v1': 'claude-3.5-sonnet',
    'geppetto.thinking_budget@v1': { max_tokens: 10000, used_tokens: 4521 },
  },
};

export const System: Story = {
  args: { block: systemBlock },
};

export const User: Story = {
  args: { block: userBlock },
};

export const ToolCall: Story = {
  args: { block: toolCallBlock },
};

export const ToolUse: Story = {
  args: { block: toolUseBlock },
};

export const LLMText: Story = {
  args: { block: llmTextBlock },
};

export const Reasoning: Story = {
  args: { block: reasoningBlock },
};

export const NewBlock: Story = {
  args: { block: llmTextBlock, isNew: true },
};

export const Compact: Story = {
  args: { block: llmTextBlock, compact: true },
};

export const LongText: Story = {
  args: {
    block: {
      ...llmTextBlock,
      payload: {
        text: `The weather in Paris is currently 18°C and cloudy with 65% humidity. 
        
This is a longer response that demonstrates how the component handles multi-line text content. The weather pattern is expected to continue for the next few days, with temperatures ranging from 15°C to 20°C.

Key points:
1. Current temperature: 18°C
2. Condition: Cloudy
3. Humidity: 65%
4. Wind: Light breeze from the west

Would you like more detailed information about the forecast for the coming week?`,
      },
    },
  },
};

export const Expanded: Story = {
  args: {
    block: {
      ...systemBlock,
      metadata: {
        'geppetto.middleware@v1': 'system-prompt-mw',
        'geppetto.session_id@v1': 'sess_01234567890abcdef',
        'geppetto.inference_id@v1': 'inf_abcdef1234567890',
        'custom.config@v1': { mode: 'agent', tools_enabled: true, max_tokens: 4096 },
      },
    },
    expanded: true,
  },
};

export const WithRichMetadata: Story = {
  args: {
    block: {
      ...llmTextBlock,
      metadata: {
        'geppetto.middleware@v1': 'agent-mode-mw',
        'geppetto.usage@v1': { prompt_tokens: 1234, completion_tokens: 567, total_tokens: 1801 },
        'planning.context@v1': { run_id: 'plan_xyz', step: 3, status: 'executing' },
        'tool.calls@v1': ['get_weather', 'search_web'],
      },
    },
  },
};

export const AllBlockTypes: Story = {
  render: () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxWidth: '600px' }}>
      <BlockCard block={systemBlock} />
      <BlockCard block={userBlock} />
      <BlockCard block={toolCallBlock} />
      <BlockCard block={toolUseBlock} />
      <BlockCard block={llmTextBlock} />
      <BlockCard block={reasoningBlock} />
    </div>
  ),
};
