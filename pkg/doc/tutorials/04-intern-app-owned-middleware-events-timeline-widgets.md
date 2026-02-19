---
Title: Intern Tutorial: Build an App-Owned Middleware Feature from Event Emission to Timeline Widget Rendering
Slug: intern-app-owned-middleware-events-timeline-widgets
Short: Exhaustive step-by-step tutorial for implementing an app-owned middleware feature from backend events to frontend timeline widget rendering.
Topics:
- webchat
- middleware
- timeline
- widgets
- sem
- protobuf
- frontend
- backend
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
---

## Who this is for

This tutorial is written for a new intern developer working in `pinocchio/` who needs to add a complete feature with the same architecture style as thinking-mode. You will implement a feature end-to-end:

1. backend middleware emits typed events,
2. events are translated into SEM frames,
3. SEM frames project into timeline entities,
4. frontend SEM handlers map frames to state,
5. React widget renderers display the entities,
6. all ownership stays app-local under `cmd/web-chat`.

The emphasis is not only "make it work," but "make it modular, testable, and maintainable." The examples in this tutorial reference the current thinking-mode implementation and teach the reusable pattern.

## What you will build

You will build a feature module called `myfeature` that mirrors thinking-mode architecture:

- backend module location: `cmd/web-chat/myfeature/`
- frontend module location: `cmd/web-chat/web/src/features/myFeature/`
- app-owned proto location: `cmd/web-chat/proto/sem/...`
- Go generated bindings location: `cmd/web-chat/myfeature/pb/`
- TS generated bindings location: `cmd/web-chat/web/src/features/myFeature/pb/`

You will not put app-specific feature logic in `pkg/webchat`.

## Architectural mental model

### Big-picture data flow

```text
Runtime middleware (Go)
  -> emits typed geppetto events
      -> SEM registry translator (Go)
          -> SEM frame { sem: true, event: { type, id, data, seq... } }
              -> StreamHub/WebSocket
                  -> frontend SEM registry handler (TS)
                      -> timeline store upsert
                          -> renderer registry dispatch by entity kind
                              -> React widget
```

### Where persistence fits

```text
SEM frame
  -> TimelineProjector.ApplySemFrame(...)
      -> timeline handler registry (type -> handler)
          -> upsert TimelineEntityV2(kind, props)
              -> TimelineStore (SQLite/in-memory)
                  -> hydration API (/api/timeline)
```

### Why this split exists

- `pkg/webchat` is reusable core.
- `cmd/web-chat` is app-owned composition.
- app-specific features must be registered explicitly from app entrypoints, not hidden behind `init()` side effects.

That split is the key design goal.

## Current reference implementation you should study first

Read these files before coding:

- backend app module bootstrap:
  - `cmd/web-chat/thinkingmode/backend.go`
- backend app-owned event contracts:
  - `cmd/web-chat/thinkingmode/events.go`
- frontend app module registration:
  - `cmd/web-chat/web/src/features/thinkingMode/registerThinkingMode.tsx`
- backend startup wiring:
  - `cmd/web-chat/main.go`
- frontend startup wiring:
  - `cmd/web-chat/web/src/ws/wsManager.ts`
- frontend renderer registry:
  - `cmd/web-chat/web/src/webchat/rendererRegistry.ts`
- frontend props normalizer registry:
  - `cmd/web-chat/web/src/sem/timelinePropsRegistry.ts`
- timeline SEM registry:
  - `cmd/web-chat/web/src/sem/registry.ts`

Study tests too:

- backend module tests:
  - `cmd/web-chat/thinkingmode/backend_test.go`
- frontend module tests:
  - `cmd/web-chat/web/src/features/thinkingMode/registerThinkingMode.test.tsx`
- isolation gate tests:
  - `cmd/web-chat/thinkingmode/isolation_test.go`

## Prerequisites and tooling

You need this toolchain:

