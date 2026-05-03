---
Title: SEM Cleanup: Architecture Analysis and Implementation Guide
Slug: sem-cleanup-architecture-analysis
DocType: design
Status: active
Topics:
  - sem
  - sessionstream
  - timeline
  - cleanup
  - documentation
  - webchat
Ticket: SEM-CLEANUP
---

# SEM Cleanup: Architecture Analysis and Implementation Guide

**Ticket:** SEM-CLEANUP
**Date:** 2026-05-03
**Audience:** New intern joining the team. This document assumes basic Go and TypeScript familiarity but no prior knowledge of this codebase.

---

## Executive Summary

The pinocchio webchat application went through a major architectural migration. The old event pipeline — called **SEM frames** (Structured Event Messaging) — has been fully replaced by the **sessionstream** + **chatapp.ChatPlugin** architecture. The migration is complete in production code, but leftover dead code, dead documentation, and a few legacy frontend modules remain.

This document explains:

- what SEM frames were and how they worked,
- what replaced them and why,
- exactly which files are dead and must be removed,
- exactly which documentation files contain stale references,
- a phased implementation plan to finish the cleanup,
- enough architectural context that a new team member can execute the cleanup without needing to guess.

The core finding: **zero production code calls into the SEM registry pipeline.** The Go `pkg/sem/registry` package has no consumers. The TypeScript `sem/registry.ts` is only used by a Storybook story file. The entire pipeline can be removed safely.

---

## 1. Problem Statement and Scope

The webchat application (in `cmd/web-chat`) previously used a custom event pipeline called SEM (Structured Event Messaging) to carry structured events from the Go backend to the React frontend over WebSocket. This pipeline has been replaced by the `sessionstream` package combined with a `chatapp.ChatPlugin` interface. The replacement is complete and in production.

However, the following artifacts from the old system remain:

- **Dead Go package:** `pkg/sem/registry/` — a generic event-to-JSON-frame translation registry with zero consumers.
- **Dead TypeScript modules:** `cmd/web-chat/web/src/sem/registry.ts` and `sem/registry.test.ts` — a frontend SEM envelope handler only used by Storybook stories.
- **Dead tutorial:** `pkg/doc/tutorials/04-intern-app-owned-middleware-events-timeline-widgets.md` — a 1041-line intern tutorial entirely about the old SEM pipeline.
- **Stale documentation:** Four topic files in `pkg/doc/topics/` that reference SEM frames, SEM envelopes, and the SEM pipeline as if they are the current architecture.
- **Questionable frontend modules:** `sem/timelineMapper.ts` and `sem/timelinePropsRegistry.ts` are still imported by the debug UI and the public webchat export surface, but their necessity should be evaluated.

The scope of this cleanup is:

1. Delete dead code (Go registry, TS registry + test).
2. Delete or archive the obsolete tutorial.
3. Update four documentation topic files.
4. Migrate the Storybook story away from SEM.
5. Evaluate and potentially relocate `timelineMapper.ts` and `timelinePropsRegistry.ts`.
6. Update cross-references in tutorial 09.

---

## 2. Background: What SEM Frames Were

### The original architecture

Before the `sessionstream` package existed, the webchat needed a way to send structured, typed events from the Go backend to the React frontend over a WebSocket connection. The solution was SEM (Structured Event Messaging).

The SEM pipeline worked like this:

```text
Runtime middleware (Go)
  → emits typed Geppetto events (e.g., EventLlmStart, EventToolStart)
      → SEM registry (Go) translates each event type into a SEM frame
          → SEM frame is a JSON envelope: { "sem": true, "event": { "type": "...", "id": "...", "data": {...} } }
              → frame is sent over WebSocket
                  → frontend SEM registry (TS) parses the envelope
                      → dispatches to registered handlers by event type
                          → handler mutates Redux/React state
                              → UI re-renders
```

### The SEM frame envelope contract

Every SEM frame was a JSON object with this shape:

```json
{
  "sem": true,
  "event": {
    "type": "llm.delta",
    "id": "msg-123",
    "seq": 42,
    "data": {
      "content": "Hello"
    }
  }
}
```

The key fields:

- `sem: true` — the magic boolean that identified this as a SEM envelope.
- `event.type` — a dotted string like `llm.delta`, `tool.start`, `timeline.upsert`.
- `event.id` — a stable entity ID for deduplication and upsert.
- `event.seq` — a sequence number for ordering.
- `event.data` — the typed payload, shape depends on `event.type`.

### The Go SEM registry

File: `pkg/sem/registry/registry.go` (70 lines)

This package provided a generic event-to-frame translation layer:

```go
// Handler maps a Geppetto event to zero or more SEM frames (each a JSON message).
type Handler func(e events.Event) ([][]byte, error)

// RegisterByType registers a handler for a specific event type T.
func RegisterByType[T any](fn func(T) ([][]byte, error))

// Handle attempts to process an event using registered handlers.
func Handle(e events.Event) ([][]byte, bool, error)
```

The intended usage pattern was:

1. At startup, call `RegisterByType[*EventLlmStart](handler)` for each event type.
2. When an event arrives, call `Handle(event)` to get JSON frames.
3. Send those frames over WebSocket.

**Current status: nothing in the codebase calls `RegisterByType` or `Handle`.** The package is dead code.

### The TypeScript SEM registry

File: `cmd/web-chat/web/src/sem/registry.ts` (256 lines)

This was the frontend counterpart. It provided:

```typescript
export type SemEnvelope = { sem: true; event: SemEvent };

export function registerSem(type: string, handler: Handler): void
export function handleSem(envelope: any, dispatch: AppDispatch): void
export function registerDefaultSemHandlers(): void
```

`registerDefaultSemHandlers` registered handlers for event types like `llm.start`, `llm.delta`, `llm.final`, `tool.start`, `tool.delta`, `tool.result`, `tool.done`, `log`, `agent.mode`, `debugger.pause`, `timeline.upsert`.

