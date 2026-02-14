import type { Meta, StoryObj } from '@storybook/react';
import { useEffect } from 'react';
import { defaultHandlers } from '../mocks/msw/defaultHandlers';
import { useAppDispatch } from '../store/hooks';
import { selectRun, setOfflineConfig } from '../store/uiSlice';
import { OfflinePage } from './OfflinePage';

function PreparedOfflinePage() {
  const dispatch = useAppDispatch();

  useEffect(() => {
    dispatch(
      setOfflineConfig({
        artifactsRoot: '/tmp/artifacts',
        turnsDB: '/tmp/turns.db',
        timelineDB: '/tmp/timeline.db',
      })
    );
    dispatch(selectRun('turns|conv_8a3f|sess_01'));
  }, [dispatch]);

  return <OfflinePage />;
}

const meta: Meta<typeof PreparedOfflinePage> = {
  title: 'Debug UI/Routes/OfflinePage',
  component: PreparedOfflinePage,
  parameters: {
    msw: {
      handlers: defaultHandlers,
    },
  },
};

export default meta;
type Story = StoryObj<typeof PreparedOfflinePage>;

export const Default: Story = {};