- Go toolchain matching repo `go.mod`
- Node/npm for `cmd/web-chat/web`
- Buf CLI for protobuf generation
- a clean branch and passing baseline tests

Baseline validation commands:

```bash
go test ./... -count=1
cd cmd/web-chat/web && npm run check
```

## Step 0: Choose ownership before writing code

This decision controls everything later.

Use this rule:

- if reusable across multiple binaries/apps in repo: candidate for `pkg/`
- if specific to web-chat application behavior: place in `cmd/web-chat/`

For thinking-mode-style features, choose app-owned by default unless you have a clear reuse requirement.

### Ownership checklist

- Does it require app-specific UX or app-specific middleware semantics?
- Does it depend on app-specific renderer behavior?
- Does it need to evolve independently from shared core release cadence?

If yes, keep it under `cmd/web-chat`.

## Step 1: Implement typed event contracts (backend)

Create event contracts in your module package, not in `pkg/inference/events`.

Target pattern (from `cmd/web-chat/thinkingmode/events.go`):

- payload struct with typed fields
- typed event structs embedding `gepevents.EventImpl`
- constructors `NewEventXxx(...)`
- explicit registration helper (called from module bootstrap)

### Template

```go
package myfeature

import gepevents "github.com/go-go-golems/geppetto/pkg/events"

type MyFeaturePayload struct {
  Mode string `json:"mode"`
  Note string `json:"note,omitempty"`
}

type EventMyFeatureStarted struct {
  gepevents.EventImpl
  ItemID string            `json:"item_id"`
  Data   *MyFeaturePayload `json:"data,omitempty"`
}

func NewMyFeatureStarted(meta gepevents.EventMetadata, itemID string, data *MyFeaturePayload) *EventMyFeatureStarted {
  return &EventMyFeatureStarted{
    EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("myfeature.started"), Metadata_: meta},
    ItemID: itemID,
    Data: data,
  }
}
```

### Explicit factory registration (important)

Do not use `init()`; expose a module function and call it from `Register()`.

```go
func registerMyFeatureEventFactories() {
  registerFactory("myfeature.started", func() gepevents.Event {
    return &EventMyFeatureStarted{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("myfeature.started")}}
  })
}
```

Reason: explicit bootstrap makes ownership and lifecycle obvious in code review.

## Step 2: Emit events from middleware/runtime flow

Now your runtime middleware emits these events.

Typical path:

- middleware does work,
- publishes progress + completion events via `events.PublishEventToContext(...)`.

### Pseudocode

```text
on middleware start:
  publish EventMyFeatureStarted

on each update:
  publish EventMyFeatureUpdate

on completion:
  publish EventMyFeatureCompleted(success,error)
```

### Example command-side registration

In app startup, middleware factory registration remains in `cmd/web-chat/main.go`:

```go
srv.RegisterMiddleware("myfeature", func(cfg any) geppettomw.Middleware {
  return myfeature.NewMiddleware(myfeature.ConfigFromAny(cfg))
})
```

Keep middleware type/config close to module unless intentionally shared.

## Step 3: Register SEM translation handlers (backend)

Event emission alone does not reach frontend. You must map typed events to SEM frames.

Use `semregistry.RegisterByType[...]` in module backend file (same style as `registerSemTranslatorHandlers()` in `cmd/web-chat/thinkingmode/backend.go`).

### Core responsibilities

- choose SEM event names (`myfeature.started`, `myfeature.update`, `myfeature.completed`)
- encode payload shape (JSON object or protobuf-authored JSON)
- emit envelope through `wrapSem` shape:

```json
{
  "sem": true,
  "event": {
    "type": "myfeature.started",
    "id": "<itemID>",
    "data": { ... }
  }
}
```

### Backend translator template

```go
semregistry.RegisterByType[*EventMyFeatureStarted](func(ev *EventMyFeatureStarted) ([][]byte, error) {
  payload := map[string]any{
    "itemId": ev.ItemID,
    "data":   payloadFromEvent(ev.Data),
  }
  return [][]byte{wrapSem(map[string]any{
    "type": "myfeature.started",
    "id":   ev.ItemID,
    "data": payload,
  })}, nil
})
```

