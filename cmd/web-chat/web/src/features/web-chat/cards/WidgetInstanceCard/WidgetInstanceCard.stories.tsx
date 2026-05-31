import type { Meta, StoryObj } from '@storybook/react';
import { widgetEntity } from '../fixtures';
import { CardStoryFrame } from '../storyDecorators';
import { WidgetInstanceCard } from './WidgetInstanceCard';

const meta: Meta<typeof WidgetInstanceCard> = {
  title: 'WebChat/Cards/WidgetInstanceCard',
  component: WidgetInstanceCard,
  decorators: [(Story) => <CardStoryFrame><Story /></CardStoryFrame>],
};

export default meta;
type Story = StoryObj<typeof WidgetInstanceCard>;

export const Streaming: Story = {
  args: { e: widgetEntity('w1', { widgetName: 'cart.review', status: 'streaming', props: { itemCount: 1 } }) },
};

export const Ready: Story = {
  args: { e: widgetEntity('w2', { widgetName: 'cart.review', status: 'ready', props: { itemCount: 2, total: 198 } }) },
};

export const Failed: Story = {
  args: { e: widgetEntity('w3', { widgetName: 'cart.review', status: 'failed', props: { error: 'cart expired' } }) },
};

export const UnknownWidget: Story = {
  args: { e: widgetEntity('w4', { widgetName: 'unknown.widget', status: 'ready', props: { raw: true } }) },
};

export const CapabilityDemo: Story = {
  args: {
    e: widgetEntity('w5', {
      widgetName: 'demo.capability_card',
      status: 'ready',
      props: { title: 'Capabilities', summary: 'Temporary showcase widget.', steps: [{ label: 'Tool call', state: 'done' }, { label: 'Widget render', state: 'running' }] },
    }),
  },
};
