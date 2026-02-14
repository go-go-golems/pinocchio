import type { Meta, StoryObj } from '@storybook/react';
import { mockMwTrace } from '../mocks/fixtures/events';
import { MiddlewareChainView } from './MiddlewareChainView';

const meta: Meta<typeof MiddlewareChainView> = {
  title: 'Debug UI/MiddlewareChainView',
  component: MiddlewareChainView,
  parameters: {
    layout: 'padded',
  },
  argTypes: {
    onLayerClick: { action: 'layer clicked' },
  },
};

export default meta;
type Story = StoryObj<typeof MiddlewareChainView>;

export const Default: Story = {
  args: {
    trace: mockMwTrace,
  },
};

export const WithChanges: Story = {
  args: {
    trace: {
      ...mockMwTrace,
      chain: [
        {
          layer: 0,
          name: 'logging-mw',
          pre_blocks: 2,
          post_blocks: 2,
          blocks_added: 0,
          blocks_removed: 0,
          blocks_changed: 0,
          metadata_changes: ['geppetto.usage@v1'],
          duration_ms: 1,
        },
        {
          layer: 1,
          name: 'system-prompt-mw',
          pre_blocks: 2,
          post_blocks: 3,
          blocks_added: 1,
          blocks_removed: 0,
          blocks_changed: 0,
          metadata_changes: [],
          duration_ms: 3,
        },
        {
          layer: 2,
          name: 'agent-mode-mw',
          pre_blocks: 3,
          post_blocks: 4,
          blocks_added: 1,
          blocks_removed: 0,
          blocks_changed: 1,
          changed_blocks: [{ index: 0, kind: 'system', change: 'content_modified' }],
          metadata_changes: ['agent.mode@v1'],
          duration_ms: 5,
        },
        {
          layer: 3,
          name: 'tool-reorder-mw',
          pre_blocks: 4,
          post_blocks: 4,
          blocks_added: 0,
          blocks_removed: 0,
          blocks_changed: 0,
          metadata_changes: [],
          duration_ms: 1,
        },
      ],
    },
  },
};

export const LongChain: Story = {
  args: {
    trace: {
      ...mockMwTrace,
      chain: [
        { layer: 0, name: 'auth-mw', pre_blocks: 1, post_blocks: 1, blocks_added: 0, blocks_removed: 0, blocks_changed: 0, metadata_changes: [], duration_ms: 2 },
        { layer: 1, name: 'rate-limit-mw', pre_blocks: 1, post_blocks: 1, blocks_added: 0, blocks_removed: 0, blocks_changed: 0, metadata_changes: [], duration_ms: 1 },
        { layer: 2, name: 'logging-mw', pre_blocks: 1, post_blocks: 1, blocks_added: 0, blocks_removed: 0, blocks_changed: 0, metadata_changes: [], duration_ms: 1 },
        { layer: 3, name: 'system-prompt-mw', pre_blocks: 1, post_blocks: 2, blocks_added: 1, blocks_removed: 0, blocks_changed: 0, metadata_changes: [], duration_ms: 2 },
        { layer: 4, name: 'context-mw', pre_blocks: 2, post_blocks: 3, blocks_added: 1, blocks_removed: 0, blocks_changed: 0, metadata_changes: [], duration_ms: 5 },
        { layer: 5, name: 'agent-mode-mw', pre_blocks: 3, post_blocks: 4, blocks_added: 1, blocks_removed: 0, blocks_changed: 0, metadata_changes: [], duration_ms: 3 },
        { layer: 6, name: 'tool-inject-mw', pre_blocks: 4, post_blocks: 4, blocks_added: 0, blocks_removed: 0, blocks_changed: 1, metadata_changes: [], duration_ms: 2 },
        { layer: 7, name: 'tool-reorder-mw', pre_blocks: 4, post_blocks: 4, blocks_added: 0, blocks_removed: 0, blocks_changed: 0, metadata_changes: [], duration_ms: 1 },
      ],
    },
  },
};

export const FastEngine: Story = {
  args: {
    trace: {
      ...mockMwTrace,
      engine: {
        ...mockMwTrace.engine,
        latency_ms: 150,
        tokens_in: 50,
        tokens_out: 25,
        model: 'gpt-4o-mini',
      },
    },
  },
};
