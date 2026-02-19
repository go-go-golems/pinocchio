# Pinocchio Webchat Flow: From SEM Event to Widget/Card Render

## Abstract

This document explains, in implementation detail, how Pinocchio webchat turns backend events into rendered UI cards, and how developers can extend rendering behavior without changing core framework code. The focus is the current code path in `pinocchio/cmd/web-chat/web/src` and `pinocchio/pkg/webchat`, with special attention to custom types, custom renderers, and widget switching semantics.

The key conclusion is that Pinocchio webchat is intentionally generic at the UI boundary. The primary renderer switch is by timeline entity `kind`, and secondary specialization can be done by inspecting payload fields such as `customKind` on `tool_result`. This is less hardcoded than older app-specific render pipelines.

---

## 1. System Model and Vocabulary

Pinocchio webchat has three major transformation stages:

1. Domain events -> SEM envelopes (backend translation).
2. SEM envelopes -> timeline entities (backend projection and frontend mapping).
3. Timeline entities -> React renderers (frontend renderer dispatch).

A simplified event contract used across the stack is:

```json
{
  "sem": true,
  "event": {
    "type": "tool.result",
    "id": "tc-1",
    "seq": 1739960001000001,
    "stream_id": "1739960001000-1",
    "data": { "id": "tc-1", "result": "2", "customKind": "calc_result" }
  }
}
```

Important terms:

- `SEM envelope`: wire-level frame (`sem=true`, `event` payload).
- `Timeline entity`: normalized UI projection unit (`id`, `kind`, `props`, timestamps).
- `kind`: primary renderer routing key (for example: `message`, `tool_result`, `thinking_mode`).
- `customKind`: optional subtype hint, currently used in `tool.result` payloads.

---

## 2. End-to-End Data Flow (Concrete Path)

### 2.1 High-level diagram

```text
Geppetto/Runtime Events
        |
        v
EventTranslator (Go)
  pinocchio/pkg/webchat/sem_translator.go
        |
        v
SEM frames (with seq/stream_id patched)
  pinocchio/pkg/webchat/stream_coordinator.go
        |
        +--> WebSocket broadcast to clients
        |
        +--> TimelineProjector.ApplySemFrame(...)
              pinocchio/pkg/webchat/timeline_projector.go
              writes TimelineStore + emits timeline.upsert
        |
        v
Frontend wsManager
  pinocchio/cmd/web-chat/web/src/ws/wsManager.ts
        |
        +--> handleSem(...) for live frames
        +--> hydration via GET /api/timeline
        |
        v
timelineSlice (Redux)
  pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts
        |
        v
ChatWidget -> ChatTimeline renderer switch
  pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx
  pinocchio/cmd/web-chat/web/src/webchat/components/Timeline.tsx
```

### 2.2 Route ownership and entrypoints

The command app mounts app-owned handlers in `pinocchio/cmd/web-chat/main.go`:

- `/chat` -> `webhttp.NewChatHandler(...)`
- `/ws` -> `webhttp.NewWSHandler(...)`
- `/api/timeline` -> `webhttp.NewTimelineHandler(...)`

References:

- `pinocchio/cmd/web-chat/main.go:195`
- `pinocchio/cmd/web-chat/main.go:204`
- `pinocchio/cmd/web-chat/main.go:207`

---

## 3. Backend Stage A: Domain Event -> SEM Envelope

### 3.1 Translator registry

`EventTranslator` delegates mappings through registry handlers:

- `semregistry.RegisterByType[T](...)`
- `semregistry.Handle(e)`

References:

- `pinocchio/pkg/sem/registry/registry.go:20`
- `pinocchio/pkg/webchat/sem_translator.go:155`

This means semantic mapping is type-driven and extensible on the Go side.

### 3.2 Built-in mappings

`EventTranslator.RegisterDefaultHandlers()` maps core event classes to SEM event types:

- LLM: `llm.start`, `llm.delta`, `llm.final`, `llm.thinking.*`
- Tools: `tool.start`, `tool.delta`, `tool.result`, `tool.done`
- Middleware-ish/control: `agent.mode`, `debugger.pause`, `thinking.mode.*`

Reference: `pinocchio/pkg/webchat/sem_translator.go:243`

### 3.3 `customKind` creation for tool results