### Naming guidance

Use stable, explicit event names with phase suffixes. For example:

- `myfeature.started`
- `myfeature.update`
- `myfeature.completed`

Avoid overloaded event types that need ad-hoc branching on many optional fields.

## Step 4: Register timeline projection handlers (backend)

SEM frames are transient unless projected to timeline entities.

Register your handlers with `webchat.RegisterTimelineHandler(eventType, handler)` from module bootstrap (see `registerTimelineProjectionHandlers()` in `cmd/web-chat/thinkingmode/backend.go`).

### Projection handler responsibilities

- parse SEM payload (`ev.Data`)
- normalize status (`active`, `completed`, `error`)
- choose stable timeline entity ID (`itemId` or fallback `ev.ID`)
- upsert `TimelineEntityV2` with `kind` and open `props`

### Projection pseudocode

```text
on myfeature.started:
  decode payload
  entity.kind = "my_feature"
  entity.props = { schemaVersion:1, status:"active", ... }
  upsert(ev.Seq, entity)

on myfeature.completed:
  decode payload
  status = success ? "completed" : "error"
  props.success = success
  props.error = error
  upsert(ev.Seq, entity)
```

### Concrete data contract guidance

Always include:

- `schemaVersion`
- `status`
- domain fields needed by widget

This prevents fragile UI logic based on implicit shape.

## Step 5: Create frontend SEM projection module

Frontend module should be self-contained in `cmd/web-chat/web/src/features/myFeature/registerMyFeature.tsx`.

Use the same three registrations in one exported bootstrap function:

1. `registerSem(...)`
2. `registerTimelinePropsNormalizer(kind, fn)`
3. `registerTimelineRenderer(kind, component)`

### Frontend registration shape

```ts
export function registerMyFeatureModule() {
  registerTimelinePropsNormalizer('my_feature', normalizeMyFeatureProps);
  registerTimelineRenderer('my_feature', MyFeatureCard);

  registerSem('myfeature.started', (ev, dispatch) => {
    // parse ev.data
    // dispatch timelineSlice.actions.upsertEntity(...)
  });
}
```

### Why normalize props

Normalizer protects rendering from schema drift and hydration inconsistencies.

Examples:

- convert nullable strings to `''`
- derive `success` from status if missing
- clamp bad numbers

Without this layer, renderers become full of repetitive defensive checks.

## Step 6: Build the React widget

Widget is a pure renderer from entity props.

### Required behavior

- render meaningful summary in header
- show status and phase
- render optional details cleanly
- handle missing fields safely

### Widget skeleton

```tsx
function MyFeatureCard({ e }: { e: RenderEntity }) {
  const status = String(e.props?.status ?? '');
  const title = String(e.props?.title ?? 'My Feature');
  const detail = String(e.props?.detail ?? '');

  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="card-header-title">{title}</div>
        {status ? <div data-part="pill">{status}</div> : null}
      </div>
      <div data-part="card-body">{detail || <div data-part="pill">No detail</div>}</div>
    </div>
  );
}
```

Keep styling consistent with existing `ChatWidget` card language by reusing existing `data-part` conventions.

## Step 7: Bootstrap registration explicitly (backend + frontend)

### Backend bootstrap

In `cmd/web-chat/main.go`:

```go
myfeature.Register()
```

Call this once during startup before request handling.

### Frontend bootstrap

In `cmd/web-chat/web/src/ws/wsManager.ts`:

```ts
registerDefaultSemHandlers();
registerMyFeatureModule();
```

Also call in story/demo bootstrap (`ChatWidget.stories.tsx`) so feature appears in Storybook fixtures.

### No hidden side effects

Do not rely on import-time registration for module behavior. Make registration calls explicit in bootstrap paths.

## Step 8: Proto ownership and generation (when needed)

