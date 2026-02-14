import type { Meta, StoryObj } from '@storybook/react';
import { DebugAppProvider } from './DebugAppProvider';

const meta: Meta<typeof DebugAppProvider> = {
  title: 'debug-app/DebugAppProvider',
  component: DebugAppProvider,
};

export default meta;
type Story = StoryObj<typeof DebugAppProvider>;

export const Default: Story = {};
