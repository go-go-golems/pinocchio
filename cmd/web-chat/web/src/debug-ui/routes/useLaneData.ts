import type { DebugEntity, DebugEvent } from '../store/debugSlice';
import { useAppSelector } from '../store/hooks';

export interface LaneData {
  entities: DebugEntity[];
  events: DebugEvent[];
  isLoading: boolean;
}

export function useLaneData(): LaneData {
  const entityMap = useAppSelector((state) => state.debug.entities);
  const events = useAppSelector((state) => state.debug.events);
  const connected = useAppSelector((state) => state.ui.follow.status === 'connected');

  return {
    entities: Object.values(entityMap),
    events,
    isLoading: !connected && Object.keys(entityMap).length === 0,
  };
}
