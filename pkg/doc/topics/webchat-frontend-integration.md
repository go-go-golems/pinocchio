---
Title: Webchat Frontend Integration
Slug: webchat-frontend-integration
Short: How frontend clients integrate with the sessionstream-backed chat API, WebSocket transport, and snapshot hydration.
Topics:
- webchat
- frontend
- websocket
- streaming
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Integration Model

Frontend integration uses three endpoints:

- `POST /api/chat/sessions/:sessionId/messages` to submit user prompts
- `WS /api/chat/ws` for streaming sessionstream UI events (subscribe to a session)
- `GET /api/chat/sessions/:sessionId` for snapshot hydration

## Architecture Overview

```
Browser UI
  -> POST /api/chat/sessions/:sessionId/messages
  -> WS /api/chat/ws (subscribe with sessionId)
  -> GET /api/chat/sessions/:sessionId (snapshot)

Go backend
  -> ChatService handles submit/queue/idempotency
  -> sessionstream Hub manages protobuf-registered event projection and UI fanout
  -> WebSocket transport delivers protobuf JSON snapshot + live UI events
```

## WebSocket Lifecycle

The sessionstream WebSocket transport uses protobuf-defined `ClientFrame` and `ServerFrame` messages serialized as JSON. The browser still sends and receives JSON objects, but the backend schema source is protobuf.

1. Client connects to `/api/chat/ws`.
2. Server sends `{ type: "hello", connectionId: "conn-N" }`.
3. Client sends `{ type: "subscribe", sessionId: "...", sinceSnapshotOrdinal: "0" }`.
4. Server sends `{ type: "snapshot", sessionId, snapshotOrdinal, entities: [...] }` with current state.
5. Server sends `{ type: "subscribed", sessionId }`.
6. Server sends `{ type: "ui-event", sessionId, eventOrdinal, name, payload }` for each live event.
7. Client may send `{ type: "unsubscribe", sessionId }` to stop receiving events.
8. Client may send `{ type: "ping" }` to check liveness (server replies `{ type: "pong" }`).

Some existing frontend helpers still accept the older `sinceOrdinal` / `ordinal` spellings while sessionstream clients are being aligned. New documentation and new clients should use the role-specific names `sinceSnapshotOrdinal`, `snapshotOrdinal`, and `eventOrdinal`.

## Recommended Client Pattern: `wsManager` (Dedicated WebSocket Lifecycle Module)

Centralize WebSocket logic into a dedicated `wsManager` module (see `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`) instead of spreading it across components.

Key reasons:

- **Deterministic lifecycle**: one place owns connect/disconnect, subscription, and hydration gating.
- **Snapshot-first**: the server always sends a full snapshot on subscribe, so the client rebuilds state from a known baseline.
- **Simpler rehydration**: reconnect flows re-subscribe and receive a fresh snapshot.

## UI Event Frame Contract

After subscribing, the server sends live updates as UI event frames. The payload is the protobuf JSON form of the registered UI event message.

```json
{
  "type": "ui-event",
  "sessionId": "sess-1",
  "eventOrdinal": "42",
  "name": "ChatMessageAppended",
  "payload": {
    "messageId": "chat-msg-1:text:2",
    "parentMessageId": "chat-msg-1",
    "segment": 2,
    "segmentType": "text",
    "role": "assistant",
    "content": "Hi",
    "status": "streaming",
    "streaming": true,
    "final": false
  }
}
```

Common UI event names:

- `ChatMessageAccepted` — user message submitted
- `ChatMessageStarted` — assistant begins responding
- `ChatMessageAppended` — assistant text segment updated during streaming
- `ChatMessageFinished` — assistant text segment or final response complete
- `ChatMessageStopped` — response stopped (user cancel or error)
- `ChatReasoningStarted`, `ChatReasoningAppended`, `ChatReasoningFinished` — shared thinking/reasoning plugin events
- `ChatToolCallStarted`, `ChatToolCallUpdated`, `ChatToolCallFinished`, `ChatToolResultReady` — shared tool-call plugin events
- `ChatAgentModePreviewUpdated`, `ChatAgentModeCommitted`, `ChatAgentModePreviewCleared` — app-owned mode switch events

## Snapshot Frame Contract

On subscribe, the server sends the current state. Entity payloads are protobuf JSON for their registered timeline entity kind.

```json
{
  "type": "snapshot",
  "sessionId": "sess-1",
  "snapshotOrdinal": "15",
  "entities": [
    {
      "kind": "ChatMessage",
      "id": "chat-msg-1:thinking:1",
      "tombstone": false,
      "createdOrdinal": "11",
      "lastEventOrdinal": "14",
      "payload": {
        "messageId": "chat-msg-1:thinking:1",
        "parentMessageId": "chat-msg-1",
        "segment": 1,
        "segmentType": "thinking",
        "role": "thinking",
        "content": "I should inspect the tool output first.",
        "status": "finished",
        "streaming": false
      }
    }
  ]
}
```

## Chat Request Example

```json
POST /api/chat/sessions/sess-1/messages

{
  "prompt": "Hello assistant",
  "profile": "default"
}
```

## Base Prefix Handling

If backend runs with `--root /chat`, prefix all endpoints:

- `/chat/api/chat/sessions/:sessionId/messages`
- `/chat/api/chat/ws`
- `/chat/api/chat/sessions/:sessionId`

Compute a base prefix from `window.location.pathname` and reuse it consistently for fetch + websocket URLs.

## Error Handling

- Treat non-2xx `POST` responses as user-visible failures.
- Reconnect websocket on disconnect and re-subscribe (snapshot is re-sent automatically).
- Always apply snapshot before processing live events.

## Key Files

- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
- `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts`
- `pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts`

## See Also

- [Chatapp Protobuf Schemas and Shared Plugins](chatapp-protobuf-plugins.md)
- [Webchat Frontend Architecture](webchat-frontend-architecture.md)
- [Webchat Debugging and Ops](webchat-debugging-and-ops.md)
