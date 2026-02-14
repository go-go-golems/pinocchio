import type { Meta, StoryObj } from '@storybook/react';
import { mockTimelineEntities } from '../mocks/fixtures/timeline';
import { ProjectionLane } from './ProjectionLane';

const meta: Meta<typeof ProjectionLane> = {
  title: 'Debug UI/ProjectionLane',
  component: ProjectionLane,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '300px', background: 'var(--bg-primary)', padding: '8px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof ProjectionLane>;

export const Default: Story = {
  args: {
    entities: mockTimelineEntities,
  },
};

export const WithSelection: Story = {
  args: {
    entities: mockTimelineEntities,
    selectedEntityId: mockTimelineEntities[0].id,
  },
};

export const Empty: Story = {
  args: {
    entities: [],
  },
};

export const StreamingMessage: Story = {
  args: {
    entities: [
      {
        id: 'msg-streaming',
        kind: 'message',
        created_at: Date.now(),
        version: 3,
        props: { role: 'assistant', content: 'Generating...', streaming: true },
      },
      ...mockTimelineEntities,
    ],
  },
};

export const WithVersions: Story = {
  args: {
    entities: [
      { ...mockTimelineEntities[0], version: 1 },
      { ...mockTimelineEntities[0], id: 'msg-v2', version: 2, created_at: Date.now() - 5000 },
      { ...mockTimelineEntities[0], id: 'msg-v5', version: 5, created_at: Date.now() - 2000 },
    ],
  },
};
