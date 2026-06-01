import type { Meta, StoryObj } from '@storybook/react';
import '../index.css';
import { debugEntities } from './debugFixtures';
import { ProjectionLane } from './ProjectionLane';

const meta: Meta<typeof ProjectionLane> = {
  title: 'Debug UI/Components/ProjectionLane',
  component: ProjectionLane,
  args: { entities: debugEntities },
};

export default meta;
type Story = StoryObj<typeof ProjectionLane>;

export const Default: Story = {};

export const Empty: Story = {
  args: { entities: [] },
};
