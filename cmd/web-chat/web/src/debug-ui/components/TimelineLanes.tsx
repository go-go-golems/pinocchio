import { useEffect, useRef } from 'react';
import type { DebugEntity, DebugEvent } from '../store/debugSlice';
import { EventTrackLane } from './EventTrackLane';
import { NowMarker } from './NowMarker';
import { ProjectionLane } from './ProjectionLane';

export interface TimelineLanesProps {
  events: DebugEvent[];
  entities: DebugEntity[];
  isLive?: boolean;
}

export function TimelineLanes({
  events,
  entities,
  isLive = false,
}: TimelineLanesProps) {
  const eventRef = useRef<HTMLDivElement>(null);
  const projRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleScroll = (source: HTMLDivElement | null) => {
      if (!source) return;
      const scrollTop = source.scrollTop;
      if (eventRef.current && eventRef.current !== source) eventRef.current.scrollTop = scrollTop;
      if (projRef.current && projRef.current !== source) projRef.current.scrollTop = scrollTop;
    };

    const refs = [eventRef, projRef];
    const handlers = refs.map(ref => {
      const handler = () => handleScroll(ref.current);
      ref.current?.addEventListener('scroll', handler);
      return { ref, handler };
    });

    return () => {
      handlers.forEach(({ ref, handler }) => {
        ref.current?.removeEventListener('scroll', handler);
      });
    };
  }, []);

  return (
    <div className="timeline-lanes">
      <div className="timeline-lanes-header">
        <div className="timeline-lane-header">
          <span className="timeline-lane-title">⚡ UI Events</span>
          <span className="timeline-lane-count">{events.length} events</span>
        </div>
        <div className="timeline-lane-header">
          <span className="timeline-lane-title">🎯 Entities</span>
          <span className="timeline-lane-count">{entities.length} entities</span>
        </div>
      </div>

      <div className="timeline-lanes-body">
        <div className="timeline-lane timeline-lane-event" ref={eventRef}>
          <EventTrackLane events={events} />
          {isLive && <NowMarker />}
        </div>

        <div className="timeline-lane timeline-lane-projection" ref={projRef}>
          <ProjectionLane entities={entities} />
          {isLive && <NowMarker />}
        </div>
      </div>
    </div>
  );
}

export default TimelineLanes;
