import type { Meta, StoryObj } from '@storybook/react';
import { mockEvents } from '../mocks/fixtures/events';
import { EventCard } from './EventCard';

const meta: Meta<typeof EventCard> = {
  title: 'Debug UI/EventCard',
  component: EventCard,
  parameters: {
    layout: 'padded',
  },
  argTypes: {
    onClick: { action: 'clicked' },
  },
};

export default meta;
type Story = StoryObj<typeof EventCard>;

export const LLMStart: Story = {
  args: {
    event: mockEvents[0],
  },
};

export const ToolStart: Story = {
  args: {
    event: mockEvents[1],
  },
};

export const ToolResult: Story = {
  args: {
    event: mockEvents[2],
  },
};

export const LLMDelta: Story = {
  args: {
    event: mockEvents[3],
  },
};

export const LLMFinal: Story = {
  args: {
    event: mockEvents[5],
  },
};

export const Selected: Story = {
  args: {
    event: mockEvents[0],
    selected: true,
  },
};

export const Compact: Story = {
  args: {
    event: mockEvents[3],
    compact: true,
  },
};

export const AllEventTypes: Story = {
  render: () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxWidth: '600px' }}>
      {mockEvents.map((event, idx) => (
        <EventCard key={idx} event={event} />
      ))}
    </div>
  ),
};

export const EventTimeline: Story = {
  render: () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', maxWidth: '400px' }}>
      {mockEvents.map((event, idx) => (
        <EventCard key={idx} event={event} compact />
      ))}
    </div>
  ),
};
