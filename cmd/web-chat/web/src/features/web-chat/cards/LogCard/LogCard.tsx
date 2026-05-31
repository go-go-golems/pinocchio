import { fmtSentAt } from '../../../../webchat/utils';
import type { LogCardProps } from './types';

export function LogCard({ e }: LogCardProps) {
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
