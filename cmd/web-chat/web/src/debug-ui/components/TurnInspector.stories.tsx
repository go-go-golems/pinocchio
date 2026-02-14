import type { Meta, StoryObj } from '@storybook/react';
import { mockTurnDetail } from '../mocks/fixtures/turns';
import { TurnInspector } from './TurnInspector';

const meta: Meta<typeof TurnInspector> = {
  title: 'Debug UI/TurnInspector',
  component: TurnInspector,
  parameters: {
    layout: 'padded',
  },
};

export default meta;
type Story = StoryObj<typeof TurnInspector>;

export const Default: Story = {
  args: {
    turnDetail: mockTurnDetail,
  },
};

export const SinglePhase: Story = {
  args: {
    turnDetail: {
      ...mockTurnDetail,
      phases: {
        final: mockTurnDetail.phases.final,
      },
    },
  },
};

export const TwoPhases: Story = {
  args: {
    turnDetail: {
      ...mockTurnDetail,
      phases: {
        pre_inference: mockTurnDetail.phases.pre_inference,
        final: mockTurnDetail.phases.final,
      },
    },
  },
};

export const ManyBlocks: Story = {
  args: {
    turnDetail: {
      ...mockTurnDetail,
      phases: {
        ...mockTurnDetail.phases,
        final: {
          ...mockTurnDetail.phases.final!,
          turn: {
            ...mockTurnDetail.phases.final!.turn,
            blocks: [
              ...mockTurnDetail.phases.final!.turn.blocks,
              {
                index: 5,
                kind: 'user' as const,
                role: 'user',
                payload: { text: 'What about tomorrow?' },
                metadata: {},
              },
              {
                index: 6,
                kind: 'tool_call' as const,
                id: 'tc_002',
                payload: { id: 'tc_002', name: 'get_forecast', args: { location: 'Paris', days: 1 } },
                metadata: {},
              },
              {
                index: 7,
                kind: 'tool_use' as const,
                payload: { id: 'tc_002', result: { forecast: 'Sunny, 22°C' } },
                metadata: {},
              },
              {
                index: 8,
                kind: 'llm_text' as const,
                role: 'assistant',
                payload: { text: 'Tomorrow in Paris will be sunny with a high of 22°C.' },
                metadata: {},
              },
            ],
          },
        },
      },
    },
  },
};
