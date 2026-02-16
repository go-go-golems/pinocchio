import type { Meta, StoryObj } from '@storybook/react';
import { CorrelationIdBar } from './CorrelationIdBar';

const meta: Meta<typeof CorrelationIdBar> = {
  title: 'Debug UI/CorrelationIdBar',
  component: CorrelationIdBar,
  parameters: {
    layout: 'padded',
  },
};

export default meta;
type Story = StoryObj<typeof CorrelationIdBar>;

export const ConversationLevel: Story = {
  args: {
    convId: 'conv_8a3f4b2c',
    sessionId: 'sess_01234567',
  },
};

export const TurnLevel: Story = {
  args: {
    convId: 'conv_8a3f4b2c',
    sessionId: 'sess_01234567',
    inferenceId: 'inf_abcdef12',
    turnId: 'turn_98765432',
  },
};

export const EventLevel: Story = {
  args: {
    convId: 'conv_8a3f4b2c',
    sessionId: 'sess_01234567',
    inferenceId: 'inf_abcdef12',
    turnId: 'turn_98765432',
    seq: 1707053365100000000,
    streamId: 'main',
  },
};

export const MinimalIds: Story = {
  args: {
    convId: 'conv_8a3f',
  },
};

export const LongIds: Story = {
  args: {
    convId: 'conv_8a3f4b2c9d0e1f2a3b4c5d6e7f8a9b0c',
    sessionId: 'sess_0123456789abcdef0123456789abcdef',
    inferenceId: 'inf_fedcba9876543210fedcba9876543210',
  },
};