If your feature uses protobuf-authored payload schemas, place app-owned schemas under:

- `cmd/web-chat/proto/sem/middleware/...`
- `cmd/web-chat/proto/sem/timeline/...`

Do not place app-owned schemas in shared `proto/sem/...`.

### Current generation setup

- root module (`buf.yaml`) excludes `cmd/web-chat/proto`
- app module config:
  - `cmd/web-chat/proto/buf.yaml`
  - `cmd/web-chat/proto/buf.gen.yaml`

Generated outputs:

- Go: `cmd/web-chat/thinkingmode/pb/*.pb.go`
- TS: `cmd/web-chat/web/src/features/thinkingMode/pb/sem/.../*_pb.ts`

### Regeneration commands

```bash
# shared/core protos
make proto-gen-core

# app-owned web-chat protos
make proto-gen-web-chat

# both
make proto-gen
```

### Rule of thumb

- Add new app feature payload kinds in `cmd/web-chat/proto`.
- Do not edit root transport schema for new timeline kinds (`TimelineEntityV2` is open `kind + props`).

## Step 9: Add tests before finishing

At minimum, add these tests:

1. backend SEM translation test
2. backend timeline projection test
3. frontend registration test
4. isolation boundary test

### Backend tests

Pattern from `cmd/web-chat/thinkingmode/backend_test.go`:

- construct event (`NewThinkingModeStarted` style)
- call `semregistry.Handle(ev)`
- assert emitted SEM type, ID, and payload shape

Projection test:

- build sem frame JSON
- apply with projector
- fetch snapshot from in-memory store
- assert entity kind and props

### Frontend tests

Pattern from `cmd/web-chat/web/src/features/thinkingMode/registerThinkingMode.test.tsx`:

- clear registries
- call `registerMyFeatureModule()`
- assert renderer and normalizer registered
- simulate `handleSem(...)`
- assert `timeline.byId[id].kind` and normalized props

### Isolation gate test

Pattern from `cmd/web-chat/thinkingmode/isolation_test.go`:

- scan source tree for feature markers
- fail if markers appear outside allowed module paths

This is how architectural rules become enforceable, not aspirational.

## Step 10: Debugging guide when something breaks

### Symptom: event emitted but not in WS

Check:

- event type string matches `RegisterByType` type expectation
- translator registration called in `Register()`
- `Register()` called in `main.go`

### Symptom: WS frame exists but no timeline entity

Check:

- timeline handler registration for the exact event type
- payload decode path (`json.Unmarshal`/`protojson.Unmarshal`) matches actual payload keys
- `p.Upsert(...)` called with stable ID

### Symptom: entity exists but widget missing

Check:

- `kind` string exactly matches `registerTimelineRenderer(kind, ...)`
- `registerMyFeatureModule()` called in `wsManager.ts`
- props normalizer not accidentally dropping fields

### Symptom: widget appears live but disappears after refresh

Check:

- backend projection handler writes to timeline
- `/api/timeline` hydration includes your entity kind
- frontend `timelineEntityFromProto` + normalizer handles your kind

## Step 11: Delivery checklist

Before opening PR:

1. run generators (if proto changed)
2. run Go tests
3. run frontend check + tests
4. verify docs and ticket diary/changelog/tasks updated
5. verify no ownership regression (`pkg/` should not contain app-owned feature logic)

Suggested command sequence:

```bash
make proto-gen
go test ./... -count=1
cd cmd/web-chat/web && npm run check
cd cmd/web-chat/web && npx vitest run src/features/myFeature/registerMyFeature.test.tsx
```

## Complete reference sequence (copy/paste workflow)

