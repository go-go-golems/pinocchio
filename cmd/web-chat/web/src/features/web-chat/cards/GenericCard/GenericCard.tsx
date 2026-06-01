import { fmtSentAt } from '../../../../webchat/utils';
import type { GenericCardProps } from './types';

export function GenericCard({ e }: GenericCardProps) {
  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="card-header-title">{e.kind}</div>
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        <pre data-part="mono">
          {JSON.stringify(e.props ?? {}, null, 2)}
        </pre>
      </div>
    </div>
  );
}
