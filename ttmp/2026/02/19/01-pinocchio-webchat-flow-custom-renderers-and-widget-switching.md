# Pinocchio Webchat Extensibility: Ground-Up Architecture, Flow, and Registration Model

## Abstract

This document defines a pinocchio-only architecture for making webchat extensible from the ground up. It explains the entire flow from incoming backend event to rendered timeline entity, identifies the current extension seams and limits, and proposes a concrete plugin style model that lets developers add new semantic types and custom renderers without modifying core code paths.

The design center is simple:

1. Treat `timeline.upsert` as the durable canonical state stream.
2. Keep semantic translation and timeline projection extensible on the backend.
3. Make frontend projection and renderer registration explicit, ordered, and composable.
4. Preserve deterministic replay and hydration behavior.

This document is implementation oriented. It references concrete symbols and files in the pinocchio codebase and includes diagrams, pseudocode, and a step-by-step developer workflow.

---

## 1. Why Extensibility Is the First Problem

Pinocchio webchat already has strong foundations: typed protobuf schemas, timeline projection, hydration, and renderer overrides. The missing piece is not capability, but composition ergonomics. Today, extension points exist in several places, but they are not yet unified as one explicit plugin model.

When extension seams are implicit, teams usually add features by patching core switch statements, singleton registries, or app-level render conditionals. That works in the short term but makes future migration expensive. The right direction is to stabilize extension APIs first, then build domain packs on top.

The rest of this document describes how to do that with the code that already exists.

---

## 2. Current End-to-End Flow in Pinocchio

### 2.1 System overview

```text
Geppetto runtime events
  -> SEM translation registry (Go)
     pkg/sem/registry + pkg/webchat/sem_translator.go
  -> Stream coordinator (seq, stream_id)
     pkg/webchat/stream_coordinator.go
  -> Timeline projector (Go)
     pkg/webchat/timeline_projector.go
     + custom timeline handlers pkg/webchat/timeline_registry.go
  -> Timeline store upsert
     pkg/persistence/chatstore/*
  -> timeline.upsert SEM emission
     pkg/webchat/timeline_upsert.go
  -> WebSocket + hydration in browser
     cmd/web-chat/web/src/ws/wsManager.ts
  -> Frontend SEM registry + proto decode
     cmd/web-chat/web/src/sem/registry.ts
  -> Redux timeline slice upsert
     cmd/web-chat/web/src/store/timelineSlice.ts
  -> ChatTimeline kind->renderer dispatch
     cmd/web-chat/web/src/webchat/components/Timeline.tsx
```

### 2.2 Backend translation stage

Core API:

- `semregistry.RegisterByType[T](fn)` in `pkg/sem/registry/registry.go`
- `semregistry.Handle(e)` in `pkg/sem/registry/registry.go`
- `EventTranslator.Translate(e)` in `pkg/webchat/sem_translator.go`

`EventTranslator.RegisterDefaultHandlers()` wires known event families (LLM, tool, logs, mode, debugger, thinking-mode). Each handler emits one or more SEM envelopes using protobuf-backed payloads (`protoToRaw`).

Important properties:

1. Type-driven registration avoids a giant monolithic switch.
2. Payloads are schema-based, not ad hoc maps.
3. IDs are stabilized via local caches (`resolveMessageID`, tool call cache), which matters for streaming updates.

### 2.3 Backend projection stage

Core API:

- `TimelineProjector.ApplySemFrame(...)` in `pkg/webchat/timeline_projector.go`
- `RegisterTimelineHandler(eventType, handler)` in `pkg/webchat/timeline_registry.go`
- `TimelineProjector.Upsert(...)` public helper for custom handlers

`ApplySemFrame` does this in order:

1. Parse SEM envelope.
2. Validate `sem`, `event.type`, `event.id`, `event.seq`.
3. Run custom timeline handlers first via `handleTimelineHandlers(...)`.
4. If not handled, run built-in projection switch (`llm.*`, `tool.*`, `thinking.mode.*`, etc.).
5. Upsert `TimelineEntityV1` into store.

This ordering is a strong extension seam: custom handlers can own event type families before core fallback logic.

