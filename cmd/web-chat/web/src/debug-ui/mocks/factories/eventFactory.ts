import type { MwTrace, SemEvent } from '../../types';
import { mockEvents, mockMwTrace } from '../fixtures/events';
import { pickByIndex } from './common';
import {
  makeDeterministicId,
  makeDeterministicIsoTime,
  makeDeterministicSeq,
  shouldApplyDeterministicOverrides,
} from './deterministic';

const EVENT_SEQ_BASE = mockEvents[0]?.seq ?? 1707053365100000000;
const EVENT_RECEIVED_AT_BASE_MS = Date.parse(mockEvents[0]?.received_at ?? '2026-02-06T14:32:08.100Z');

function eventIdPrefix(eventType: string): string {
  if (eventType.startsWith('tool.')) {
    return 'tc';
  }

  if (eventType.startsWith('llm.')) {
    return 'msg';
  }

  return 'evt';
}

export function makeEvent(options: {
  index?: number;
  overrides?: Partial<SemEvent>;
} = {}): SemEvent {
  const { index = 0, overrides = {} } = options;
  return {
    ...pickByIndex(mockEvents, index),
    ...overrides,
  };
}

export function makeEvents(
  count: number,
  options: {
    startIndex?: number;
    mapOverrides?: (listIndex: number) => Partial<SemEvent>;
  } = {}
): SemEvent[] {
  const { startIndex = 0, mapOverrides } = options;
  return Array.from({ length: count }, (_, listIndex) => {
    const absoluteIndex = startIndex + listIndex;
    const baseEvent = makeEvent({ index: absoluteIndex });
    const synthetic = shouldApplyDeterministicOverrides(absoluteIndex, mockEvents.length);

    const deterministicOverrides: Partial<SemEvent> = synthetic
      ? {
        id: makeDeterministicId(eventIdPrefix(baseEvent.type), absoluteIndex, 3),
        seq: makeDeterministicSeq(EVENT_SEQ_BASE, absoluteIndex, 1_000_000),
        received_at: makeDeterministicIsoTime(EVENT_RECEIVED_AT_BASE_MS, absoluteIndex, 800),
      }
      : {};

    return {
      ...baseEvent,
      ...deterministicOverrides,
      ...(mapOverrides?.(listIndex) ?? {}),
    };
  });
}

export function makeMwTrace(options: {
  overrides?: Partial<MwTrace>;
} = {}): MwTrace {
  const { overrides = {} } = options;
  return {
    ...pickByIndex([mockMwTrace], 0),
    ...overrides,
  };
}