```text
A. Create feature module skeleton
  cmd/web-chat/myfeature/
    events.go
    backend.go
    backend_test.go

B. Add frontend module skeleton
  cmd/web-chat/web/src/features/myFeature/
    registerMyFeature.tsx
    registerMyFeature.test.tsx

C. Wire explicit bootstrap
  cmd/web-chat/main.go -> myfeature.Register()
  cmd/web-chat/web/src/ws/wsManager.ts -> registerMyFeatureModule()

D. If protobuf contracts needed
  cmd/web-chat/proto/sem/middleware/my_feature.proto
  cmd/web-chat/proto/sem/timeline/my_feature.proto
  run make proto-gen-web-chat

E. Add isolation checks
  cmd/web-chat/myfeature/isolation_test.go

F. Validate and commit
```

## Pseudocode blueprint you can adapt quickly

```text
module Register():
  once:
    registerEventFactories()
    registerSemTranslatorHandlers()
    registerTimelineProjectionHandlers()

registerSemTranslatorHandlers():
  RegisterByType(EventStarted) -> emit sem frame type "feature.started"
  RegisterByType(EventUpdate) -> emit sem frame type "feature.update"
  RegisterByType(EventCompleted) -> emit sem frame type "feature.completed"

registerTimelineProjectionHandlers():
  RegisterTimelineHandler("feature.started", projectionHandler)
  RegisterTimelineHandler("feature.update", projectionHandler)
  RegisterTimelineHandler("feature.completed", projectionHandler)

projectionHandler(ev):
  decode ev.data
  derive id/status
  build props map with schemaVersion
  Upsert(ev.Seq, TimelineEntityV2{kind:"feature_kind", props})

frontend registerFeatureModule():
  registerTimelinePropsNormalizer("feature_kind", normalize)
  registerTimelineRenderer("feature_kind", FeatureCard)
  registerSem("feature.started", semHandlerStarted)
  registerSem("feature.update", semHandlerUpdate)
  registerSem("feature.completed", semHandlerCompleted)
```

## Design principles to enforce during review

1. explicit bootstrap over hidden side effects
2. app-owned feature logic in `cmd/web-chat`, not `pkg/`
3. stable IDs for upsert semantics
4. normalized props before render
5. tests for translation, projection, rendering, and isolation boundaries
6. no transport oneof edits for new entity kinds

If you follow these six rules, your feature will scale with the architecture instead of fighting it.

## Final notes for interns

When in doubt, copy the thinking-mode pattern first, then rename and simplify. Do not invent a new lifecycle shape unless your feature truly needs one.

Safe strategy:

- make backend translation pass,
- then make projection pass,
- then make frontend render pass,
- then harden with tests and isolation guard,
- then document what you changed.

That order keeps debugging shallow and prevents the "everything changed and nothing works" trap.

## Deep dive: file-by-file implementation map

This section gives you a concrete map of what each file should contain. Use it as a practical coding checklist while implementing your own module.

### Backend module files

#### `cmd/web-chat/myfeature/events.go`

Purpose:

- define event payload structs and event constructors,
- register event factories for decode paths,
- keep event type constants and naming close to feature.

Recommended structure:

1. payload types (`MyFeaturePayload`, `MyFeatureResultPayload`),
2. `EventMyFeatureStarted`, `EventMyFeatureUpdate`, `EventMyFeatureCompleted`,
3. constructor functions for each event,
4. `registerMyFeatureEventFactories()` helper.

Design note:

- keep payloads small and explicit, even if you also carry `ExtraData`.
- avoid adding unrelated metadata in payload when it already exists in `EventMetadata`.

#### `cmd/web-chat/myfeature/backend.go`

Purpose:

- be the only backend integration entrypoint for this feature.

Must include:

- `Register()` with `sync.Once`,
- SEM translation registration,
- timeline projection handler registration,
- decode helpers,
- SEM envelope helper.

Sequence inside `Register()` should be deterministic:

```text
register event factories
-> register sem translator handlers
-> register timeline handlers
```

Why this order:

- factory registration should happen before any decode path needs it,
- SEM and timeline handlers depend on stable type names.

#### `cmd/web-chat/myfeature/backend_test.go`

Purpose:

- prove registration behaves as expected,
- prove translation output is correct,
- prove projection writes expected entity shape.

At minimum include:

- `TestRegister_RegistersSemTranslation`,
- `TestRegister_ProjectsTimelineEntity`.

Use in-memory timeline store to avoid SQLite complexity in module tests.

#### `cmd/web-chat/myfeature/isolation_test.go` (recommended)

Purpose:

- prevent ownership regressions.

Pattern:

- scan source tree for marker strings,
- allow only feature module path and explicit bootstrap locations,
- fail if marker appears in `pkg/webchat` or generic frontend registry files.

This is how you prevent architecture drift over time.

### Frontend module files

#### `cmd/web-chat/web/src/features/myFeature/registerMyFeature.tsx`

Purpose:

- own SEM->entity projection,
- own renderer registration,
- own props normalization.

Keep these in one module so bootstrapping is one call:

```ts
registerMyFeatureModule();
```

Do not scatter these across random files.

#### `cmd/web-chat/web/src/features/myFeature/registerMyFeature.test.tsx`

Purpose:

- ensure registry calls happen,
- ensure emitted SEM frames produce expected timeline entries.

Test both:

- registration side effects (renderer + normalizer present),
- end-to-end handler dispatch through timeline reducer.

#### Optional UI component split

If the renderer gets large, split into:

- `MyFeatureCard.tsx` for presentation,
- `registerMyFeature.tsx` for registration/bootstrap logic.

Do not split too early. Start simple, then refactor when needed.

## Concrete end-to-end implementation sketch

This sketch is intentionally explicit and repetitive so a new contributor can use it as a scaffolding template.

### Backend sketch (high-level pseudocode)

```text
File: events.go
  - define payload + event structs
  - constructors
  - registerMyFeatureEventFactories()

File: backend.go
  - Register() once
  - registerSemTranslatorHandlers()
    - EventMyFeatureStarted -> sem event myfeature.started
    - EventMyFeatureUpdate -> sem event myfeature.update
    - EventMyFeatureCompleted -> sem event myfeature.completed
  - registerTimelineProjectionHandlers()
    - register handler for each sem event type
  - myFeatureTimelineHandler()
    - decode payload
    - derive status
    - upsert TimelineEntityV2(kind="my_feature", props=...)
```

### Frontend sketch (high-level pseudocode)

```text
File: registerMyFeature.tsx
  - normalizeMyFeatureProps(props)
  - MyFeatureCard component
  - registerMyFeatureModule():
      registerTimelinePropsNormalizer("my_feature", normalize)
      registerTimelineRenderer("my_feature", MyFeatureCard)
      registerSem("myfeature.started", semHandlerStarted)
      registerSem("myfeature.update", semHandlerUpdate)
      registerSem("myfeature.completed", semHandlerCompleted)

File: wsManager.ts
  - registerDefaultSemHandlers()
  - registerMyFeatureModule()
```

### Runtime sequence trace

```text
User sends prompt
  -> middleware starts and emits EventMyFeatureStarted
  -> sem translator emits myfeature.started
  -> timeline handler upserts kind=my_feature status=active
  -> ws frame received by frontend
  -> frontend handler upserts same entity id
  -> renderer registry finds my_feature
  -> MyFeatureCard renders
```

## Data contract design guidance

A feature succeeds or fails mostly based on contract stability. This section explains what to include in payloads and props.

### SEM payload contract

Recommended fields:

- `itemId`: stable logical entity id
- `data`: structured payload
- `success`/`error` for completion/failure events

`itemId` is critical because it gives you deterministic upsert behavior across updates.

### Timeline props contract (`TimelineEntityV2.props`)

Recommended minimum:

- `schemaVersion` (number),
- `status` (string enum),
- domain data fields (strings/numbers/booleans),
- optional `error`.

Status normalization should be centralized. Example canonical statuses:

- `active`
- `completed`
- `error`

Do not create many similar status strings (`done`, `finished`, `ok`, `success`) unless there is a concrete product need.