**Current status: only `ChatWidget.stories.tsx` imports this module.** The production app (`wsManager.ts`) does not use it.

---

## 3. The Replacement: Sessionstream + ChatPlugin

### Why SEM was replaced

The SEM pipeline had several problems:

- **Tight coupling to JSON envelope shape.** Every frame had to be wrapped in `{ sem: true, event: { ... } }`, which added overhead and made the protocol harder to evolve.
- **No built-in persistence or hydration.** SEM frames were fire-and-forget over WebSocket. If the client reconnected, there was no way to replay missed events without ad-hoc workarounds.
- **No projection layer.** Each frontend handler had to independently figure out how to turn raw event data into UI state. There was no backend-side projection that derived durable timeline entities.
- **No schema registry.** Event types and payload shapes were informal string conventions, not enforced by any schema mechanism.
- **Global mutable registries.** Both the Go and TS SEM registries used package-level mutable maps, making testing and isolation harder.

### How sessionstream works

The `sessionstream` package (external dependency: `github.com/go-go-golems/sessionstream`) provides:

- **Hub** — a command/event bus that accepts typed commands, publishes typed events, and supports subscription.
- **SchemaRegistry** — requires every event name, UI event name, and timeline entity kind to be registered with a prototype before use.
- **Projections** — functions that transform raw events into durable timeline entities and ephemeral UI events.
- **Hydration** — persistent snapshots (SQLite or in-memory) that allow clients to reconnect and rebuild state.
- **WebSocket transport** — a built-in fanout mechanism that delivers snapshots and live UI events.

### How chatapp.ChatPlugin extends sessionstream

File: `pkg/chatapp/features.go` (97 lines)

The `ChatPlugin` interface is the seam that lets app-owned product features hook into the sessionstream pipeline without polluting the shared core. The name "plugin" is deliberate — each implementation is a self-contained extension that translates one product feature (agent mode, reasoning, etc.) from Geppetto runtime events into sessionstream projections. It is not a data structure; it is an extension interface you implement and register with the chat engine via `chatapp.WithPlugins(...)`.

```go
type ChatPlugin interface {
    RegisterSchemas(reg *sessionstream.SchemaRegistry) error
    HandleRuntimeEvent(ctx context.Context, runtime RuntimeEventContext, event gepevents.Event) (handled bool, err error)
    ProjectUI(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error)
    ProjectTimeline(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error)
}
```

Each method corresponds to a phase in the pipeline:

1. **RegisterSchemas** — declare all event names, UI event names, and entity kinds this feature uses.
2. **HandleRuntimeEvent** — translate a Geppetto runtime event into a sessionstream event (published via `runtime.Publish`).
3. **ProjectUI** — derive ephemeral UI events from a sessionstream event.
4. **ProjectTimeline** — derive durable timeline entities from a sessionstream event.

### The current data flow (post-migration)

```text
Runtime middleware (Go)
  → emits typed Geppetto events
      → chatapp.Engine.handlePluginRuntimeEvent()
          → plugin.HandleRuntimeEvent() publishes sessionstream event
              → sessionstream Hub stores event
                  → projections run:
                      → base chat projection (messages, tokens, status)
                      → plugin.ProjectUI() (ephemeral UI events)
                      → plugin.ProjectTimeline() (durable entities)
                  → WebSocket transport:
                      → snapshot first (hydrated entities)
                      → live UI events after subscribe
                          → React wsManager.ts:
                              → maps UI events to timeline entities
                              → updates Redux store
                              → rendererRegistry dispatches to components
```

### Concrete example: agentmode feature

File: `cmd/web-chat/agentmode_chat_feature.go` (174 lines)

This is the reference implementation of a ChatPlugin:

```go
// Schema registration
func (agentModeChatFeature) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
    reg.RegisterEvent("ChatAgentModePreviewUpdated", &structpb.Struct{})
    reg.RegisterEvent("ChatAgentModeCommitted", &structpb.Struct{})
    reg.RegisterUIEvent("ChatAgentModePreviewUpdated", &structpb.Struct{})
    reg.RegisterUIEvent("ChatAgentModeCommitted", &structpb.Struct{})
    reg.RegisterUIEvent("ChatAgentModePreviewCleared", &structpb.Struct{})
    reg.RegisterTimelineEntity("AgentMode", &structpb.Struct{})
    return nil
}

// Runtime event → sessionstream event
func (agentModeChatFeature) HandleRuntimeEvent(ctx context.Context, runtime RuntimeEventContext, event gepevents.Event) (bool, error) {
    switch ev := event.(type) {
    case *agentmode.EventModeSwitchPreview:
        return true, runtime.Publish(ctx, "ChatAgentModePreviewUpdated", map[string]any{...})
    case *gepevents.EventAgentModeSwitch:
        return true, runtime.Publish(ctx, "ChatAgentModeCommitted", map[string]any{...})
    default:
        return false, nil
    }
}

// Sessionstream event → UI events
func (agentModeChatFeature) ProjectUI(ctx context.Context, ev sessionstream.Event, ...) ([]sessionstream.UIEvent, bool, error) {
    switch ev.Name {
    case "ChatAgentModePreviewUpdated":
        return []sessionstream.UIEvent{{Name: "ChatAgentModePreviewUpdated", Payload: pb}}, true, nil
    case "ChatAgentModeCommitted":
        return []sessionstream.UIEvent{committedEvent, previewClearEvent}, true, nil
    default:
        return nil, false, nil
    }
}

// Sessionstream event → timeline entities
func (agentModeChatFeature) ProjectTimeline(ctx context.Context, ev sessionstream.Event, ...) ([]sessionstream.TimelineEntity, bool, error) {
    if ev.Name != "ChatAgentModeCommitted" { return nil, false, nil }
    // upsert AgentMode entity with props
    return []sessionstream.TimelineEntity{{Kind: "AgentMode", Id: "session", Payload: pb}}, true, nil
}
```

