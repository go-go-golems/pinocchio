import { describe, expect, it } from 'vitest';
import { isFrontendToolMode } from './ProviderToolCallRenderer';

describe('isFrontendToolMode', () => {
  it.each(['TOOL_EXECUTION_MODE_FRONTEND_AUTO', 'TOOL_EXECUTION_MODE_FRONTEND_HUMAN', 1, 2])('recognizes %s as frontend', (mode) => {
    expect(isFrontendToolMode(mode)).toBe(true);
  });

  it.each(['', 'TOOL_EXECUTION_MODE_BACKEND', 0, 3])('keeps %s read-only/backend', (mode) => {
    expect(isFrontendToolMode(mode)).toBe(false);
  });
});