### 2.4 `timeline.upsert` emission

Core API:

- `emitTimelineUpsert(...)` in `pkg/webchat/timeline_upsert.go`
- equivalent service-level emission in `pkg/webchat/conversation_service.go`

After store upsert, backend emits:

```json
{
  "sem": true,
  "event": {
    "type": "timeline.upsert",
    "id": "<entity-id>",
    "seq": <version>,
    "data": {
      "convId": "...",
      "version": <version>,
      "entity": { "id": "...", "kind": "...", "snapshot": ... }
    }
  }
}
```

That event is the canonical projection stream and should be treated as authoritative UI state.

### 2.5 Frontend ingestion stage

Core API:

- `registerSem(type, handler)` in `cmd/web-chat/web/src/sem/registry.ts`
- `registerDefaultSemHandlers()` in `cmd/web-chat/web/src/sem/registry.ts`
- `wsManager.connect(...)` in `cmd/web-chat/web/src/ws/wsManager.ts`

`wsManager.connect` behavior:

1. Calls `registerDefaultSemHandlers()`.
2. Opens websocket.
3. Buffers frames until hydration completes.
4. Hydrates full snapshot from `/api/timeline`.
5. Replays buffered frames in seq order.

This ensures UI can recover from reconnects and multi-tab races.

### 2.6 Frontend render stage

Core API:

- `timelineSlice.upsertEntity(...)` in `cmd/web-chat/web/src/store/timelineSlice.ts`
- `ChatWidget` renderer merge in `cmd/web-chat/web/src/webchat/ChatWidget.tsx`
- `ChatTimeline` dispatch in `cmd/web-chat/web/src/webchat/components/Timeline.tsx`

Rendering is by entity `kind`:

```ts
const Renderer = renderers[e.kind] ?? renderers.default;
```

Default mapping in `ChatWidget` is:

- `message -> MessageCard`
- `tool_call -> ToolCallCard`
- `tool_result -> ToolResultCard`
- `thinking_mode -> ThinkingModeCard`
- fallback `default -> GenericCard`

Consumers can pass `renderers` prop to override or add kinds without editing core files.

---

## 3. Extensibility Surfaces That Already Exist

Pinocchio already has four real extension layers.

### 3.1 SEM translator extension (backend)

Use `RegisterByType` to map new runtime events into SEM frames.

Where:

- `pkg/sem/registry/registry.go`
- usually invoked from init/bootstrap near `pkg/webchat/sem_translator.go`

What this gives you:

1. No core switch edit required for new event classes.
2. Precise control over SEM event type names and payload schemas.

### 3.2 Timeline projection extension (backend)

Use `RegisterTimelineHandler(eventType, handler)`.

Where:

- `pkg/webchat/timeline_registry.go`
- built-in example in `pkg/webchat/timeline_handlers_builtin.go`

What this gives you:

1. Custom mapping from SEM event to timeline kinds.
2. Ability to upsert one or multiple entities per SEM event.
3. Full access to projector upsert path with version sequencing.

### 3.3 Frontend SEM dispatch extension

Use `registerSem(type, handler)` in `cmd/web-chat/web/src/sem/registry.ts`.

What this gives you:

1. Add direct handling for SEM event types.
2. Decode custom protobuf JSON via `fromJson`.
3. Upsert entities into timeline slice.

Current caveat: `registerDefaultSemHandlers()` clears the map, so extension handlers need deterministic registration ordering.

### 3.4 Frontend rendering extension

Use `ChatWidget` prop `renderers?: Partial<ChatWidgetRenderers>`.

Where:

- type in `cmd/web-chat/web/src/webchat/types.ts`
- merge in `cmd/web-chat/web/src/webchat/ChatWidget.tsx`

What this gives you:

1. Override default renderer for any built-in kind.
2. Add renderer for new kinds emitted by timeline projection.
3. Keep core timeline component unchanged.

---

## 4. Core Issues Blocking "Easy Extensibility"

The system is close to ideal, but several issues should be fixed to make extension easy and safe.

### 4.1 Frontend handler registry lifecycle is implicit

