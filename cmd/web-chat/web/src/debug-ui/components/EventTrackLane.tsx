import type { DebugEvent } from '../store/debugSlice';

export interface EventTrackLaneProps {
  events: DebugEvent[];
}

export function EventTrackLane({ events }: EventTrackLaneProps) {
  if (events.length === 0) {
    return <div className="lane-empty">No events yet.</div>;
  }

  return (
    <div className="event-track-lane">
      {[...events].reverse().map((event, i) => (
        <div key={i} className="event-track-item">
          <div className="event-track-item-header">
            <span className="event-name">{event.name}</span>
            <span className="event-ordinal">#{event.ordinal}</span>
          </div>
        </div>
      ))}
    </div>
  );
}

export default EventTrackLane;
