import {
  ChatProvider,
  defineChatExtensions,
  defineTool,
  defineWidget,
  selectOverlay,
  selectTimelineEntities,
  ToolCallOutlet,
  useChatClient,
  useChatExtensions,
  useAppSelector as useChatProviderSelector,
  WidgetOutlet,
  type WidgetProps,
} from '@go-go-golems/chat-provider';
import type React from 'react';
import { useEffect, useMemo, useState } from 'react';
import { pinocchioWebChatProjectors } from '../features/web-chat';
import { basePrefixFromLocation } from '../utils/basePrefix';
import { Markdown } from './Markdown';
import { fmtSentAt } from './utils';

type ConfirmActionInput = {
  title?: string;
  body?: string;
  confirmLabel?: string;
  cancelLabel?: string;
};

type ConfirmActionResult = {
  approved: boolean;
  decision: 'approved' | 'denied';
  decidedAt: string;
};

type CapabilityStep = {
  id?: string;
  label?: string;
  state?: string;
};

function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) return value as Record<string, unknown>;
  return {};
}

function stepVariant(state: string): string {
  switch (state) {
    case 'done':
      return 'accent';
    case 'running':
      return 'warning';
    case 'failed':
      return 'danger';
    default:
      return 'ghost';
  }
}

function CapabilityCard({ status, props }: WidgetProps) {
  const title = String(props.title ?? 'Capabilities showcase');
  const summary = String(props.summary ?? 'Demonstrating the headless ChatProvider runtime.');
  const steps = Array.isArray(props.steps) ? (props.steps as CapabilityStep[]) : [];
  const result = props.result ? String(props.result) : '';
  return (
    <div data-part="card" data-variant="widget">
      <div data-part="card-header">
        <div data-part="card-header-title">{title}</div>
        <div data-part="pill" data-variant="accent" data-mono="true">demo.capability_card</div>
        <div data-part="pill" data-mono="true">{status}</div>
      </div>
      <div data-part="card-body">
        <div style={{ color: 'var(--pwchat-muted)', marginBottom: 10 }}>{summary}</div>
        <div style={{ display: 'grid', gap: 8 }}>
          {steps.map((step, index) => {
            const state = String(step.state ?? 'pending');
            return (
              <div key={step.id ?? index} data-part="row" style={{ justifyContent: 'space-between', gap: 10 }}>
                <span>{String(step.label ?? step.id ?? `Step ${index + 1}`)}</span>
                <span data-part="pill" data-variant={stepVariant(state)} data-mono="true">{state}</span>
              </div>
            );
          })}
        </div>
        {result ? <div style={{ marginTop: 10 }}>{result}</div> : null}
      </div>
    </div>
  );
}

const capabilityWidget = defineWidget('demo.capability_card', CapabilityCard);

export const webChatProviderCapabilitiesExtension = defineChatExtensions({
  name: 'web-chat-provider-demo',
  widgets: [capabilityWidget],
  tools: [
    defineTool({
      name: 'browser.get_page_context',
      description: 'Return URL, viewport, and browser metadata from the web-chat page.',
      mode: 'frontend',
      inputSchema: { type: 'object', additionalProperties: false },
      execute: async () => ({
        url: window.location.href,
        viewport: { width: window.innerWidth, height: window.innerHeight },
        userAgent: window.navigator.userAgent,
        timestamp: new Date().toISOString(),
      }),
    }),
    defineTool<{
      name: 'browser.confirm_action';
      description: string;
      mode: 'human';
      inputSchema: Record<string, unknown>;
      render: (props: {
        input: ConfirmActionInput;
        respond: (result: ConfirmActionResult) => void;
        reject: (message?: string) => void;
      }) => React.ReactNode;
    }>({
      name: 'browser.confirm_action',
      description: 'Ask the user to approve or deny a browser-side action.',
      mode: 'human',
      inputSchema: {
        type: 'object',
        properties: {
          title: { type: 'string' },
          body: { type: 'string' },
          confirmLabel: { type: 'string' },
          cancelLabel: { type: 'string' },
        },
        additionalProperties: true,
      },
      render: ({ input, respond, reject }) => {
        const title = input.title ?? 'Confirm action';
        const body = input.body ?? 'The assistant is asking the browser to approve an action.';
        return (
          <div data-part="callout" data-variant="warning">
            <div data-part="pill" data-variant="accent" data-mono="true" style={{ marginBottom: 8 }}>browser.confirm_action</div>
            <strong>{title}</strong>
            <div style={{ marginTop: 6 }}>{body}</div>
            <div data-part="toolbar" style={{ marginTop: 10 }}>
              <button
                type="button"
                data-part="button"
                data-variant="primary"
                onClick={() => respond({ approved: true, decision: 'approved', decidedAt: new Date().toISOString() })}
              >
                {input.confirmLabel ?? 'Approve'}
              </button>
              <button type="button" data-part="button" data-variant="ghost" onClick={() => reject('User denied the request')}>
                {input.cancelLabel ?? 'Deny'}
              </button>
            </div>
          </div>
        );
      },
    }),
  ],
});

