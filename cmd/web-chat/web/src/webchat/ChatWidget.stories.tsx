import type { Meta, StoryObj } from '@storybook/react';
import type { ComponentProps } from 'react';
import { useEffect } from 'react';
import { useAppDispatch } from '../store/hooks';
import type { TimelineEntity } from '../store/timelineSlice';
import { timelineSlice } from '../store/timelineSlice';
import { ChatWidget } from './ChatWidget';

const meta: Meta<typeof ChatWidget> = {
  title: 'WebChat/ChatWidget',
  component: ChatWidget,
};

export default meta;
type Story = StoryObj<typeof ChatWidget>;

export const Default: Story = {};

export const ThemeOverrides: Story = {
  args: {
    themeVars: {
      '--pwchat-accent': '#ff9f1a',
      '--pwchat-border': '#3b2c1f',
      '--pwchat-surface-1': '#14110e',
      '--pwchat-surface-2': '#1b1510',
      '--pwchat-muted': '#c9a67a',
    },
  },
};

export const Unstyled: Story = {
  render: () => (
    <div>
      <style>{`
        [data-pwchat][data-part="root"] {
          --pwchat-bg: #f7f4ef;
          --pwchat-fg: #222;
          --pwchat-surface-1: #ffffff;
          --pwchat-surface-2: #f1ece6;
          --pwchat-border: #d8cfc4;
          --pwchat-accent: #1f6feb;
          --pwchat-muted: '#6b6258';
          --pwchat-radius: 16px;
          --pwchat-gap: 12px;
        }
      `}</style>
      <ChatWidget unstyled />
    </div>
  ),
};

export const CustomRenderer: Story = {
  render: () => (
    <ScenarioRunner
      entities={[
        msg('m1', { role: 'assistant', content: 'Custom renderer example', status: 'finished', streaming: false }),
        log('log-1', { level: 'info', message: 'hydrated: scenario start' }),
      ]}
      widgetProps={{
        renderers: {
          log: ({ e }: { e: any }) => (
            <div data-part="card">
              <div data-part="card-body">
                <strong>LOG:</strong> {String(e.props?.message ?? '')}
              </div>
            </div>
          ),
        },
      }}
    />
  ),
};

function msg(id: string, props: Record<string, unknown>): TimelineEntity {
  return { id, kind: 'message', createdAt: Date.now(), updatedAt: Date.now(), props };
}

function log(id: string, props: Record<string, unknown>): TimelineEntity {
  return { id, kind: 'log', createdAt: Date.now(), updatedAt: Date.now(), props };
}

function tool(id: string, props: Record<string, unknown>): TimelineEntity {
  return { id, kind: 'tool', createdAt: Date.now(), updatedAt: Date.now(), props };
}

type ScenarioRunnerProps = {
  entities?: TimelineEntity[];
  delayMs?: number;
  widgetProps?: ComponentProps<typeof ChatWidget>;
};

function ScenarioRunner({ entities, delayMs, widgetProps }: ScenarioRunnerProps) {
  const dispatch = useAppDispatch();
  useEffect(() => {
    dispatch(timelineSlice.actions.clear());
    if (!delayMs || !entities) {
      if (entities) {
        for (const e of entities) dispatch(timelineSlice.actions.upsertEntity(e));
      }
      return;
    }

    let cancelled = false;
    let idx = 0;
    const tick = () => {
      if (cancelled || !entities) return;
      if (idx >= entities.length) return;
      dispatch(timelineSlice.actions.upsertEntity(entities[idx]));
      idx++;
      setTimeout(tick, delayMs);
    };
    tick();
    return () => { cancelled = true; };
  }, [delayMs, dispatch, entities]);
  return <ChatWidget {...widgetProps} />;
}

export const ScenarioBasic: Story = {
  render: () => (
    <ScenarioRunner
      entities={[
        log('log-1', { level: 'info', message: 'hydrated: scenario start' }),
        msg('m1', { role: 'assistant', content: 'Hello, world.', status: 'finished', streaming: false }),
        tool('t1', { name: 'calc', input: { expression: '1+1' }, result: '2', status: 'done' }),
      ]}
    />
  ),
};

export const ScenarioStreamingAndTools: Story = {
  render: () => (
    <ScenarioRunner
      delayMs={250}
      entities={[
        msg('m2', { role: 'assistant', content: "Let's compute: 42", status: 'finished', streaming: false }),
        tool('t2', { name: 'calc', input: { expression: '40+2' }, result: '42', status: 'done' }),
      ]}
    />
  ),
};

export const ScenarioReconnectIdempotentReplay: Story = {
  render: () => (
    <ScenarioRunner
      entities={[
        msg('m3', { role: 'assistant', content: 'A.', status: 'finished', streaming: false }),
        msg('m4', { role: 'assistant', content: 'B.', status: 'finished', streaming: false }),
      ]}
    />
  ),
};

export const WidgetOnlyDebuggerPause: Story = {
  render: () => (
    <ScenarioRunner
      entities={[
        { id: 'pause-1', kind: 'debugger_pause', createdAt: Date.now(), updatedAt: Date.now(), props: { pauseId: 'pause-1', phase: 'toolloop', summary: 'paused for approval', deadlineMs: '1737760000000' } },
      ]}
    />
  ),
};

export const WidgetOnlyAgentMode: Story = {
  render: () => (
    <ScenarioRunner
      entities={[
        { id: 'agent-1', kind: 'agent_mode', createdAt: Date.now(), updatedAt: Date.now(), props: { title: 'agentmode: mode switched', from: 'financial_analyst', to: 'category_regexp_reviewer', analysis: 'The draft patterns are ready for a dedicated review pass.' } },
      ]}
    />
  ),
};
