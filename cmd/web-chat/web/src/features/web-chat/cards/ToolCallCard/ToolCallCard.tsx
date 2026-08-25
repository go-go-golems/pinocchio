import { logWarn } from '../../../../utils/logger';
import { isTerminalFrontendToolResultStatus } from '../../../../ws/frontendTools';
import { fmtSentAt } from '../../format';
import type { ToolCallCardProps } from './types';

export function ToolCallCard({ e }: ToolCallCardProps) {
  const name = String(e.props?.name ?? e.props?.toolName ?? 'tool');
  const input = e.props?.input ?? {};
  const result = e.props?.result;
  const status = String(e.props?.status ?? '');
  const done = !!e.props?.done || !!result || isTerminalFrontendToolResultStatus(status);
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
