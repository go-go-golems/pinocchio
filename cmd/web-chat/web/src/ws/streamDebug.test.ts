import { afterEach, describe, expect, it, vi } from 'vitest';
import { clearStreamDebugEntries, recordStreamDebug, uploadAndDownloadSQLite } from './streamDebug';

function installWindow(basePrefix: string) {
  const store = new Map<string, string>([['pinocchio.debugStream', '1']]);
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: {
      location: { pathname: `${basePrefix || ''}/debug` },
      __PINOCCHIO_WEBCHAT_CONFIG__: { basePrefix },
      localStorage: {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => store.set(key, value),
        removeItem: (key: string) => store.delete(key),
      },
    },
  });
}

describe('stream debug upload', () => {
  const originalWindow = (globalThis as { window?: Window }).window;
  const originalFetch = globalThis.fetch;
  const originalAlert = globalThis.alert;

  afterEach(() => {
    clearStreamDebugEntries();
    vi.restoreAllMocks();
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: originalWindow,
    });
    Object.defineProperty(globalThis, 'fetch', {
      configurable: true,
      value: originalFetch,
    });
    Object.defineProperty(globalThis, 'alert', {
      configurable: true,
      value: originalAlert,
    });
  });

  it('uploads SQLite debug logs under the configured runtime base prefix', async () => {
    installWindow('/chat');
    recordStreamDebug({ type: 'ui-event', sessionId: 'session/with slash' });

    const fetchMock = vi.fn(async () => ({
      ok: false,
      status: 404,
      text: async () => 'not found',
    })) as unknown as typeof fetch;
    Object.defineProperty(globalThis, 'fetch', { configurable: true, value: fetchMock });
    Object.defineProperty(globalThis, 'alert', { configurable: true, value: vi.fn() });

    await uploadAndDownloadSQLite();

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith('/chat/api/debug/sessions/session%2Fwith%20slash/reconcile/upload', expect.any(Object));
  });
});
