import { logWarn } from '../utils/logger';
import { Markdown } from './Markdown';
import type { RenderEntity } from './types';
import { fmtSentAt } from './utils';

export function MessageCard({ e }: { e: RenderEntity }) {
  const role = String(e.props?.role ?? 'assistant');
  const content = String(e.props?.content ?? '');
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
        {content ? <Markdown text={content} /> : <div data-part="pill">...</div>}
      </div>
    </div>
  );
}

export function ToolCallCard({ e }: { e: RenderEntity }) {
  const name = String(e.props?.name ?? 'tool');
  const input = e.props?.input ?? {};
  const done = !!e.props?.done;
  const title = done ? `${name} (done)` : name;
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

export function ThinkingModeCard({ e }: { e: RenderEntity }) {
  const mode = String(e.props?.mode ?? '');
  const phase = String(e.props?.phase ?? '');
  const status = String(e.props?.status ?? '');
  const success = e.props?.success;
  const error = e.props?.error ? String(e.props.error) : '';
  const reasoning = e.props?.reasoning ? String(e.props.reasoning) : '';
  const header = mode ? `Thinking mode: ${mode}` : 'Thinking mode';

  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="card-header-title">{header}</div>
        {phase ? (
          <div data-part="pill" data-mono="true">
            {phase}
          </div>
        ) : null}
        {status ? <div data-part="pill">{status}</div> : null}
        {typeof success === 'boolean' ? (
          <div data-part="pill" data-variant={success ? 'ok' : 'error'}>
            {success ? 'ok' : 'fail'}
          </div>
        ) : null}
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        {reasoning ? <Markdown text={reasoning} /> : <div data-part="pill">No reasoning</div>}
        {error ? (
          <div data-part="status-text" data-variant="error" style={{ marginTop: 10 }}>
            {error}
          </div>
        ) : null}
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
