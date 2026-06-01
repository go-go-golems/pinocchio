import type { Meta, StoryObj } from '@storybook/react';
import { Route, Routes } from 'react-router-dom';
import '../index.css';
import { AppShell } from './AppShell';
import { debugEntities, debugEvents } from './debugFixtures';
import { TimelineLanes } from './TimelineLanes';

function StoryOutlet() {
  return (
    <div className="timeline-page">
      <TimelineLanes events={debugEvents} entities={debugEntities} isLive />
    </div>
  );
}

const meta: Meta<typeof AppShell> = {
  title: 'Debug UI/Components/AppShell',
  component: AppShell,
  decorators: [
    () => (
      <Routes>
        <Route path="*" element={<AppShell />}>
          <Route index element={<StoryOutlet />} />
          <Route path="timeline" element={<StoryOutlet />} />
          <Route path="events" element={<StoryOutlet />} />
        </Route>
      </Routes>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AppShell>;

export const Default: Story = {};