`wsManager.connect()` always calls `registerDefaultSemHandlers()`, which calls `handlers.clear()` and re-registers defaults. Any custom handler registered earlier can be dropped silently.

Impact:

1. Plugins depend on call order.
2. Reconnect can unintentionally disable extension behavior.

### 4.2 Frontend extension API is not first class

Today developers can call `registerSem` manually, but there is no official "webchat plugin" contract that defines registration phases and ownership.

Impact:

1. Integration code repeats ad hoc bootstrap logic.
2. Teams must read internal lifecycle details to avoid bugs.

### 4.3 Canonical projection path is not strongly enforced

Frontend still has handlers for non-`timeline.upsert` live SEM events (`llm.*`, `tool.*`, etc.). This is useful as fallback, but it can drift from backend projection semantics if not treated carefully.

Impact:

1. Potential double maintenance for mapping rules.
2. More room for subtle shape differences between live and hydrated data.

### 4.4 `tool_result.customKind` specialization is shallow by default

`ToolResultCard` currently displays `customKind` as a label and raw result text. That is generic and safe, but not sufficient for rich custom UI.

Impact:

1. Teams often add custom logic outside a stable extension contract.
2. Rich widget behavior becomes inconsistent across apps.

### 4.5 Lack of one unified registration story across backend and frontend

Backend has translator and projector registries; frontend has renderer overrides and SEM registry. They are powerful, but there is no single documented and typed contract that binds them.

Impact:

1. Onboarding friction.
2. Increased risk of "works in one app, not in another".

---

## 5. Ground-Up Target Model for Pinocchio

To make pinocchio easy to extend, define a single plugin contract with explicit lifecycle phases.

### 5.1 Proposed plugin contract

```text
PinocchioWebchatPlugin
  Backend phase:
    - registerSemTranslators(registry)
    - registerTimelineHandlers(registry)
  Frontend phase:
    - registerSemHandlers(frontendRegistry)
    - registerTimelineMappers(optional)
    - registerRenderers(rendererRegistry)
  Metadata:
    - name
    - version
    - supported kinds/types
```

This contract can be implemented incrementally without breaking current code.

### 5.2 Canonical-first ingest policy

Recommended policy:

1. `timeline.upsert` is primary for durable entities.
2. Live `llm.*` and `tool.*` handlers remain as compatibility fallback only.
3. New custom UI features should rely on projected timeline entities.

Why this matters:

- Hydration and live path use the same entity shapes.
- Replay determinism improves.
- Frontend mapping logic shrinks.

### 5.3 Renderer registry policy

Prefer `kind`-based renderer registration.

For generic kinds with subtypes (for example `tool_result`), add a local dispatcher renderer that switches by `props.customKind`, but keep it inside plugin surface rather than core switch code.

Example pattern:

```ts
function ToolResultDispatcher({ e }: { e: RenderEntity }) {
  const ck = String(e.props?.customKind ?? '');
  const R = toolResultRenderers[ck] ?? DefaultToolResultCard;
  return <R e={e} />;
}
```

This preserves generic core while allowing rich subtype rendering.

---

## 6. Concrete Registration Flows

### 6.1 Backend: adding a custom semantic family

#### Step A: Define protobuf payloads

Add schema under `pinocchio/proto/sem/...` and generate Go/TS code.

Rules:

1. Version message names (`FooEventV1`, `FooEventV2`).
2. Additive changes only for compatibility.
3. Avoid opaque map payloads when fields are known.

#### Step B: Register SEM translation

Use `RegisterByType`.

Pseudocode:

```go
semregistry.RegisterByType[*events.EventMyDomainReady](func(ev *events.EventMyDomainReady) ([][]byte, error) {
    data, err := protoToRaw(&mypb.MyDomainReadyV1{
        ItemId: ev.ItemID,
        Title:  ev.Title,
        State:  ev.State,
    })
    if err != nil { return nil, err }

    return [][]byte{wrapSem(map[string]any{
        "type": "mydomain.ready",
        "id":   ev.ItemID,
        "data": data,
    })}, nil
})
```

#### Step C: Register timeline projection

Use `RegisterTimelineHandler`.

Pseudocode:

