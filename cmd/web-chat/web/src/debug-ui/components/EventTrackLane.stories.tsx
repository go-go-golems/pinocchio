import type { Meta, StoryObj } from '@storybook/react';
import '../index.css';
import { debugEvents } from './debugFixtures';
import { EventTrackLane } from './EventTrackLane';

const meta: Meta<typeof EventTrackLane> = {
  title: 'Debug UI/Components/EventTrackLane',
  component: EventTrackLane,
  args: { events: debugEvents },
};

export default meta;
type Story = StoryObj<typeof EventTrackLane>;

export const Default: Story = {};

export const Empty: Story = {
  args: { events: [] },
};
