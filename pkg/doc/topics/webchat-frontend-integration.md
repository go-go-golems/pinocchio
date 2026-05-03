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
  -> sessionstream Hub manages event projection and UI fanout
  -> WebSocket transport delivers snapshot + live UI events
```

## WebSocket Lifecycle

The sessionstream WebSocket transport speaks a simple protocol:

1. Client connects to `/api/chat/ws`.
2. Server sends `{ type: "hello", connectionId: "conn-N" }`.
3. Client sends `{ type: "subscribe", sessionId: "...", sinceOrdinal: "0" }`.
4. Server sends `{ type: "snapshot", sessionId, ordinal, entities: [...] }` with current state.
5. Server sends `{ type: "subscribed", sessionId }`.
6. Server sends `{ type: "ui-event", sessionId, ordinal, name, payload }` for each live event.
7. Client may send `{ type: "unsubscribe", sessionId }` to stop receiving events.
8. Client may send `{ type: "ping" }` to check liveness (server replies `{ type: "pong" }`).

## Recommended Client Pattern: `wsManager` (Dedicated WebSocket Lifecycle Module)

Centralize WebSocket logic into a dedicated `wsManager` module (see `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`) instead of spreading it across components.

Key reasons:

- **Deterministic lifecycle**: one place owns connect/disconnect, subscription, and hydration gating.
- **Snapshot-first**: the server always sends a full snapshot on subscribe, so the client rebuilds state from a known baseline.
- **Simpler rehydration**: reconnect flows re-subscribe and receive a fresh snapshot.

## UI Event Frame Contract

After subscribing, the server sends live updates as UI event frames:

```json
{
  "type": "ui-event",
  "sessionId": "sess-1",
  "ordinal": "42",
  "name": "ChatMessageAppended",
  "payload": {
    "messageId": "msg-1",
    "role": "assistant",
    "content": "Hi",
    "status": "streaming",
    "streaming": true
  }
}
```

Common UI event names:

- `ChatMessageAccepted` — user message submitted
- `ChatMessageStarted` — assistant begins responding
- `ChatMessageAppended` — token/chunk appended during streaming
- `ChatMessageFinished` — assistant response complete
- `ChatMessageStopped` — response stopped (user cancel or error)
- `ChatReasoningStarted`, `ChatReasoningAppended`, `ChatReasoningFinished` — thinking/reasoning blocks
- `ChatAgentModePreviewUpdated`, `ChatAgentModeCommitted`, `ChatAgentModePreviewCleared` — mode switch events

## Snapshot Frame Contract

On subscribe, the server sends the current state:

```json
{
  "type": "snapshot",
  "sessionId": "sess-1",
  "ordinal": "15",
  "entities": [
    {
      "kind": "message",
      "id": "msg-1",
      "tombstone": false,
      "payload": { "role": "user", "content": "Hello", "status": "submitted" }
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

- [Webchat Frontend Architecture](webchat-frontend-architecture.md)
- [Webchat Debugging and Ops](webchat-debugging-and-ops.md)