```go
webchat.RegisterTimelineHandler("mydomain.ready", func(ctx context.Context, p *webchat.TimelineProjector, ev webchat.TimelineSemEvent, now int64) error {
    var pb mypb.MyDomainReadyV1
    if err := protojson.Unmarshal(ev.Data, &pb); err != nil {
        return nil
    }

    return p.Upsert(ctx, ev.Seq, &timelinepb.TimelineEntityV1{
        Id:   pb.ItemId,
        Kind: "my_domain",
        Snapshot: &timelinepb.TimelineEntityV1_Status{
            Status: &timelinepb.StatusSnapshotV1{
                SchemaVersion: 1,
                Type:          "info",
                Text:          pb.Title,
            },
        },
    })
})
```

### 6.2 Frontend: adding custom projection and rendering

#### Step D: Ensure entity mapping exists

If new `kind` uses existing oneof mapping, use current `timelineEntityFromProto` behavior. If it introduces new snapshot cases, update `propsFromTimelineEntity` in `cmd/web-chat/web/src/sem/timelineMapper.ts`.

#### Step E: Register renderer

Supply `renderers` prop when mounting `ChatWidget`:

```tsx
<ChatWidget
  renderers={{
    my_domain: MyDomainCard,
  }}
/>
```

If using `tool_result` + `customKind` specialization, override `tool_result` with a dispatcher.

#### Step F: Optional custom SEM handlers

If you truly need non-projected live handling, register custom SEM handlers after default registration in a deterministic bootstrap phase.

Long-term recommendation: expose `registerDefaultSemHandlers({ append })` style API so plugins can add handlers without race with `handlers.clear()`.

---

## 7. Widget Switching Model in Pinocchio

Pinocchio has two layers of switching.

### 7.1 Primary switch: timeline `kind`

Implemented in `ChatTimeline`:

```ts
const Renderer = renderers[e.kind] ?? renderers.default;
```

This is the stable and preferred switch boundary.

### 7.2 Secondary switch: subtype inside renderer

For shared kinds like `tool_result`, subtype switching can happen inside a custom renderer using fields like `props.customKind`.

Benefits:

1. Core timeline stays generic.
2. Domain-specific variants are isolated.
3. No edits to `ChatTimeline` for each new subtype.

### 7.3 Recommended pattern

1. Use one top-level timeline kind per UI family whenever possible.
2. Use subtype switching only when semantics truly share base kind behavior.
3. Keep fallback renderer robust (`GenericCard` style) for unknown kinds.

---

## 8. Hydration and Replay Invariants

Extensibility is only safe if replay behavior is deterministic.

### 8.1 Existing mechanism

`wsManager`:

1. Clears state before hydration.
2. Fetches `/api/timeline` snapshot.
3. Applies snapshot entities.
4. Replays buffered WS events sorted by `seq`.

### 8.2 Required invariants for plugin features

Any new plugin must preserve:

1. Stable entity IDs across live and replay.
2. Monotonic version handling (`timelineSlice` drops stale versions).
3. Idempotent upserts.
4. Tolerance to unknown fields during protobuf decode.

### 8.3 Practical rule

If a feature cannot be represented by `TimelineEntityV1`, it is not ready for production webchat state.

---

## 9. Testing Strategy for Extensible Pinocchio

A plugin architecture fails without contract tests. Add tests at three levels.

### 9.1 Backend unit tests

- translator tests:
  - given domain event -> expected SEM type/id/data
- projector tests:
  - given SEM frame -> expected timeline entity kind/snapshot/id

Reference examples:

- `pkg/webchat/sem_translator_test.go`
- projector behavior in `pkg/webchat/timeline_projector.go`

### 9.2 Backend integration tests

Add app-owned integration tests similar to `cmd/web-chat/app_owned_chat_integration_test.go` that verify custom entities appear in `/api/timeline` and websocket `timeline.upsert` stream.

### 9.3 Frontend tests

1. `timelineMapper` snapshot-case mapping.
2. renderer dispatch by kind.
3. reconnect/hydration replay with buffered custom upserts.
4. fallback behavior for unknown kinds/customKind values.

---

