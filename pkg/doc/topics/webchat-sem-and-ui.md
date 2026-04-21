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

Pinocchio uses protobuf-authored contracts under the hood, but frames are transmitted as JSON for WebSocket compatibility.

- Shared/core SEM contracts are generated under `pinocchio/pkg/sem/pb/`.
- App-owned web-chat proto sources live under `pinocchio/cmd/web-chat/proto/`.
- Current canonical custom-event translation for the live app happens in `pinocchio/pkg/evtstream/apps/chat/chat.go`, and the frontend mapping lives in `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`.

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
| `agent.mode.preview` | `{ messageId, title, candidateMode, analysis, rawText }` | Provisional agent-mode preview card |
| `agent.mode` | `{ messageId, title, from, to, analysis }` | Committed agent-mode card |
| `timeline.upsert` | `{ entity, version }` | Durable timeline entity upsert |

## SEM Frame Payload Examples

### LLM Streaming Sequence

A typical assistant response produces three frames:

**`llm.start`** — opens a new message entity:

```json
{
  "sem": true,
  "event": {
    "type": "llm.start",
    "id": "msg-a1b2c3d4",
    "seq": 1707053365100000000,
    "data": { "role": "assistant" }
  }
}
```

**`llm.delta`** — cumulative text (not just the new chunk):

```json
{
  "sem": true,
  "event": {
    "type": "llm.delta",
    "id": "msg-a1b2c3d4",
    "seq": 1707053365200000000,
    "data": { "cumulative": "Hello! How can I" }
  }
}
```

**`llm.final`** — closes the message:

```json
{
  "sem": true,
  "event": {
    "type": "llm.final",
    "id": "msg-a1b2c3d4",
    "seq": 1707053365300000000,
    "data": { "text": "Hello! How can I help you today?" }
  }
}
```

### Tool Call Sequence

**`tool.start`** — tool invocation begins:

```json
{
  "sem": true,
  "event": {
    "type": "tool.start",
    "id": "tc-e5f6g7h8",
    "seq": 1707053365400000000,
    "data": {
      "id": "tc-e5f6g7h8",
      "name": "search",
      "input": "{\"query\": \"weather in Paris\"}"
    }
  }
}
```

**`tool.result`** — tool returns output:

```json
{
  "sem": true,
  "event": {
    "type": "tool.result",
    "id": "tc-e5f6g7h8",
    "seq": 1707053365500000000,
    "data": {
      "result": "{\"temperature\": 18, \"condition\": \"cloudy\"}",
      "customKind": ""
    }
  }
}
```

**`tool.done`** — signals tool completion:

```json
{
  "sem": true,
  "event": {
    "type": "tool.done",
    "id": "tc-e5f6g7h8",
    "seq": 1707053365600000000,
    "data": { "id": "tc-e5f6g7h8" }
  }
}
```

### Thinking Sequence

```json
{
  "sem": true,
  "event": {
    "type": "llm.thinking.start",
    "id": "msg-a1b2c3d4:thinking",
    "seq": 1707053365050000000,
    "data": { "role": "thinking" }
  }
}
```

Note: thinking events use the same message ID with `:thinking` appended, creating a separate timeline entity from the main assistant message.

### Log Event

```json
{
  "sem": true,
  "event": {
    "type": "log",
    "id": "log-i9j0k1l2",
    "seq": 1707053365700000000,
    "data": {
      "message": "Starting inference with model gpt-4",
      "level": "info",
      "fields": { "model": "gpt-4", "temperature": 0.7 }
    }
  }
}
```

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

#### Core Entities

| Kind | Description | Created By | Widget |
|------|-------------|------------|--------|
| `message` | User or assistant text (streaming supported) | `llm.*` events, hydration snapshots | `MessageWidget` |
| `tool_call` | Tool invocation card with progress | `tool.start` + updates via `tool.done` | `ToolCallWidget` |
| `tool_result` | Tool execution output | `tool.result` | `ToolResultWidget` |
| `log` | Backend log/status messages | `log` | `StatusWidget` |

#### Agent and Planning Entities

| Kind | Description | Created By | Widget |
|------|-------------|------------|--------|
| `agent_mode_preview` | Provisional agent-mode preview while structured output is still incomplete | `agent.mode.preview` | `AgentModeCard` |
| `agent_mode` | Committed agent-mode switch card | `agent.mode` | `AgentModeCard` |
| `planning` | Planning run with iterations and execution | `planning.*`, `execution.*` | `PlanningWidget` |

#### UI / Interaction Entities

| Kind | Description | Created By | Widget |
|------|-------------|------------|--------|
| `debugger_pause` | Step-controller pause prompt | `debugger.pause` | `DebugPauseWidget` |
| `multiple_choice` | Multiple choice selection prompt | `multiple.choice.*` | `MultipleChoiceWidget` |
| `default` | Fallback card for unknown kinds | (any unregistered kind) | `GenericCard` |

#### Application-Specific Entities (go-go-mento examples)

These entity kinds exist in the go-go-mento reference application and illustrate how teams extend the widget system for domain-specific features:

| Kind | Description | Created By |
|------|-------------|------------|
| `team_analysis` | Team analysis results | `team.analysis.*` |
| `debate_persona` | Debate persona entries | `debate.persona.*` |
| `debate_question` | Debate questions | `debate.question.*` |
| `debate_response` | Debate responses | `debate.response.*` |
| `debate_vote_prompt` | Debate vote prompts | `debate.vote.*` |
| `inner_thoughts` | Inner thoughts display | `inner.thoughts.*` |
| `mode_evaluation` | Mode evaluation results | `mode.evaluation.*` |
| `drive_document` | Google Drive document results | `tool.result` (with `customKind`) |