`EventToolResult` and `EventToolCallExecutionResult` can emit `customKind` for `calc` tool:

- `ToolResult{custom_kind: "calc_result"}`

Reference:

- `pinocchio/pkg/webchat/sem_translator.go:426`
- `pinocchio/pkg/webchat/sem_translator_test.go:81`

This is the canonical subtype hint path that avoids introducing a brand-new top-level SEM event type for every tool result widget variant.

### 3.4 Sequence assignment and normalization

`StreamCoordinator` assigns stable monotonic `seq` and attaches `stream_id`:

- `SemanticEventsFromEventWithCursor(...)`
- `patchSEMPayloadWithCursor(...)`

Reference: `pinocchio/pkg/webchat/stream_coordinator.go:145`

This is critical for deterministic ordering and reconciliation in the browser.

---

## 4. Backend Stage B: SEM -> Timeline Projection

### 4.1 Timeline projector role

`TimelineProjector.ApplySemFrame(...)` converts SEM frames into typed timeline snapshots and persists them to `TimelineStore`.

Reference: `pinocchio/pkg/webchat/timeline_projector.go:83`

### 4.2 Projection output model

Projection writes `TimelineEntityV1` with a `kind` plus `oneof snapshot` payload.

Reference: `pinocchio/proto/sem/timeline/transport.proto:15`

Built-in snapshot variants include:

- `message`, `tool_call`, `tool_result`, `status`
- `thinking_mode`, `mode_evaluation`, `inner_thoughts`
- `team_analysis`, `disco_dialogue_line/check/state`

Reference: `pinocchio/proto/sem/timeline/transport.proto:22`

### 4.3 Tool result identity strategy

For `tool.result`, projector writes:

- ID: `<toolCallId>:result`, or `<toolCallId>:custom` when `customKind` is set
- Kind: `tool_result`
- Payload: raw + structured + `custom_kind`

Reference: `pinocchio/pkg/webchat/timeline_projector.go:327`

This mirrors frontend live-sem behavior and preserves subtype hint continuity across hydration.

### 4.4 Custom timeline handler registry

Before switch-based built-ins run, projector checks custom timeline handlers:

- `RegisterTimelineHandler(eventType, handler)`
- `handleTimelineHandlers(...)`

References:

- `pinocchio/pkg/webchat/timeline_registry.go:28`
- `pinocchio/pkg/webchat/timeline_projector.go:117`

Built-in example:

- `chat.message` -> `message` snapshot handler (`timeline_handlers_builtin.go`)

Reference: `pinocchio/pkg/webchat/timeline_handlers_builtin.go:10`

This is an important extension seam: backend teams can project additional SEM event types without editing projector switch logic, as long as payloads map to existing timeline snapshot types.

### 4.5 `timeline.upsert` publication

After store upsert, router/service emits websocket frame:

```text
event.type = "timeline.upsert"
event.id   = entity.Id
event.seq  = version
event.data = TimelineUpsertV1
```

References:

- `pinocchio/pkg/webchat/timeline_upsert.go:22`
- `pinocchio/pkg/webchat/conversation_service.go:233`

This gives a stable projection-first stream to the frontend.

---

## 5. Frontend Ingestion: WebSocket, Hydration, Registry

### 5.1 Connection lifecycle

`ChatWidget` calls `wsManager.connect(...)` after resolving `conv_id`.

Reference: `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:121`

`wsManager.connect(...)` performs:

1. `registerDefaultSemHandlers()`
2. open websocket `/ws?conv_id=...`
3. buffer incoming frames until hydration completes
4. `GET /api/timeline?conv_id=...`
5. replay buffered frames in `seq` order

References:

- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:74`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:121`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:171`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:212`

### 5.2 Registry dispatch

Frontend SEM dispatch is map-driven:

- `registerSem(type, handler)`
- `handleSem(envelope, dispatch)`

Reference: `pinocchio/cmd/web-chat/web/src/sem/registry.ts:36`

Default handlers include both live event shapes and projection events:

- live: `llm.*`, `tool.*`, `agent.mode`, `debugger.pause`, `thinking.mode.*`
- projection: `timeline.upsert`

Reference: `pinocchio/cmd/web-chat/web/src/sem/registry.ts:69`

### 5.3 Important caveat: registry reset behavior

