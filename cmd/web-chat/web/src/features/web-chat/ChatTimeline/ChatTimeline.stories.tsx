import type { Meta, StoryObj } from '@storybook/react';
import { useRef } from 'react';
import type { ChatWidgetRenderers, RenderEntity } from '../../../webchat/types';
import { ChatTimeline } from './ChatTimeline';
import type { ChatTimelineProps } from './types';

function message(id: string, role: string, content: string): RenderEntity {
  return { id, kind: 'message', createdAt: Date.now(), props: { role, content } };
}

function tool(id: string, name: string, status: string): RenderEntity {
  return { id, kind: 'tool_call', createdAt: Date.now(), props: { name, status, input: { query: 'boots' } } };
}

function widget(id: string, widgetType: string, status: string): RenderEntity {
  return { id, kind: 'widget', createdAt: Date.now(), props: { widgetType, status, title: 'Cart review' } };
}

const renderers: ChatWidgetRenderers = {
  default: ({ e }) => <pre data-part="card">{JSON.stringify(e.props, null, 2)}</pre>,
  message: ({ e }) => <div>{String(e.props.content ?? '')}</div>,
  tool_call: ({ e }) => <div data-part="card">tool: {String(e.props.name)} · {String(e.props.status)}</div>,
  widget: ({ e }) => <div data-part="card">widget: {String(e.props.widgetType)} · {String(e.props.status)}</div>,
};

function TimelineStory(args: Omit<ChatTimelineProps, 'bottomRef'>) {
  const bottomRef = useRef<HTMLDivElement>(null);
  return <ChatTimeline {...args} bottomRef={bottomRef} />;
}

const meta: Meta<typeof TimelineStory> = {
  title: 'WebChat/Components/ChatTimeline',
  component: TimelineStory,
  args: {
    entities: [],
    errors: [],
    showErrors: false,
    errorCount: 0,
    onClearErrors: () => undefined,
    onToggleErrors: () => undefined,
    renderers,
    state: 'idle',
  },
  decorators: [
    (Story) => (
      <div data-pwchat="" data-part="root" data-theme="default" style={{ height: 520 }}>
        <main data-part="main">
          <Story />
        </main>
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TimelineStory>;

export const Empty: Story = {};

export const MessageOnly: Story = {
  args: {
    entities: [
      message('m1', 'user', 'show me boots'),
      message('m2', 'assistant', 'Here are several boot options with sizes and prices.'),
    ],
  },
};

export const ToolAndWidget: Story = {
  args: {
    entities: [
      message('m1', 'user', 'add boots to cart'),
      tool('tool-1', 'cart.add_item', 'completed'),
      widget('widget-1', 'cart.review', 'ready'),
    ],
    state: 'streaming',
  },
};

export const ErrorPanel: Story = {
  args: {
    showErrors: true,
    errorCount: 1,
    errors: [
      {
        id: 'err-1',
        scope: 'chat-provider',
        message: 'WebSocket closed while streaming.',
        detail: 'close code 1006',
        time: Date.now(),
      },
    ],
  },
};

export const DetachedScroll: Story = {
  args: {
    entities: Array.from({ length: 18 }, (_, index) =>
      message(`m-${index}`, index % 2 === 0 ? 'assistant' : 'user', `Timeline item ${index + 1}`),
    ),
    state: 'streaming',
  },
};
