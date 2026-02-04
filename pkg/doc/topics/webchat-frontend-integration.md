---
Title: Webchat Frontend Integration
Slug: webchat-frontend-integration
Short: How the frontend integrates with the webchat backend via WebSocket and HTTP.
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

This guide explains how the frontend integrates with the webchat backend. The system uses a hybrid architecture:

- **WebSocket** for real-time streaming (events from backend → UI state)
- **HTTP POST** for initiating chat messages (user input → backend)
- **HTTP GET** for hydrating timeline (page reload → restore state)

## Architecture Overview

```
┌──────────────────────────────────────────────────────┐
│              Browser (Preact/React + Zustand/Redux)  │
│                                                      │
│  ┌──────────────────────────────────────────────┐   │
│  │            Chat Component                     │   │
│  │  - Manages conv_id lifecycle                 │   │
│  │  - Dispatches actions for user messages      │   │
│  │  - Reads entities from store for rendering   │   │
│  └───────┬──────────────────────┬───────────────┘   │
│          │                      │                    │
│          ↓                      ↓                    │
│  ┌───────────────┐    ┌─────────────────────────┐   │
│  │  wsManager    │    │  HTTP API calls         │   │
│  │  (WebSocket)  │    │  - POST /chat           │   │
│  │  - Hydration  │    │  - GET /hydrate         │   │
│  │  - SEM events │    │                         │   │
│  └───────┬───────┘    └────────────┬────────────┘   │
│          │                         │                 │
│          ↓                         ↓                 │
│  ┌──────────────────────────────────────────────┐   │
│  │            Timeline State Store              │   │
│  │  - byConvId: { [id]: { byId, order } }      │   │
│  │  - Entities: message, tool_call, status...  │   │
│  └──────────────────────────────────────────────┘   │
└────────────┬───────────────────────┬────────────────┘
             │ WebSocket             │ HTTP
             │ /ws?conv_id=...       │ /chat, /hydrate
             ↓                       ↓
┌──────────────────────────────────────────────────────┐
│               Backend (Go - pinocchio)               │
│                                                      │
│  Router → Conversation → StreamCoordinator          │
│           ↓                                          │
│  Engine + Tool Loop → Events → SEM → WebSocket      │
└──────────────────────────────────────────────────────┘
```

## WebSocket Lifecycle

### Connection Establishment

The `wsManager` handles WebSocket connection lifecycle:

```typescript
// Connect with conversation ID and profile
wsManager.connect({
  convId: 'conv-123',
  profile: 'default',
});
```

**Key behaviors:**

- **Hydration gating**: Waits for document ready before opening socket
- **URL detection**: Uses relative paths or configured backend URL
- **Logging**: Enable with `?ws_debug=1` or `window.__WS_DEBUG__ = true`

### SEM Event Format

Backend emits JSON frames over WebSocket:

```json
{
  "sem": true,
  "event": {
    "type": "llm.delta",
    "id": "eae401d8-...",
    "delta": "Hi"
  }
}
```

Frontend parses and routes through the SEM registry to update state.

### Event Types

| Event | Description |
|-------|-------------|
| `llm.start` | Model started generating |
| `llm.delta` | Incremental text chunk |
| `llm.final` | Generation complete |
| `tool.start` | Tool invocation started |
| `tool.delta` | Tool progress update |
| `tool.result` | Tool execution result |
| `tool.done` | Tool execution complete |
| `log` | Log message from backend |

## HTTP API

### Start Chat

**POST** `/chat` or `/chat/{profile}`

```json
{
  "prompt": "Hello, assistant",
  "conv_id": "conv-123",
  "overrides": {
    "system_prompt": "Be helpful",
    "middlewares": []
  }
}
```

**Response:**

```json
{
  "run_id": "uuid",
  "conv_id": "conv-123"
}
```

### Hydrate Timeline

**GET** `/hydrate?conv_id={id}&since_seq={n}`

Returns buffered SEM frames for hydration gating. Used when reconnecting to restore recent events.

### Timeline Snapshots

**GET** `/timeline?conv_id={id}&since_version={n}`

Returns durable timeline entities from SQLite (when enabled with `--timeline-db`).

## Timeline State Structure

```typescript
{
  byConvId: {
    "conv-123": {
      byId: {
        "user-1": {
          id: "user-1",
          kind: "message",
          timestamp: 1763501038000,
          props: { role: "user", content: "hello" }
        },
        "asst-1": {
          id: "asst-1",
          kind: "message",
          timestamp: 1763501040615,
          props: { role: "assistant", content: "Hi", streaming: false }
        }
      },
      order: ["user-1", "asst-1"]
    }
  }
}
```

### State Actions

| Action | Purpose | When Used |
|--------|---------|-----------|
| `addEntity` | Add new entity | User message, llm.start |
| `upsertEntity` | Create or merge entity | Hydration, tool updates |
| `appendMessageText` | Streaming text append | llm.delta events |
| `finalizeMessage` | Stop streaming | llm.final event |

### Version-Based Merging

Entities track versions for correct hydration merging:

- **Streaming events**: Carry `event.seq` (monotonic stream order); use `version = seq`
- **DB snapshots**: Use the same version values stored by the backend
- **Merge rule**: Higher version wins; equal versions merge shallowly

## Conversation ID Lifecycle

Conversation IDs persist in the URL for bookmarking and reload:

```
/chat              → generates new conv_id, redirects
/chat/conv-123     → loads existing conversation
```

**Priority chain:**

1. URL params (primary)
2. State store (fallback)
3. Generate new (`conv-${Date.now()}`)

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts` | WebSocket connection manager |
| `pinocchio/cmd/web-chat/web/src/sem/registry.ts` | SEM event routing |
| `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts` | Timeline state management |
| `pinocchio/cmd/web-chat/web/src/chat/ChatWidget.tsx` | Main chat component |

## Best Practices

- **One connection per conversation**: Avoid duplicate WebSocket connections
- **Handle disconnects gracefully**: Reconnect and hydrate missed events
- **Use hydration on reconnect**: Restore state from `/hydrate` or `/timeline`
- **Enable debug logs during development**: `?ws_debug=1`

## See Also

- [SEM and UI](webchat-sem-and-ui.md) — SEM event types and UI mapping
- [Backend Reference](webchat-backend-reference.md) — Backend API contracts
- [Debugging and Ops](webchat-debugging-and-ops.md) — Troubleshooting
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage
