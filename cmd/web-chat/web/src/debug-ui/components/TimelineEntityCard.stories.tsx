import type { Meta, StoryObj } from '@storybook/react';
import { mockTimelineEntities } from '../mocks/fixtures/timeline';
import { TimelineEntityCard } from './TimelineEntityCard';

const meta: Meta<typeof TimelineEntityCard> = {
  title: 'Debug UI/TimelineEntityCard',
  component: TimelineEntityCard,
  parameters: {
    layout: 'padded',
  },
  argTypes: {
    onClick: { action: 'clicked' },
  },
};

export default meta;
type Story = StoryObj<typeof TimelineEntityCard>;

export const UserMessage: Story = {
  args: {
    entity: mockTimelineEntities[0],
  },
};

export const ToolCall: Story = {
  args: {
    entity: mockTimelineEntities[1],
  },
};

export const ToolResult: Story = {
  args: {
    entity: mockTimelineEntities[2],
  },
};

export const AssistantMessage: Story = {
  args: {
    entity: mockTimelineEntities[3],
  },
};

export const Selected: Story = {
  args: {
    entity: mockTimelineEntities[0],
    selected: true,
  },
};

export const Compact: Story = {
  args: {
    entity: mockTimelineEntities[3],
    compact: true,
  },
};

export const StreamingMessage: Story = {
  args: {
    entity: {
      id: 'msg-streaming',
      kind: 'message',
      created_at: Date.now(),
      version: 1,
      props: {
        role: 'assistant',
        content: 'I am currently generating this response...',
        streaming: true,
      },
    },
  },
};

export const AllEntities: Story = {
  render: () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxWidth: '400px' }}>
      {mockTimelineEntities.map((entity) => (
        <TimelineEntityCard key={entity.id} entity={entity} />
      ))}
    </div>
  ),
};

export const TimelineCompact: Story = {
  render: () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', maxWidth: '300px' }}>
      {mockTimelineEntities.map((entity) => (
        <TimelineEntityCard key={entity.id} entity={entity} compact />
      ))}
    </div>
  ),
};
