import type { Meta, StoryObj } from '@storybook/react';
import { fn } from '@storybook/test';
import { DefaultComposer } from './ChatComposer';

const meta: Meta<typeof DefaultComposer> = {
  title: 'WebChat/Components/ChatComposer',
  component: DefaultComposer,
  args: {
    text: '',
    disabled: true,
    onChangeText: fn(),
    onSubmit: fn(),
    onNewConversation: fn(),
    onKeyDown: fn(),
  },
  decorators: [
    (Story) => (
      <div data-pwchat="" data-part="root" data-theme="default" style={{ padding: 16 }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof DefaultComposer>;

export const Empty: Story = {};

export const Typed: Story = {
  args: {
    text: 'Show me the latest tool calls.',
    disabled: false,
  },
};

export const DisabledStreaming: Story = {
  args: {
    text: 'Waiting for the assistant…',
    disabled: true,
  },
};

export const LongText: Story = {
  args: {
    text: 'Please summarize the previous run, identify any backend tool failures, explain what happened, and suggest the next validation command before I continue with the migration.',
    disabled: false,
  },
};