The key insight is that **no SEM frames are involved anywhere in this flow**. Events go directly from Geppetto types to sessionstream events to UI events and timeline entities, all within the `sessionstream` Hub. Each `ChatPlugin` owns one product feature end-to-end: schema registration, runtime event translation, and both projection phases.

---

## 4. Evidence: Complete Inventory of Dead and Stale Code

### 4.1 Dead Go code

#### `pkg/sem/registry/registry.go` (70 lines) — DELETE

This file defines the Go SEM registry: `Handler`, `RegisterByType`, `Handle`, `Clear`.

Evidence of zero consumers:

- `grep -rn 'RegisterByType\|semregistry' --include="*.go" /path/to/pinocchio/` returns only the definition itself.
- `grep -rn '"github.com/go-go-golems/pinocchio/pkg/sem/registry' --include="*.go"` returns nothing.
- No file in the repository imports this package.

Action: **Delete the entire `pkg/sem/registry/` directory.**

#### What stays in `pkg/sem/`

The `pkg/sem/pb/` directory contains protobuf-generated Go types. These are still imported:

- `pkg/ui/timeline_persist.go` imports `timelinepb`
- `pkg/persistence/chatstore/` (multiple files) import `timelinepb`
- `cmd/switch-profiles-tui/main.go` imports `timelinepb`
- `cmd/web-chat/timeline/` (snapshot, verify, entity_helpers) imports `timelinepb`

Action: **Keep `pkg/sem/pb/` untouched.** Only `pkg/sem/registry/` is dead.

### 4.2 Dead TypeScript code

#### `cmd/web-chat/web/src/sem/registry.ts` (256 lines) — DELETE

This file defines the frontend SEM registry: `registerSem`, `handleSem`, `registerDefaultSemHandlers`.

Evidence of single consumer:

- `grep -rn 'from.*sem/registry'` returns only `ChatWidget.stories.tsx`.
- The production app (`wsManager.ts`) does not import it.

Action: **Delete this file.**

#### `cmd/web-chat/web/src/sem/registry.test.ts` (154 lines) — DELETE

Tests for the SEM registry. Obsolete once registry.ts is deleted.

Action: **Delete this file.**

#### `cmd/web-chat/web/src/sem/timelineMapper.ts` (28 lines) — EVALUATE

This file maps protobuf `TimelineEntityV2` objects to the frontend `TimelineEntity` type. It is imported by:

1. `sem/registry.ts` (dead, being deleted)
2. `debug-ui/ws/debugTimelineWsManager.ts` (alive, active debug UI)

The function `timelineEntityFromProto` is still needed by the debug UI. The question is whether it should be moved out of `sem/` or kept.

Action: **Move to a neutral location** (e.g., `web/src/utils/timelineMapper.ts` or `web/src/debug-ui/ws/timelineMapper.ts`) since the `sem/` directory will otherwise be mostly protobuf-only.

#### `cmd/web-chat/web/src/sem/timelinePropsRegistry.ts` (43 lines) — EVALUATE

This file provides `registerTimelinePropsNormalizer` and `normalizeTimelineProps`. It is imported by:

1. `sem/timelineMapper.ts` (being moved)
2. `webchat/index.ts` (the public export surface)

This is a legitimate utility that normalizes timeline entity props before rendering. It has nothing to do with SEM frames specifically — it's just colocated under `sem/` for historical reasons.

Action: **Move to `web/src/webchat/timelinePropsRegistry.ts`** since it's exported from the webchat public surface and used by the webchat rendering pipeline.

#### What stays in `cmd/web-chat/web/src/sem/`

The `sem/pb/` directory contains protobuf-generated TypeScript types. These are imported by the debug UI, the Storybook stories, and other modules. They represent the protobuf schema types, not the SEM pipeline.

Action: **Keep `sem/pb/` untouched.** Delete or move everything else in `sem/`.

### 4.3 Dead documentation

#### `pkg/doc/tutorials/04-intern-app-owned-middleware-events-timeline-widgets.md` (1041 lines) — DELETE

This is a comprehensive intern tutorial that teaches the entire old SEM pipeline: backend SEM registration, frontend SEM handler registration, timeline projection through SEM frames, widget rendering from SEM-derived entities, and protobuf ownership conventions. Every instruction references the SEM pipeline, `semregistry.RegisterByType`, `wrapSem`, and `registerSem`.

The tutorial is entirely obsolete because:

- No production code uses SEM frames anymore.
- The `sessionstream` + `ChatPlugin` pattern replaces every concept described.
- Tutorial 09 (`09-building-sessionstream-react-chat-apps.md`) covers the new architecture comprehensively.

Cross-references:

- Tutorial 09 mentions this file in its "See Also" section.

Action: **Delete this file.** Remove the cross-reference from tutorial 09.

### 4.4 Stale documentation (needs updating, not deletion)

#### `pkg/doc/topics/webchat-frontend-integration.md` (160 lines)

Stale references:

- Line 23: `GET /ws?conv_id=<id> for streaming SEM events`
- Line 64: `type SemEnvelope = { sem: true; event: { type: string; id: string; seq?: number; data?: unknown } }`
- Lines 79+: `## SEM Frame Contract` section describing the old envelope
- Line 152: reference to `sem/registry.ts`

Action: **Replace SEM frame sections with sessionstream UI event descriptions.** Update the WebSocket contract to describe the canonical frame shape (`{ type: "ui-event", name, sessionId, payload }`).

#### `pkg/doc/topics/webchat-frontend-architecture.md` (92 lines)

Stale references:

- Line 28: `sem/` directory reference
- Line 39: `4. SEM registry decodes websocket events.`
- Lines 64+: `## SEM Pipeline` section
- Lines 66-69: Pipeline steps referencing `sem: true` envelope validation

Action: **Replace SEM Pipeline section with sessionstream projection pipeline.** Update the architecture diagram.

