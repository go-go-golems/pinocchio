import type React from 'react';
import { useState } from 'react';
import type { ParsedBlock, SemEvent, TimelineEntity } from '../types';
import { formatDateTime } from '../ui/format/time';
import { getEventPresentation } from '../ui/presentation/events';
import { CorrelationIdBar } from './CorrelationIdBar';

export type ViewMode = 'semantic' | 'sem' | 'raw';

export interface CorrelatedNodes {
  block?: ParsedBlock;
  prevEvent?: SemEvent;
  nextEvent?: SemEvent;
  entity?: TimelineEntity;
}

export interface TrustCheck {
  name: string;
  passed: boolean;
  message?: string;
}

export interface EventInspectorProps {
  event: SemEvent;
  correlatedNodes?: CorrelatedNodes;
  trustChecks?: TrustCheck[];
  onBlockClick?: (block: ParsedBlock) => void;
  onEventClick?: (event: SemEvent) => void;
  onEntityClick?: (entity: TimelineEntity) => void;
}

export function EventInspector({
  event,
  correlatedNodes,
  trustChecks,
  onBlockClick,
  onEventClick,
  onEntityClick,
}: EventInspectorProps) {
  const [viewMode, setViewMode] = useState<ViewMode>('semantic');

  // Extract correlation IDs from event data
  const sessionId = (event.data as Record<string, unknown>)?.session_id as string | undefined;
  const inferenceId = (event.data as Record<string, unknown>)?.inference_id as string | undefined;
  const turnId = (event.data as Record<string, unknown>)?.turn_id as string | undefined;

  return (
    <div className="event-inspector">
      {/* Correlation IDs */}
      <CorrelationIdBar
        sessionId={sessionId}
        inferenceId={inferenceId}
        turnId={turnId}
        seq={event.seq}
        streamId={event.stream_id}
      />

      {/* View mode tabs */}
      <ViewModeTabs activeMode={viewMode} onModeChange={setViewMode} />

      {/* Content based on view mode */}
      <div className="event-content">
        {viewMode === 'semantic' && <SemanticView event={event} />}
        {viewMode === 'sem' && <SemEnvelopeView event={event} />}
        {viewMode === 'raw' && <RawWireView event={event} />}
      </div>

      {/* Correlated nodes panel */}
      {correlatedNodes && (
        <CorrelatedNodesPanel
          nodes={correlatedNodes}
          onBlockClick={onBlockClick}
          onEventClick={onEventClick}
          onEntityClick={onEntityClick}
        />
      )}

      {/* Trust signals */}
      {trustChecks && trustChecks.length > 0 && (
        <TrustSignals checks={trustChecks} />
      )}
    </div>
  );
}

interface ViewModeTabsProps {
  activeMode: ViewMode;
  onModeChange: (mode: ViewMode) => void;
}

function ViewModeTabs({ activeMode, onModeChange }: ViewModeTabsProps) {
  const modes: { mode: ViewMode; label: string; icon: string }[] = [
    { mode: 'semantic', label: 'Semantic', icon: '📖' },
    { mode: 'sem', label: 'SEM Frame', icon: '{ }' },
    { mode: 'raw', label: 'Raw Wire', icon: '⚡' },
  ];

  return (
    <div className="view-mode-tabs">
      {modes.map(({ mode, label, icon }) => (
        <button
          key={mode}
          className={`view-mode-tab ${activeMode === mode ? 'active' : ''}`}
          onClick={() => onModeChange(mode)}
        >
          <span className="tab-icon">{icon}</span>
          <span className="tab-label">{label}</span>
        </button>
      ))}
    </div>
  );
}

interface SemanticViewProps {
  event: SemEvent;
}

function SemanticView({ event }: SemanticViewProps) {
  const { type, id, received_at, data } = event;
  const time = formatDateTime(received_at);
  const typeInfo = getEventPresentation(type, {
    thinkingIcon: 'bot',
    unknownColor: 'var(--text-muted)',
  });

  return (
    <div className="semantic-view">
      {/* Event header */}
      <div className="semantic-header">
        <span className="event-icon">{typeInfo.icon}</span>
        <span className="event-type" style={{ color: typeInfo.color }}>{type}</span>
      </div>

      {/* Event ID and time */}
      <div className="semantic-meta">
        <div className="meta-row">
          <span className="meta-label">ID:</span>
          <span className="meta-value mono">{id}</span>
        </div>
        <div className="meta-row">
          <span className="meta-label">Received:</span>
          <span className="meta-value">{time}</span>
        </div>
      </div>

      {/* Human-readable content */}
      <div className="semantic-content">
        {renderSemanticContent(type, data)}
      </div>
    </div>
  );
}