`registerDefaultSemHandlers()` starts with `handlers.clear()`. Because `wsManager.connect()` calls it on every connect, ad-hoc custom registrations done earlier can be overwritten.

References:

- `pinocchio/cmd/web-chat/web/src/sem/registry.ts:70`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:74`

For extension design, this means UI-only custom SEM handlers are possible in principle, but fragile in the stock app unless you control registration timing or wrapper lifecycle.

---

## 6. Frontend Projection Mapping and State Semantics

### 6.1 Timeline snapshot mapper

`timelineEntityFromProto(...)` converts protobuf snapshot entities to Redux timeline entities.

Reference: `pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts:91`

For known kinds, props are normalized; for unknown/unhandled oneof cases, mapper falls back to raw `value`.

Reference: `pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts:88`

### 6.2 State shape and upsert policy

Redux slice stores:

- `byId: Record<string, TimelineEntity>`
- `order: string[]`

Reference: `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts:13`

`upsertEntity` merges props and honors `version` monotonicity when present.

Reference: `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts:59`

This is why timeline-first flows are robust under reconnect/hydration: versioned snapshots win over stale updates.

---

## 7. Renderer Dispatch and Widget Switching

### 7.1 Primary switch: by `kind`

`ChatTimeline` chooses renderer by exact entity `kind`:

```ts
const Renderer = renderers[e.kind] ?? renderers.default;
```

Reference: `pinocchio/cmd/web-chat/web/src/webchat/components/Timeline.tsx:102`

### 7.2 Default renderer map

`ChatWidget` composes built-in renderer map and merges caller overrides:

- `message` -> `MessageCard`
- `tool_call` -> `ToolCallCard`
- `tool_result` -> `ToolResultCard`
- `log` -> `LogCard`
- `thinking_mode` -> `ThinkingModeCard`
- fallback -> `GenericCard`

Reference: `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:218`

### 7.3 Secondary switch: subtype inside renderer

Current default `ToolResultCard` does not switch component by `customKind`; it displays `customKind` as a label and raw result text.

Reference: `pinocchio/cmd/web-chat/web/src/webchat/cards.tsx:65`

Therefore, subtype-aware widget switching is expected to be implemented by host-provided custom renderer logic.

### 7.4 Which middleware kinds are custom-rendered by default?

Today, only `thinking_mode` has a dedicated middleware card. Other middleware-related kinds (`agent_mode`, `debugger_pause`, `disco_dialogue_*`, etc.) fall through to `GenericCard` unless custom renderers are supplied.

References:

- `pinocchio/cmd/web-chat/web/src/sem/registry.ts:183`
- `pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts:39`
- `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:225`

---

## 8. Extension Patterns Without Core Code Changes

This section is the practical developer playbook.

## 8.1 Pattern A: Override renderer for an existing `kind`

If backend already emits a known `kind` (for example, `agent_mode` or `tool_result`), pass a renderer via `ChatWidgetProps.renderers`.

Reference types:

- `ChatWidgetRenderers` in `pinocchio/cmd/web-chat/web/src/webchat/types.ts:74`
- story example in `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.stories.tsx:52`

Pseudocode:

```tsx
<ChatWidget
  renderers={{
    agent_mode: AgentModeCard,
    tool_result: ToolResultSwitchCard,
  }}
