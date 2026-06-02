import type { Meta, StoryObj } from '@storybook/react';
import { toolResultEntity } from '../fixtures';
import { CardStoryFrame } from '../storyDecorators';
import { ToolResultCard } from './ToolResultCard';

const meta: Meta<typeof ToolResultCard> = {
  title: 'WebChat/Cards/ToolResultCard',
  component: ToolResultCard,
  decorators: [(Story) => <CardStoryFrame><Story /></CardStoryFrame>],
};

export default meta;
type Story = StoryObj<typeof ToolResultCard>;

export const Json: Story = {
  args: { e: toolResultEntity('r1', { customKind: 'inventory.search', result: { items: [{ sku: 'boot-1', price: 129 }] } }) },
};

export const Text: Story = {
  args: { e: toolResultEntity('r2', { customKind: 'browser.page_text', result: 'The cart contains one item.' }) },
};

export const Empty: Story = {
  args: { e: toolResultEntity('r3', { customKind: 'noop' }) },
};

export const Error: Story = {
  args: { e: toolResultEntity('r4', { customKind: 'inventory.search', error: 'timeout waiting for inventory service' }) },
};