#### `pkg/doc/topics/13-js-api-reference.md` (190 lines)

Stale references:

- Line 4: Short description mentions "JavaScript SEM handlers"
- Lines 39-40: `registerSemReducer(eventType, fn)` and `onSem(eventType, fn)` API docs
- Line 49: `// observe SEM stream` example
- Lines 66-70: SEM event fields reference
- Line 85: Wildcard subscription for all SEM events
- Line 133: `For each SEM frame:` prose
- Line 189: Cross-reference to `webchat-sem-and-ui.md`

Action: **Replace SEM API references with sessionstream UI event API.** Update the JS API to describe the canonical frame handlers in `wsManager.ts`.

#### `pkg/doc/topics/webchat-debugging-and-ops.md` (145 lines)

Stale references:

- Line 89: `Wildcard hooks use p.timeline.onSem("*", fn) and run for all SEM event types.`

Action: **Update or remove the single stale line.** If `onSem` is no longer the debugging API, describe the sessionstream debugging approach instead.

#### Tutorial 09 cross-reference

File: `pkg/doc/tutorials/09-building-sessionstream-react-chat-apps.md`

Line 602: `- intern-app-owned-middleware-events-timeline-widgets — deeper feature and widget reference`

Action: **Remove this line** since the referenced tutorial is being deleted.

### 4.5 Debug UI: broken against current server

The debug-ui (`cmd/web-chat/web/src/debug-ui/`) was built for the old `pkg/webchat` server. Against the current sessionstream-based `cmd/web-chat` server, it is entirely non-functional:

- `debugTimelineWsManager.ts` connects to `/ws?conv_id=...` — **no such route exists**. The live follow feature is broken.
- `debugApi.ts` calls `/api/debug/conversations`, `/api/debug/turns`, `/api/debug/events/:convId`, `/api/debug/timeline` — **none of these routes exist** in the current server.
- `timelineMapper.ts` is imported only by the debug WS manager to decode protobuf `TimelineEntityV2` objects from SEM-style `{ sem: true }` envelopes — **these envelopes are never sent**.

The fix is to rewrite the debug-ui to consume the same production sessionstream WebSocket (`/api/chat/ws` with `{ type: "subscribe", sessionId }` protocol) and the same REST snapshot endpoint (`GET /api/chat/sessions/:id`). This requires **zero new Go endpoints**. See Phase 8 in the implementation plan.

### 4.6 Storybook migration

#### `cmd/web-chat/web/src/webchat/ChatWidget.stories.tsx`

This file imports `handleSem` and `registerDefaultSemHandlers` from `sem/registry.ts`.

Lines 84-87 (approximately):

```typescript
registerDefaultSemHandlers();
// ...
for (const fr of frames) handleSem(fr, dispatch);
```

After deleting `sem/registry.ts`, this story will break.

Action: **Migrate the story to use sessionstream-derived fixture data** (the canonical frame shapes used by `wsManager.ts`) or generate mock timeline entities directly. The story should simulate what `wsManager.ts` does: create timeline entities from a mock snapshot, not from SEM frames.

---

## 5. Implementation Plan

This section gives a phased, file-level plan. Each phase is independent enough to be reviewed separately.

### Phase 1: Delete dead Go code

**Goal:** Remove `pkg/sem/registry/` with zero risk.

**Steps:**

