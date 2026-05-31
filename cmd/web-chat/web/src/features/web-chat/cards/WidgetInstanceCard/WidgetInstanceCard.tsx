import { fmtSentAt } from '../../../../webchat/utils';
import { asRecord } from '../utils';
import type { CapabilityStep, WidgetInstanceCardProps } from './types';

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

export function CapabilityCard({ e }: WidgetInstanceCardProps) {
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

export function WidgetInstanceCard({ e }: WidgetInstanceCardProps) {
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