### Versioning strategy

When changing props schema:

1. bump `schemaVersion`,
2. update frontend normalizer to handle old/new forms if needed during transition,
3. add tests for both expected shapes.

In hard-cutover phases, you can drop backward compatibility deliberately, but document it and update tests immediately.

## Frontend rendering strategy and UX quality

A common intern mistake is focusing only on data plumbing and leaving widget UX vague. The widget should communicate state clearly.

### Widget readability checklist

- clear title/header,
- status indicator (pill/badge),
- meaningful empty-state text,
- error output separated visually,
- compact layout for timeline scanning,
- avoid giant unbounded blobs in default view.

### Suggested card composition

```text
Header: feature name + phase + status + timestamp
Body: summary content
Footer (optional): debug metadata (only if needed)
```

### Handling large payloads

If payload can be large (reasoning text, logs, JSON):

- show summarized preview in card,
- provide expandable section for full detail,
- avoid rendering huge raw JSON by default.

## Failure patterns and recovery playbook

This section describes common real failures and direct fixes.

### Failure 1: Duplicate entity rows

Symptom:

- every update creates a new card instead of updating existing one.

Cause:

- unstable `id` usage, often fallback to random ID per event.

Fix:

- ensure translators and handlers carry `itemId`,
- use same `itemId` for lifecycle events,
- fallback to `ev.ID` only when payload is missing item id.

### Failure 2: Feature works live but not after refresh

Symptom:

- card appears during stream, disappears on page reload.

Cause:

- frontend handler exists, but backend projection handler missing or failing.

Fix:

- verify `webchat.RegisterTimelineHandler` calls exist,
- verify projection decode path is not returning early due unmarshal failure,
- inspect `/api/timeline?conv_id=...` payload.

### Failure 3: Renderer not found

Symptom:

- entity appears in state, fallback renderer shown.

Cause:

- `kind` mismatch (`myFeature` vs `my_feature`) or module bootstrap not called.

Fix:

- standardize kind string in one constant used by backend and frontend,
- ensure `registerMyFeatureModule()` called after default registrations.

### Failure 4: Registry reset drops feature handlers

Symptom:

- works once, fails after reconnect.

Cause:

- default registry reset clears handlers and feature module did not re-register.

Fix:

- register feature module each time `registerDefaultSemHandlers()` runs (see `wsManager.ts` pattern).

## Intern execution plan (day-by-day)

This is a practical schedule for a new teammate.

### Day 1: Contract + backend wiring

Deliverables:

- event contracts,
- SEM translator handlers,
- timeline projection handlers,
- backend tests passing.

Commands:

```bash
go test ./cmd/web-chat/myfeature -count=1
```

### Day 2: Frontend registration + widget

Deliverables:

- feature module registration file,
- renderer component,
- frontend tests passing.

Commands:

```bash
cd cmd/web-chat/web && npx vitest run src/features/myFeature/registerMyFeature.test.tsx
cd cmd/web-chat/web && npm run check
```

### Day 3: Hardening and docs

Deliverables:

- isolation gate test,
- docs update (feature flow and ownership),
- ticket diary/changelog/tasks updated.

Commands:

```bash
go test ./... -count=1
```

## Review rubric for mentors

Use this rubric during code review to evaluate intern submissions.

### Architecture (must pass)

- feature logic isolated in `cmd/web-chat`,
- no accidental additions to `pkg/webchat` for app-specific behavior,
- explicit bootstrap in `main.go` and `wsManager.ts`.

### Correctness (must pass)

- SEM types consistent across emitter/translator/handler,
- entity IDs stable across lifecycle,
- timeline persistence verified by tests.

### Maintainability (must pass)

- no hidden registration side effects,
- normalization logic centralized,
- tests cover translation + projection + rendering.

### UX quality (should pass)

- widget communicates status clearly,
- failure states visible and readable,
- no noisy or unreadable default rendering.
