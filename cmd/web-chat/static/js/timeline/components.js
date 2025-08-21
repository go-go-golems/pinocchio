// Preact components for timeline entities

import { h } from 'https://esm.sh/preact@10.22.0';
import htm from 'https://esm.sh/htm@3.1.1';

const html = htm.bind(h);

export function LLMText({ entity }) {
  const role = (entity.props && entity.props.role) || 'assistant';
  const text = (entity.props && entity.props.text) || '';
  const streaming = !!(entity.props && entity.props.streaming);
  const metadata = entity.props && entity.props.metadata;
  return html`
    <div class=${`msg ${role === 'user' ? 'user' : 'assistant'}`}>
      <span class="role-label">(${role}):</span>
      <div class="text-content">${text}</div>
      ${streaming || metadata ? html`<div class="status-line">
        ${streaming ? html`<span class="spinner">Generating...</span>` : null}
        ${metadata ? html`<span class="metadata">${formatMetadata(metadata)}</span>` : null}
      </div>` : null}
    </div>
  `;
}

export function ToolCall({ entity }) {
  const name = entity.props && entity.props.name;
  const input = entity.props && entity.props.input;
  const exec = !!(entity.props && entity.props.exec);
  return html`
    <div class="tool-call">
      <div class="tool-header">
        <span class="tool-icon">🔧</span>
        <span class="tool-name">${name || 'Unknown Tool'}</span>
        ${exec ? html`<span class="exec-indicator">executing...</span>` : null}
      </div>
      ${input !== undefined && input !== null ? html`<div class="tool-input">${typeof input === 'string' ? input : formatJSON(input)}</div>` : null}
    </div>
  `;
}

export function ToolResult({ entity }) {
  const result = entity.props && entity.props.result;
  return html`
    <div class="tool-result">
      <div class="result-header">
        <span class="result-icon">📋</span>
        <span> Result:</span>
      </div>
      <pre class="result-content">${typeof result === 'string' ? result : formatJSON(result)}</pre>
    </div>
  `;
}

export function AgentMode({ entity }) {
  const title = (entity.props && entity.props.title) || 'Agent Mode';
  const from = entity.props && entity.props.from;
  const to = entity.props && entity.props.to;
  const analysis = entity.props && entity.props.analysis;
  let header = title;
  const fromStr = (from || '').trim();
  const toStr = (to || '').trim();
  if (fromStr || toStr) header = `${title} — ${fromStr} → ${toStr}`;
  return html`
    <div class="agent-mode">
      <div class="agent-mode-header"><span class="mode-icon">🤖</span><span> ${header}</span></div>
      ${analysis ? html`<details><summary>Analysis</summary><div class="analysis-content">${analysis}</div></details>` : null}
    </div>
  `;
}

export function LogEvent({ entity }) {
  const level = (entity.props && entity.props.level) || 'info';
  const message = entity.props && entity.props.message;
  const fields = entity.props && entity.props.fields;
  return html`
    <div class=${`log-event log-${level}`}>
      <div class="log-content"><span class="log-level">[${String(level).toUpperCase()}]</span><span class="log-message"> ${message}</span></div>
      ${fields && Object.keys(fields).length > 0 ? html`<details><summary>Fields</summary><pre class="log-fields">${formatJSON(fields)}</pre></details>` : null}
    </div>
  `;
}

export function Timeline({ entities }) {
  return html`<div>
    ${entities.map((e) => html`${renderEntity(e)}`)}
  </div>`;
}

export function renderEntity(entity) {
  switch (entity.kind) {
    case 'llm_text':
      return html`<${LLMText} entity=${entity} />`;
    case 'tool_call':
      return html`<${ToolCall} entity=${entity} />`;
    case 'tool_call_result':
      return html`<${ToolResult} entity=${entity} />`;
    case 'agent_mode':
      return html`<${AgentMode} entity=${entity} />`;
    case 'log_event':
      return html`<${LogEvent} entity=${entity} />`;
    default:
      return html`<div class="timeline-entity timeline-unknown">
        <div class="unknown-header">Unknown entity type: ${entity.kind}</div>
        <pre class="unknown-props">${formatJSON(entity.props)}</pre>
      </div>`;
  }
}

function formatJSON(obj) {
  try { return JSON.stringify(obj, null, 2); } catch(e) { return String(obj); }
}

function formatMetadata(metadata) {
  if (!metadata) return '';
  const parts = [];
  if (metadata.model) parts.push(metadata.model);
  if (metadata.usage) {
    const { input_tokens, output_tokens } = metadata.usage;
    if (input_tokens || output_tokens) parts.push(`in: ${input_tokens || 0} out: ${output_tokens || 0}`);
  }
  if (metadata.duration_ms) parts.push(`${metadata.duration_ms}ms`);
  return parts.join(' ');
}


