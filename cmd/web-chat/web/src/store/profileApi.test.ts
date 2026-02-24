import { configureStore } from '@reduxjs/toolkit';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { profileApi } from './profileApi';

function createTestStore() {
  return configureStore({
    reducer: {
      [profileApi.reducerPath]: profileApi.reducer,
    },
    middleware: (getDefaultMiddleware) => getDefaultMiddleware().concat(profileApi.middleware),
  });
}

describe('profileApi', () => {
  const originalFetch = globalThis.fetch;
  const originalRequest = globalThis.Request;
  const originalWindow = (globalThis as { window?: Window }).window;

  beforeEach(() => {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        location: {
          pathname: '/',
        },
        __PINOCCHIO_WEBCHAT_CONFIG__: {
          basePrefix: '/chat',
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

  it('decodes indexed-object profile list shape and sorts by index', async () => {
    const store = createTestStore();
    const calls: string[] = [];

    globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const rawUrl = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const url = rawUrl.replace(/^https?:\/\/[^/]+/, '');
      calls.push(url);
      return new Response(
        JSON.stringify({
          1: { slug: 'planner' },
          0: { slug: 'default', is_default: true },
        }),
        {
          status: 200,
          headers: { 'content-type': 'application/json' },
        },
      );
    }) as typeof fetch;

    const profiles = await store.dispatch(profileApi.endpoints.getProfiles.initiate()).unwrap();
    expect(profiles.map((p) => p.slug)).toEqual(['default', 'planner']);
    expect(calls).toEqual(['/chat/api/chat/profiles']);
  });

  it('normalizes current-profile payloads for get/set profile APIs', async () => {
    const store = createTestStore();
    const seen: Array<{ url: string; method: string; body: string }> = [];

    globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      let rawUrl = '';
      let method = init?.method ?? 'GET';
      let body = typeof init?.body === 'string' ? init.body : '';
      if (typeof input === 'string') {
        rawUrl = input;
      } else if (input instanceof URL) {
        rawUrl = input.toString();
      } else {
        rawUrl = input.url;
        method = input.method;
        body = await input.clone().text();
      }
      const url = rawUrl.replace(/^https?:\/\/[^/]+/, '');
      seen.push({ url, method, body });

      if (url === '/chat/api/chat/profile' && method === 'GET') {
        return new Response(JSON.stringify({ profile: 'inventory' }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        });
      }
      if (url === '/chat/api/chat/profile' && method === 'POST') {
        return new Response(JSON.stringify({ profile: 'planner' }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        });
      }
      return new Response('not found', { status: 404 });
    }) as typeof fetch;

    const current = await store.dispatch(profileApi.endpoints.getProfile.initiate()).unwrap();
    expect(current.slug).toBe('inventory');

    const updated = await store.dispatch(profileApi.endpoints.setProfile.initiate({ slug: 'planner' })).unwrap();
    expect(updated.slug).toBe('planner');

    expect(seen[0].url).toBe('/chat/api/chat/profile');
    expect(seen[0].method).toBe('GET');
    expect(seen[1].url).toBe('/chat/api/chat/profile');
    expect(seen[1].method).toBe('POST');
    expect(JSON.parse(seen[1].body)).toEqual({ slug: 'planner' });
  });
});
