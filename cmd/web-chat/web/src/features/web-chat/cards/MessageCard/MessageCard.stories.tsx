import type { Meta, StoryObj } from '@storybook/react';
import { messageEntity } from '../fixtures';
import { CardStoryFrame } from '../storyDecorators';
import { MessageCard } from './MessageCard';

const meta: Meta<typeof MessageCard> = {
  title: 'WebChat/Cards/MessageCard',
  component: MessageCard,
  decorators: [(Story) => <CardStoryFrame><Story /></CardStoryFrame>],
};

export default meta;
type Story = StoryObj<typeof MessageCard>;

export const Assistant: Story = {
  args: { e: messageEntity('m1', { role: 'assistant', content: 'Here are **three** options:\n\n- Boots\n- Sandals\n- Sneakers' }) },
};

export const User: Story = {
  args: { e: messageEntity('m2', { role: 'user', content: 'show me boots' }) },
};

export const Streaming: Story = {
  args: { e: messageEntity('m3', { role: 'assistant', content: 'Thinking through the request…', streaming: true }) },
};

export const Error: Story = {
  args: { e: messageEntity('m4', { role: 'assistant', error: 'run stopped by backend' }) },
};