1. Verify no imports exist:
   ```bash
   grep -rn 'pkg/sem/registry' --include="*.go" .
   grep -rn 'semregistry' --include="*.go" .
   ```
   Both should return nothing (besides the registry's own files).

2. Delete the directory:
   ```bash
   rm -rf pkg/sem/registry/
   ```

3. Build and test:
   ```bash
   go test ./pkg/sem/... -count=1
   make build
   ```

4. Verify no breakage:
   ```bash
   go test ./... -count=1
   ```

**Risk:** None. The package has zero consumers.

### Phase 2: Delete dead TypeScript code

**Goal:** Remove `sem/registry.ts` and `sem/registry.test.ts`.

**Precondition:** Phase 4 (Storybook migration) must be done first, or done simultaneously, because `ChatWidget.stories.tsx` imports `registry.ts`.

**Steps:**

1. Verify import graph after Storybook migration:
   ```bash
   grep -rn 'from.*sem/registry' cmd/web-chat/web/src/
   ```
   Should return nothing.

2. Delete the files:
   ```bash
   rm cmd/web-chat/web/src/sem/registry.ts
   rm cmd/web-chat/web/src/sem/registry.test.ts
   ```

3. Verify frontend builds:
   ```bash
   cd cmd/web-chat/web && npm run check
   ```

**Risk:** Low after Storybook migration. If `registry.ts` has any hidden consumers, the TypeScript compiler will catch them.

### Phase 3: Relocate `timelinePropsRegistry.ts` out of `sem/`

**Goal:** Move `timelinePropsRegistry.ts` out of the `sem/` directory since it is not SEM-specific. After Phase 8, `timelineMapper.ts` will have no remaining consumers and can be deleted entirely.

1. Move the file:
   ```bash
   mv cmd/web-chat/web/src/sem/timelinePropsRegistry.ts cmd/web-chat/web/src/webchat/timelinePropsRegistry.ts
   ```

2. Update imports in `webchat/index.ts`:
   ```typescript
   // Before:
   export { ... } from '../sem/timelinePropsRegistry';
   // After:
   export { ... } from './timelinePropsRegistry';
   ```

3. Verify:
   ```bash
   cd cmd/web-chat/web && npm run check
   ```

**Note on `timelineMapper.ts`:** This file will be deleted in Phase 8 after the debug-ui migration removes its last consumer. Do not move it — just leave it in `sem/` until Phase 8 cleans it up.

### Phase 4: Migrate Storybook story

**Goal:** Remove the SEM dependency from `ChatWidget.stories.tsx`.

**Current usage pattern** (in `ChatWidget.stories.tsx`):

```typescript
import { handleSem, registerDefaultSemHandlers } from '../sem/registry';

// In story setup:
registerDefaultSemHandlers();

// In story render:
for (const fr of frames) handleSem(fr, dispatch);
```

**Migration approach:** Replace with direct Redux store population. Instead of running SEM frames through the registry, create the timeline entities directly:

```typescript
// Replace:
import { handleSem, registerDefaultSemHandlers } from '../sem/registry';

// With direct store population:
import { timelineSlice } from '../store/timelineSlice';

// In story setup, dispatch entities directly:
dispatch(timelineSlice.actions.upsertEntities({
  entities: [
    {
      id: 'msg-1',
      kind: 'message',
      createdAt: Date.now(),
      updatedAt: Date.now(),
      props: { role: 'user', content: 'Hello', status: 'complete' }
    },
    {
      id: 'msg-2',
      kind: 'message',
      createdAt: Date.now(),
      updatedAt: Date.now(),
      props: { role: 'assistant', content: 'Hi there!', status: 'complete' }
    }
  ]
}));
```

This is simpler and more honest — it tests the rendering, not the SEM dispatch machinery.

**Verify:** Storybook renders correctly.

```bash
cd cmd/web-chat/web && npx storybook build
```

### Phase 5: Delete obsolete tutorial

**Goal:** Remove `04-intern-app-owned-middleware-events-timeline-widgets.md`.

**Steps:**

1. Delete:
   ```bash
   rm pkg/doc/tutorials/04-intern-app-owned-middleware-events-timeline-widgets.md
   ```

2. Update tutorial 09's "See Also" section. Remove the line:
   ```markdown
   - `intern-app-owned-middleware-events-timeline-widgets` — deeper feature and widget reference
   ```

**Risk:** None. The tutorial has no code dependencies.

### Phase 6: Update stale documentation topics

**Goal:** Remove all SEM frame references from the four affected topic files.

**File-by-file guidance:**

#### `webchat-frontend-integration.md`

- Replace "streaming SEM events" with "streaming sessionstream UI events".
- Replace the `SemEnvelope` type definition with the canonical frame shape:
  ```typescript
  type UIEventFrame = {
    type: "ui-event";
    name: string;        // e.g., "ChatTokensDelta", "ChatAgentModePreviewUpdated"
    sessionId: string;
    ordinal: number;
    payload: Record<string, unknown>;
  };
  ```
- Replace "SEM Frame Contract" section heading with "UI Event Frame Contract".
- Update the example frame to show a real canonical frame instead of `{ sem: true, event: { ... } }`.
- Update file references to point to `wsManager.ts` instead of `sem/registry.ts`.

#### `webchat-frontend-architecture.md`

- Remove `sem/` from the directory listing or mark it as protobuf-only.
- Replace "SEM registry decodes websocket events" with "wsManager processes canonical UI event frames".
- Replace "SEM Pipeline" section with "Sessionstream Projection Pipeline":
  ```text
  1. WebSocket receives canonical frame { type: "ui-event", name, payload }
  2. wsManager dispatches to timeline slice mutations
  3. Timeline entities update in Redux store
  4. RendererRegistry resolves component by entity kind
  5. React re-renders
  ```

#### `13-js-api-reference.md`

- Update the short description from "JavaScript SEM handlers" to "JavaScript sessionstream UI event handlers".
- Replace `registerSemReducer(eventType, fn)` and `onSem(eventType, fn)` with the actual current API (or remove if these functions no longer exist in the production code).
- Replace SEM event field descriptions with canonical frame fields.
- Update cross-references.

#### `webchat-debugging-and-ops.md`

- Replace the `p.timeline.onSem("*", fn)` reference with the current debugging approach (if applicable) or remove the line.

### Phase 7: Verify and close

**Goal:** Confirm everything builds, tests pass, and no stale references remain.

**Verification commands:**

```bash
# Go builds and tests
make build
go test ./... -count=1

# Frontend builds and checks
cd cmd/web-chat/web && npm run check

# No remaining SEM frame references in docs
grep -rn 'sem: true\|"sem": true\|wrapSem\|semregistry\|RegisterByType\|registerSem\\(\|handleSem\\(' pkg/doc/

# No remaining SEM registry references in Go code
grep -rn 'sem/registry\|semregistry' --include="*.go" .

# No remaining imports of deleted TS files
grep -rn 'from.*sem/registry' cmd/web-chat/web/src/
```

All of these should return nothing.

### Phase 8: Migrate debug-ui to sessionstream

**Goal:** Rewrite the debug-ui to consume the same production sessionstream endpoints (`/api/chat/ws`, `GET /api/chat/sessions/:id`) instead of the broken old `/ws?conv_id=` WebSocket and the non-existent `/api/debug/*` REST endpoints.

#### Why this is part of the SEM cleanup

The debug-ui is the last remaining consumer of SEM-style `{ sem: true }` envelope parsing. Its `debugTimelineWsManager.ts` opens a WebSocket to `/ws?conv_id=...` (which does not exist on the current server), receives `{ sem: true, event: { type: "timeline.upsert", data: { entity: ... } } }` frames, and decodes protobuf `TimelineEntityV2` objects through `timelineMapper.ts`. All of this is dead — the endpoint doesn't exist, the frames are never sent, and the live follow feature is broken.

#### Current state of the debug-ui

The debug-ui is a developer inspection tool activated by appending `?debug=1` to the webchat URL. It shows three synchronized scroll lanes:

1. **State Track** — middleware turn phases (draft → pre_inference → post_inference → post_tools → final). This data comes from `GET /api/debug/turns`, which does not exist in the current server.
2. **Events** — raw event buffer from `GET /api/debug/events/:convId`, which does not exist in the current server.
3. **Projection** — timeline entities from `GET /api/debug/timeline?conv_id=...` and live WebSocket updates from `/ws?conv_id=...`, neither of which exists.

All three data sources hit endpoints that only existed in the old `pkg/webchat` server (used by `web-agent-example`). Against the current sessionstream-based `cmd/web-chat` server, the debug-ui shows nothing useful — every API call returns 404 and the WebSocket connection fails.

#### What the production sessionstream WS already provides

The production WebSocket at `/api/chat/ws` sends these frame types after a client sends `{ type: "subscribe", sessionId: "..." }`:

```text
1. { type: "hello", connectionId: "conn-1" }
2. { type: "snapshot", sessionId, ordinal, entities: [{kind, id, tombstone, payload}] }
3. { type: "subscribed", sessionId }
4. { type: "ui-event", sessionId, ordinal, name, payload }  (repeated for each live event)
```

These frames are plain JSON. No protobuf decoding. No `{ sem: true }` envelope. The snapshot contains all current timeline entities with their full props. The live ui-events carry every state change the chat widget itself uses.

This means the debug-ui can render a useful view with **zero new Go endpoints**. The production server already sends everything an entity inspector and event viewer needs.

#### What we lose (and why that's fine)

The debug-ui currently shows two things that sessionstream does not provide:

- **Turn phase diffs** (how blocks change as middleware executes: draft → pre_inference → post_inference → post_tools → final). This is deep middleware-chain inspection that the old `pkg/webchat` server provided via a turns API. Sessionstream has no concept of middleware phases. If we want this back, it would be a new feature requiring new backend support, not a cleanup task.
- **Raw backend event buffer** (unprojected Geppetto events). Sessionstream only exposes projected UI events and timeline entities, not the raw event stream.

Both are acceptable losses for a debug tool. The primary value of the debug-ui is seeing what entities exist and what events are flowing — and sessionstream gives us exactly that.

#### Migration plan

**Step 8a: Replace `debugTimelineWsManager.ts`**

The current file (248 lines) reimplements WebSocket connection, SEM envelope parsing, protobuf decoding, high-water-mark version tracking, and bootstrap-from-timeline logic. Replace it with a thin wrapper that connects to the production WS endpoint and speaks the sessionstream protocol.

New shape:

```typescript
class DebugWsManager {
  private ws: WebSocket | null = null;
  private nonce = 0;

  async connect(args: { sessionId: string; basePrefix: string; dispatch: AppDispatch }) {
    this.disconnect();
    this.nonce++;
    const nonce = this.nonce;

    // Same endpoint as production chat widget
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${window.location.host}${args.basePrefix}/api/chat/ws`;
    const ws = new WebSocket(url);
    this.ws = ws;

    ws.onopen = () => {
      if (nonce !== this.nonce) return;
      // Speak sessionstream protocol
      ws.send(JSON.stringify({ type: 'subscribe', sessionId: args.sessionId, sinceOrdinal: '0' }));
    };

    ws.onmessage = (msg) => {
      if (nonce !== this.nonce) return;
      const frame = JSON.parse(String(msg.data));
      const type = String(frame.type ?? '');

      if (type === 'snapshot') {
        // Clear and populate from snapshot
        args.dispatch(debugSlice.actions.clear());
        for (const entity of (frame.entities ?? [])) {
          args.dispatch(debugSlice.actions.upsertEntity({
            id: entity.id,
            kind: entity.kind,
            tombstone: entity.tombstone ?? false,
            createdAt: Date.now(),
            updatedAt: Date.now(),
            props: entity.payload ?? {},
          }));
        }
        args.dispatch(debugSlice.actions.setSnapshotOrdinal(Number(frame.ordinal ?? 0)));
      }

      if (type === 'ui-event') {
        // Append to event log for the events lane
        args.dispatch(debugSlice.actions.appendEvent({
          name: frame.name,
          ordinal: Number(frame.ordinal ?? 0),
          sessionId: frame.sessionId,
          payload: frame.payload ?? {},
          receivedAt: new Date().toISOString(),
        }));
      }
    };
  }

  disconnect() { /* close ws, reset state */ }
}
```

This is roughly 80 lines instead of 248, with no protobuf imports, no SEM envelope parsing, no high-water-mark tracking (sessionstream handles dedup server-side), and no bootstrap HTTP call (the snapshot arrives on subscribe).

**Step 8b: Delete `debugApi.ts` and its test**

File: `cmd/web-chat/web/src/debug-ui/api/debugApi.ts` (280+ lines)
File: `cmd/web-chat/web/src/debug-ui/api/debugApi.test.ts`

This file defines RTK Query endpoints against `/api/debug/conversations`, `/api/debug/turns`, `/api/debug/events/:convId`, `/api/debug/timeline`, `/api/debug/runs`. None of these routes exist in the current server. Delete entirely.

**Step 8c: Rewrite `useLaneData.ts`**

Current: calls 3 RTK Query endpoints that 404.

Replace with a simple selector that reads from the debug Redux slice (populated by the new WS manager):

```typescript
export function useLiveLaneData(sessionId: string | null): LiveLaneData {
  const entities = useAppSelector(state => Object.values(state.debug.entities));
  const events = useAppSelector(state => state.debug.events);
  return { entities, events, isLoading: false };
}
```

**Step 8d: Simplify `TimelineLanes.tsx`**

Remove the StateTrackLane (turn phases). Keep two lanes:

1. **Events** — shows every `ui-event` frame received (name, ordinal, payload preview).
2. **Entities** — shows every timeline entity from the snapshot (kind, id, props).

This is a simpler but still useful debug view: you see what entities the backend has projected and what events are flowing.

**Step 8e: Replace conversation selector**

Current: fetches conversation list from `/api/debug/conversations`.

Replace with a text input where the developer types a session ID. Simpler, no backend change needed.

**Step 8f: Delete dead files**

Files to delete:

- `debug-ui/api/debugApi.ts` — RTK Query against non-existent endpoints
- `debug-ui/api/debugApi.test.ts` — tests for above
- `debug-ui/api/turnParsing.ts` — middleware turn block parser (not needed without turn phases)
- `debug-ui/api/turnParsing.test.ts` — tests for above
- `debug-ui/mocks/` — MSW mocks for the old debug API (entire directory)
- `debug-ui/ws/debugTimelineWsManager.ts` — replaced by new WS manager
- `debug-ui/ws/debugTimelineWsManager.test.ts` — tests for old WS manager

Files to keep but simplify:

- `debug-ui/routes/OverviewPage.tsx` — remove turn inspector, keep entity + event lanes
- `debug-ui/routes/TimelinePage.tsx` — simplify to 2-lane view
- `debug-ui/routes/EventsPage.tsx` — keep, reads from Redux slice instead of API
- `debug-ui/routes/TurnDetailPage.tsx` — delete or stub (no turn data without backend support)
- `debug-ui/routes/useLaneData.ts` — rewrite to read from Redux
- `debug-ui/components/TimelineLanes.tsx` — simplify to 2 lanes
- `debug-ui/components/StateTrackLane.tsx` — delete
- `debug-ui/components/EventTrackLane.tsx` — keep, reads events from Redux
- `debug-ui/components/ProjectionLane.tsx` — keep, reads entities from Redux
- `debug-ui/components/TurnInspector.tsx` — delete or stub

**Step 8g: Remove `sem/timelineMapper.ts` import**

After the debug-ui no longer decodes protobuf entities, `timelineMapper.ts` has no remaining consumers and can be deleted (as planned in Phase 3).

#### What the debug-ui looks like after migration

```text
┌──────────────────────────────────────────────────────────┐
│ Debug UI                              [session id input] │
├──────────────────────┬───────────────────────────────────┤
│ ⚡ UI Events          │ 🎯 Timeline Entities              │
│                      │                                   │
│ #1 ChatMessageAccept │ message  msg-1  role=user ...      │
│ #2 ChatMessageStart  │ message  msg-2  role=assistant ... │
│ #3 ChatMessageAppend │ agent_mode  session  title=...     │
│ #4 ChatAgentMode..   │                                   │
│ #5 ChatMessageFinish │                                   │
│                      │                                   │
│ [live indicator ●]   │                                   │
└──────────────────────┴───────────────────────────────────┘
```

Left lane: every UI event frame received over WS, with name and ordinal.
Right lane: every entity from the snapshot, with kind, id, and props summary.

Both update live as the WS pushes new events. Clicking an entity or event shows its full payload in an inspector panel.

#### Verification

```bash
# Start the server
go run ./cmd/web-chat --addr :8080

