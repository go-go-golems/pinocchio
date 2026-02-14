import type { TimelineEntity } from '../../types';
import { mockTimelineEntities } from '../fixtures/timeline';
import { pickByIndex } from './common';
import {
  makeDeterministicId,
  makeDeterministicTimeMs,
  shouldApplyDeterministicOverrides,
} from './deterministic';

const TIMELINE_CREATED_AT_BASE_MS = mockTimelineEntities[0]?.created_at ?? 1707229920000;

export function makeTimelineEntity(options: {
  index?: number;
  overrides?: Partial<TimelineEntity>;
} = {}): TimelineEntity {
  const { index = 0, overrides = {} } = options;
  return {
    ...pickByIndex(mockTimelineEntities, index),
    ...overrides,
  };
}

export function makeTimelineEntities(
  count: number,
  options: {
    startIndex?: number;
    mapOverrides?: (listIndex: number) => Partial<TimelineEntity>;
  } = {}
): TimelineEntity[] {
  const { startIndex = 0, mapOverrides } = options;
  return Array.from({ length: count }, (_, listIndex) => {
    const absoluteIndex = startIndex + listIndex;
    const baseEntity = makeTimelineEntity({ index: absoluteIndex });
    const synthetic = shouldApplyDeterministicOverrides(absoluteIndex, mockTimelineEntities.length);

    const deterministicOverrides: Partial<TimelineEntity> = synthetic
      ? {
        id: makeDeterministicId('entity', absoluteIndex, 3),
        created_at: makeDeterministicTimeMs(TIMELINE_CREATED_AT_BASE_MS, absoluteIndex, 1_000),
        ...(baseEntity.updated_at !== undefined
          ? { updated_at: makeDeterministicTimeMs(TIMELINE_CREATED_AT_BASE_MS + 500, absoluteIndex, 1_000) }
          : {}),
      }
      : {};

    return {
      ...baseEntity,
      ...deterministicOverrides,
      ...(mapOverrides?.(listIndex) ?? {}),
    };
  });
}
