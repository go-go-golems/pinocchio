import type { Anomaly } from '../../components/AnomalyPanel';
import {
  mockAnomalies,
  mockAppShellAnomalies,
} from '../fixtures/anomalies';
import { pickByIndex } from './common';
import {
  makeDeterministicId,
  makeDeterministicIsoTime,
  shouldApplyDeterministicOverrides,
} from './deterministic';

const ANOMALY_TIME_BASE_MS = Date.parse(mockAnomalies[0]?.timestamp ?? '2026-02-06T14:32:15.000Z');
const APP_SHELL_ANOMALY_TIME_BASE_MS = Date.parse(
  mockAppShellAnomalies[0]?.timestamp ?? '2026-02-06T14:32:15.000Z'
);

export function makeAnomaly(options: {
  index?: number;
  overrides?: Partial<Anomaly>;
} = {}): Anomaly {
  const { index = 0, overrides = {} } = options;
  return {
    ...pickByIndex(mockAnomalies, index),
    ...overrides,
  };
}

export function makeAnomalies(
  count: number,
  options: {
    startIndex?: number;
    mapOverrides?: (listIndex: number) => Partial<Anomaly>;
  } = {}
): Anomaly[] {
  const { startIndex = 0, mapOverrides } = options;
  return Array.from({ length: count }, (_, listIndex) => {
    const absoluteIndex = startIndex + listIndex;
    const baseAnomaly = makeAnomaly({ index: absoluteIndex });
    const synthetic = shouldApplyDeterministicOverrides(absoluteIndex, mockAnomalies.length);

    const deterministicOverrides: Partial<Anomaly> = synthetic
      ? {
        id: makeDeterministicId('anom', absoluteIndex, 3),
        timestamp: makeDeterministicIsoTime(ANOMALY_TIME_BASE_MS, absoluteIndex, 5_000),
      }
      : {};

    return {
      ...baseAnomaly,
      ...deterministicOverrides,
      ...(mapOverrides?.(listIndex) ?? {}),
    };
  });
}

export function makeAppShellAnomaly(options: {
  index?: number;
  overrides?: Partial<Anomaly>;
} = {}): Anomaly {
  const { index = 0, overrides = {} } = options;
  return {
    ...pickByIndex(mockAppShellAnomalies, index),
    ...overrides,
  };
}

export function makeAppShellAnomalies(
  count: number,
  options: {
    startIndex?: number;
    mapOverrides?: (listIndex: number) => Partial<Anomaly>;
  } = {}
): Anomaly[] {
  const { startIndex = 0, mapOverrides } = options;
  return Array.from({ length: count }, (_, listIndex) => {
    const absoluteIndex = startIndex + listIndex;
    const baseAnomaly = makeAppShellAnomaly({ index: absoluteIndex });
    const synthetic = shouldApplyDeterministicOverrides(absoluteIndex, mockAppShellAnomalies.length);

    const deterministicOverrides: Partial<Anomaly> = synthetic
      ? {
        id: makeDeterministicId('anom_app', absoluteIndex, 3),
        timestamp: makeDeterministicIsoTime(APP_SHELL_ANOMALY_TIME_BASE_MS, absoluteIndex, 5_000),
      }
      : {};

    return {
      ...baseAnomaly,
      ...deterministicOverrides,
      ...(mapOverrides?.(listIndex) ?? {}),
    };
  });
}
