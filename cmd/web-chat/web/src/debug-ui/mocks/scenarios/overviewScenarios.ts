import type { Anomaly } from '../../components/AnomalyPanel';
import type { ConversationSummary, SemEvent, TimelineEntity, TurnSnapshot } from '../../types';
import {
  makeAnomalies,
  makeConversations,
  makeEvents,
  makeTimelineEntities,
  makeTurnSnapshots,
} from '../factories';

export interface OverviewScenario {
  name: string;
  conversations: ConversationSummary[];
  turns: TurnSnapshot[];
  events: SemEvent[];
  entities: TimelineEntity[];
  anomalies: Anomaly[];
}

export const overviewScenarios: Record<string, OverviewScenario> = {
  default: {
    name: 'default',
    conversations: makeConversations(3),
    turns: makeTurnSnapshots(2),
    events: makeEvents(6),
    entities: makeTimelineEntities(4),
    anomalies: makeAnomalies(2),
  },
  empty: {
    name: 'empty',
    conversations: [],
    turns: [],
    events: [],
    entities: [],
    anomalies: [],
  },
  busy: {
    name: 'busy',
    conversations: makeConversations(8, {
      mapOverrides: (i) => ({
        id: `conv_busy_${i + 1}`,
        session_id: `sess_busy_${i + 1}`,
        is_running: i % 2 === 0,
      }),
    }),
    turns: makeTurnSnapshots(8, {
      mapOverrides: (i) => ({
        turn_id: `turn_busy_${i + 1}`,
      }),
    }),
    events: makeEvents(18, {
      mapOverrides: (i) => ({
        seq: 1707053365600000000 + i,
        id: `evt_busy_${i + 1}`,
      }),
    }),
    entities: makeTimelineEntities(10, {
      mapOverrides: (i) => ({
        id: `entity_busy_${i + 1}`,
      }),
    }),
    anomalies: makeAnomalies(8, {
      mapOverrides: (i) => ({
        id: `anom_busy_${i + 1}`,
      }),
    }),
  },
};

export function makeOverviewScenario(name: keyof typeof overviewScenarios): OverviewScenario {
  return overviewScenarios[name];
}
