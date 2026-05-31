import { basePrefixFromLocation } from '../utils/basePrefix';
import { asRecord, asString, type CanonicalFrame } from './protocol';

export type FrontendToolResultStatus = 'success' | 'failed' | 'cancelled' | 'denied';

export async function submitFrontendToolResult(args: {
  sessionId: string;
  toolCallId: string;
  toolName: string;
  status?: FrontendToolResultStatus;
  result?: Record<string, unknown>;
  error?: string;
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
    }),
  });
  if (!response.ok) {
    throw new Error(`submit frontend tool result failed: ${response.status} ${await response.text()}`);
  }
}

export function handleAutomaticFrontendTool(frame: CanonicalFrame, sessionId: string) {
  if (asString(frame.name) !== 'ChatFrontendToolCallRequested') return;
  const payload = asRecord(frame.payload);
  const toolCallId = asString(payload.toolCallId);
  const toolName = asString(payload.toolName);
  if (!toolCallId || !toolName || toolName !== 'browser.get_page_context') return;

  void submitFrontendToolResult({
    sessionId,
    toolCallId,
    toolName,
    status: 'success',
    result: {
      url: window.location.href,
      viewport: {
        width: window.innerWidth,
        height: window.innerHeight,
      },
      userAgent: window.navigator.userAgent,
      basePrefix: basePrefixFromLocation(),
      timestamp: new Date().toISOString(),
    },
  }).catch((err) => {
    void submitFrontendToolResult({
      sessionId,
      toolCallId,
      toolName,
      status: 'failed',
      error: err instanceof Error ? err.message : String(err),
    }).catch(() => undefined);
  });
}