/>
```

No framework modification is required.

## 8.2 Pattern B: Switch by `customKind` inside `tool_result`

Use one renderer for `tool_result`, then branch on `e.props.customKind`.

Pseudocode:

```tsx
function ToolResultSwitchCard({ e }) {
  const customKind = String(e.props?.customKind ?? "");

  if (customKind === "calc_result") {
    return <CalcResultCard e={e} />;
  }
  if (customKind === "hypercard.widget.v1") {
    return <HypercardWidgetCard e={e} />;
  }
  if (customKind === "hypercard.card.v2") {
    return <HypercardCardCodeCard e={e} />;
  }

  return <DefaultToolResultCard e={e} />;
}
```

This is the lowest-friction “custom widget” strategy in current Pinocchio webchat.

## 8.3 Pattern C: Add backend semantic mapping via translator registry

If you own backend event types, register a Go translator handler via `semregistry.RegisterByType` so your event emits a standard SEM type and payload.

Pseudocode:

```go
semregistry.RegisterByType[*events.EventMyToolResult](func(ev *events.EventMyToolResult) ([][]byte, error) {
    data, _ := protoToRaw(&sempb.ToolResult{
        Id:         ev.ID,
        Result:     ev.Raw,
        CustomKind: "my_widget.v1",
    })
    return [][]byte{wrapSem(map[string]any{
        "type": "tool.result",
        "id":   ev.ID,
        "data": data,
    })}, nil
})
```

Reference: `pinocchio/pkg/webchat/sem_translator.go:243`

## 8.4 Pattern D: Add timeline projection handler without projector edits

Register custom timeline handler for your SEM type and upsert a supported snapshot kind.

Pseudocode:

```go
func init() {
    webchat.RegisterTimelineHandler("my.feature.status", func(ctx context.Context, p *webchat.TimelineProjector, ev webchat.TimelineSemEvent, now int64) error {
        // decode ev.Data and map to existing snapshot kind, e.g. status
        return p.Upsert(ctx, ev.Seq, &timelinepb.TimelineEntityV1{
            Id:   ev.ID,
            Kind: "status",
            Snapshot: &timelinepb.TimelineEntityV1_Status{Status: &timelinepb.StatusSnapshotV1{
                SchemaVersion: 1,
                Text:          "Custom status text",
                Type:          "info",
            }},
        })
    })
}
```

Reference: `pinocchio/pkg/webchat/timeline_registry.go:28`

This preserves hydration/reconnect behavior because projected entities are persisted.

## 8.5 Pattern E: Frontend custom SEM handlers (advanced caveat)

`registerSem("my.event", handler)` exists, but stock `ChatWidget` connection flow calls `registerDefaultSemHandlers()` which clears all handlers.

Reference:

- `pinocchio/cmd/web-chat/web/src/sem/registry.ts:36`
- `pinocchio/cmd/web-chat/web/src/sem/registry.ts:70`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:74`

So this route is viable only when you control lifecycle timing or own a wrapper around connection setup.

---

## 9. What You Can and Cannot Do Without Core Changes

### 9.1 You can do without core changes

- Replace any existing renderer by `kind` via `renderers` prop.
- Add subtype switching via `tool_result.customKind`.
- Add Go semantic mappings for custom event classes -> existing SEM types.
- Add timeline projection handlers for new SEM event types -> existing timeline snapshot kinds.

### 9.2 You cannot do without core changes

- Introduce truly new timeline payload schemas that are not represented in `TimelineEntityV1.oneof snapshot`.
- Persist/hydrate an entirely new `kind` with strongly typed payload unless proto + mapper are extended.
- Reliably add frontend SEM handlers in stock flow without accounting for registry reset behavior.

---

## 10. Practical Blueprint: Building a Custom Widget Stack

The following blueprint gives a stable, non-invasive extension path.

### Step 1: Emit custom hint from backend

Emit `tool.result` with `custom_kind` set to your widget discriminator (`my_widget.v1`).

Reference: `pinocchio/proto/sem/base/tool.proto:20`

### Step 2: Keep projection compatible

Ensure projector can preserve that result in `tool_result` snapshot (`custom_kind`, `result_raw`).

Reference: `pinocchio/pkg/webchat/timeline_projector.go:327`

### Step 3: Inject custom UI renderer

Pass custom `tool_result` renderer that dispatches by `customKind`.

Reference: `pinocchio/cmd/web-chat/web/src/webchat/types.ts:78`

### Step 4: Validate both live and hydrated flows

- Live: websocket frame path (`tool.result` event)
- Hydrated: `/api/timeline` snapshot path (`tool_result` entity)

References:

- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:115`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:171`

### Step 5: Add tests where responsibility lives

- Backend: translator test for `customKind`
- Frontend: renderer test for subtype branch
- Optional: timeline projector test for custom ID/result persistence

Example backend test already present:

- `pinocchio/pkg/webchat/sem_translator_test.go:81`

---

## 11. Control-Flow Pseudocode (Condensed)

### 11.1 Backend stream loop

```text
for each broker message:
  seq = nextSeq(stream_id)
  if payload already SEM:
    patch seq + stream_id
    onFrame(sem)
  else:
    event = decodeDomainEvent(payload)
    semFrames = translate(event)
    for semFrame in semFrames:
      inject seq + stream_id
      onFrame(semFrame)

onFrame:
  broadcast websocket frame
  append sem buffer
  timelineProjector.ApplySemFrame(frame)
```

