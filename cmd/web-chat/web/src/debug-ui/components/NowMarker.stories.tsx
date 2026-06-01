import type { Meta, StoryObj } from '@storybook/react';
import '../index.css';
import { NowMarker } from './NowMarker';

const meta: Meta<typeof NowMarker> = {
  title: 'Debug UI/Components/NowMarker',
  component: NowMarker,
  args: { label: 'Live' },
  decorators: [
    (Story) => (
      <div className="timeline-lane" style={{ height: 180, position: 'relative' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof NowMarker>;

export const Default: Story = {};

export const CustomLabel: Story = {
  args: { label: 'Now' },
};
