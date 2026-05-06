export type CanonicalFrame = Record<string, unknown>;

export type SnapshotEntityFrame = {
  kind?: unknown;
  id?: unknown;
  tombstone?: unknown;
  payload?: unknown;
};

export function buildWebSocketURL(args: { basePrefix?: string }): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
  return `${proto}://${window.location.host}${args.basePrefix ?? ''}/api/chat/ws`;
}

export function encodeSubscribeFrame(sessionId: string, sinceSnapshotOrdinal: number | string = 0): string {
  return JSON.stringify({
    subscribe: {
      sessionId,
      sinceSnapshotOrdinal: String(sinceSnapshotOrdinal),
    },
  });
}

export function safeOrdinal(raw: unknown): number | null {
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    return Number.isSafeInteger(raw) ? raw : null;
  }
  if (typeof raw === 'string' && raw.trim()) {
    const n = Number(raw);
    if (Number.isFinite(n) && Number.isSafeInteger(n)) return n;
  }
  return null;
}

export function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

export function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

export function unwrapAnyPayload(value: unknown): Record<string, unknown> {
  const payload = asRecord(value);
  const nestedValue = asRecord(payload.value);
  // google.protobuf.Struct payloads arrive as {"@type":"...Struct", "value": {...}}.
  // Concrete protobuf Any payloads arrive with their fields next to "@type".
  return Object.keys(nestedValue).length > 0 ? nestedValue : payload;
}

export function normalizeServerFrame(frame: CanonicalFrame): CanonicalFrame {
  if ('type' in frame) return frame;
  if (frame.hello) return { type: 'hello', ...asRecord(frame.hello) };
  if (frame.snapshot) {
    const snapshot = asRecord(frame.snapshot);
    return {
      type: 'snapshot',
      sessionId: asString(snapshot.sessionId),
      ordinal: snapshot.snapshotOrdinal,
      entities: Array.isArray(snapshot.entities) ? snapshot.entities : [],
    };
  }
  if (frame.subscribed) {
    const subscribed = asRecord(frame.subscribed);
    return {
      type: 'subscribed',
      sessionId: asString(subscribed.sessionId),
      ordinal: subscribed.sinceSnapshotOrdinal,
    };
  }
  if (frame.unsubscribed) return { type: 'unsubscribed', ...asRecord(frame.unsubscribed) };
  if (frame.uiEvent) {
    const uiEvent = asRecord(frame.uiEvent);
    return {
      type: 'ui-event',
      sessionId: asString(uiEvent.sessionId),
      ordinal: uiEvent.eventOrdinal,
      name: asString(uiEvent.name),
      payload: unwrapAnyPayload(uiEvent.payload),
    };
  }
  if (frame.error) {
    const error = asRecord(frame.error);
    return {
      type: 'error',
      sessionId: asString(error.sessionId),
      error: asString(error.message),
      code: asString(error.code),
      detail: asString(error.detail),
    };
  }
  if (frame.ping) return { type: 'ping', ...asRecord(frame.ping) };
  if (frame.pong) return { type: 'pong', ...asRecord(frame.pong) };
  return frame;
}

export function parseServerFrame(raw: string): CanonicalFrame {
  return normalizeServerFrame(JSON.parse(raw) as CanonicalFrame);
}
