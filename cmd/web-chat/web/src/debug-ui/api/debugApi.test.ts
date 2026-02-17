import { configureStore } from '@reduxjs/toolkit';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { debugApi } from './debugApi';

function createTestStore() {
  return configureStore({
    reducer: {
      [debugApi.reducerPath]: debugApi.reducer,
    },
    middleware: (getDefaultMiddleware) => getDefaultMiddleware().concat(debugApi.middleware),
  });
}

describe('debugApi baseQuery prefix resolution', () => {
  const originalFetch = globalThis.fetch;
  const originalRequest = globalThis.Request;
  const originalWindow = (globalThis as { window?: Window }).window;

  beforeEach(() => {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        location: {
          pathname: '/',
          search: '?debug=1',
        },
        __PINOCCHIO_WEBCHAT_CONFIG__: {
          basePrefix: '/chat',
          debugApiEnabled: true,
        },
      },
    });

    globalThis.Request = class extends originalRequest {
      constructor(input: RequestInfo | URL, init?: RequestInit) {
        if (typeof input === 'string' && input.startsWith('/')) {
          super(`http://localhost${input}`, init);
          return;
        }
        if (input instanceof URL && input.pathname.startsWith('/')) {
          super(`http://localhost${input.pathname}${input.search}`, init);
          return;
        }
        super(input, init);
      }
    } as typeof Request;
  });

  afterEach(() => {
    vi.restoreAllMocks();
    globalThis.fetch = originalFetch;
    globalThis.Request = originalRequest;
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: originalWindow,
    });
  });

  it('uses configured base prefix for debug API paths', async () => {
    const store = createTestStore();
    const calls: string[] = [];

    globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const rawUrl = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const url = rawUrl.replace(/^https?:\/\/[^/]+/, '');
      calls.push(url);
      if (url === '/chat/api/debug/conversations') {
        return new Response(JSON.stringify({ items: [] }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        });
      }
      if (url === '/chat/api/debug/conversations/conv-1') {
        return new Response(
          JSON.stringify({
            conv_id: 'conv-1',
            session_id: 'session-1',
            runtime_key: 'default',
            active_sockets: 0,
            stream_running: false,
            queue_depth: 0,
            buffered_events: 0,
            last_activity_ms: 0,
            has_timeline_source: false,
          }),
          {
            status: 200,
            headers: { 'content-type': 'application/json' },
          },
        );
      }
      return new Response('not found', { status: 404 });
    }) as typeof fetch;

    const conversations = await store.dispatch(debugApi.endpoints.getConversations.initiate()).unwrap();
    expect(conversations).toEqual([]);

    const conversation = await store.dispatch(debugApi.endpoints.getConversation.initiate('conv-1')).unwrap();
    expect(conversation.id).toBe('conv-1');

    expect(calls).toEqual(['/chat/api/debug/conversations', '/chat/api/debug/conversations/conv-1']);
  });
});
