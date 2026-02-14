import { describe, expect, it } from 'vitest';
import {
  makeDeterministicId,
  makeDeterministicIsoTime,
  makeDeterministicSeq,
  makeDeterministicTimeMs,
  shouldApplyDeterministicOverrides,
} from './deterministic';

describe('deterministic mock factory helpers', () => {
  it('normalizes indexes into stable deterministic ids', () => {
    expect(makeDeterministicId('turn', 0, 2)).toBe('turn_01');
    expect(makeDeterministicId('turn', 2.7, 2)).toBe('turn_03');
    expect(makeDeterministicId('turn', -5, 2)).toBe('turn_01');
  });

  it('calculates deterministic times and iso values', () => {
    const base = Date.parse('2026-02-06T14:30:00.000Z');
    expect(makeDeterministicTimeMs(base, 3, 1_000)).toBe(base + 3_000);
    expect(makeDeterministicIsoTime(base, 3, 1_000)).toBe('2026-02-06T14:30:03.000Z');
  });

  it('calculates deterministic sequence values', () => {
    expect(makeDeterministicSeq(100, 0, 5)).toBe(100);
    expect(makeDeterministicSeq(100, 4, 5)).toBe(120);
  });

  it('detects when fixture wrapping needs deterministic overrides', () => {
    expect(shouldApplyDeterministicOverrides(0, 3)).toBe(false);
    expect(shouldApplyDeterministicOverrides(2, 3)).toBe(false);
    expect(shouldApplyDeterministicOverrides(3, 3)).toBe(true);
  });
});
