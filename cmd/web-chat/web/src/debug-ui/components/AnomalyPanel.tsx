import React, { useState } from 'react';

export type AnomalyType = 'orphan_event' | 'missing_correlation' | 'timing_outlier' | 'sequence_gap' | 'schema_error';
export type AnomalySeverity = 'error' | 'warning' | 'info';

export interface Anomaly {
  id: string;
  type: AnomalyType;
  severity: AnomalySeverity;
  message: string;
  details?: string;
  timestamp: string;
  relatedIds?: {
    eventId?: string;
    turnId?: string;
    blockIndex?: number;
  };
}

export interface AnomalyPanelProps {
  anomalies: Anomaly[];
  isOpen: boolean;
  onClose: () => void;
  onAnomalyClick?: (anomaly: Anomaly) => void;
}

export function AnomalyPanel({ anomalies, isOpen, onClose, onAnomalyClick }: AnomalyPanelProps) {
  const [selectedAnomaly, setSelectedAnomaly] = useState<Anomaly | null>(null);
  const [filterSeverity, setFilterSeverity] = useState<AnomalySeverity | 'all'>('all');

  const filteredAnomalies = filterSeverity === 'all'
    ? anomalies
    : anomalies.filter(a => a.severity === filterSeverity);

  const counts = {
    error: anomalies.filter(a => a.severity === 'error').length,
    warning: anomalies.filter(a => a.severity === 'warning').length,
    info: anomalies.filter(a => a.severity === 'info').length,
  };

  const handleAnomalyClick = (anomaly: Anomaly) => {
    setSelectedAnomaly(anomaly);
    onAnomalyClick?.(anomaly);
  };

  if (!isOpen) return null;

  return (
    <div className="anomaly-panel-overlay">
      <div className="anomaly-panel">
        {/* Header */}
        <div className="anomaly-header">
          <div className="anomaly-title">
            <h3>⚠️ Anomalies</h3>
            <span className="anomaly-total">{anomalies.length} detected</span>
          </div>
          <button className="btn btn-ghost" onClick={onClose}>✕</button>
        </div>

        {/* Severity filter */}
        <div className="severity-filter">
          <button
            className={`severity-btn ${filterSeverity === 'all' ? 'active' : ''}`}
            onClick={() => setFilterSeverity('all')}
          >
            All ({anomalies.length})
          </button>
          <button
            className={`severity-btn severity-error ${filterSeverity === 'error' ? 'active' : ''}`}
            onClick={() => setFilterSeverity('error')}
          >
            🔴 Errors ({counts.error})
          </button>
          <button
            className={`severity-btn severity-warning ${filterSeverity === 'warning' ? 'active' : ''}`}
            onClick={() => setFilterSeverity('warning')}
          >
            🟡 Warnings ({counts.warning})
          </button>
          <button
            className={`severity-btn severity-info ${filterSeverity === 'info' ? 'active' : ''}`}
            onClick={() => setFilterSeverity('info')}
          >
            🔵 Info ({counts.info})
          </button>
        </div>

        {/* Anomaly list */}
        <div className="anomaly-list">
          {filteredAnomalies.length === 0 ? (
            <div className="anomaly-empty-state">
              No anomalies in this category
            </div>
          ) : (
            filteredAnomalies.map(anomaly => (
              <AnomalyCard
                key={anomaly.id}
                anomaly={anomaly}
                selected={selectedAnomaly?.id === anomaly.id}
                onClick={() => handleAnomalyClick(anomaly)}
              />
            ))
          )}
        </div>

        {/* Detail view */}
        {selectedAnomaly && (
          <AnomalyDetail
            anomaly={selectedAnomaly}
            onClose={() => setSelectedAnomaly(null)}
          />
        )}
      </div>
    </div>
  );
}

interface AnomalyCardProps {
  anomaly: Anomaly;
  selected: boolean;
  onClick: () => void;
}

function AnomalyCard({ anomaly, selected, onClick }: AnomalyCardProps) {
  const { type, severity, message, timestamp } = anomaly;
  const time = new Date(timestamp).toLocaleTimeString();

  return (
    <div
      className={`anomaly-card severity-${severity} ${selected ? 'selected' : ''}`}
      onClick={onClick}
    >
      <div className="anomaly-card-header">
        <span className="anomaly-type">{getTypeLabel(type)}</span>
        <span className="anomaly-time">{time}</span>
      </div>
      <div className="anomaly-message">{message}</div>
    </div>
  );
}

interface AnomalyDetailProps {
  anomaly: Anomaly;
  onClose: () => void;
}

function AnomalyDetail({ anomaly, onClose }: AnomalyDetailProps) {
  const { type, severity, message, details, timestamp, relatedIds } = anomaly;

  return (
    <div className="anomaly-detail">
      <div className="anomaly-detail-header">
        <h4>Anomaly Details</h4>
        <button className="btn btn-ghost" onClick={onClose}>✕</button>
      </div>

      <div className="anomaly-detail-content">
        <div className="anomaly-detail-row">
          <span className="anomaly-detail-label">Type:</span>
          <span className="anomaly-detail-value">{getTypeLabel(type)}</span>
        </div>
        <div className="anomaly-detail-row">
          <span className="anomaly-detail-label">Severity:</span>
          <span className={`severity-badge severity-${severity}`}>{severity}</span>
        </div>
        <div className="anomaly-detail-row">
          <span className="anomaly-detail-label">Time:</span>
          <span className="anomaly-detail-value">{new Date(timestamp).toLocaleString()}</span>
        </div>
        <div className="anomaly-detail-row">
          <span className="anomaly-detail-label">Message:</span>
          <span className="anomaly-detail-value">{message}</span>
        </div>
        {details && (
          <div className="anomaly-detail-row">
            <span className="anomaly-detail-label">Details:</span>
            <pre className="anomaly-detail-pre">{details}</pre>
          </div>
        )}
        {relatedIds && (
          <div className="anomaly-detail-row">
            <span className="anomaly-detail-label">Related:</span>
            <div className="anomaly-related-ids">
              {relatedIds.eventId && <span>Event: {relatedIds.eventId}</span>}
              {relatedIds.turnId && <span>Turn: {relatedIds.turnId}</span>}
              {relatedIds.blockIndex !== undefined && <span>Block: #{relatedIds.blockIndex}</span>}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function getTypeLabel(type: AnomalyType): string {
  switch (type) {
    case 'orphan_event': return 'Orphan Event';
    case 'missing_correlation': return 'Missing Correlation';
    case 'timing_outlier': return 'Timing Outlier';
    case 'sequence_gap': return 'Sequence Gap';
    case 'schema_error': return 'Schema Error';
    default: return type;
  }
}

export default AnomalyPanel;
