import type { Meta, StoryObj } from '@storybook/react';
import '../index.css';
import { debugEntities, debugEvents } from './debugFixtures';
import { TimelineLanes } from './TimelineLanes';

const meta: Meta<typeof TimelineLanes> = {
  title: 'Debug UI/Components/TimelineLanes',
  component: TimelineLanes,
  args: {
    events: debugEvents,
    entities: debugEntities,
    isLive: true,
  },
};

export default meta;
type Story = StoryObj<typeof TimelineLanes>;

export const Live: Story = {};

export const StaticSnapshot: Story = {
  args: { isLive: false },
};

export const Empty: Story = {
  args: { events: [], entities: [], isLive: false },
};
