import type { Meta, StoryObj } from '@storybook/react';
import React, { useEffect } from 'react';
import { ChatWidget } from './ChatWidget';
import { handleSem } from '../sem/registry';
import { useAppDispatch } from '../store/hooks';
import { timelineSlice } from '../store/timelineSlice';

const meta: Meta<typeof ChatWidget> = {
  title: 'WebChat/ChatWidget',
  component: ChatWidget,
};

export default meta;
type Story = StoryObj<typeof ChatWidget>;

export const Default: Story = {};

function ScenarioRunner() {
  const dispatch = useAppDispatch();
  useEffect(() => {
    dispatch(timelineSlice.actions.clear());
    const frames = [
      { sem: true, event: { type: 'log', id: 'log-1', data: { level: 'info', message: 'hydrated: scenario start' } } },
      { sem: true, event: { type: 'llm.start', id: 'm1', data: { role: 'assistant' } } },
      { sem: true, event: { type: 'llm.delta', id: 'm1', data: { delta: 'Hello', cumulative: 'Hello' } } },
      { sem: true, event: { type: 'llm.delta', id: 'm1', data: { delta: ', world', cumulative: 'Hello, world' } } },
      { sem: true, event: { type: 'tool.start', id: 't1', data: { name: 'calc', input: { expression: '1+1' } } } },
      { sem: true, event: { type: 'tool.result', id: 't1', data: { result: '2', customKind: 'calc_result' } } },
      { sem: true, event: { type: 'llm.final', id: 'm1', data: { text: 'Hello, world.' } } },
    ];
    for (const fr of frames) {
      handleSem(fr, dispatch);
    }
  }, [dispatch]);
  return <ChatWidget />;
}

export const ScenarioBasic: Story = {
  render: () => <ScenarioRunner />,
};
