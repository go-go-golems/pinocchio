import type { Meta, StoryObj } from '@storybook/react';
import { mockTurns } from '../mocks/fixtures/turns';
import { StateTrackLane } from './StateTrackLane';

const meta: Meta<typeof StateTrackLane> = {
  title: 'Debug UI/StateTrackLane',
  component: StateTrackLane,
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
type Story = StoryObj<typeof StateTrackLane>;

export const Default: Story = {
  args: {
    turns: mockTurns,
  },
};

export const WithSelection: Story = {
  args: {
    turns: mockTurns,
    selectedTurnId: 'turn_01',
  },
};

export const Empty: Story = {
  args: {
    turns: [],
  },
};

export const SingleTurn: Story = {
  args: {
    turns: [mockTurns[0]],
  },
};

export const AllPhases: Story = {
  args: {
    turns: [
      { ...mockTurns[0], phase: 'pre_inference' as const },
      { ...mockTurns[0], phase: 'post_inference' as const },
      { ...mockTurns[0], phase: 'post_tools' as const },
      { ...mockTurns[0], phase: 'final' as const },
    ],
  },
};
