import type { Meta, StoryObj } from '@storybook/react';
import { makeConversations } from '../mocks/factories';
import { mockConversations } from '../mocks/fixtures/conversations';
import { createDefaultDebugHandlers } from '../mocks/msw/defaultHandlers';
import { SessionList } from './SessionList';

const meta: Meta<typeof SessionList> = {
  title: 'Debug UI/SessionList',
  component: SessionList,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof SessionList>;

export const WithData: Story = {
  args: {
    conversations: mockConversations,
    isLoading: false,
  },
};

export const Loading: Story = {
  args: {
    isLoading: true,
  },
};

export const Error: Story = {
  args: {
    error: 'Failed to connect to server',
    isLoading: false,
  },
};

export const Empty: Story = {
  args: {
    conversations: [],
    isLoading: false,
  },
};

export const SingleConversation: Story = {
  args: {
    conversations: [mockConversations[0]],
    isLoading: false,
  },
};

export const ManyConversations: Story = {
  args: {
    conversations: makeConversations(8),
    isLoading: false,
  },
};

// Story with MSW mocking
export const WithMSW: Story = {
  parameters: {
    msw: {
      handlers: createDefaultDebugHandlers(),
    },
  },
};
