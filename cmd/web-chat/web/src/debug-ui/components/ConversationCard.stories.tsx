import type { Meta, StoryObj } from '@storybook/react';
import { mockConversations } from '../mocks/fixtures/conversations';
import { ConversationCard } from './ConversationCard';

const meta: Meta<typeof ConversationCard> = {
  title: 'Debug UI/ConversationCard',
  component: ConversationCard,
  parameters: {
    layout: 'padded',
  },
  argTypes: {
    onClick: { action: 'clicked' },
  },
};

export default meta;
type Story = StoryObj<typeof ConversationCard>;

export const Default: Story = {
  args: {
    conversation: mockConversations[0],
  },
};

export const Running: Story = {
  args: {
    conversation: mockConversations[1],
  },
};

export const Selected: Story = {
  args: {
    conversation: mockConversations[0],
    selected: true,
  },
};

export const NoConnections: Story = {
  args: {
    conversation: mockConversations[2],
  },
};

export const AllVariants: Story = {
  render: () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxWidth: '320px' }}>
      <ConversationCard conversation={mockConversations[0]} />
      <ConversationCard conversation={mockConversations[1]} />
      <ConversationCard conversation={mockConversations[2]} />
      <ConversationCard conversation={mockConversations[0]} selected />
    </div>
  ),
};
