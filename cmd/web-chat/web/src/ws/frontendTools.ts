import { basePrefixFromLocation } from '../utils/basePrefix';

export type FrontendToolResultStatus = 'success' | 'failed' | 'cancelled' | 'denied' | 'timeout';

export type FrontendToolExecutor = {
  clientInstanceId: string;
  connectionId: string;
  assignmentId: string;
};

const terminalFrontendToolResultStatuses = new Set<FrontendToolResultStatus>(['success', 'failed', 'cancelled', 'denied', 'timeout']);

export function isTerminalFrontendToolResultStatus(status: string): status is FrontendToolResultStatus {
  return terminalFrontendToolResultStatuses.has(status as FrontendToolResultStatus);
}

export async function submitFrontendToolResult(args: {
  sessionId: string;
  toolCallId: string;
  toolName: string;
  status?: FrontendToolResultStatus;
  result?: Record<string, unknown>;
  error?: string;
  executor: FrontendToolExecutor;
}) {
  const basePrefix = basePrefixFromLocation();
  const response = await fetch(`${basePrefix}/api/chat/sessions/${encodeURIComponent(args.sessionId)}/tools/results`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      toolCallId: args.toolCallId,
      toolName: args.toolName,
      status: args.status ?? 'success',
      result: args.result ?? {},
      error: args.error ?? '',
      executor: args.executor,
    }),
  });
  if (!response.ok) {
    throw new Error(`submit frontend tool result failed: ${response.status} ${await response.text()}`);
  }
}
