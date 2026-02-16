import { configureStore } from '@reduxjs/toolkit';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { debugApi } from '../api/debugApi';
import { uiSlice } from '../store/uiSlice';
import { DebugTimelineWsManager } from './debugTimelineWsManager';

class MockWebSocket {
  static instances: MockWebSocket[] = [];

  url: string;
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  sendCalls: unknown[] = [];
  closed = false;

  constructor(url: string | URL) {
    this.url = String(url);
    MockWebSocket.instances.push(this);
  }

  close() {
    this.closed = true;
    this.onclose?.({} as CloseEvent);
  }

  send(payload: unknown) {
    this.sendCalls.push(payload);
  }

  emitOpen() {
    this.onopen?.({} as Event);
  }

  emitMessage(payload: unknown) {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent<string>);
  }

  emitClose() {
    this.onclose?.({} as CloseEvent);
  }
}

function createTestStore() {
  return configureStore({
    reducer: {
      [debugApi.reducerPath]: debugApi.reducer,
      ui: uiSlice.reducer,
    },
    middleware: (getDefaultMiddleware) => getDefaultMiddleware().concat(debugApi.middleware),
  });
}

describe('DebugTimelineWsManager', () => {
  const originalFetch = globalThis.fetch;
  const originalWebSocket = globalThis.WebSocket;
  const originalWindow = (globalThis as { window?: Window }).window;

  beforeEach(() => {
    MockWebSocket.instances = [];
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: { location: { protocol: 'http:', host: 'debug.example' } },
    });
    globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
  });

  afterEach(() => {
    vi.restoreAllMocks();
    globalThis.fetch = originalFetch;
    globalThis.WebSocket = originalWebSocket;
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: originalWindow,
    });
  });

  it('replays buffered timeline.upsert after bootstrap (two-tab follow path)', async () => {
    const store = createTestStore();
    const manager = new DebugTimelineWsManager();

    globalThis.fetch = vi.fn(async () =>
      new Response(
        JSON.stringify({
          convId: 'conv-1',
          version: '5',
          serverTimeMs: '1000',
          entities: [
            {
              id: 'msg-1',
              kind: 'message',
              createdAtMs: '100',
              updatedAtMs: '100',
              message: {
                schemaVersion: 1,
                role: 'assistant',
                content: 'before',
                streaming: false,
              },
            },
          ],
        }),
        { status: 200, headers: { 'content-type': 'application/json' } }
      )
    ) as typeof fetch;

    const connectPromise = manager.connect({
      convId: 'conv-1',
      basePrefix: '/chat',
      dispatch: store.dispatch,
    });

    expect(MockWebSocket.instances).toHaveLength(1);
    const socket = MockWebSocket.instances[0];
    expect(socket.url).toBe('ws://debug.example/chat/ws?conv_id=conv-1');

    socket.emitMessage({
      sem: true,
      event: {
        type: 'timeline.upsert',
        id: 'evt-upsert-6',
        seq: 6,
        data: {
          convId: 'conv-1',
          version: '6',
          entity: {
            id: 'msg-1',
            kind: 'message',
            createdAtMs: '100',
            updatedAtMs: '120',
            message: {
              schemaVersion: 1,
              role: 'assistant',
              content: 'after',
              streaming: false,
            },
          },
        },
      },
    });

    socket.emitOpen();
    await connectPromise;

    expect(globalThis.fetch).toHaveBeenCalledWith('/chat/api/timeline?conv_id=conv-1');
    expect(store.getState().ui.follow.status).toBe('connected');

    const timeline = debugApi.endpoints.getTimeline.select({ convId: 'conv-1' })(store.getState()).data;
    expect(timeline?.version).toBe(6);
    expect(timeline?.entities).toHaveLength(1);
    expect(timeline?.entities[0]?.id).toBe('msg-1');
    expect(timeline?.entities[0]?.props.content).toBe('after');

    const events = debugApi.endpoints.getEvents.select({ convId: 'conv-1' })(store.getState()).data;
    expect(events?.events).toHaveLength(1);
    expect(events?.events[0]?.type).toBe('timeline.upsert');
    expect(events?.events[0]?.seq).toBe(6);
    expect(socket.sendCalls).toHaveLength(0);

    manager.disconnect();
  });

  it('handles connect-switch lifecycle across conversations', async () => {
    const store = createTestStore();
    const manager = new DebugTimelineWsManager();

    const fetchMock = vi
      .fn(async (_input: RequestInfo | URL) =>
        new Response(
          JSON.stringify({
            convId: 'conv-1',
            version: '1',
            serverTimeMs: '1000',
            entities: [],
          }),
          { status: 200, headers: { 'content-type': 'application/json' } }
        )
      )
      .mockImplementationOnce(async () =>
        new Response(
          JSON.stringify({
            convId: 'conv-1',
            version: '1',
            serverTimeMs: '1000',
            entities: [],
          }),
          { status: 200, headers: { 'content-type': 'application/json' } }
        )
      )
      .mockImplementationOnce(async () =>
        new Response(
          JSON.stringify({
            convId: 'conv-2',
            version: '2',
            serverTimeMs: '1001',
            entities: [],
          }),
          { status: 200, headers: { 'content-type': 'application/json' } }
        )
      );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const firstConnect = manager.connect({
      convId: 'conv-1',
      basePrefix: '/chat',
      dispatch: store.dispatch,
    });
    const firstSocket = MockWebSocket.instances[0];
    firstSocket.emitOpen();
    await firstConnect;

    const secondConnect = manager.connect({
      convId: 'conv-2',
      basePrefix: '/chat',
      dispatch: store.dispatch,
    });

    expect(MockWebSocket.instances).toHaveLength(2);
    expect(firstSocket.closed).toBe(true);
    const secondSocket = MockWebSocket.instances[1];
    expect(secondSocket.url).toBe('ws://debug.example/chat/ws?conv_id=conv-2');

    secondSocket.emitOpen();
    await secondConnect;

    expect(fetchMock).toHaveBeenCalledWith('/chat/api/timeline?conv_id=conv-1');
    expect(fetchMock).toHaveBeenCalledWith('/chat/api/timeline?conv_id=conv-2');
    expect(store.getState().ui.follow.status).toBe('connected');

    secondSocket.emitClose();
    expect(store.getState().ui.follow.status).toBe('closed');

    manager.disconnect();
  });
});
