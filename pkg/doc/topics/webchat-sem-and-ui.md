---
Title: Webchat SEM Events and UI
Slug: webchat-sem-and-ui
Short: SEM event format, routing patterns, and timeline entity types.
Topics:
- webchat
- sem
- frontend
- events
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

SEM (Stream Event Message) events are the communication protocol between backend and frontend. Events are translated from Geppetto events to SEM frames, sent over WebSocket, and routed through handlers to update timeline state.

## Event Format

SEM events arrive as JSON frames:

```json
{
  "sem": true,
  "event": {
    "type": "llm.delta",
    "id": "eae401d8-...",
    "seq": 1707053365123000000,
    "stream_id": "1707053365123-0",
    "data": { "cumulative": "Hi" }
  }
}
```

`event.seq` is always present and monotonic. When Redis stream metadata is present (`xid`/`redis_xid`), it is derived from that stream ID; otherwise the backend falls back to a time-based sequence. `event.stream_id` is optional.

Pinocchio uses protobuf-backed payloads under the hood (see `sem/pb/` directory), but frames are transmitted as JSON for WebSocket compatibility.

## Event Types

### LLM Events

| Type | Payload | Description |
|------|---------|-------------|
| `llm.start` | `{ role? }` | Model started generating |
| `llm.delta` | `{ cumulative }` | Incremental text (cumulative) |
| `llm.final` | `{ text? }` | Generation complete |
| `llm.thinking.start` | `{ role? }` | Thinking stream started |
| `llm.thinking.delta` | `{ cumulative }` | Thinking stream delta |
| `llm.thinking.final` | `{}` | Thinking stream complete |

### Tool Events

| Type | Payload | Description |
|------|---------|-------------|
| `tool.start` | `{ id, name, input }` | Tool invocation started |
| `tool.delta` | `{ patch }` | Tool update patch |
| `tool.result` | `{ result, customKind? }` | Tool execution result |
| `tool.done` | `{ id }` | Tool execution complete |

### UI / System Events

| Type | Payload | Description |
|------|---------|-------------|
| `log` | `{ message, level, fields? }` | Backend log message |
| `agent.mode` | `{ title, data }` | Agent mode widget |
| `debugger.pause` | `{ pauseId, phase, summary }` | Step-controller pause prompt |
| `thinking.mode.started` | `{ mode, phase, reasoning }` | Thinking mode widget |
| `thinking.mode.update` | `{ mode, phase, reasoning }` | Thinking mode widget update |
| `thinking.mode.completed` | `{ mode, phase, reasoning, success }` | Thinking mode widget complete |
| `planning.start` | `{ run }` | Planning widget (start) |
| `planning.iteration` | `{ run, iterationIndex, ... }` | Planning widget update |
| `planning.reflection` | `{ run, iterationIndex, ... }` | Planning widget reflection |
| `planning.complete` | `{ run, ... }` | Planning widget complete |
| `execution.start` | `{ runId, ... }` | Planning widget execution status |
| `execution.complete` | `{ runId, status, ... }` | Planning widget execution status |
| `timeline.upsert` | `{ entity, version }` | Durable timeline entity upsert |

## Handler Registration

Handlers are registered in the SEM registry:

```typescript
// sem/registry.ts
type Handler = (ev: SemEvent, dispatch: AppDispatch) => void;

const handlers = new Map<string, Handler>();

export function registerSem(type: string, handler: Handler) {
  handlers.set(type, handler);
}

export function handleSem(envelope: any, dispatch: AppDispatch) {
  if (!envelope || envelope.sem !== true || !envelope.event) return;
  const ev = envelope.event as SemEvent;
  const h = handlers.get(ev.type);
  if (!h) return;
  h(ev, dispatch);
}
```

### Handler Pattern

```typescript
registerSem('llm.delta', (ev, dispatch) => {
  const cumulative = (ev.data as any)?.cumulative ?? '';
  dispatch(
    timelineSlice.actions.upsertEntity({
      id: ev.id,
      kind: 'message',
      createdAt: Date.now(),
      updatedAt: Date.now(),
      props: { content: String(cumulative), streaming: true },
    }),
  );
});
```

## Timeline Entities

### Base Entity Shape

```typescript
type TimelineEntity = {
  id: string;
  kind: string;
  createdAt: number;
  updatedAt?: number;
  version?: number;
  props: Record<string, unknown>;
};
```

### Entity Types

| Kind | Description | Created By |
|------|-------------|------------|
| `message` | User or assistant text | `llm.*` events |
| `tool_call` | Tool invocation | `tool.start` + updates |
| `tool_result` | Tool output | `tool.result` |
| `log` | Log message | `log` |
| `thinking_mode` | Thinking mode widget | `thinking.mode.*` |
| `planning` | Planning widget | `planning.*`, `execution.*` |
| `agent_mode` | Agent mode widget | `agent.mode` |
| `debugger_pause` | Step-controller pause | `debugger.pause` |
| `default` | Fallback card | Unknown kinds |

### Message Entity

```typescript
{
  id: "asst-123",
  kind: "message",
  createdAt: 1763501040615,
  props: {
    role: "assistant",
    content: "Hello!",
    streaming: false,
  }
}
```

### Tool Call Entity

```typescript
{
  id: "tool-456",
  kind: "tool_call",
  createdAt: 1763501041000,
  props: {
    name: "search",
    input: { query: "..." },
    done: false,
  }
}
```

## Routing Flow

```
Backend emits event
    ↓
StreamCoordinator translates → SEM frame
    ↓
WebSocket delivers → wsManager
    ↓
handleSem() looks up handler in registry
    ↓
Handler dispatches timelineSlice actions
    ↓
State store updated
    ↓
UI component re-renders
```

## Hydration vs Streaming

Entities come from two sources:

- **Streaming** (WebSocket): Frames include `event.seq` (monotonic stream order); most streaming entities do not carry a version.
- **Hydration** (HTTP): Snapshots include per-entity `version` values stored by the backend.

**Merge rules:**

- Higher version wins
- Equal versions merge shallowly
- Hydration can overwrite stale streaming data

## Adding New Event Handlers

1. **Add a handler** in `sem/registry.ts` (or import a helper module that calls `registerSem`)
2. **Map to a timeline entity** via `addEntity`/`upsertEntity`
3. **Add a renderer** in `webchat/cards.tsx` (or rely on `GenericCard`)
4. **Wire the renderer** in `ChatWidget` via the `renderers` map if it needs custom UI
5. **Verify** by watching WS frames and timeline state

### Implementation Tips

- Keep handlers idempotent (safe to replay)
- Derive entity IDs from `ev.id`
- Use `upsertEntity` for updates, `addEntity` for new entities
- Log unhandled events for debugging

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/pkg/webchat/sem_translator.go` | Backend event translation |
| `pinocchio/cmd/web-chat/web/src/sem/registry.ts` | Frontend SEM registry |
| `pinocchio/cmd/web-chat/web/src/sem/pb/` | Protobuf definitions |
| `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts` | Timeline state |
| `pinocchio/cmd/web-chat/web/src/webchat/cards.tsx` | Default entity renderers |
| `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx` | Renderer wiring |

## See Also

- [Frontend Integration](webchat-frontend-integration.md) — WebSocket and HTTP patterns
- [Backend Reference](webchat-backend-reference.md) — StreamCoordinator API
- [Debugging and Ops](webchat-debugging-and-ops.md) — Troubleshooting
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage
