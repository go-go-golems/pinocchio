import React, { useEffect, useRef } from 'react';
import type { SemEvent, TimelineEntity, TurnSnapshot } from '../types';
import { EventTrackLane } from './EventTrackLane';
import { NowMarker } from './NowMarker';
import { ProjectionLane } from './ProjectionLane';
import { StateTrackLane } from './StateTrackLane';

export interface TimelineLanesProps {
  turns: TurnSnapshot[];
  events: SemEvent[];
  entities: TimelineEntity[];
  isLive?: boolean;
  onTurnSelect?: (turn: TurnSnapshot) => void;
  onEventSelect?: (event: SemEvent) => void;
  onEntitySelect?: (entity: TimelineEntity) => void;
  selectedTurnId?: string;
  selectedEventSeq?: number;
  selectedEntityId?: string;
}

export function TimelineLanes({
  turns,
  events,
  entities,
  isLive = false,
  onTurnSelect,
  onEventSelect,
  onEntitySelect,
  selectedTurnId,
  selectedEventSeq,
  selectedEntityId,
}: TimelineLanesProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const stateRef = useRef<HTMLDivElement>(null);
  const eventRef = useRef<HTMLDivElement>(null);
  const projRef = useRef<HTMLDivElement>(null);

  // Sync scroll across lanes
  useEffect(() => {
    const handleScroll = (source: HTMLDivElement | null) => {
      if (!source) return;
      const scrollTop = source.scrollTop;
      
      [stateRef, eventRef, projRef].forEach(ref => {
        if (ref.current && ref.current !== source) {
          ref.current.scrollTop = scrollTop;
        }
      });
    };

    const refs = [stateRef, eventRef, projRef];
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
    <div className="timeline-lanes" ref={containerRef}>
      {/* Header row */}
      <div className="timeline-lanes-header">
        <div className="timeline-lane-header">
          <span className="timeline-lane-title">📋 State Track</span>
          <span className="timeline-lane-count">{turns.length} turns</span>
        </div>
        <div className="timeline-lane-header">
          <span className="timeline-lane-title">⚡ Events</span>
          <span className="timeline-lane-count">{events.length} events</span>
        </div>
        <div className="timeline-lane-header">
          <span className="timeline-lane-title">🎯 Projection</span>
          <span className="timeline-lane-count">{entities.length} entities</span>
        </div>
      </div>

      {/* Lane columns */}
      <div className="timeline-lanes-body">
        <div className="timeline-lane timeline-lane-state" ref={stateRef}>
          <StateTrackLane
            turns={turns}
            selectedTurnId={selectedTurnId}
            onTurnSelect={onTurnSelect}
          />
          {isLive && <NowMarker />}
        </div>

        <div className="timeline-lane timeline-lane-event" ref={eventRef}>
          <EventTrackLane
            events={events}
            selectedSeq={selectedEventSeq}
            onEventSelect={onEventSelect}
          />
          {isLive && <NowMarker />}
        </div>

        <div className="timeline-lane timeline-lane-projection" ref={projRef}>
          <ProjectionLane
            entities={entities}
            selectedEntityId={selectedEntityId}
            onEntitySelect={onEntitySelect}
          />
          {isLive && <NowMarker />}
        </div>
      </div>
    </div>
  );
}

export default TimelineLanes;
