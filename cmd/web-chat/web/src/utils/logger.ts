export type LogContext = {
  scope: string;
  convId?: string;
  sessionId?: string;
  seq?: number;
  extra?: Record<string, unknown>;
};

function formatContext(ctx?: LogContext) {
  return ctx ?? {};
}

export function logInfo(message: string, ctx?: LogContext) {
  console.info(`[webchat] ${message}`, formatContext(ctx));
}

export function logWarn(message: string, ctx?: LogContext, err?: unknown) {
  if (err !== undefined) {
    console.warn(`[webchat] ${message}`, formatContext(ctx), err);
    return;
  }
  console.warn(`[webchat] ${message}`, formatContext(ctx));
}

export function logError(message: string, err?: unknown, ctx?: LogContext) {
  if (err !== undefined) {
    console.error(`[webchat] ${message}`, formatContext(ctx), err);
    return;
  }
  console.error(`[webchat] ${message}`, formatContext(ctx));
}

export function errorToString(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === 'string') return err;
  try {
    return JSON.stringify(err);
  } catch {
    return 'unknown error';
  }
}
