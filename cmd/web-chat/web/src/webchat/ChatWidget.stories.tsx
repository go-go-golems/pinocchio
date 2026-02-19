import type { Meta, StoryObj } from '@storybook/react';
import type { ComponentProps } from 'react';
import { useEffect } from 'react';
import { registerThinkingModeModule } from '../features/thinkingMode/registerThinkingMode';
import { handleSem, registerDefaultSemHandlers } from '../sem/registry';
import { useAppDispatch } from '../store/hooks';
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
          --pwchat-muted: #6b6258;
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
      frames={[
        { sem: true, event: { type: 'log', id: 'log-1', seq: 1, data: { id: 'log-1', level: 'info', message: 'hydrated: scenario start', fields: {} } } },
        { sem: true, event: { type: 'llm.start', id: 'm1', seq: 2, data: { id: 'm1', role: 'assistant' } } },
        { sem: true, event: { type: 'llm.final', id: 'm1', seq: 3, data: { id: 'm1', text: 'Custom renderer example' } } },
      ]}
      widgetProps={{
        renderers: {
          log: ({ e }) => (
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

type ScenarioRunnerProps = {
  frames: any[];
  delayMs?: number;
  widgetProps?: ComponentProps<typeof ChatWidget>;
};

function ScenarioRunner({ frames, delayMs, widgetProps }: ScenarioRunnerProps) {
  const dispatch = useAppDispatch();
  useEffect(() => {
    registerDefaultSemHandlers();
    registerThinkingModeModule();
    dispatch(timelineSlice.actions.clear());
    if (!delayMs) {
      for (const fr of frames) handleSem(fr, dispatch);
      return;
    }

    let cancelled = false;
    let idx = 0;
    const tick = () => {
      if (cancelled) return;
      if (idx >= frames.length) return;
      handleSem(frames[idx], dispatch);
      idx++;
      setTimeout(tick, delayMs);
    };
    tick();
    return () => {
      cancelled = true;
    };
  }, [delayMs, dispatch, frames]);
  return <ChatWidget {...widgetProps} />;
}

export const ScenarioBasic: Story = {
  render: () => (
    <ScenarioRunner
      frames={[
        { sem: true, event: { type: 'log', id: 'log-1', seq: 1, data: { id: 'log-1', level: 'info', message: 'hydrated: scenario start', fields: {} } } },
        { sem: true, event: { type: 'llm.start', id: 'm1', seq: 2, data: { id: 'm1', role: 'assistant' } } },
        { sem: true, event: { type: 'llm.delta', id: 'm1', seq: 3, data: { id: 'm1', delta: 'Hello', cumulative: 'Hello' } } },
        { sem: true, event: { type: 'llm.delta', id: 'm1', seq: 4, data: { id: 'm1', delta: ', world', cumulative: 'Hello, world' } } },
        { sem: true, event: { type: 'tool.start', id: 't1', seq: 5, data: { id: 't1', name: 'calc', input: { expression: '1+1' } } } },
        { sem: true, event: { type: 'tool.result', id: 't1', seq: 6, data: { id: 't1', result: '2', customKind: 'calc_result' } } },
        { sem: true, event: { type: 'tool.done', id: 't1', seq: 7, data: { id: 't1' } } },
        { sem: true, event: { type: 'llm.final', id: 'm1', seq: 8, data: { id: 'm1', text: 'Hello, world.' } } },
      ]}
    />
  ),
};

export const ScenarioStreamingAndTools: Story = {
  render: () => (
    <ScenarioRunner
      delayMs={250}
      frames={[
        { sem: true, event: { type: 'llm.start', id: 'm2', seq: 1, data: { id: 'm2', role: 'assistant' } } },
        { sem: true, event: { type: 'llm.delta', id: 'm2', seq: 2, data: { id: 'm2', delta: 'Let', cumulative: 'Let' } } },
        { sem: true, event: { type: 'llm.delta', id: 'm2', seq: 3, data: { id: 'm2', delta: "'s ", cumulative: "Let's " } } },
        { sem: true, event: { type: 'llm.delta', id: 'm2', seq: 4, data: { id: 'm2', delta: 'compute: ', cumulative: "Let's compute: " } } },
        { sem: true, event: { type: 'tool.start', id: 't2', seq: 5, data: { id: 't2', name: 'calc', input: { expression: '40+2' } } } },
        { sem: true, event: { type: 'tool.result', id: 't2', seq: 6, data: { id: 't2', result: '42', customKind: 'calc_result' } } },
        { sem: true, event: { type: 'tool.done', id: 't2', seq: 7, data: { id: 't2' } } },
        { sem: true, event: { type: 'llm.final', id: 'm2', seq: 8, data: { id: 'm2', text: "Let's compute: 42" } } },
      ]}
    />
  ),
};

export const ScenarioReconnectIdempotentReplay: Story = {
  render: () => (
    <ScenarioRunner
      frames={[
        { sem: true, event: { type: 'log', id: 'log-r1', seq: 1, data: { id: 'log-r1', level: 'info', message: 'first run', fields: {} } } },
        { sem: true, event: { type: 'llm.start', id: 'm3', seq: 2, data: { id: 'm3', role: 'assistant' } } },
        { sem: true, event: { type: 'llm.delta', id: 'm3', seq: 3, data: { id: 'm3', delta: 'A', cumulative: 'A' } } },
        { sem: true, event: { type: 'llm.final', id: 'm3', seq: 4, data: { id: 'm3', text: 'A.' } } },
        // Simulate hydration replay on reconnect (same IDs, different seq ordering shouldn't create duplicates).
        { sem: true, event: { type: 'log', id: 'log-r1', seq: 5, data: { id: 'log-r1', level: 'info', message: 'first run', fields: {} } } },
        { sem: true, event: { type: 'llm.final', id: 'm3', seq: 6, data: { id: 'm3', text: 'A.' } } },
        { sem: true, event: { type: 'llm.delta', id: 'm3', seq: 7, data: { id: 'm3', delta: 'A', cumulative: 'A' } } },
        { sem: true, event: { type: 'llm.start', id: 'm3', seq: 8, data: { id: 'm3', role: 'assistant' } } },
        // Then a new message after reconnect.
        { sem: true, event: { type: 'llm.start', id: 'm4', seq: 9, data: { id: 'm4', role: 'assistant' } } },
        { sem: true, event: { type: 'llm.final', id: 'm4', seq: 10, data: { id: 'm4', text: 'B.' } } },
      ]}
    />
  ),
};

export const WidgetOnlyDebuggerPause: Story = {
  render: () => (
    <ScenarioRunner
      frames={[
        {
          sem: true,
          event: {
            type: 'debugger.pause',
            id: 'pause-1',
            seq: 1,
            data: {
              id: 'pause-1',
              pauseId: 'pause-1',
              phase: 'toolloop',
              summary: 'paused for approval',
              deadlineMs: '1737760000000',
              extra: { owner: 'storybook' },
            },
          },
        },
      ]}
    />
  ),
};

export const WidgetOnlyAgentMode: Story = {
  render: () => (
    <ScenarioRunner
      frames={[
        {
          sem: true,
          event: { type: 'agent.mode', id: 'agent-1', seq: 1, data: { id: 'agent-1', title: 'Research mode', data: { depth: 'high' } } },
        },
      ]}
    />
  ),
};

export const WidgetOnlyThinkingMode: Story = {
  render: () => (
    <ScenarioRunner
      frames={[
        {
          sem: true,
          event: {
            type: 'thinking.mode.started',
            id: 'tm-1',
            seq: 1,
            data: {
              itemId: 'tm-1',
              data: { mode: 'deep', phase: 'selection', reasoning: 'The task benefits from deep reasoning and careful planning.' },
            },
          },
        },
        {
          sem: true,
          event: {
            type: 'thinking.mode.update',
            id: 'tm-1',
            seq: 2,
            data: {
              itemId: 'tm-1',
              data: { mode: 'deep', phase: 'confirmed', reasoning: 'Proceeding with deep mode.' },
            },
          },
        },
        {
          sem: true,
          event: {
            type: 'thinking.mode.completed',
            id: 'tm-1',
            seq: 3,
            data: {
              itemId: 'tm-1',
              data: { mode: 'deep', phase: 'confirmed', reasoning: 'Locked in.' },
              success: true,
              error: '',
            },
          },
        },
      ]}
    />
  ),
};
