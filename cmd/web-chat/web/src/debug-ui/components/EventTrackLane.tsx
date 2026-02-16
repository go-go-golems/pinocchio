
import type { SemEvent } from '../types';
import { formatTimeShort } from '../ui/format/time';
import { getEventPresentation } from '../ui/presentation/events';

export interface EventTrackLaneProps {
  events: SemEvent[];
  selectedSeq?: number;
  onEventSelect?: (event: SemEvent) => void;
}

export function EventTrackLane({ events, selectedSeq, onEventSelect }: EventTrackLaneProps) {
  if (events.length === 0) {
    return (
      <div className="empty-lane">
        <span className="text-muted">No events</span>
      </div>
    );
  }

  return (
    <div className="event-track-lane">
      {events.map((event) => (
        <EventDot
          key={event.seq}
          event={event}
          selected={event.seq === selectedSeq}
          onClick={() => onEventSelect?.(event)}
        />
      ))}
    </div>
  );
}

interface EventDotProps {
  event: SemEvent;
  selected?: boolean;
  onClick?: () => void;
}

function EventDot({ event, selected, onClick }: EventDotProps) {
  const { type, id, stream_id, received_at } = event;
  const time = formatTimeShort(received_at);
  const typeInfo = getEventPresentation(type);

  return (
    <div
      className={`event-dot ${selected ? 'selected' : ''}`}
      onClick={onClick}
      style={{ borderLeftColor: typeInfo.color }}
    >
      <div className="event-dot-header">
        <span className="event-type" style={{ color: typeInfo.color }}>
          {typeInfo.icon} {type}
        </span>
        <span className="event-time">{time}</span>
      </div>

      <div className="event-dot-meta">
        <span className="event-id" title={id}>{id.slice(0, 12)}</span>
        {stream_id && <span className="event-stream">#{stream_id}</span>}
      </div>
    </div>
  );
}

export default EventTrackLane;
