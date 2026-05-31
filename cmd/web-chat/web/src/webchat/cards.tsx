import { logWarn } from '../utils/logger';
import { submitFrontendToolResult } from '../ws/frontendTools';
import { normalizeAgentModeAnalysis } from './agentModeMarkdown';
import { Markdown } from './Markdown';
import type { RenderEntity } from './types';
import { fmtSentAt } from './utils';

function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

export function MessageCard({ e }: { e: RenderEntity }) {
  const role = String(e.props?.role ?? 'assistant');
  const content = String(e.props?.content ?? '');
  const error = String(e.props?.error ?? '');
  const streaming = !!e.props?.streaming;
  const roleAttr = role === 'user' || role === 'assistant' || role === 'thinking' ? role : 'assistant';

  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="message-role" data-role={roleAttr}>
          {role}
        </div>
        {streaming ? <div data-part="streaming-dot" /> : null}
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        {error ? (
          <div data-part="error-item">
            <div data-part="pill" data-variant="danger">
              stopped
            </div>
            <div data-part="error-item-detail" data-mono="true" style={{ marginTop: 8 }}>
              {error}
            </div>
          </div>
        ) : null}
        {content ? <Markdown text={content} /> : error ? null : <div data-part="pill">...</div>}
      </div>
    </div>
  );
}

export function ToolCallCard({ e }: { e: RenderEntity }) {
  const name = String(e.props?.name ?? e.props?.toolName ?? 'tool');
  const input = e.props?.input ?? {};
  const result = e.props?.result;
  const status = String(e.props?.status ?? '');
  const sessionId = String(e.props?.sessionId ?? '');
  const toolCallId = String(e.props?.toolCallId ?? e.id ?? '');
  const done = !!e.props?.done || !!result || status === 'success' || status === 'denied' || status === 'failed';
  const isHumanConfirm = name === 'browser.confirm_action' && !done && sessionId && toolCallId;
  const title = done ? `${name} (done)` : name;
  const inputRecord = asRecord(input);
  const confirmTitle = String(inputRecord.title ?? 'Confirm action');
  const confirmBody = String(inputRecord.body ?? 'The assistant is asking the browser to approve an action.');
  const confirmLabel = String(inputRecord.confirmLabel ?? 'Approve');
  const cancelLabel = String(inputRecord.cancelLabel ?? 'Deny');
  const respond = (approved: boolean) => {
    void submitFrontendToolResult({
      sessionId,
      toolCallId,
      toolName: name,
      status: approved ? 'success' : 'denied',
      result: {
        approved,
        decision: approved ? 'approved' : 'denied',
        decidedAt: new Date().toISOString(),
      },
    }).catch((err) => logWarn('frontend tool result submission failed', { scope: 'tool.frontend.result', extra: { toolCallId, name } }, err));
  };
  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="card-header-title">Tool</div>
        <div data-part="pill" data-variant="accent" data-mono="true">
          {title}
        </div>
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        <div data-part="toolbar">
          <button
            type="button"
            data-part="button"
            data-variant="ghost"
            onClick={() =>
              void navigator.clipboard
                .writeText(JSON.stringify(input ?? {}, null, 2))
                .catch((err) => logWarn('clipboard copy failed', { scope: 'tool.copyArgs' }, err))
            }
          >
            Copy args
          </button>
        </div>
        {isHumanConfirm ? (
          <div data-part="callout" data-variant="warning" style={{ marginBottom: 10 }}>
            <strong>{confirmTitle}</strong>
            <div style={{ marginTop: 6 }}>{confirmBody}</div>
            <div data-part="toolbar" style={{ marginTop: 10 }}>
              <button type="button" data-part="button" data-variant="primary" onClick={() => respond(true)}>
                {confirmLabel}
              </button>
              <button type="button" data-part="button" data-variant="ghost" onClick={() => respond(false)}>
                {cancelLabel}
              </button>
            </div>
          </div>
        ) : null}
        {result ? (
          <pre data-part="mono" style={{ margin: '0 0 10px', whiteSpace: 'pre-wrap' }}>
            {JSON.stringify(result, null, 2)}
          </pre>
        ) : null}
        <pre data-part="mono" style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {JSON.stringify(input ?? {}, null, 2)}
        </pre>
      </div>
    </div>
  );
}

export function ToolResultCard({ e }: { e: RenderEntity }) {
  const customKind = e.props?.customKind ? String(e.props.customKind) : '';
  const result = String(e.props?.result ?? '');
  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="card-header-title">Result</div>
        {customKind ? (
          <div data-part="pill" data-mono="true">
            {customKind}
          </div>
        ) : null}
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        <div data-part="toolbar">
          <button
            type="button"
            data-part="button"
            data-variant="ghost"
            onClick={() => void navigator.clipboard.writeText(result).catch((err) => logWarn('clipboard copy failed', { scope: 'tool.copyResult' }, err))}
          >
            Copy
          </button>
        </div>
        <pre data-part="mono" style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {result}
        </pre>
      </div>
    </div>
  );
}

