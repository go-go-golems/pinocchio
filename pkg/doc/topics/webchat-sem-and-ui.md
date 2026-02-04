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
    "delta": "Hi"
  }
}
```

Pinocchio uses protobuf-backed payloads under the hood (see `sem/pb/` directory), but frames are transmitted as JSON for WebSocket compatibility.

## Event Types

### LLM Events

| Type | Payload | Description |
|------|---------|-------------|
| `llm.start` | `{ id }` | Model started generating |
| `llm.delta` | `{ id, delta, cumulative? }` | Incremental text chunk |
| `llm.final` | `{ id, text? }` | Generation complete |

### Tool Events

| Type | Payload | Description |
|------|---------|-------------|
| `tool.start` | `{ id, name, input }` | Tool invocation started |
| `tool.delta` | `{ id, progress? }` | Tool progress update |
| `tool.result` | `{ id, result }` | Tool execution result |
| `tool.done` | `{ id }` | Tool execution complete |
| `tool.log` | `{ id, message, level? }` | Tool log message |

### Status Events

| Type | Payload | Description |
|------|---------|-------------|
| `log` | `{ message, level }` | Backend log message |
| `status` | `{ text, type }` | Status update |

### Middleware Events

These events are emitted by optional middleware/features (e.g., thinking mode). Planning events are only emitted if explicitly enabled elsewhere.

| Type | Description |
|------|-------------|
| `planning.*` | Planning lifecycle events (optional/legacy) |
| `thinking.mode.*` | Thinking mode indicators |
| `mode.evaluation.*` | Mode evaluation results |

## Handler Registration

Handlers are registered in the SEM registry:

```typescript
// sem/registry.ts
type SemHandler = (ctx: { ev: BaseEvent; now: number; convId: string }) => SemCmd | null;

const registry = new Map<string, SemHandler>();

export function registerSem(type: string, handler: SemHandler): void {
  registry.set(type, handler);
}

export function handleSem(ctx: { ev: BaseEvent; now: number; convId: string }): SemCmd | null {
  const handler = registry.get(ctx.ev.type);
  return handler ? handler(ctx) : null;
}
```

### Handler Pattern

```typescript
registerSem('llm.delta', ({ ev, now, convId }) => {
  return {
    kind: 'upsert',
    convId,
    entity: {
      id: ev.id,
      kind: 'message',
      timestamp: now,
      props: {
        role: 'assistant',
        streaming: true,
        delta: ev.delta ?? '',
      },
    },
  };
});
```

### Command Types

| Command | Use When |
|---------|----------|
| `AddCmd` | Event creates new entity |
| `UpsertCmd` | Event updates existing entity by ID |

## Timeline Entities

### Base Entity Shape

```typescript
type TimelineEntity = {
  id: string;
  kind: string;
  timestamp: number;
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
| `tool_log` | Tool logs | `tool.log` |
| `status` | Status banners | `log`, `status` events |

### Message Entity

```typescript
{
  id: "asst-123",
  kind: "message",
  timestamp: 1763501040615,
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
  timestamp: 1763501041000,
  props: {
    name: "search",
    input: { query: "..." },
    status: "running",
    progress: 0.5,
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
Handler returns SemCmd (add/upsert)
    ↓
State store updated
    ↓
UI component re-renders
```

## Hydration vs Streaming

Entities come from two sources:

- **Streaming** (WebSocket): Frames include `event.seq` (monotonic stream order); timeline entities use `version = seq`.
- **Hydration** (HTTP): Snapshots use the same version values stored by the backend.

**Merge rules:**

- Higher version wins
- Equal versions merge shallowly
- Hydration can overwrite stale streaming data

## Adding New Event Handlers

1. **Define the handler** in `sem/handlers/` directory
2. **Call `registerSem`** at module load time
3. **Ensure module is imported** in app bundle
4. **Add UI component** to render the entity kind
5. **Verify** with `?ws_debug=1` logs

### Implementation Tips

- Keep handlers idempotent (safe to replay)
- Derive entity IDs from `ev.id`
- Use `upsert` for updates, `add` for new entities
- Log unhandled events for debugging

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/pkg/webchat/sem_translator.go` | Backend event translation |
| `pinocchio/cmd/web-chat/web/src/sem/registry.ts` | Frontend SEM registry |
| `pinocchio/cmd/web-chat/web/src/sem/pb/` | Protobuf definitions |
| `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts` | Timeline state |

## See Also

- [Frontend Integration](webchat-frontend-integration.md) — WebSocket and HTTP patterns
- [Backend Reference](webchat-backend-reference.md) — StreamCoordinator API
- [Debugging and Ops](webchat-debugging-and-ops.md) — Troubleshooting
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage
