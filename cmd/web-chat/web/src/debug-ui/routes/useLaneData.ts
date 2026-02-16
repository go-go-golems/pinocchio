import {
  useGetEventsQuery,
  useGetTimelineQuery,
  useGetTurnsQuery,
} from '../api/debugApi';
import type { SemEvent, TimelineEntity, TurnSnapshot } from '../types';

export interface LiveLaneData {
  turns: TurnSnapshot[];
  events: SemEvent[];
  entities: TimelineEntity[];
  isLoading: boolean;
}

export function useLiveLaneData(convId: string | null, sessionId?: string): LiveLaneData {
  const skip = !convId;

  const { data: turns, isLoading: turnsLoading } = useGetTurnsQuery(
    { convId: convId ?? '', sessionId },
    { skip }
  );
  const { data: eventsData, isLoading: eventsLoading } = useGetEventsQuery(
    { convId: convId ?? '' },
    { skip }
  );
  const { data: timelineData, isLoading: timelineLoading } = useGetTimelineQuery(
    { convId: convId ?? '' },
    { skip }
  );

  return {
    turns: turns ?? [],
    events: eventsData?.events ?? [],
    entities: timelineData?.entities ?? [],
    isLoading: turnsLoading || eventsLoading || timelineLoading,
  };
}