### 11.2 Frontend runtime

```text
ChatWidget mounts
  -> wsManager.connect(convId)
       registerDefaultSemHandlers()
       open ws
       begin hydration GET /api/timeline
       buffer ws frames until hydrated
       apply snapshot entities to store
       replay buffered sem frames by seq

on each sem frame:
  handleSem(envelope)
    lookup handler by event.type
    dispatch timelineSlice.add/upsert

ChatTimeline render:
  entities = selectTimelineEntities()
  for e in entities:
    Renderer = renderers[e.kind] || renderers.default
    render <Renderer e={e}>
```

---

## 12. Architecture Notes for Teams Migrating from Hardcoded Widgets

If your previous app had hand-built widget/card event types (for example, explicit hypercard widget events), Pinocchio webchat’s architecture encourages a different decomposition:

- Keep transport generic.
- Preserve stable `kind` boundaries.
- Use subtype fields (`customKind`) for specialization.
- Move rendering policy into host-supplied renderer functions.

This typically reduces framework churn. New visual behaviors become application-level renderers, not framework-level switch cases.

A useful mental split is:

- Framework responsibilities:
  - transport, sequencing, hydration, entity lifecycle, default cards.
- Application responsibilities:
  - domain-specific visual semantics and specialized card/widget rendering.

---

## 13. Known Implementation Gaps and Design Implications

1. Frontend custom SEM handler registration is not first-class in stock widget lifecycle due default handler reset on connect.
2. Not every projected kind has a dedicated default renderer; many intentionally route to `GenericCard`.
3. Timeline proto constrains which payload schemas can be persisted and hydrated without core changes.
4. Some documentation pages describe renderer names that are not currently wired as defaults; always trust current code symbols.

This is not a defect by itself; it is a tradeoff favoring a minimal core renderer surface.

---

## 14. Developer Checklist (No-Core-Change Path)

1. Choose a routing strategy:
   - Existing `kind` override, or
   - `tool_result` + `customKind` subtype.
2. Emit SEM payload with stable IDs and deterministic `customKind` values.
3. Ensure timeline projection preserves needed data for hydration.
4. Inject `renderers` map in your ChatWidget host integration.
5. Build subtype switch logic in your custom renderer.
6. Test reconnect/hydration (`/api/timeline`) and live stream (`/ws`) parity.
7. Keep fallback renderer behavior explicit for unknown subtype values.

---

## 15. Reference Index

Backend semantic translation and stream orchestration:

- `pinocchio/pkg/webchat/sem_translator.go`
- `pinocchio/pkg/sem/registry/registry.go`
- `pinocchio/pkg/webchat/stream_coordinator.go`
- `pinocchio/pkg/webchat/conversation.go`
- `pinocchio/pkg/webchat/conversation_service.go`
- `pinocchio/pkg/webchat/timeline_projector.go`
- `pinocchio/pkg/webchat/timeline_registry.go`
- `pinocchio/pkg/webchat/timeline_upsert.go`

HTTP/websocket and timeline API:

- `pinocchio/cmd/web-chat/main.go`
- `pinocchio/pkg/webchat/http/api.go`
- `pinocchio/pkg/webchat/router_timeline_api.go`

Frontend ingestion and rendering:

- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
- `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
- `pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts`
- `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts`
- `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx`
- `pinocchio/cmd/web-chat/web/src/webchat/components/Timeline.tsx`
- `pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`
- `pinocchio/cmd/web-chat/web/src/webchat/types.ts`

Protocol schema:

- `pinocchio/proto/sem/base/tool.proto`
- `pinocchio/proto/sem/timeline/transport.proto`
- `pinocchio/proto/sem/timeline/tool.proto`
- `pinocchio/proto/sem/timeline/middleware.proto`

Validation examples:

- `pinocchio/pkg/webchat/sem_translator_test.go`
- `pinocchio/pkg/webchat/timeline_projector_test.go`
- `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.stories.tsx`


---

## 16. Temporal Walkthrough (One Prompt, One Tool, One Custom Result)

This section walks through a concrete timeline with sequencing semantics.

Assume a user sends a prompt that triggers one tool call (`calc`) and returns a custom-tagged result.

### 16.1 Sequence table

```text
T0  User POST /chat
T1  Backend emits domain event: EventToolCall(id=tc-1,name=calc)
T2  EventTranslator -> SEM tool.start(id=tc-1)
T3  StreamCoordinator assigns seq S, broadcasts frame
T4  TimelineProjector writes tool_call(tc-1, running), version S
T5  Router emits timeline.upsert(seq=S)
T6  Browser receives frame; registry upserts tool_call entity

