import { normalizeAgentModeAnalysis } from '../../../../webchat/agentModeMarkdown';
import { fmtSentAt } from '../../../../webchat/utils';
import { Markdown } from '../Markdown';
import { asRecord } from '../utils';
import type { AgentModeCardProps } from './types';

export function AgentModeCard({ e }: AgentModeCardProps) {
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
          <pre data-part="mono" data-spacing="top">
            {JSON.stringify(extraData, null, 2)}
          </pre>
        ) : null}
      </div>
    </div>
  );
}
