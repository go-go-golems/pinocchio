export type FrontendToolResultStatus = 'success' | 'failed' | 'cancelled' | 'denied' | 'timeout';

const terminalFrontendToolResultStatuses = new Set<FrontendToolResultStatus>(['success', 'failed', 'cancelled', 'denied', 'timeout']);

export function isTerminalFrontendToolResultStatus(status: string): status is FrontendToolResultStatus {
  return terminalFrontendToolResultStatuses.has(status as FrontendToolResultStatus);
}