export function LogCard({ e }: { e: RenderEntity }) {
  const level = String(e.props?.level ?? 'info');
  const message = String(e.props?.message ?? '');
  return (
    <div data-part="card" data-variant="log">
      <div data-part="card-body">
        <div data-part="row" style={{ justifyContent: 'space-between' }}>
          <div data-part="pill" data-mono="true">
            {level}
          </div>
          <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
        </div>
        <div style={{ marginTop: 8, color: 'var(--pwchat-muted)', fontSize: 13 }}>{message}</div>
      </div>
    </div>
  );
}

export function AgentModeCard({ e }: { e: RenderEntity }) {
  const title = String(e.props?.title ?? 'Agent mode switch');
  const data = asRecord(e.props?.data);
  const from = typeof data.from === 'string' ? data.from : '';
  const to = typeof data.to === 'string' ? data.to : '';
  const analysis = typeof data.analysis === 'string' ? normalizeAgentModeAnalysis(data.analysis) : '';
  const preview = e.props?.preview === true;
  const extraData: Record<string, unknown> = { ...data };
  delete extraData.from;
  delete extraData.to;
  delete extraData.analysis;
  const hasExtraData = Object.keys(extraData).length > 0;

  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="card-header-title">{title}</div>
        {preview ? (
          <div data-part="pill" data-variant="warning">
            preview
          </div>
        ) : null}
        {from ? (
          <div data-part="pill" data-mono="true">
            from {from}
          </div>
        ) : null}
        {to ? (
          <div data-part="pill" data-variant="accent" data-mono="true">
            to {to}
          </div>
        ) : null}
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        {analysis ? <Markdown text={analysis} /> : <div data-part="pill">No analysis</div>}
        {hasExtraData ? (
          <pre data-part="mono" style={{ margin: '10px 0 0', whiteSpace: 'pre-wrap' }}>
            {JSON.stringify(extraData, null, 2)}
          </pre>
        ) : null}
      </div>
    </div>
  );
}

type CapabilityStep = {
  id?: string;
  label?: string;
  state?: string;
};

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

export function CapabilityCard({ e }: { e: RenderEntity }) {
  const status = String(e.props?.status ?? 'unknown');
  const props = asRecord(e.props?.props);
  const title = String(props.title ?? 'Capabilities showcase');
  const summary = String(props.summary ?? 'Demonstrating web-chat package capabilities.');
  const steps = Array.isArray(props.steps) ? (props.steps as CapabilityStep[]) : [];
  const result = props.result ? String(props.result) : '';
  return (
    <div data-part="card" data-variant="widget">
      <div data-part="card-header">
        <div data-part="card-header-title">{title}</div>
        <div data-part="pill" data-variant="accent" data-mono="true">
          demo.capability_card
        </div>
        <div data-part="pill" data-mono="true">
          {status}
        </div>
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        <div style={{ color: 'var(--pwchat-muted)', marginBottom: 10 }}>{summary}</div>
        <div style={{ display: 'grid', gap: 8 }}>
          {steps.map((step, index) => {
            const state = String(step.state ?? 'pending');
            return (
              <div key={step.id ?? index} data-part="row" style={{ justifyContent: 'space-between', gap: 10 }}>
                <span>{String(step.label ?? step.id ?? `Step ${index + 1}`)}</span>
                <span data-part="pill" data-variant={stepVariant(state)} data-mono="true">
                  {state}
                </span>
              </div>
            );
          })}
        </div>
        {result ? <div style={{ marginTop: 10 }}>{result}</div> : null}
      </div>
    </div>
  );
}

export function WidgetInstanceCard({ e }: { e: RenderEntity }) {
  const widgetName = String(e.props?.widgetName ?? e.props?.widget_name ?? 'widget');
  if (widgetName === 'demo.capability_card') {
    return <CapabilityCard e={e} />;
  }
  const status = String(e.props?.status ?? 'unknown');
  const props = asRecord(e.props?.props);
  return (
    <div data-part="card" data-variant="widget">
      <div data-part="card-header">
        <div data-part="card-header-title">Widget</div>
        <div data-part="pill" data-variant="accent" data-mono="true">
          {widgetName}
        </div>
        <div data-part="pill" data-mono="true">
          {status}
        </div>
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        <pre data-part="mono" style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {JSON.stringify(props, null, 2)}
        </pre>
      </div>
    </div>
  );
}

export function GenericCard({ e }: { e: RenderEntity }) {
  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="card-header-title">{e.kind}</div>
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        <pre data-part="mono" style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {JSON.stringify(e.props ?? {}, null, 2)}
        </pre>
      </div>
    </div>
  );
}
