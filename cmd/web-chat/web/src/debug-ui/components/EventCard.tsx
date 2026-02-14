import type React from 'react';
import { useState } from 'react';
import type { SemEvent } from '../types';
import { safeStringify, truncateText } from '../ui/format/text';
import { formatTimeShort } from '../ui/format/time';
import { getEventPresentation } from '../ui/presentation/events';

export interface EventCardProps {
  event: SemEvent;
  selected?: boolean;
  onClick?: () => void;
  compact?: boolean;
}

export function EventCard({ event, selected = false, onClick, compact = false }: EventCardProps) {
  const [showRaw, setShowRaw] = useState(false);
  const { type, id, seq, stream_id, data, received_at } = event;

  const typeInfo = getEventPresentation(type);
  const time = formatTimeShort(received_at);

  return (
    <div
      className={`card ${selected ? 'selected' : ''}`}
      onClick={onClick}
      style={{ 
        cursor: onClick ? 'pointer' : 'default',
        padding: compact ? '8px' : '12px',
        borderLeft: `3px solid ${typeInfo.color}`,
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2">
          <span className={`badge ${typeInfo.badgeClass}`}>
            {typeInfo.icon} {type}
          </span>
          {stream_id && (
            <span className="text-xs text-muted">{stream_id}</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted">{time}</span>
          {!compact && (
            <button
              className="btn btn-ghost text-xs"
              onClick={(e) => {
                e.stopPropagation();
                setShowRaw(!showRaw);
              }}
              style={{ padding: '2px 6px' }}
            >
              {showRaw ? '📖' : '{ }'}
            </button>
          )}
        </div>
      </div>

      {/* ID and Seq */}
      <div className="flex items-center gap-3 text-xs text-secondary mb-1">
        <span style={{ fontFamily: 'monospace' }}>id: {truncateId(id)}</span>
        <span style={{ fontFamily: 'monospace' }}>seq: {formatSeq(seq)}</span>
      </div>

      {/* Content */}
      {!compact && (
        showRaw ? (
          <pre style={{ fontSize: '11px', marginTop: '8px', maxHeight: '150px', overflow: 'auto' }}>
            {safeStringify(data, 2)}
          </pre>
        ) : (
          <div className="mt-2">
            {renderEventContent(type, data)}
          </div>
        )
      )}
    </div>
  );
}

function truncateId(id: string): string {
  if (id.length <= 12) return id;
  return id.slice(0, 12) + '...';
}

function formatSeq(seq: number): string {
  // Format large sequence numbers more readably
  const str = String(seq);
  if (str.length > 10) {
    return str.slice(0, 4) + '...' + str.slice(-4);
  }
  return str;
}

function renderEventContent(type: string, data: unknown): React.ReactNode {
  if (!data || typeof data !== 'object') return null;
  
  const d = data as Record<string, unknown>;

  switch (type) {
    case 'llm.start':
      return <span className="text-sm text-secondary">Role: {d.role as string}</span>;
    case 'llm.delta':
      return (
        <div className="text-sm" style={{ maxHeight: '60px', overflow: 'hidden' }}>
          {truncateText(d.cumulative as string, 100)}
        </div>
      );
    case 'llm.final':
      return (
        <div className="text-sm" style={{ maxHeight: '60px', overflow: 'hidden' }}>
          {truncateText(d.text as string, 100)}
        </div>
      );
    case 'tool.start':
      return (
        <div className="text-sm">
          <span className="badge badge-yellow">{d.name as string}</span>
        </div>
      );
    case 'tool.result':
      return (
        <pre className="text-xs" style={{ maxHeight: '60px', overflow: 'hidden' }}>
          {truncateText(safeStringify(d.result), 80)}
        </pre>
      );
    default:
      return null;
  }
}

export default EventCard;