export function WebChatProviderCapabilities() {
  const client = useChatClient();
  useChatExtensions(webChatProviderCapabilitiesExtension);
  useEffect(() => {
    client.open();
  }, [client]);
  return null;
}

function ProviderMessageCard({ entity }: { entity: { id: string; createdAt: number; props: Record<string, unknown> } }) {
  const role = String(entity.props.role ?? 'assistant');
  const content = String(entity.props.content ?? '');
  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="message-role" data-role={role === 'user' || role === 'thinking' ? role : 'assistant'}>{role}</div>
        <div data-part="card-header-meta">{fmtSentAt(entity.createdAt)}</div>
      </div>
      <div data-part="card-body">{content ? <Markdown text={content} /> : <div data-part="pill">...</div>}</div>
    </div>
  );
}

function ProviderTimeline() {
  const entities = useChatProviderSelector(selectTimelineEntities);
  if (entities.length === 0) {
    return (
      <div data-part="timeline">
        <div data-part="empty-state">Send “run the capabilities demo” to see ChatProvider tools and widgets.</div>
      </div>
    );
  }
  return (
    <div data-part="timeline">
      {entities.map((entity) => {
        if (entity.kind === 'widget') {
          const widgetName = String(entity.props.widgetName ?? 'widget');
          return (
            <WidgetOutlet
              key={entity.id}
              instanceId={String(entity.props.instanceId ?? entity.id)}
              widgetName={widgetName}
              status={String(entity.props.status ?? 'unknown')}
              props={asRecord(entity.props.props)}
            />
          );
        }
        if (entity.kind === 'tool_call') {
          return (
            <ToolCallOutlet
              key={entity.id}
              toolCallId={String(entity.props.toolCallId ?? entity.id)}
              toolName={String(entity.props.toolName ?? 'tool')}
              status={String(entity.props.status ?? 'requested')}
              input={entity.props.input}
              result={entity.props.result}
              error={typeof entity.props.error === 'string' ? entity.props.error : undefined}
            />
          );
        }
        return <ProviderMessageCard key={entity.id} entity={entity} />;
      })}
    </div>
  );
}

function ProviderComposer() {
  const client = useChatClient();
  const [text, setText] = useState('run the capabilities demo');
  return (
    <form
      data-part="composer"
      onSubmit={(event) => {
        event.preventDefault();
        const prompt = text.trim();
        if (!prompt) return;
        void client.send(prompt);
        setText('');
      }}
    >
      <textarea aria-label="Ask something…" value={text} onChange={(event) => setText(event.target.value)} />
      <div data-part="toolbar">
        <button type="submit" data-part="button" data-variant="primary">Send</button>
        <button type="button" data-part="button" data-variant="ghost" onClick={() => client.reset()}>Reset</button>
      </div>
    </form>
  );
}

function ProviderStatusBar() {
  const overlay = useChatProviderSelector(selectOverlay);
  return (
    <div data-part="statusbar">
      <span>ChatProvider demo</span>
      <span>session: {overlay.sessionId || 'new'}</span>
      <span>run: {overlay.runStatus}</span>
      <span>ws: {overlay.wsStatus}</span>
      {overlay.error ? <span data-part="pill" data-variant="danger">{overlay.error}</span> : null}
    </div>
  );
}

function ProviderDemoShell() {
  return (
    <div data-pwchat="" data-part="root" data-theme="default" data-fullscreen="true">
      <header data-part="header">
        <div>
          <div data-part="header-title">Web Chat — ChatProvider API Demo</div>
          <div style={{ color: 'var(--pwchat-muted)', fontSize: 13, marginTop: 4 }}>
            Headless ChatProvider runtime with web-chat-owned page chrome, frontend tools, and typed widgets.
          </div>
        </div>
        <ProviderStatusBar />
      </header>
      <WebChatProviderCapabilities />
      <main data-part="main">
        <ProviderTimeline />
      </main>
      <ProviderComposer />
    </div>
  );
}

export function ProviderDemoPage() {
  const basePrefix = basePrefixFromLocation();
  const config = useMemo(() => ({
    basePrefix,
    extensions: [pinocchioWebChatProjectors],
    createSessionBody: () => ({ profile: 'gpt-5-nano-low' }),
    sendMessageBody: ({ prompt }: { prompt: string }) => ({ prompt, profile: 'gpt-5-nano-low' }),
  }), [basePrefix]);
  return (
    <ChatProvider config={config}>
      <ProviderDemoShell />
    </ChatProvider>
  );
}