## 10. Reference Implementation Sketch (No Core Modification Path)

This section shows how teams can extend with minimal core changes today, and where one small core enhancement would improve ergonomics.

### 10.1 What is possible today without core edits

1. Backend:
- register new SEM translators via `RegisterByType`
- register timeline handlers via `RegisterTimelineHandler`

2. Frontend:
- mount `ChatWidget` with `renderers` overrides
- if needed, register extra SEM handlers in app bootstrap after defaults

This already supports substantial extension.

### 10.2 One recommended core enhancement

Add explicit frontend plugin bootstrap API:

```ts
type WebchatFrontendPlugin = {
  name: string;
  registerSem?: (r: SemRegistry) => void;
  registerRenderers?: (r: RendererRegistry) => void;
};

bootstrapWebchat({
  base: registerDefaultSemHandlers,
  plugins: [pluginA, pluginB],
});
```

This removes lifecycle ambiguity and makes ordering explicit and testable.

---

## 11. Migration Plan to "Easy Extensible" Pinocchio

### Phase 1: Documentation and contracts

1. Freeze naming conventions for SEM event types and timeline kinds.
2. Publish plugin registration order and invariants.
3. Add examples for one custom family.

### Phase 2: Frontend bootstrap API

1. Introduce registry composition API.
2. Ensure reconnect does not drop plugin registrations.
3. Add tests for plugin ordering.

### Phase 3: Canonical-first enforcement

1. Prefer timeline-projected entity rendering.
2. Keep direct `llm.*`/`tool.*` handlers as compatibility fallback.
3. Add lint/docs rule for new features: projection required.

### Phase 4: Hardening

1. Contract tests across translation, projection, render.
2. Replay equivalence tests (live stream vs hydrated snapshot).
3. Unknown-kind observability metrics.

---

## 12. File-and-Symbol Map for Developers

Backend translation:

- `pinocchio/pkg/sem/registry/registry.go`
  - `RegisterByType`, `Handle`, `Clear`
- `pinocchio/pkg/webchat/sem_translator.go`
  - `EventTranslator.Translate`, `RegisterDefaultHandlers`, `protoToRaw`

Backend projection:

- `pinocchio/pkg/webchat/timeline_registry.go`
  - `RegisterTimelineHandler`, `ClearTimelineHandlers`
- `pinocchio/pkg/webchat/timeline_projector.go`
  - `ApplySemFrame`, `Upsert`
- `pinocchio/pkg/webchat/timeline_handlers_builtin.go`
  - built-in custom handler pattern (`chat.message`)
- `pinocchio/proto/sem/timeline/transport.proto`
  - `TimelineEntityV1`, `TimelineUpsertV1`, `TimelineSnapshotV1`

Backend publication:

- `pinocchio/pkg/webchat/timeline_upsert.go`
  - `emitTimelineUpsert`
- `pinocchio/pkg/webchat/conversation_service.go`
  - service-level upsert emission

Frontend ingestion and mapping:

- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
  - connect/hydrate/buffer/replay behavior
- `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
  - `registerSem`, `registerDefaultSemHandlers`
- `pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts`
  - `timelineEntityFromProto`, `propsFromTimelineEntity`
- `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts`
  - upsert merge/version behavior

Frontend rendering:

- `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx`
  - default renderers + override merge
- `pinocchio/cmd/web-chat/web/src/webchat/components/Timeline.tsx`
  - renderer dispatch by `kind`
- `pinocchio/cmd/web-chat/web/src/webchat/types.ts`
  - `ChatWidgetRenderers`, `RenderEntity`
- `pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`
  - built-in renderer implementations

---

## 13. Final Recommendations

1. Keep backend extensibility as registry first architecture.
2. Promote `timeline.upsert` to explicit canonical contract for UI state.
3. Add a first-class frontend plugin bootstrap API to remove registration order ambiguity.
4. Standardize renderer strategy as `kind` primary and subtype secondary.
5. Enforce replay/hydration invariants in tests for every new custom timeline family.

With these changes, pinocchio can be extended by adding plugin packs rather than editing core flow, which is the correct foundation for all downstream domain-specific chat experiences.
