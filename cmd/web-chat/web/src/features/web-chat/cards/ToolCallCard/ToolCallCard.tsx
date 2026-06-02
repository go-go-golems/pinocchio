import { logWarn } from '../../../../utils/logger';
import { submitFrontendToolResult } from '../../../../ws/frontendTools';
import { fmtSentAt } from '../../format';
import { asRecord } from '../utils';
import type { ToolCallCardProps } from './types';

export function ToolCallCard({ e }: ToolCallCardProps) {
  const name = String(e.props?.name ?? e.props?.toolName ?? 'tool');
  const input = e.props?.input ?? {};
  const result = e.props?.result;
  const status = String(e.props?.status ?? '');
  const sessionId = String(e.props?.sessionId ?? '');
  const toolCallId = String(e.props?.toolCallId ?? e.id ?? '');
  const done = !!e.props?.done || !!result || status === 'success' || status === 'denied' || status === 'failed';
  const inputRecord = asRecord(input);
  const isHumanConfirm = !done && !!sessionId && !!toolCallId && (typeof inputRecord.title === 'string' || typeof inputRecord.confirmLabel === 'string' || typeof inputRecord.cancelLabel === 'string');
  const title = done ? `${name} (done)` : name;
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
          <div data-part="callout" data-variant="warning" data-spacing="bottom">
            <strong>{confirmTitle}</strong>
            <div data-part="callout-body">{confirmBody}</div>
            <div data-part="toolbar" data-spacing="top">
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
          <pre data-part="mono" data-spacing="bottom">
            {JSON.stringify(result, null, 2)}
          </pre>
        ) : null}
        <pre data-part="mono">
          {JSON.stringify(input ?? {}, null, 2)}
        </pre>
      </div>
    </div>
  );
}
