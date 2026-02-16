import type { TimelineLanesProps } from '../../components/TimelineLanes';
import {
  makeEvent,
  makeEvents,
  makeTimelineEntities,
  makeTimelineEntity,
  makeTurnSnapshot,
  makeTurnSnapshots,
} from '../factories';

type TimelineScenarioArgs = Pick<
  TimelineLanesProps,
  'turns' | 'events' | 'entities' | 'isLive' | 'selectedTurnId' | 'selectedEventSeq' | 'selectedEntityId'
>;

export interface TimelineScenario {
  name: string;
  args: TimelineScenarioArgs;
}

const fixedNow = 1707230000000;

export const timelineScenarios: Record<string, TimelineScenario> = {
  default: {
    name: 'default',
    args: {
      turns: makeTurnSnapshots(2),
      events: makeEvents(6),
      entities: makeTimelineEntities(4),
    },
  },
  withSelection: {
    name: 'withSelection',
    args: {
      turns: makeTurnSnapshots(2),
      events: makeEvents(6),
      entities: makeTimelineEntities(4),
      selectedTurnId: makeTurnSnapshot({ index: 0 }).turn_id,
      selectedEventSeq: makeEvent({ index: 0 }).seq,
      selectedEntityId: makeTimelineEntity({ index: 0 }).id,
    },
  },
  live: {
    name: 'live',
    args: {
      turns: makeTurnSnapshots(2),
      events: makeEvents(6),
      entities: makeTimelineEntities(4),
      isLive: true,
    },
  },
  turnsOnly: {
    name: 'turnsOnly',
    args: {
      turns: makeTurnSnapshots(2),
      events: [],
      entities: [],
    },
  },
  eventsOnly: {
    name: 'eventsOnly',
    args: {
      turns: [],
      events: makeEvents(6),
      entities: [],
    },
  },
  empty: {
    name: 'empty',
    args: {
      turns: [],
      events: [],
      entities: [],
    },
  },
  manyItems: {
    name: 'manyItems',
    args: {
      turns: [
        ...makeTurnSnapshots(2),
        ...makeTurnSnapshots(4, {
          mapOverrides: (i) => ({
            turn_id: `turn_extra_${i + 1}`,
            phase: i % 2 === 0 ? 'pre_inference' : 'final',
          }),
        }),
      ],
      events: [
        ...makeEvents(6),
        ...makeEvents(6, {
          mapOverrides: (i) => ({
            seq: 1707053365700000000 + i,
            id: `evt_extra_${i + 1}`,
          }),
        }),
      ],
      entities: [
        ...makeTimelineEntities(4),
        ...makeTimelineEntities(3, {
          mapOverrides: (i) => ({
            id: `entity_extra_${i + 1}`,
            created_at: fixedNow + i * 1000,
          }),
        }),
      ],
    },
  },
};

export function makeTimelineScenario(name: keyof typeof timelineScenarios): TimelineScenario {
  return timelineScenarios[name];
}
