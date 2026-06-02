import type { Meta, StoryObj } from '@storybook/react';
import type { StatusbarSlotProps } from '../types';
import { DefaultHeader } from './ChatHeader';

const profiles = [
  { slug: 'default', display_name: 'Default' },
  { slug: 'researcher', display_name: 'Researcher' },
  { slug: 'toolsmith', display_name: 'Toolsmith' },
];

function StoryStatusbar({ wsStatus, status, errorCount }: StatusbarSlotProps) {
  return (
    <div data-part="statusbar" data-state={wsStatus}>
      <span data-part="pill">ws: {wsStatus}</span>
      <span data-part="pill">{status}</span>
      {errorCount > 0 ? <span data-part="pill" data-variant="danger">errors: {errorCount}</span> : null}
    </div>
  );
}

const meta: Meta<typeof DefaultHeader> = {
  title: 'WebChat/Components/ChatHeader',
  component: DefaultHeader,
  args: {
    Statusbar: StoryStatusbar,
    title: 'Pinocchio Web Chat',
    profile: 'default',
    profiles,
    wsStatus: 'connected',
    status: 'idle',
    queueDepth: 0,
    lastSeq: 42,
    errorCount: 0,
    showErrors: false,
    onProfileChange: () => undefined,
    onToggleErrors: () => undefined,
  },
  decorators: [
    (Story) => (
      <div data-pwchat="" data-part="root" data-theme="default" style={{ minHeight: 180 }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof DefaultHeader>;

export const Default: Story = {};

export const ManyProfiles: Story = {
  args: {
    profiles: [
      ...profiles,
      { slug: 'planner' },
      { slug: 'reviewer' },
      { slug: 'debugger' },
      { slug: 'release-manager' },
    ],
  },
};

export const ErrorCount: Story = {
  args: {
    wsStatus: 'disconnected',
    status: 'error',
    errorCount: 3,
    showErrors: true,
  },
};

export const NarrowWidth: Story = {
  decorators: [
    (Story) => (
      <div data-pwchat="" data-part="root" data-theme="default" style={{ width: 360, minHeight: 180 }}>
        <Story />
      </div>
    ),
  ],
};
