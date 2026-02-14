import type { Meta, StoryObj } from '@storybook/react';
import { TurnsEnvelopeCard } from './TurnsEnvelopeCard';

const meta: Meta<typeof TurnsEnvelopeCard> = {
  title: 'debug-components/TurnsEnvelopeCard',
  component: TurnsEnvelopeCard,
};

export default meta;
type Story = StoryObj<typeof TurnsEnvelopeCard>;

export const Default: Story = {
  args: {
    envelope: {
      conv_id: 'conv-123',
      session_id: 'session-abc',
      phase: 'final',
      since_ms: 1700000000000,
      items: [
        {
          conv_id: 'conv-123',
          session_id: 'session-abc',
          turn_id: 'turn-1',
          phase: 'final',
          created_at_ms: 1700000001234,
          payload: 'id: turn-1\\nblocks: []\\n',
        },
      ],
    },
  },
};
