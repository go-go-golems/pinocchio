import type { Meta, StoryObj } from '@storybook/react';
import { makeAnomalyScenario } from '../mocks/scenarios';
import { AnomalyPanel } from './AnomalyPanel';

const meta: Meta<typeof AnomalyPanel> = {
  title: 'Debug UI/AnomalyPanel',
  component: AnomalyPanel,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof AnomalyPanel>;
const onClose = () => {};

export const Default: Story = {
  args: {
    ...makeAnomalyScenario('default').args,
    onClose,
  },
};

export const Closed: Story = {
  args: {
    ...makeAnomalyScenario('closed').args,
    onClose,
  },
};

export const Empty: Story = {
  args: {
    ...makeAnomalyScenario('empty').args,
    onClose,
  },
};

export const ErrorsOnly: Story = {
  args: {
    ...makeAnomalyScenario('errorsOnly').args,
    onClose,
  },
};

export const ManyAnomalies: Story = {
  args: {
    ...makeAnomalyScenario('manyAnomalies').args,
    onClose,
  },
};
