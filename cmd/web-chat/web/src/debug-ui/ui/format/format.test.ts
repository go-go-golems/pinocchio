import { describe, expect, it } from 'vitest';
import { safeString, safeStringify, truncateText } from './text';
import { formatDateTime, formatTimeShort } from './time';

describe('format helpers', () => {
  it('truncates text consistently', () => {
    expect(truncateText(undefined, 5)).toBe('');
    expect(truncateText('hello', 5)).toBe('hello');
    expect(truncateText('hello world', 5)).toBe('hello...');
  });

  it('stringifies values safely', () => {
    expect(safeStringify({ a: 1 })).toBe('{"a":1}');
    expect(safeStringify({ a: 1 }, 2)).toBe('{\n  "a": 1\n}');

    const circular: Record<string, unknown> = {};
    circular.self = circular;
    expect(safeStringify(circular)).toBe('[unserializable]');
  });

  it('converts values to safe display strings', () => {
    expect(safeString('abc')).toBe('abc');
    expect(safeString(null)).toBe('');
    expect(safeString(undefined)).toBe('');
    expect(safeString({ a: 1 })).toBe('{"a":1}');
  });

  it('formats valid timestamps and handles invalid timestamps safely', () => {
    expect(formatTimeShort('not-a-date')).toBe('—');
    expect(formatDateTime('not-a-date')).toBe('—');

    const dateValue = '2026-02-07T12:00:00.000Z';
    expect(formatTimeShort(dateValue)).not.toBe('—');
    expect(formatDateTime(dateValue)).not.toBe('—');
  });
});
