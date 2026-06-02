import type { Meta, StoryObj } from '@storybook/react';
import { type ComponentProps, useEffect } from 'react';
import { appSlice } from '../../../store/appSlice';
import { useAppDispatch } from '../../../store/hooks';
import { DefaultStatusbar } from './ChatStatusbar';

const profiles = [
  { slug: 'default', display_name: 'Default' },
  { slug: 'researcher', display_name: 'Researcher' },
  { slug: 'debugger', display_name: 'Debugger' },
];

const meta: Meta<typeof DefaultStatusbar> = {
  title: 'WebChat/Components/ChatStatusbar',
  component: DefaultStatusbar,
  args: {
    profile: 'default',
    profiles,
    wsStatus: 'connected',
    status: 'idle',
    queueDepth: 0,
    lastSeq: 128,
    errorCount: 0,
    showErrors: false,
    onProfileChange: () => undefined,
    onToggleErrors: () => undefined,
  },
  decorators: [
    (Story) => (
      <div data-pwchat="" data-part="root" data-theme="default" style={{ padding: 16 }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof DefaultStatusbar>;

export const Connected: Story = {};

export const Disconnected: Story = {
  args: {
    wsStatus: 'disconnected',
    status: 'idle',
    lastSeq: 0,
  },
};

export const Error: Story = {
  args: {
    wsStatus: 'error',
    status: 'failed',
    errorCount: 2,
    showErrors: true,
  },
};

function ExportVisibleStory(args: ComponentProps<typeof DefaultStatusbar>) {
  const dispatch = useAppDispatch();
  useEffect(() => {
    dispatch(appSlice.actions.setConvId('story-session-123'));
  }, [dispatch]);
  return <DefaultStatusbar {...args} />;
}

export const ExportVisible: Story = {
  args: {
    status: 'finished',
    queueDepth: 1,
    lastSeq: 2048,
  },
  render: ExportVisibleStory,
};
