import type { Meta, StoryObj } from '@storybook/react';
import { NowMarker } from './NowMarker';

const meta: Meta<typeof NowMarker> = {
  title: 'Debug UI/NowMarker',
  component: NowMarker,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '300px', background: 'var(--bg-primary)', padding: '16px' }}>
        <div style={{ 
          background: 'var(--bg-card)', 
          padding: '12px', 
          borderRadius: '6px',
          marginBottom: '8px' 
        }}>
          Sample content above marker
        </div>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof NowMarker>;

export const Default: Story = {
  args: {},
};

export const CustomLabel: Story = {
  args: {
    label: 'Streaming',
  },
};

export const Recording: Story = {
  args: {
    label: 'Recording',
  },
};
