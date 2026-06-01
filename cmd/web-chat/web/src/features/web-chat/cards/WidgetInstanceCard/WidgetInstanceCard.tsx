import { fmtSentAt } from '../../../../webchat/utils';
import { asRecord } from '../utils';
import type { WidgetInstanceCardProps } from './types';

export function WidgetInstanceCard({ e }: WidgetInstanceCardProps) {
  const widgetName = String(e.props?.widgetName ?? e.props?.widget_name ?? 'widget');
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
        <pre data-part="mono">
          {JSON.stringify(props, null, 2)}
        </pre>
      </div>
    </div>
  );
}
