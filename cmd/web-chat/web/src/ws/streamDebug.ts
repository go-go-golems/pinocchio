import { basePrefixFromLocation } from '../utils/basePrefix';
import type { CanonicalFrame } from './protocol';

type DebugTimelineEntity = {
  id: string;
  kind: string;
};

type DebugEntryBase = {
  id: number;
  timestamp: number;
  type: string;
  sessionId?: string;
};

export type StreamDebugEntry = DebugEntryBase & Record<string, unknown>;

type StreamDebugState = {
  enabled: boolean;
  entries: StreamDebugEntry[];
};

const STORAGE_KEY = 'pinocchio.debugStream';
const MAX_ENTRIES = 10000;
let nextId = 1;
const state: StreamDebugState = { enabled: false, entries: [] };

function isEnabled(): boolean {
  try {
    return window.localStorage.getItem(STORAGE_KEY) === '1';
  } catch {
    return false;
  }
}

function refreshEnabled() {
  state.enabled = isEnabled();
}

export function streamDebugEnabled(): boolean {
  refreshEnabled();
  return state.enabled;
}

export function toggleStreamDebug(): boolean {
  const now = !isEnabled();
  try {
    if (now) {
      window.localStorage.setItem(STORAGE_KEY, '1');
    } else {
      window.localStorage.removeItem(STORAGE_KEY);
    }
  } catch {
    // Ignore non-browser environments.
  }
  state.enabled = now;
  return now;
}

export function recordStreamDebug(entry: Omit<StreamDebugEntry, 'id' | 'timestamp'>) {
  if (!streamDebugEnabled()) return;
  state.entries.push({ ...(entry as Record<string, unknown>), id: nextId++, timestamp: Date.now(), type: String(entry.type) });
  if (state.entries.length > MAX_ENTRIES) {
    state.entries.splice(0, state.entries.length - MAX_ENTRIES);
  }
}

export function getStreamDebugEntries(): StreamDebugEntry[] {
  refreshEnabled();
  return [...state.entries];
}

export function clearStreamDebugEntries() {
  state.entries = [];
}

export function exportStreamDebugJSON() {
  const blob = new Blob([JSON.stringify(getStreamDebugEntries(), null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `pinocchio-stream-debug-${new Date().toISOString().replace(/[:.]/g, '-')}.json`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function recordRawWS(sessionId: string, data: string) {
  recordStreamDebug({ type: 'raw-ws', sessionId, size: data.length, preview: data.slice(0, 1000), raw: data });
}

export function recordParsedFrame(sessionId: string, frame: CanonicalFrame) {
  recordStreamDebug({
    type: 'parsed-frame',
    sessionId,
    frameType: frame.type,
    name: frame.name,
    ordinal: frame.ordinal,
    frame,
  });
}

export function recordSnapshotDebug(sessionId: string, ordinal: unknown, entities: Array<{ raw: unknown; mapped: DebugTimelineEntity | null }>) {
  recordStreamDebug({
    type: 'snapshot',
    sessionId,
    ordinal,
    entityCount: entities.length,
    droppedCount: entities.filter((e) => !e.mapped).length,
    entities: entities.map((e) => ({
      rawKind: (e.raw as any)?.kind,
      rawId: (e.raw as any)?.id,
      mappedId: e.mapped?.id,
      mappedKind: e.mapped?.kind,
      dropped: !e.mapped,
    })),
  });
}

export function recordUIEventDebug(sessionId: string, frame: CanonicalFrame, mutation: unknown) {
  recordStreamDebug({
    type: 'ui-event',
    sessionId,
    ordinal: frame.ordinal,
    name: frame.name,
    messageId: (frame.payload as any)?.messageId,
    mutation,
  });
}

export function recordLifecycle(sessionId: string, event: string, extra?: Record<string, unknown>) {
  recordStreamDebug({ type: 'ws-lifecycle', sessionId, event, ...(extra ?? {}) });
}

export async function uploadAndDownloadSQLite(): Promise<void> {
  const entries = getStreamDebugEntries();
  // Derive session ID from the most recent entry that has one.
  let sessionId = '';
  for (let i = entries.length - 1; i >= 0; i--) {
    if (entries[i].sessionId) {
      sessionId = entries[i].sessionId as string;
      break;
    }
  }
  if (!sessionId) {
    alert('No session ID found in debug entries');
    return;
  }
  const body = JSON.stringify({ records: entries });
  const basePrefix = basePrefixFromLocation();
  const resp = await fetch(`${basePrefix}/api/debug/sessions/${encodeURIComponent(sessionId)}/reconcile/upload`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  });
  if (!resp.ok) {
    const text = await resp.text();
    alert(`Upload failed: ${resp.status} ${text}`);
    return;
  }
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  const cd = resp.headers.get('Content-Disposition') || '';
  const match = cd.match(/filename="?([^";]+)"?/);
  a.download = match ? match[1] : `pinocchio-stream-debug-${sessionId}.sqlite`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

try {
  (window as any).__pinocchioStreamDebug = {
    entries: getStreamDebugEntries,
    clear: clearStreamDebugEntries,
    exportJSON: exportStreamDebugJSON,
    uploadSQLite: uploadAndDownloadSQLite,
    enable: () => window.localStorage.setItem(STORAGE_KEY, '1'),
    disable: () => window.localStorage.removeItem(STORAGE_KEY),
  };
} catch {
  // Ignore non-browser environments.
}
