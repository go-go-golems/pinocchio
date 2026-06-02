import type { Meta, StoryObj } from '@storybook/react';
import { renderEntity } from '../fixtures';
import { CardStoryFrame } from '../storyDecorators';
import { GenericCard } from './GenericCard';

const meta: Meta<typeof GenericCard> = {
  title: 'WebChat/Cards/GenericCard',
  component: GenericCard,
  decorators: [(Story) => <CardStoryFrame><Story /></CardStoryFrame>],
};

export default meta;
type Story = StoryObj<typeof GenericCard>;

export const UnknownEntity: Story = {
  args: { e: renderEntity('unknown.custom_event', 'g1', { title: 'Unknown event', payload: { nested: true } }) },
};

export const EmptyProps: Story = {
  args: { e: renderEntity('empty.entity', 'g2') },
};
