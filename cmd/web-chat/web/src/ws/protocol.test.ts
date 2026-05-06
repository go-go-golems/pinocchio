import { afterEach, describe, expect, it } from 'vitest';
import { buildWebSocketURL, encodeSubscribeFrame, parseServerFrame, safeOrdinal } from './protocol';

const originalWindow = globalThis.window;

function setWindowLocation(protocol: string, host: string) {
  Object.defineProperty(globalThis, 'window', {
    value: {
      location: {
        protocol,
        host,
      },
    },
    configurable: true,
    writable: true,
  });
}

describe('sessionstream websocket protocol helpers', () => {
  afterEach(() => {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, 'window');
      return;
    }
    Object.defineProperty(globalThis, 'window', {
      value: originalWindow,
      configurable: true,
      writable: true,
    });
  });

  it('builds the canonical websocket URL', () => {
    setWindowLocation('http:', 'localhost:5173');

    expect(buildWebSocketURL({ basePrefix: '' })).toBe('ws://localhost:5173/api/chat/ws');
  });

  it('uses wss and preserves a base prefix', () => {
    setWindowLocation('https:', 'chat.example.com');

    expect(buildWebSocketURL({ basePrefix: '/app' })).toBe('wss://chat.example.com/app/api/chat/ws');
  });

  it('encodes subscribe as the sessionstream protobuf JSON oneof shape', () => {
    expect(JSON.parse(encodeSubscribeFrame('session-1', 42))).toEqual({
      subscribe: {
        sessionId: 'session-1',
        sinceSnapshotOrdinal: '42',
      },
    });
  });

  it('normalizes snapshot frames', () => {
    const frame = parseServerFrame(JSON.stringify({
      snapshot: {
        sessionId: 'session-1',
        snapshotOrdinal: '11',
        entities: [{ kind: 'ChatMessage', id: 'chat-msg-1', payload: { content: 'hello' } }],
      },
    }));

    expect(frame).toEqual({
      type: 'snapshot',
      sessionId: 'session-1',
      ordinal: '11',
      entities: [{ kind: 'ChatMessage', id: 'chat-msg-1', payload: { content: 'hello' } }],
    });
  });

  it('unwraps google.protobuf.Struct uiEvent payload values', () => {
    const frame = parseServerFrame(JSON.stringify({
      uiEvent: {
        sessionId: 'session-1',
        eventOrdinal: '12',
        name: 'ChatReasoningAppended',
        payload: {
          '@type': 'type.googleapis.com/google.protobuf.Struct',
          value: {
            messageId: 'chat-msg-1:thinking:1',
            role: 'thinking',
            content: 'draft plan',
          },
        },
      },
    }));

    expect(frame).toEqual({
      type: 'ui-event',
      sessionId: 'session-1',
      ordinal: '12',
      name: 'ChatReasoningAppended',
      payload: {
        messageId: 'chat-msg-1:thinking:1',
        role: 'thinking',
        content: 'draft plan',
      },
    });
  });

  it('parses safe numeric ordinals only', () => {
    expect(safeOrdinal('12')).toBe(12);
    expect(safeOrdinal(13)).toBe(13);
    expect(safeOrdinal('')).toBeNull();
    expect(safeOrdinal(String(Number.MAX_SAFE_INTEGER + 1))).toBeNull();
  });
});
