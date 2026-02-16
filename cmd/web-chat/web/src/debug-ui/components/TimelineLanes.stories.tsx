import type { Meta, StoryObj } from '@storybook/react';
import { makeTimelineScenario } from '../mocks/scenarios';
import { TimelineLanes } from './TimelineLanes';

const meta: Meta<typeof TimelineLanes> = {
  title: 'Debug UI/TimelineLanes',
  component: TimelineLanes,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      <div style={{ height: '600px', background: 'var(--bg-primary)' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TimelineLanes>;

export const Default: Story = {
  args: makeTimelineScenario('default').args,
};

export const WithSelection: Story = {
  args: makeTimelineScenario('withSelection').args,
};

export const Live: Story = {
  args: makeTimelineScenario('live').args,
};

export const Empty: Story = {
  args: makeTimelineScenario('empty').args,
};

export const TurnsOnly: Story = {
  args: makeTimelineScenario('turnsOnly').args,
};

export const EventsOnly: Story = {
  args: makeTimelineScenario('eventsOnly').args,
};

export const ManyItems: Story = {
  args: makeTimelineScenario('manyItems').args,
};