# Open browser
open http://localhost:8080?debug=1

# Enter a session ID, verify:
# - snapshot loads entities into the right lane
# - sending a chat message produces live ui-events in the left lane
# - clicking an entity shows its full props
# - clicking an event shows its full payload
# - reconnect after server restart works (snapshot re-sent on subscribe)
```

**Risk:** Low. No Go changes. All TS. The worst case is the debug-ui shows nothing, which is the current state anyway.

---

## 6. API Reference: Key Interfaces

### Go: `chatapp.ChatPlugin`

File: `pkg/chatapp/features.go`

The name was previously `FeatureSet`, which was confusing because it sounded like a data structure (`map[string]bool`). The rename to `ChatPlugin` makes it clear this is an extension interface you implement and register.

```go
type ChatPlugin interface {
    RegisterSchemas(reg *sessionstream.SchemaRegistry) error
    HandleRuntimeEvent(ctx context.Context, runtime RuntimeEventContext, event gepevents.Event) (handled bool, err error)
    ProjectUI(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error)
    ProjectTimeline(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error)
}

type RuntimeEventContext struct {
    SessionID sessionstream.SessionId
    MessageID string
    Publish   func(ctx context.Context, eventName string, payload map[string]any) error
}
```

- `RegisterSchemas` is called once at startup. All event names must be declared here.
- `HandleRuntimeEvent` is called for every Geppetto event during inference. Return `handled=true` to stop further feature processing.
- `ProjectUI` is called for every sessionstream event. Return `(uiEvents, true, nil)` to emit UI events.
- `ProjectTimeline` is called for every sessionstream event. Return `(entities, true, nil)` to emit timeline entities.
- The `handled` boolean uses a "first match wins" pattern — if one feature handles an event, later features skip it.

### Go: `sessionstream.SchemaRegistry`

From external package `github.com/go-go-golems/sessionstream`.

Key methods:

```go
reg.RegisterEvent(name string, prototype proto.Message) error
reg.RegisterUIEvent(name string, prototype proto.Message) error
reg.RegisterTimelineEntity(kind string, prototype proto.Message) error
```

All event names and entity kinds must be registered before the Hub starts accepting events. This prevents runtime typos and ensures payload shape validation.

### Go: `sessionstream.Hub`

Key lifecycle:

```go
hub, _ := sessionstream.NewHub(
    sessionstream.WithSchemaRegistry(reg),
    sessionstream.WithHydrationStore(store),
    sessionstream.WithUIFanout(wsTransport),
)

