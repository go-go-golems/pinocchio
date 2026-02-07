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
│              Browser (React + Redux Toolkit)         │
│                                                      │
│  ┌──────────────────────────────────────────────┐   │
│  │            Chat Widget                        │   │
│  │  - Manages conv_id lifecycle                 │   │
│  │  - Dispatches actions for user messages      │   │
│  │  - Reads entities from store for rendering   │   │
│  └───────┬──────────────────────┬───────────────┘   │
│          │                      │                    │
│          ↓                      ↓                    │
│  ┌───────────────┐    ┌─────────────────────────┐   │
│  │  wsManager    │    │  HTTP API calls         │   │
│  │  (WebSocket)  │    │  - POST /chat           │   │
│  │  - Hydration  │    │  - GET /timeline        │   │
│  │  - SEM events │    │                         │   │
│  └───────┬───────┘    └────────────┬────────────┘   │
│          │                         │                 │
│          ↓                         ↓                 │
│  ┌──────────────────────────────────────────────┐   │
│  │            Timeline State Store              │   │
│  │  - byId + order (single convo)              │   │
│  │  - Entities: message, tool_call, log...     │   │
│  └──────────────────────────────────────────────┘   │
└────────────┬───────────────────────┬────────────────┘
             │ WebSocket             │ HTTP
             │ /ws?conv_id=...       │ /chat, /timeline
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
// Connect with conversation ID and base prefix
wsManager.connect({
  convId: 'conv-123',
  basePrefix: '',
  dispatch,
});
```

**Key behaviors:**

- **Hydration gating**: Buffers WS events until hydration completes
- **URL detection**: Uses `window.location` to derive the base prefix

### SEM Event Format

Backend emits JSON frames over WebSocket:

```json
{
  "sem": true,
  "event": {
    "type": "llm.delta",
    "id": "eae401d8-...",
    "seq": 1707053365123000000,
    "data": { "cumulative": "Hi" }
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
| `tool.delta` | Tool update patch |
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

### Hydrate Timeline (Canonical)

**GET** `/timeline?conv_id={id}&since_version={n}&limit={n}`

Returns durable timeline entities from SQLite (when enabled with `--timeline-db`) or the in-memory store. This is the canonical hydration path used by the frontend on load/reconnect.

## Timeline State Structure

```typescript
{
  byId: {
    "user-1": {
      id: "user-1",
      kind: "message",
      createdAt: 1763501038000,
      props: { role: "user", content: "hello" }
    },
    "asst-1": {
      id: "asst-1",
      kind: "message",
      createdAt: 1763501040615,
      props: { role: "assistant", content: "Hi", streaming: false }
    }
  },
  order: ["user-1", "asst-1"]
}
```

### State Actions

| Action | Purpose | When Used |
|--------|---------|-----------|
| `addEntity` | Add new entity | llm.start, tool.start |
| `upsertEntity` | Create or merge entity | timeline.upsert, tool updates |
| `rekeyEntity` | Change entity ID | Hydration fixes |
| `clear` | Reset timeline state | New conversation |

### Version-Based Merging

Entities track versions for correct hydration merging:

- **Streaming events**: Carry `event.seq` (monotonic stream order)
- **DB snapshots**: Use the same version values stored by the backend
- **Merge rule**: Higher version wins; equal versions merge shallowly

## Conversation ID Lifecycle

Conversation IDs persist in the URL for bookmarking and reload:

```
/?conv_id=<uuid>  → loads existing conversation
/                → generates new conv_id on first send
```

**Priority chain:**

1. URL params (primary)
2. State store (fallback)
3. Generate new UUID

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts` | WebSocket connection manager |
| `pinocchio/cmd/web-chat/web/src/sem/registry.ts` | SEM event routing |
| `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts` | Timeline state management |
| `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx` | Main chat component |

## Error Handling Patterns

### WebSocket Disconnects

The `wsManager` handles disconnects by cleaning up state. On reconnect, hydrate from the timeline endpoint before processing new events:

```typescript
// Reconnection pattern
wsManager.connect({
  convId,
  basePrefix,
  dispatch,
  onDisconnect: () => {
    // Optionally show "reconnecting..." UI
  },
  onReconnect: async () => {
    // Re-hydrate from timeline to fill any gaps
    const resp = await fetch(`/timeline?conv_id=${convId}&since_version=${lastVersion}`);
    const entities = await resp.json();
    entities.forEach((e: any) => dispatch(timelineSlice.actions.upsertEntity(e)));
  },
});
```

### Backend Error Events

Backend errors arrive as SEM frames with type `error` or as HTTP error responses. Handle both paths:

```typescript
// SEM error events (from WebSocket)
registerSem('error', (ev, dispatch) => {
  dispatch(timelineSlice.actions.addEntity({
    id: ev.id,
    kind: 'error',
    createdAt: Date.now(),
    props: {
      message: (ev.data as any)?.error ?? 'Unknown error',
    },
  }));
});

// HTTP error responses (from POST /chat)
async function sendMessage(prompt: string, convId: string) {
  const resp = await fetch('/chat', {
    method: 'POST',
    body: JSON.stringify({ prompt, conv_id: convId }),
  });
  if (!resp.ok) {
    const body = await resp.text();
    // Show error in UI — don't silently swallow
    throw new Error(`Chat request failed (${resp.status}): ${body}`);
  }
  return resp.json();
}
```

### Hydration Race Conditions

When the WebSocket connects and hydration runs simultaneously, events can arrive before hydration completes. The `wsManager` addresses this with a hydration gate:

1. **Buffer phase**: WebSocket events are buffered during hydration
2. **Hydration**: `/timeline` response loads durable entities into the store
3. **Replay phase**: Buffered events are replayed in order
4. **Live phase**: Subsequent events are dispatched immediately

If you build a custom integration, ensure hydration completes before processing live events — otherwise streaming entities may appear before their context (e.g., a `tool.result` before its `tool.start`).

### Stale Entity Merging

When hydration loads entities that also arrive via WebSocket, version-based merging prevents data loss:

- Entity with **higher version** wins
- Entity with **equal version** merges `props` shallowly (WebSocket data may have more recent streaming state)
- Entity with **lower version** is ignored

This means you don't need to deduplicate manually — the timeline store handles it.

## Best Practices

- **One connection per conversation**: Avoid duplicate WebSocket connections
- **Handle disconnects gracefully**: Reconnect and re-hydrate timeline
- **Use hydration on reconnect**: Restore state from `/timeline`
- **Don't ignore HTTP errors**: Always check POST /chat responses and surface failures to the user
- **Trust the hydration gate**: Don't process WebSocket events before hydration completes

## See Also

- [SEM and UI](webchat-sem-and-ui.md) — SEM event types and UI mapping
- [Backend Reference](webchat-backend-reference.md) — Backend API contracts
- [Debugging and Ops](webchat-debugging-and-ops.md) — Troubleshooting
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage
