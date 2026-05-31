import type { Meta, StoryObj } from '@storybook/react';
import { toolCallEntity } from '../fixtures';
import { CardStoryFrame } from '../storyDecorators';
import { ToolCallCard } from './ToolCallCard';

const meta: Meta<typeof ToolCallCard> = {
  title: 'WebChat/Cards/ToolCallCard',
  component: ToolCallCard,
  decorators: [(Story) => <CardStoryFrame><Story /></CardStoryFrame>],
};

export default meta;
type Story = StoryObj<typeof ToolCallCard>;

export const Requested: Story = {
  args: { e: toolCallEntity('t1', { name: 'inventory.search', status: 'requested', input: { query: 'boots' } }) },
};

export const Running: Story = {
  args: { e: toolCallEntity('t2', { name: 'inventory.search', status: 'running', input: { query: 'boots', limit: 5 } }) },
};

export const Completed: Story = {
  args: { e: toolCallEntity('t3', { name: 'inventory.search', status: 'success', input: { query: 'boots' }, result: { count: 3 } }) },
};

export const Failed: Story = {
  args: { e: toolCallEntity('t4', { name: 'inventory.search', status: 'failed', input: { query: 'boots' }, result: { error: 'inventory offline' } }) },
};

export const HumanTool: Story = {
  args: {
    e: toolCallEntity('t5', {
      name: 'app.confirm_action',
      status: 'requested',
      sessionId: 'story-session',
      toolCallId: 'tool-call-1',
      input: { title: 'Confirm checkout', body: 'Approve adding boots to the cart?', confirmLabel: 'Approve', cancelLabel: 'Deny' },
    }),
  },
};
