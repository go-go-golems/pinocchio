import { logWarn } from '../../../../utils/logger';
import { fmtSentAt } from '../../../../webchat/utils';
import type { ToolResultCardProps } from './types';

export function ToolResultCard({ e }: ToolResultCardProps) {
  const customKind = e.props?.customKind ? String(e.props.customKind) : '';
  const rawResult = e.props?.result;
  const result = typeof rawResult === 'string' ? rawResult : rawResult === undefined ? '' : JSON.stringify(rawResult, null, 2);
  const error = String(e.props?.error ?? '');
  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="card-header-title">Result</div>
        {customKind ? (
          <div data-part="pill" data-mono="true">
            {customKind}
          </div>
        ) : null}
        {error ? (
          <div data-part="pill" data-variant="danger">
            error
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
            onClick={() => void navigator.clipboard.writeText(result || error).catch((err) => logWarn('clipboard copy failed', { scope: 'tool.copyResult' }, err))}
          >
            Copy
          </button>
        </div>
        {error ? <div data-part="error-item-detail">{error}</div> : null}
        {result ? (
          <pre data-part="mono">
            {result}
          </pre>
        ) : error ? null : (
          <div data-part="pill">empty result</div>
        )}
      </div>
    </div>
  );
}
