import { describe, expect, it } from 'vitest';
import { isTerminalFrontendToolResultStatus } from './frontendTools';

describe('isTerminalFrontendToolResultStatus', () => {
  it.each(['success', 'failed', 'denied', 'cancelled', 'timeout'])('treats %s as terminal', (status) => {
    expect(isTerminalFrontendToolResultStatus(status)).toBe(true);
  });

  it.each(['', 'requested', 'running', 'unknown'])('keeps %s non-terminal', (status) => {
    expect(isTerminalFrontendToolResultStatus(status)).toBe(false);
  });
});
