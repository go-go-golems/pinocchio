import type { Meta, StoryObj } from '@storybook/react';
import { makeEvents } from '../mocks/factories';
import { EventTrackLane } from './EventTrackLane';

const meta: Meta<typeof EventTrackLane> = {
  title: 'Debug UI/EventTrackLane',
  component: EventTrackLane,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '300px', background: 'var(--bg-primary)', padding: '8px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof EventTrackLane>;
const baseEvents = makeEvents(6);

export const Default: Story = {
  args: {
    events: baseEvents,
  },
};

export const WithSelection: Story = {
  args: {
    events: baseEvents,
    selectedSeq: baseEvents[0].seq,
  },
};

export const Empty: Story = {
  args: {
    events: [],
  },
};

export const SingleEvent: Story = {
  args: {
    events: [baseEvents[0]],
  },
};

export const ManyEvents: Story = {
  args: {
    events: makeEvents(12),
  },
};