function renderSemanticContent(type: string, data: unknown): React.ReactNode {
  const d = data as Record<string, unknown>;

  switch (type) {
    case 'llm.start':
      return (
        <div>
          <p>Started generating response as <strong>{String(d.role)}</strong></p>
        </div>
      );
    case 'llm.delta':
      return (
        <div>
          <p className="text-muted" style={{ marginBottom: '8px' }}>Streaming content:</p>
          <div style={{ whiteSpace: 'pre-wrap', fontSize: '14px' }}>
            {String(d.cumulative || '')}
          </div>
        </div>
      );
    case 'llm.final':
      return (
        <div>
          <p className="text-muted" style={{ marginBottom: '8px' }}>Final response:</p>
          <div style={{ whiteSpace: 'pre-wrap', fontSize: '14px' }}>
            {String(d.text || '')}
          </div>
        </div>
      );
    case 'tool.start':
      return (
        <div>
          <p>Calling tool: <strong style={{ color: 'var(--accent-yellow)' }}>{String(d.name)}</strong></p>
          {Boolean(d.input) && (
            <pre style={{ marginTop: '8px', fontSize: '12px' }}>
              {String(typeof d.input === 'string' ? d.input : JSON.stringify(d.input, null, 2))}
            </pre>
          )}
        </div>
      );
    case 'tool.result':
      return (
        <div>
          <p className="text-muted" style={{ marginBottom: '8px' }}>Tool result:</p>
          <pre style={{ fontSize: '12px' }}>
            {typeof d.result === 'string' ? d.result : JSON.stringify(d.result, null, 2)}
          </pre>
        </div>
      );
    default:
      return (
        <pre style={{ fontSize: '12px' }}>
          {JSON.stringify(d, null, 2)}
        </pre>
      );
  }
}

interface SemEnvelopeViewProps {
  event: SemEvent;
}

function SemEnvelopeView({ event }: SemEnvelopeViewProps) {
  return (
    <div className="sem-envelope-view">
      <h4>SEM Frame Envelope</h4>
      <pre className="json-view">
        {JSON.stringify(event, null, 2)}
      </pre>
    </div>
  );
}

interface RawWireViewProps {
  event: SemEvent;
}

function RawWireView({ event }: RawWireViewProps) {
  // In a real implementation, this would show the provider-native format
  const rawData = (event.data as Record<string, unknown>)?.raw_wire;

  return (
    <div className="raw-wire-view">
      <h4>Raw Wire Format (Provider-Native)</h4>
      <pre className="json-view">
        {rawData 
          ? JSON.stringify(rawData, null, 2)
          : '// Raw wire data not available for this event\n// This would show the original provider response format'}
      </pre>
    </div>
  );
}

interface CorrelatedNodesPanelProps {
  nodes: CorrelatedNodes;
  onBlockClick?: (block: ParsedBlock) => void;
  onEventClick?: (event: SemEvent) => void;
  onEntityClick?: (entity: TimelineEntity) => void;
}

function CorrelatedNodesPanel({ nodes, onBlockClick, onEventClick, onEntityClick }: CorrelatedNodesPanelProps) {
  const { block, prevEvent, nextEvent, entity } = nodes;

  return (
    <div className="correlated-nodes-panel">
      <h4>Correlated Nodes</h4>

      <div className="nodes-grid">
        {block && (
          <div className="node-link" onClick={() => onBlockClick?.(block)}>
            <span className="node-icon">📦</span>
            <span className="node-label">Linked Block</span>
            <span className="node-id">#{block.index} {block.kind}</span>
          </div>
        )}

        {prevEvent && (
          <div className="node-link" onClick={() => onEventClick?.(prevEvent)}>
            <span className="node-icon">⬅️</span>
            <span className="node-label">Previous Event</span>
            <span className="node-id">{prevEvent.type}</span>
          </div>
        )}

        {nextEvent && (
          <div className="node-link" onClick={() => onEventClick?.(nextEvent)}>
            <span className="node-icon">➡️</span>
            <span className="node-label">Next Event</span>
            <span className="node-id">{nextEvent.type}</span>
          </div>
        )}

        {entity && (
          <div className="node-link" onClick={() => onEntityClick?.(entity)}>
            <span className="node-icon">🎯</span>
            <span className="node-label">Timeline Entity</span>
            <span className="node-id">{entity.kind}</span>
          </div>
        )}
      </div>
    </div>
  );
}

interface TrustSignalsProps {
  checks: TrustCheck[];
}

function TrustSignals({ checks }: TrustSignalsProps) {
  const passed = checks.filter(c => c.passed).length;
  const total = checks.length;

  return (
    <div className="trust-signals">
      <div className="trust-header">
        <h4>Trust Signals</h4>
        <span className={`trust-score ${passed === total ? 'all-pass' : 'has-fail'}`}>
          {passed}/{total} passed
        </span>
      </div>

      <div className="trust-checks">
        {checks.map((check, idx) => (
          <div key={idx} className={`trust-check ${check.passed ? 'passed' : 'failed'}`}>
            <span className="check-icon">{check.passed ? '✓' : '✗'}</span>
            <span className="check-name">{check.name}</span>
            {check.message && <span className="check-message">{check.message}</span>}
          </div>
        ))}
      </div>
    </div>
  );
}

export default EventInspector;