// Hub accepts commands and publishes events
// Projections run automatically after event publication
// Hydration snapshots are available for reconnect
```

### TypeScript: Canonical WebSocket frame shape

File: `cmd/web-chat/web/src/ws/wsManager.ts`

The production frontend processes these frame shapes:

```typescript
// UI event frame (live streaming)
type UIEventFrame = {
  type: "ui-event";
  name: string;
  sessionId: string;
  ordinal: number;
  payload: Record<string, unknown>;
};

// Snapshot frame (initial hydration)
type SnapshotFrame = {
  type: "snapshot";
  sessionId: string;
  ordinal: number;
  entities: Array<{ kind: string; id: string; payload: Record<string, unknown> }>;
};
```

### TypeScript: `timelineSlice`

File: `cmd/web-chat/web/src/store/timelineSlice.ts`

Redux slice that holds all timeline entities. Key actions:

- `upsertEntity(entity)` — insert or update a single entity by `id`.
- `upsertEntities({ entities })` — batch upsert.

### TypeScript: `rendererRegistry`

File: `cmd/web-chat/web/src/webchat/rendererRegistry.ts`

Maps entity `kind` strings to React components:

```typescript
registerTimelineRenderer("AgentMode", AgentModeCard);
registerTimelineRenderer("message", MessageCard);
```

The renderer receives a `RenderEntity` (a `TimelineEntity` with normalized props) and renders the appropriate card.

---

## 7. Risks, Alternatives, and Open Questions

### Risks

1. **Protobuf types in `sem/pb/` are widely used.** Do not touch these during cleanup. They are data types, not pipeline code.

2. **Storybook story fixture data.** The migration from SEM frames to direct store population means the story fixtures will change shape. Review the visual output carefully.

3. **Debug UI simplification removes turn phase inspection.** The migrated debug-ui will not show middleware turn phase diffs (draft → pre_inference → post_inference → final). If this capability is needed later, it requires new backend support — a separate feature, not part of this cleanup.

### Alternatives considered

1. **Keep `sem/registry.ts` as a compatibility shim.** Not worth it — the code is only used by stories and has no production consumers.

2. **Keep `pkg/sem/registry/` as a utility library.** Not worth it — the Go generics-based event registry pattern is simple enough to recreate if needed, and nothing uses it.

3. **Rename `sem/` directory entirely.** Too broad — the protobuf types are still imported everywhere. Renaming would require updating dozens of import paths for no architectural benefit.

### Open questions

1. Are there any other binaries in the repository (beyond `cmd/web-chat`) that use the SEM pipeline? The investigation focused on `cmd/web-chat` but did not scan all `cmd/` directories exhaustively.

---

## 8. Key File Reference

### Files to DELETE

| File | Lines | Reason |
|------|-------|--------|
| `pkg/sem/registry/registry.go` | 70 | Dead Go SEM registry, zero consumers |
| `cmd/web-chat/web/src/sem/registry.ts` | 256 | Dead TS SEM registry, only used by Storybook |
| `cmd/web-chat/web/src/sem/registry.test.ts` | 154 | Tests for dead TS SEM registry |
| `pkg/doc/tutorials/04-intern-app-owned-middleware-events-timeline-widgets.md` | 1041 | Entirely obsolete SEM pipeline tutorial |
| `cmd/web-chat/web/src/debug-ui/api/debugApi.ts` | ~280 | RTK Query against non-existent `/api/debug/*` endpoints |
| `cmd/web-chat/web/src/debug-ui/api/debugApi.test.ts` | — | Tests for dead API layer |
| `cmd/web-chat/web/src/debug-ui/api/turnParsing.ts` | — | Turn block parser, not needed without turn phases |
| `cmd/web-chat/web/src/debug-ui/api/turnParsing.test.ts` | — | Tests for turn parser |
| `cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts` | 248 | Broken WS client for non-existent `/ws` endpoint |
| `cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.test.ts` | — | Tests for broken WS client |
| `cmd/web-chat/web/src/debug-ui/mocks/` (entire directory) | — | MSW mocks for old debug API, no longer relevant |

### Files to MOVE

| Source | Destination | Reason |
|--------|-------------|--------|
| `cmd/web-chat/web/src/sem/timelinePropsRegistry.ts` | `cmd/web-chat/web/src/webchat/timelinePropsRegistry.ts` | Active utility misplaced under `sem/` |
| `cmd/web-chat/web/src/sem/timelineMapper.ts` | Delete after Phase 8 (no remaining consumers) | Was only used by debug WS manager |

### Files to UPDATE

| File | Change |
|------|--------|
| `cmd/web-chat/web/src/webchat/ChatWidget.stories.tsx` | Remove SEM imports, use direct store population |
| `cmd/web-chat/web/src/webchat/index.ts` | Update import paths after move |
| `cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts` | Replace with new sessionstream WS client |
| `cmd/web-chat/web/src/debug-ui/routes/useLaneData.ts` | Rewrite to read from Redux instead of dead API endpoints |
| `cmd/web-chat/web/src/debug-ui/routes/OverviewPage.tsx` | Remove turn inspector, keep entity + event lanes |
| `cmd/web-chat/web/src/debug-ui/routes/TimelinePage.tsx` | Simplify to 2-lane view |
| `cmd/web-chat/web/src/debug-ui/components/TimelineLanes.tsx` | Remove StateTrackLane, keep 2 lanes |
| `pkg/doc/tutorials/09-building-sessionstream-react-chat-apps.md` | Remove cross-reference to deleted tutorial 04 |
| `pkg/doc/topics/webchat-frontend-integration.md` | Replace SEM frame references with sessionstream |
| `pkg/doc/topics/webchat-frontend-architecture.md` | Replace SEM pipeline section with sessionstream |
| `pkg/doc/topics/13-js-api-reference.md` | Replace SEM API references with sessionstream API |
| `pkg/doc/topics/webchat-debugging-and-ops.md` | Remove stale `onSem` reference |

### Files to KEEP UNTOUCHED

| File/Directtory | Reason |
|-----------------|--------|
| `pkg/sem/pb/` (all protobuf Go files) | Widely imported data types |
| `cmd/web-chat/web/src/sem/pb/` (all protobuf TS files) | Widely imported data types |
| `pkg/chatapp/features.go` | Active `ChatPlugin` interface |
| `cmd/web-chat/agentmode_chat_feature.go` | Reference ChatPlugin implementation (agentmode) |
| `cmd/web-chat/reasoning_chat_feature.go` | Active ChatPlugin implementation (reasoning) |
| `cmd/web-chat/web/src/ws/wsManager.ts` | Production frontend, no SEM dependency |
| `cmd/web-chat/web/src/ws/wsManager.ts` | Production frontend, no SEM dependency |
| `cmd/web-chat/web/src/webchat/rendererRegistry.ts` | Active renderer registry |
