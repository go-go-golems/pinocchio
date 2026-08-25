import { afterEach, describe, expect, it, vi } from 'vitest';
import { isTerminalFrontendToolResultStatus, submitFrontendToolResult } from './frontendTools';

afterEach(() => vi.restoreAllMocks());

describe('isTerminalFrontendToolResultStatus', () => {
  it.each(['success', 'failed', 'denied', 'cancelled', 'timeout'])('treats %s as terminal', (status) => {
    expect(isTerminalFrontendToolResultStatus(status)).toBe(true);
  });

  it.each(['', 'requested', 'running', 'unknown'])('keeps %s non-terminal', (status) => {
    expect(isTerminalFrontendToolResultStatus(status)).toBe(false);
  });

  it('submits the assigned executor with a human result', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{}', { status: 200 }));
    const executor = { clientInstanceId: 'client-a', connectionId: 'connection-a', assignmentId: 'assignment-a' };

    await submitFrontendToolResult({
      sessionId: 'session-1',
      toolCallId: 'call-1',
      toolName: 'confirm',
      status: 'success',
      result: { approved: true },
      executor,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/chat/sessions/session-1/tools/results',
      expect.objectContaining({
        body: JSON.stringify({
          toolCallId: 'call-1',
          toolName: 'confirm',
          status: 'success',
          result: { approved: true },
          error: '',
          executor,
        }),
      }),
    );
  });
});