T7  Backend emits domain event: EventToolResult(id=tc-1,result=2)
T8  EventTranslator -> SEM tool.result(customKind=calc_result) + tool.done
T9  StreamCoordinator assigns seq S+1, S+2 and broadcasts
T10 TimelineProjector writes tool_result(id=tc-1:custom, customKind=calc_result)
T11 TimelineProjector writes tool_call(tc-1, done=true)
T12 Browser upserts entities and rerenders
```

### 16.2 Why this matters for rendering

The renderer never sees domain events directly. It sees timeline entities (`RenderEntity`) after projection/mapping. Therefore UI behavior should be authored against stable `kind` + `props` contracts, not backend event classes.

This is exactly where Pinocchio is more generic than app-specific pipelines: semantic evolution happens in event and projection layers, while renderer switching remains simple and compositional.

---

## 17. Widget Switching Algorithm, Precisely

### 17.1 Primary dispatch function

In `ChatTimeline`:

1. Iterate entity list in store order.
2. Select renderer by `renderers[e.kind]`.
3. Fallback to `renderers.default`.
4. Render card with role-aligned bubble container.

Reference: `pinocchio/cmd/web-chat/web/src/webchat/components/Timeline.tsx:101`

### 17.2 Role influences layout, not renderer identity

`roleFromEntity` assigns visual role buckets:

- `message` -> `user` or `assistant` or `thinking`
- `tool_call`/`tool_result` -> `tool`
- `thinking_mode` and `disco_dialogue_*` -> `system`

Reference: `pinocchio/cmd/web-chat/web/src/webchat/components/Timeline.tsx:18`

This role tagging affects bubble alignment and styling, but does not select which React component handles content. Content handler selection remains purely `kind`-based.

### 17.3 `customKind` is payload-level polymorphism

`customKind` is not a top-level renderer key. It is a field in `tool_result` props:

- SEM: `sem.base.tool.ToolResult.custom_kind`
- Timeline snapshot: `sem.timeline.ToolResultSnapshotV1.custom_kind`
- Frontend props: `e.props.customKind`

References:

- `pinocchio/proto/sem/base/tool.proto:20`
- `pinocchio/proto/sem/timeline/tool.proto:19`
- `pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts:24`

This layered subtype marker is the intended hook for custom visual branching inside a single `tool_result` renderer.

---

## 18. Implementation Cookbook for Teams

This section is a full, no-core-modification recipe for custom cards.

### 18.1 Objective

Create two specialized UI cards:

- `hypercard.widget.v1`
- `hypercard.card.v2`

while keeping Pinocchio core untouched.

### 18.2 Backend recipe

1. Emit `tool.result` with `custom_kind` set to one of the two values.
2. Keep data in `result` (string, optionally JSON).
3. Emit `tool.done` as usual.

If your event source is a custom geppetto event class, register semantic mapper:

```go
semregistry.RegisterByType[*events.EventHypercardResult](func(ev *events.EventHypercardResult) ([][]byte, error) {
    tr := &sempb.ToolResult{Id: ev.CallID, Result: ev.RawPayload, CustomKind: ev.WidgetKind}
    data, err := protoToRaw(tr)
    if err != nil { return nil, err }

    td, err := protoToRaw(&sempb.ToolDone{Id: ev.CallID})
    if err != nil { return nil, err }

    return [][]byte{
      wrapSem(map[string]any{"type":"tool.result","id":ev.CallID,"data":data}),
      wrapSem(map[string]any{"type":"tool.done","id":ev.CallID,"data":td}),
    }, nil
})
```

### 18.3 Frontend recipe

Provide a custom `tool_result` renderer and route by `customKind`:

```tsx
type CardProps = { e: RenderEntity };

function HypercardWidgetCard({ e }: CardProps) {
  const raw = String(e.props?.result ?? "");
  return <div data-part="card">/* parse and render widget */{raw}</div>;
}