These demonstrate the pattern: define a `kind`, register a SEM handler that dispatches `upsertEntity`/`addEntity`, and register a widget renderer for that kind.

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

## Adding New Widgets

### Quick Steps

1. **Design the entity shape** — decide what `props` your React component needs
2. **Create the SEM handler** — register via `registerSem()` to emit `addEntity` or `upsertEntity`
3. **Create the React widget** — component that renders the entity's props
4. **Register the widget renderer** — use `registerTimelineRenderer(kind, component)` in a feature module
5. **Wire the import** — ensure the handler and widget modules are imported at startup
6. **Verify** — watch WS frames, check Redux state, confirm rendering

For a complete end-to-end tutorial with code examples for each layer, see [Adding a New Event Type](webchat-adding-event-types.md).

### Choosing Entity Actions

| Action | When to Use | Example |
|--------|-------------|---------|
| `addEntity` | Event creates a new, one-shot entity | Notifications, log messages |
| `upsertEntity` | Event updates an existing entity by stable ID | Streaming text, progress bars, tool calls |

### Handler Implementation Tips

- **Keep handlers idempotent** — they may run multiple times during hydration replay
- **Derive entity IDs from `ev.id`** — ensures consistency between streaming and hydration
- **Use protobuf decoding** for complex payloads (`fromJson` from `@bufbuild/protobuf`)
- **Log unhandled events** for debugging (the registry silently drops unknown types by default)

### Path Conventions

Widget files follow these conventions:

- Feature modules live under `pinocchio/cmd/web-chat/web/src/features/<feature>/`
- Frontend registration for custom kinds should be colocated with the feature module (SEM handler + props normalizer + renderer)
- Generated TS protobuf files for app-owned middleware live under `pinocchio/cmd/web-chat/web/src/features/<feature>/pb/`
- File names should match the component name in PascalCase; keep each feature widget self-contained

## Debugging SEM Events

### Enable WebSocket Debug Logging

Add `?ws_debug=1` to the URL to enable verbose WebSocket logging in the browser console. All SEM-related messages are prefixed with `[sem]` or `[ws]`.

### Decision Tree: Event Not Reaching the UI

```
Event not appearing in UI?
│
├── Check backend logs for "[ws.mgr] message:forward"
│   ├── Not present → Event not reaching StreamCoordinator
│   │   └── Verify event is being published via PublishEventToContext()
│   └── Present → SEM frame is being broadcast
│
├── Check browser WS frames (DevTools → Network → WS)
│   ├── Frame not received → ConnectionPool issue
│   │   └── Check pool.Count() > 0, verify connection is alive
│   └── Frame received → Check SEM routing
│
├── Check browser console for "[sem] event:routed"
│   ├── Not present → Handler not registered or import missing
│   │   └── Verify the handler module is imported (side-effect import)
│   └── Present → Handler fired
│
└── Check Redux DevTools → timeline.byId
    ├── Entity missing → Handler didn't dispatch correctly
    └── Entity present → Widget not rendering for this kind
        └── Verify widget is registered for the entity's kind
```

### Common Issues

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| Handler never fires | Module not imported | Add `import './handlers/myHandler'` to the SEM index |
| Entity appears but widget is blank | `kind` mismatch | Ensure entity `kind` matches the registered widget kind exactly |
| Entity appears only during streaming, gone on reload | Missing projector case | Add a case in `TimelineProjector.ApplySemFrame()` |
| Duplicate entities | Unstable IDs | Use `ev.id` consistently; ensure ID resolution is deterministic |
| Old data after reconnect | Hydration version mismatch | Check `since_version` parameter in `/api/timeline` request |

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/pkg/webchat/sem_translator.go` | Backend event translation for the legacy SEM path |
| `pinocchio/pkg/evtstream/apps/chat/chat.go` | Canonical custom-event translation and hydration for the live app cutover path |
| `pinocchio/cmd/web-chat/web/src/sem/registry.ts` | Frontend SEM registry for legacy/custom SEM handlers |
| `pinocchio/cmd/web-chat/proto/` | App-owned middleware/timeline proto definitions |
| `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts` | Canonical frontend mapping from snapshots/UI events into timeline entities |
| `pinocchio/cmd/web-chat/web/src/ws/wsManager.test.ts` | Focused tests for canonical agent-mode/custom-event mapping |
| `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts` | Timeline state |
| `pinocchio/cmd/web-chat/web/src/webchat/cards.tsx` | Default entity renderers including `AgentModeCard` |
| `pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts` | Renderer wiring |

## See Also

- [Adding a New Event Type](webchat-adding-event-types.md) — End-to-end tutorial for adding custom events
- [Frontend Integration](webchat-frontend-integration.md) — WebSocket and HTTP patterns
- [Backend Internals](webchat-backend-internals.md) — Timeline projector, StreamCoordinator internals
- [Backend Reference](webchat-backend-reference.md) — StreamCoordinator API
- [Debugging and Ops](webchat-debugging-and-ops.md) — Troubleshooting
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage
- [Events (geppetto)](../../../../geppetto/pkg/doc/topics/04-events.md) — Geppetto event system and SEM translation bridge
