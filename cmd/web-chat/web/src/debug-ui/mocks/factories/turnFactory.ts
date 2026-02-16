import type { TurnDetail, TurnSnapshot } from '../../types';
import { mockTurnDetail, mockTurns } from '../fixtures/turns';
import { pickByIndex } from './common';
import {
  makeDeterministicId,
  makeDeterministicTimeMs,
  shouldApplyDeterministicOverrides,
} from './deterministic';

const TURN_CREATED_AT_BASE_MS = mockTurns[0]?.created_at_ms ?? 1707229938000;

export function makeTurnSnapshot(options: {
  index?: number;
  overrides?: Partial<TurnSnapshot>;
} = {}): TurnSnapshot {
  const { index = 0, overrides = {} } = options;
  return {
    ...pickByIndex(mockTurns, index),
    ...overrides,
  };
}

export function makeTurnSnapshots(
  count: number,
  options: {
    startIndex?: number;
    mapOverrides?: (listIndex: number) => Partial<TurnSnapshot>;
  } = {}
): TurnSnapshot[] {
  const { startIndex = 0, mapOverrides } = options;
  return Array.from({ length: count }, (_, listIndex) => {
    const absoluteIndex = startIndex + listIndex;
    const baseSnapshot = makeTurnSnapshot({ index: absoluteIndex });
    const synthetic = shouldApplyDeterministicOverrides(absoluteIndex, mockTurns.length);
    const deterministicTurnId = makeDeterministicId('turn', absoluteIndex, 2);

    const deterministicOverrides: Partial<TurnSnapshot> = synthetic
      ? {
        turn_id: deterministicTurnId,
        created_at_ms: makeDeterministicTimeMs(TURN_CREATED_AT_BASE_MS, absoluteIndex, 60_000),
        turn: {
          ...baseSnapshot.turn,
          id: deterministicTurnId,
        },
      }
      : {};

    return {
      ...baseSnapshot,
      ...deterministicOverrides,
      ...(mapOverrides?.(listIndex) ?? {}),
    };
  });
}

export function makeTurnDetail(options: {
  overrides?: Partial<TurnDetail>;
} = {}): TurnDetail {
  const { overrides = {} } = options;
  return {
    ...pickByIndex([mockTurnDetail], 0),
    ...overrides,
  };
}
