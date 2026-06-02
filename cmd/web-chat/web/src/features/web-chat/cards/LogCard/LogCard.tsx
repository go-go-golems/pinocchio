import { fmtSentAt } from '../../format';
import type { LogCardProps } from './types';

export function LogCard({ e }: LogCardProps) {
  const level = String(e.props?.level ?? 'info');
  const message = String(e.props?.message ?? '');
  return (
    <div data-part="card" data-variant="log">
      <div data-part="card-body">
        <div data-part="row" data-align="spread">
          <div data-part="pill" data-mono="true">
            {level}
          </div>
          <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
        </div>
        <div data-part="log-message">{message}</div>
      </div>
    </div>
  );
}