function HypercardCodeCard({ e }: CardProps) {
  const raw = String(e.props?.result ?? "");
  return <div data-part="card">/* parse and render card/code */{raw}</div>;
}

function RoutedToolResultCard({ e }: CardProps) {
  const kind = String(e.props?.customKind ?? "");
  if (kind === "hypercard.widget.v1") return <HypercardWidgetCard e={e} />;
  if (kind === "hypercard.card.v2") return <HypercardCodeCard e={e} />;
  return <ToolResultCard e={e} />;
}

<ChatWidget renderers={{ tool_result: RoutedToolResultCard }} />
```

### 18.4 Hydration consistency check

Because projector persists `custom_kind` in `tool_result` snapshot, both live and rehydrated entities keep the subtype selector.

Reference: `pinocchio/pkg/webchat/timeline_projector.go:356`

So subtype-specific widget routing survives reload.

---

## 19. Debugging and Verification Strategy

### 19.1 Backend checks

- Confirm SEM output includes expected `type`, `id`, `seq`, `customKind`.
- Confirm projector writes expected entity `kind` and ID form (`:custom` vs `:result`).
- Confirm `/api/timeline` includes entity with `tool_result.customKind`.

Useful references:

- `pinocchio/pkg/webchat/sem_translator_test.go:81`
- `pinocchio/pkg/webchat/timeline_projector.go:332`
- `pinocchio/pkg/webchat/router_timeline_api.go:70`

### 19.2 Frontend checks

- Verify `timelineSlice.byId` has entity props with `customKind`.
- Verify `renderers.tool_result` is injected (not shadowed).
- Verify fallback renderer handles unknown subtypes safely.

References:

- `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts:18`
- `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:218`

### 19.3 Reconnect checks

- Simulate full reload.
- Observe hydration first, then buffered replay.
- Confirm no duplicate cards and subtype preserved.

Reference: `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:167`

---

## 20. Comparison: Generic Pinocchio Model vs Hypercard-Specific Model

Pinocchio’s webchat model is deliberately generic in three ways.

### 20.1 Generic transport

SEM event types are typed but minimal; many UI distinctions are intentionally deferred to `kind`/`props` and optional subtype hints, rather than creating one event type per visual card.

### 20.2 Generic projection

Timeline entities provide a stable, replayable state model independent of transient stream timing. UI consumes projected entities, not transient transport semantics.

### 20.3 Generic rendering contract

Renderer switching is a data-driven map (`kind` -> React component), with host overrides via props.

In contrast, older hypercard-specific pipelines often bind semantic event names directly to bespoke components in core code. That can give quick feature delivery initially, but tends to increase coupling between backend event evolution and frontend component wiring.

Pinocchio’s approach lowers framework churn:

- new domain behavior often means new payload semantics, not new core switch cases;
- application teams own specialized rendering policy by passing renderers.

---

## 21. Advanced Notes: Choosing the Right Extension Surface

When adding a new visualization, choose extension surface by stability requirement.

### 21.1 If you only need live session rendering

You can rely on live SEM handler paths (`registerSem`) and ephemeral entities.

Tradeoff:

- fastest to prototype,
- weakest reconnect guarantees,
- easy to accidentally break with handler reset lifecycle.

### 21.2 If you need durable/replayable rendering

Prefer projection-first:

- backend emits SEM -> projector writes timeline entities,
- UI renders from hydrated timeline snapshots.

Tradeoff:

- requires payload to fit timeline schema,
- but gives deterministic hydration and versioned reconciliation.

### 21.3 If you need brand-new structured payload shapes

You will eventually need core-level schema expansion:

1. extend timeline proto oneof,
2. regenerate pb code,
3. extend projector mapping,
4. extend frontend `timelineMapper`,
5. add renderer override or default renderer.

This is a conscious schema evolution path, not an anti-pattern.

---

## 22. Final Mental Model for Developers

When deciding where to implement custom widgets, use the following rule set:

1. Route first by entity `kind`.
2. Sub-route by payload fields (`customKind`, `status`, etc.) only when needed.
3. Keep backend semantic translation separate from renderer concerns.
4. Preserve hydration by projecting durable entities whenever replay matters.
5. Treat frontend SEM registry customization as an advanced lifecycle-managed option.

If you follow these constraints, you can build substantial custom widget systems on top of Pinocchio webchat while leaving core framework code untouched.

