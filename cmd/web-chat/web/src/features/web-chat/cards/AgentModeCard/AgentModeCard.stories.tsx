import type { Meta, StoryObj } from '@storybook/react';
import { agentModeEntity } from '../fixtures';
import { CardStoryFrame } from '../storyDecorators';
import { AgentModeCard } from './AgentModeCard';

const meta: Meta<typeof AgentModeCard> = {
  title: 'WebChat/Cards/AgentModeCard',
  component: AgentModeCard,
  decorators: [(Story) => <CardStoryFrame><Story /></CardStoryFrame>],
};

export default meta;
type Story = StoryObj<typeof AgentModeCard>;

export const Preview: Story = {
  args: {
    e: agentModeEntity('a1', {
      title: 'agentmode: preview switch',
      preview: true,
      data: { from: 'assistant', to: 'category_reviewer', analysis: 'A review pass would help validate the generated patterns.' },
    }),
  },
};

export const Committed: Story = {
  args: {
    e: agentModeEntity('a2', {
      title: 'agentmode: mode switched',
      data: { from: 'assistant', to: 'toolsmith', analysis: '- Tools are now available\n- Next step: run validation', reason: 'user requested implementation' },
    }),
  },
};
