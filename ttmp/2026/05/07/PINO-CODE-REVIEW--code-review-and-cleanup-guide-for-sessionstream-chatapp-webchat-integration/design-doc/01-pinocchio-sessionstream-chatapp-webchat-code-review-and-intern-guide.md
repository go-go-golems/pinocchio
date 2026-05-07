---
Title: Pinocchio sessionstream, chatapp, and web-chat code review and intern onboarding guide
Ticket: PINO-CODE-REVIEW
Status: active
Topics:
  - code-review
  - cleanup
  - sessionstream
  - chatapp
  - web-chat
  - frontend
  - observability
DocType: design-doc
Intent: long-term
Owners:
  - manuel
RelatedFiles:
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/chat.go
    Note: Main chat domain engine, runtime sink, UI projection, and timeline projection implementation
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/features.go
    Note: ChatPlugin extension seam for reasoning/tool/agent-mode features
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/service.go
    Note: App-facing service wrapper around sessionstream commands and snapshots
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/plugins/reasoning.go
    Note: Reasoning/thinking plugin and provider-ID propagation into ReasoningUpdate
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/plugins/toolcall.go
    Note: Tool-call plugin and typed tool timeline entities
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/proto/pinocchio/chatapp/v1/chat.proto
    Note: Canonical chatapp protobuf payload schema
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/server.go
    Note: HTTP app server composition, session routes, websocket mounting, runtime resolver hookup
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_recorder.go
    Note: Backend stream/debug recorder for pipeline, transport, and Geppetto records
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_db.go
    Note: SQLite reconcile export schema, inserts, views, and provider/browser correlation
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/main.go
    Note: Cobra/Glazed command, static UI serving, app mux, runtime config, profile wiring
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/runtime_composer.go
    Note: Profile/runtime composition into Geppetto engine/middleware/tool-loop runtime
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/ws/wsManager.ts
    Note: Frontend websocket, snapshot hydration, UI event mapping, and stream debug capture
  - Path: /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/ws/protocol.ts
    Note: Frontend canonicalization of sessionstream websocket frames
ExternalSources: []
Summary: Architecture map and cleanup plan for the Pinocchio sessionstream/chatapp/web-chat integration, aimed at onboarding a new intern and prioritizing maintainability work after recent debug/protobuf/observability changes.
LastUpdated: 2026-05-07T00:00:00-04:00
WhatFor: Use this to understand and improve Pinocchio's session-based web-chat runtime pipeline.
WhenToUse: Read before changing chatapp events, sessionstream wiring, web-chat debug routes, frontend websocket state, or provider/browser correlation.
---

# Pinocchio sessionstream, chatapp, and web-chat code review and intern onboarding guide

## 1. Executive summary

Pinocchio's current web-chat stack is much healthier than the older ad-hoc stream/UI architecture. The important ownership split is now visible in code:

- **Sessionstream** owns sessions, command/event append, projection, hydration snapshots, websocket fanout, and transport/debug observer hooks.
- **Pinocchio `pkg/chatapp`** owns chat-domain commands, events, timeline entities, and the translation from Geppetto runtime events to sessionstream events.
- **Pinocchio `cmd/web-chat/app`** owns HTTP routes, runtime/profile resolution, debug APIs, SQLite reconcile export, and the production-ish app server composition.
- **Pinocchio `cmd/web-chat/web`** owns React rendering, frontend websocket normalization, snapshot hydration, UI event application, and frontend-side stream debug capture.
- **Geppetto** owns inference providers and provider/event observability records; Pinocchio records and exports those records but should not become a provider parser.

The main cleanup opportunity is not a conceptual rewrite. It is **reducing concentration**: several files now contain multiple layers at once because recent work landed quickly and correctly prioritized proving behavior. The next cleanup should make those proven seams easier to review and extend.

Highest-priority cleanup recommendations:

1. **Resolve cross-repository release alignment** so Pinocchio validates with pinned Sessionstream and Geppetto modules under `GOWORK=off`.
2. **Split `pkg/chatapp/chat.go`** into engine orchestration, runtime sink, projections, message helpers, and demo inference.
3. **Split `cmd/web-chat/app/debug_reconcile_db.go`** into schema, backend inserts, frontend inserts, Geppetto inserts, views, and provider interfaces.
4. **Split frontend `wsManager.ts`** into a websocket transport client, hydration coordinator, UI event mapper, and debug hooks.
5. **Make frontend payload mapping typed** for `ChatMessageUpdate`, `ReasoningUpdate`, `ToolCallUpdate`, and `ToolResultUpdate` instead of hand-reading generic JSON in one switch.
6. **Clarify app-local versus reusable plugin ownership**, especially for agent-mode code under `cmd/web-chat` versus shared plugins under `pkg/chatapp/plugins`.

## 2. Repository map for a new intern

### 2.1 Main directories

```text
pinocchio/
  proto/pinocchio/chatapp/v1/chat.proto      # canonical chat protobuf payloads
  pkg/chatapp/                               # reusable chat-domain sessionstream integration
    chat.go                                  # engine, runtime sink, projections, helpers
    features.go                              # ChatPlugin interface and feature fanout
    service.go                               # app-facing service wrapper
    plugins/                                 # shared reasoning/tool plugins
    export/                                  # timeline/turn export formats
  cmd/web-chat/                              # concrete app using chatapp + sessionstream
    main.go                                  # CLI, mux, static UI, profile/runtime wiring
    runtime_composer.go                      # profile -> Geppetto engine/middleware runtime
    app/                                     # HTTP app server and debug APIs
      server.go                              # session routes, websocket, snapshot/export
      debug_recorder.go                      # pipeline/transport/Geppetto record capture
      debug_reconcile_db.go                  # SQLite reconcile export and views
    web/src/                                 # React frontend
      ws/                                    # websocket protocol, manager, debug capture
      store/                                 # Redux app/timeline/profile slices
      webchat/                               # ChatWidget and timeline rendering
  pkg/persistence/chatstore/                 # turn/timeline stores
  pkg/inference/runtime/                     # runtime request/profile/fingerprint structs
  pkg/doc/topics/                            # architecture docs and playbooks
```

### 2.2 Large-file inventory

The current largest relevant files are:

| File | Approx lines | Review note |
| --- | ---: | --- |
| `pkg/chatapp/chat.go` | 802 | Multiple responsibilities in one file: engine state, runtime sink, projections, helpers, demo inference. |
| `cmd/web-chat/app/debug_reconcile_db.go` | 778 | Schema, views, frontend parsing, backend parsing, inserts, and exports all in one file. |
| `cmd/web-chat/app/debug_recorder.go` | 473 | Three recorder domains in one file: pipeline, transport, Geppetto. |
| `cmd/web-chat/main.go` | 451 | CLI settings, profile wiring, HTTP mux, static serving, runtime config. |
| `cmd/web-chat/web/src/ws/wsManager.ts` | 432 | Websocket lifecycle, hydration, event mapping, timeline conversion, debug hooks. |
| `pkg/chatapp/plugins/reasoning.go` | 483 | Reasoning state machine plus provider metadata parsing plus projections. |
| `cmd/web-chat/profiles/api.go` | 534 | Profile HTTP API surface; not central to this report but relevant to web-chat complexity. |

Generated files such as `pkg/chatapp/pb/.../chat.pb.go` are large by nature and should not drive cleanup decisions unless generation ownership is unclear.

## 3. Runtime flow: from browser prompt to model answer

### 3.1 Big picture

```text
Browser React UI
  |
  | POST /api/chat/sessions/{sessionId}/messages
  v
cmd/web-chat/app.Server.handleSubmitMessage
  |
  | resolve profile/runtime if configured
  v
pkg/chatapp.Service.SubmitPromptRequest
  |
  | hub.Submit(ChatStartInference, StartInferenceCommand)
  v
sessionstream.Hub
  |
  | command handler
  v
pkg/chatapp.Engine.handleStartInference
  |
  | publish ChatUserMessageAccepted
  | start goroutine runPrompt
  v
pkg/chatapp.Engine.runRuntimeInference
  |
  | Geppetto session + runtimeEventSink
  v
Geppetto engine emits events
  |
  | EventPartialCompletion, EventInfo(reasoning-summary), EventToolCall, ...
  v
runtimeEventSink + ChatPlugin handlers
  |
  | publish ChatTokensDelta / ChatReasoningDelta / ChatToolCallStarted / ...
  v
sessionstream projections
  |
  | UI events + timeline entities
  v
websocket fanout + hydration store
  |
  | /api/chat/ws frames
  v
Frontend wsManager.ts
  |
  | normalize frame -> mutation -> Redux timeline
  v
ChatWidget renders timeline
```

### 3.2 Backend ownership by layer

`cmd/web-chat/app/server.go` constructs the live backend:

```go
reg := sessionstream.NewSchemaRegistry()
_ = chatapp.RegisterSchemas(reg, s.chatPlugins...)
store, cleanup, _ := newHydrationStore(s, reg)
ws := wstransport.NewServer(provider, wstransport.WithTransportObserver(...))
engine := chatapp.NewEngine(chatapp.WithPlugins(...), chatapp.WithTurnStore(...))
hub := sessionstream.NewHub(
    sessionstream.WithSchemaRegistry(reg),
    sessionstream.WithHydrationStore(store),
    sessionstream.WithUIFanout(ws),
    sessionstream.WithPipelineObserver(...),
)
_ = chatapp.Install(hub, engine)
service := chatapp.NewService(hub, engine)
```

Important API references:

- `chatapp.RegisterSchemas(reg, features...)`: registers command/event/UI/timeline payload types.
- `chatapp.Install(hub, engine)`: installs command handlers and projection functions.
- `chatapp.Service.SubmitPromptRequest`: app-facing method used by HTTP handlers.
- `sessionstream.Hub.Submit`: lower-level command submission.
- `wstransport.Server`: websocket transport and fanout target.

### 3.3 The event contract

The current chat app uses typed protobuf payloads from `proto/pinocchio/chatapp/v1/chat.proto`.

Core command/event/entity names:

| Domain | Names | Payload |
| --- | --- | --- |
| Commands | `ChatStartInference`, `ChatStopInference` | `StartInferenceCommand`, `StopInferenceCommand` |
| Base events | `ChatUserMessageAccepted`, `ChatInferenceStarted`, `ChatTokensDelta`, `ChatInferenceFinished`, `ChatInferenceStopped` | `ChatMessageUpdate` |
| Base UI events | `ChatMessageAccepted`, `ChatMessageStarted`, `ChatMessageAppended`, `ChatMessageFinished`, `ChatMessageStopped` | `ChatMessageUpdate` |
| Base timeline | `ChatMessage` | `ChatMessageEntity` |
| Reasoning | `ChatReasoningStarted`, `ChatReasoningDelta`, `ChatReasoningFinished` | `ReasoningUpdate` |
| Tools | `ChatToolCallStarted`, `ChatToolCallUpdated`, `ChatToolCallFinished`, `ChatToolResultReady` | `ToolCallUpdate`, `ToolResultUpdate` |
| Tool timeline | `ChatToolCall`, `ChatToolResult` | `ToolCallEntity`, `ToolResultEntity` |

The protobuf migration is the right direction. The architecture test and docs around schema policy are valuable because top-level `google.protobuf.Struct` payloads become unreviewable once persisted and hydrated.

## 4. Recent improvements worth preserving

### 4.1 Typed protobuf chatapp payloads

The `PINO-PROTO-SCHEMAS` work moved important chatapp payloads into `chat.proto`. This gives the backend and frontend a shared durable schema. Keep this rule:

> A top-level sessionstream command, backend event, UI event, or timeline entity should use a named protobuf message, not a generic struct map.

Open-ended fields are still acceptable inside typed messages for intentionally open-ended tool input/output or provider metadata.

### 4.2 Shared ChatPlugin seam

`pkg/chatapp/features.go` is a strong extension seam:

```go
type ChatPlugin interface {
    RegisterSchemas(reg *sessionstream.SchemaRegistry) error
    HandleRuntimeEvent(ctx context.Context, runtime RuntimeEventContext, event gepevents.Event) (bool, error)
    ProjectUI(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error)
    ProjectTimeline(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error)
}
```

This is good because it keeps reasoning/tool/agent-mode features out of the base chat message projection. It is also the right place to add future product-specific features without modifying Sessionstream.

### 4.3 Debug recorder and reconcile DB

The recent debug recorder is valuable. It captures three independent evidence streams:

- Sessionstream pipeline records.
- Websocket transport records.
- Geppetto provider/event observability records.

The SQLite exporter then lets us query browser delivery and provider-to-browser correlation. This is exactly the right validation style for high-frequency streaming bugs.

### 4.4 Provider IDs on `ReasoningUpdate`

The new `ReasoningUpdate` fields close a major debugging gap:

- `provider`
- `response_id`
- `item_id`
- `output_index`
- `summary_index`

Before this, provider-to-browser correlation required row order and chunk matching. With these fields, future SQL can join through durable browser-visible payloads.

## 5. Code-quality findings and cleanup sketches

### 5.1 `pkg/chatapp/chat.go` is doing too much

Problem: `chat.go` contains engine state, command handling, runtime inference, demo inference, the Geppetto runtime event sink, base UI projection, base timeline projection, ID helpers, protobuf JSON helpers, and text chunking. This makes it hard for a new intern to reason about one layer without reading all layers.

Where to look:

- `pkg/chatapp/chat.go`
- `Engine.handleStartInference`
- `Engine.runRuntimeInference`
- `runtimeEventSink.PublishEvent`
- `baseUIProjection`
- `baseTimelineProjection`

Example snippet:

```go
func (s *runtimeEventSink) PublishEvent(event gepevents.Event) error {
    switch ev := event.(type) {
    case *gepevents.EventPartialCompletion:
        textMessageID, segment := s.ensureTextSegmentID()
        ...
        return s.engine.publish(..., EventTokensDelta, payload)
    case *gepevents.EventFinal:
        ...
    default:
        if isTranscriptBoundaryEvent(event) { ... }
        return s.engine.handleFeatureRuntimeEvent(...)
    }
}
```

Why it matters:

- Runtime event semantics are subtle: text segments close before tool/reasoning boundaries, final events should not be duplicated, and context cancellation is intentionally stripped for publish-after-cancel.
- Reviewers currently need to understand these invariants while navigating unrelated helper code.

Cleanup sketch:

```text
pkg/chatapp/
  engine.go              # Engine, activeRun, NewEngine, command handlers
  runtime_inference.go   # runRuntimeInference and turn history loading
  runtime_sink.go        # runtimeEventSink, terminal/text-segment state
  projections.go         # baseUIProjection, baseTimelineProjection
  messages.go            # newChatMessageUpdate/Delta, IDs, status helpers
  demo.go                # runDemoInference, renderAnswer, chunkText
  json.go                # protoMessageAsMap / encode helper if still needed
```

Start with a file split only. Do not change package names or public APIs in the first cleanup commit.

### 5.2 The runtime sink is a small state machine but is not named as one

Problem: `runtimeEventSink` holds state (`lastText`, `terminal`, `textSegment`, `textActive`) and transitions it based on Geppetto events. That is a state machine, but the code exposes the state as scattered mutex-protected fields.

Where to look:

- `pkg/chatapp/chat.go`: `runtimeEventSink`
- `ensureTextSegmentID`
- `finishTextSegment`
- `stoppedMessageUpdate`
- `PublishEvent`

Why it matters:

- Bugs here show up as duplicate final messages, missing partial text after tool calls, stopped events on the wrong entity, or parent/segment IDs that do not match frontend expectations.
- The logic is now important because reasoning/tool plugins create sibling timeline entities under the same parent message.

Cleanup sketch:

```go
type textSegmentState struct {
    parentMessageID string
    segment int32
    active bool
    lastText string
    terminal bool
}

func (s *textSegmentState) ApplyPartial(delta, completion string) TextSegmentUpdate
func (s *textSegmentState) FinishForBoundary() (TextSegmentUpdate, bool)
func (s *textSegmentState) Stop(err string) TextSegmentUpdate
func (s *textSegmentState) Final(text string) TextSegmentUpdate
```

Then `runtimeEventSink.PublishEvent` becomes a dispatcher that asks the state machine for updates and publishes the returned protobuf payloads.

### 5.3 `cmd/web-chat/app/debug_reconcile_db.go` should be split by artifact type

Problem: `debug_reconcile_db.go` contains frontend upload parsing, SQLite schema creation, view creation, backend record inserts, frontend record inserts, Geppetto record inserts, timeline inserts, turn inserts, and helper functions in one file.

Where to look:

- `cmd/web-chat/app/debug_reconcile_db.go`
- `BuildSQLiteReconcileDB`
- `createDebugSQLiteSchema`
- `createDebugSQLiteViews`
- `insertBackendDebugRecords`
- `insertFrontendDebugRecords`
- `insertGeppettoRecord`

Example snippet:

```go
func (r *StreamDebugRecorder) BuildSQLiteReconcileDB(ctx context.Context, sessionID string, body io.Reader, provider DebugDataProvider) ([]byte, error) {
    frontendRecords, err := parseFrontendLogUpload(body)
    ...
    if err := createDebugSQLiteSchema(ctx, db); err != nil { ... }
    backendRecords := r.Records(sessionID, "")
    if err := insertDebugSQLiteMeta(ctx, db, sessionID, len(backendRecords), len(frontendRecords)); err != nil { ... }
    if err := insertBackendDebugRecords(ctx, db, backendRecords); err != nil { ... }
    if err := insertFrontendDebugRecords(ctx, db, frontendRecords); err != nil { ... }
    ...
    if err := createDebugSQLiteViews(ctx, db); err != nil { ... }
}
```

Why it matters:

- This file is becoming a mini data warehouse. Each new correlation question adds columns/views and increases the chance of breaking unrelated export paths.
- SQLite schema and view definitions are easier to review if they are grouped by record domain.

Cleanup sketch:

```text
cmd/web-chat/app/debugdb/
  builder.go             # BuildSQLiteReconcileDB orchestration
  schema.go              # CREATE TABLE / indexes
  views.go               # CREATE VIEW strings
  insert_backend.go      # pipeline + transport inserts
  insert_frontend.go     # frontend upload parser + inserts
  insert_geppetto.go     # Geppetto record flattening
  insert_snapshots.go    # timeline entities + turns
```

If keeping package `app` is simpler initially, split into `debug_reconcile_schema.go`, `debug_reconcile_views.go`, etc. first.

### 5.4 `debug_recorder.go` mixes three recorder domains

Problem: one recorder type is fine, but one file currently defines all record DTOs and encoders for pipeline, transport, and Geppetto domains.

Where to look:

- `cmd/web-chat/app/debug_recorder.go`
- `DebugRecordKindPipeline`
- `DebugRecordKindTransport`
- `DebugRecordKindGeppetto`
- `encodePipelineRecord`
- `encodeTransportRecord`
- `encodeGeppettoRecord`

Why it matters:

- The recorder is a boundary object. Changes in Sessionstream observer APIs and Geppetto observability APIs both land in this file, so it becomes a conflict magnet.
- It is easy to accidentally add app-specific interpretation to a neutral provider record.

Cleanup sketch:

```text
cmd/web-chat/app/
  debug_recorder.go             # StreamDebugRecorder, append, Records, Reconcile
  debug_record_pipeline.go      # Pipeline DTO + encoder
  debug_record_transport.go     # Transport DTO + encoder
  debug_record_geppetto.go      # Geppetto DTO + encoder
```

Keep Geppetto flattening mechanical: decode JSON, copy IDs, do not reinterpret provider semantics in Pinocchio.

### 5.5 Frontend `wsManager.ts` is a transport client, hydrator, mapper, and debug recorder

Problem: `wsManager.ts` owns websocket lifecycle, snapshot application, UI event mutation mapping, status updates, buffered hydration, frontend debug logging, and entity constructors.

Where to look:

- `cmd/web-chat/web/src/ws/wsManager.ts`
- `timelineEntityFromSnapshotEntity`
- `timelineMutationFromUIEvent`
- `WsManager.connect`
- `WsManager.handleFrame`

Example snippet:

```ts
case 'ChatReasoningAppended':
  return {
    upsert: messageEntity(messageId, {
      role: 'thinking',
      content: asString(payload.content) || asString(payload.text) || asString(payload.chunk),
      status: asString(payload.status) || 'streaming',
      streaming: payload.streaming !== false,
    }),
    status: 'streaming',
  };
```

Why it matters:

- When new protobuf fields were added to `ReasoningUpdate`, the frontend transport path did not need to understand them for rendering, but debug/correlation benefits from preserving them in parsed frame JSON.
- If rendering later needs provider fields, this switch will grow again.
- Hydration buffering and event mapping are separate concerns with separate tests.

Cleanup sketch:

```text
cmd/web-chat/web/src/ws/
  client.ts                 # WebSocket lifecycle and subscribe frame
  hydration.ts              # snapshot gate + buffered UI events
  protocol.ts               # frame canonicalization (already exists)
  timelineMapping.ts        # snapshot/entity/UI-event -> TimelineMutation
  streamDebug.ts            # debug hooks (already exists)
  wsManager.ts              # small facade used by ChatWidget
```

Pseudocode:

```ts
const client = createSessionstreamClient({ basePrefix });
const hydrator = createHydrationCoordinator({ applySnapshot, applyUIEvent });
client.onFrame((frame) => hydrator.accept(frame));
```

### 5.6 Frontend mapping is not yet schema-aware enough

Problem: backend payloads are typed protobuf messages, but frontend mapping reads `Record<string, unknown>` manually. This is pragmatic but weakens the value of the schema on the browser side.

Where to look:

- `cmd/web-chat/web/src/ws/protocol.ts`
- `cmd/web-chat/web/src/ws/wsManager.ts`
- generated TS protobuf files under `cmd/web-chat/web/src/sem/pb/...` and missing/partial chatapp generated TS usage

Why it matters:

- Field renames or optional field presence can silently fail in UI mapping.
- Reasoning provider IDs are now payload fields, but there is no typed frontend utility that says, “this frame is a `ReasoningUpdate`.”

Cleanup sketch:

```ts
type KnownUIEvent =
  | { name: 'ChatMessageAppended'; payload: ChatMessageUpdate }
  | { name: 'ChatReasoningAppended'; payload: ReasoningUpdate }
  | { name: 'ChatToolCallUpdated'; payload: ToolCallUpdate };

function decodeKnownUIEvent(frame: CanonicalFrame): KnownUIEvent | null {
  switch (frame.name) {
    case 'ChatReasoningAppended':
      return { name: frame.name, payload: ReasoningUpdate.fromJson(frame.payload) };
  }
}
```

The current JSON frame shape may require using `@bufbuild/protobuf` helpers or generated `fromJson` APIs. Do this after confirming generated chatapp TS code is present and stable.

### 5.7 Reasoning plugin state should be keyed by parent and provider item where possible

Problem: `ReasoningPlugin` currently tracks segment state by parent message ID. This works for the observed provider stream, but provider IDs are now available and can make the state machine more explicit.

Where to look:

- `pkg/chatapp/plugins/reasoning.go`
- `reasoningSegmentState`
- `updateReasoningProviderInfo`
- `summaryReasoningSegment`

Why it matters:

- If a provider interleaves multiple reasoning items under one assistant message, parent-only state may be too coarse.
- The plugin now carries provider IDs into `ReasoningUpdate`; those IDs can also protect the segment state machine.

Cleanup sketch:

```go
type reasoningKey struct {
    parentMessageID string
    provider string
    responseID string
    itemID string
    outputIndex *int32
    summaryIndex *int32
}

type ReasoningPlugin struct {
    mu sync.Mutex
    byParent map[string]parentReasoningState // current compatibility path
    byItem map[reasoningKey]reasoningSegmentState // explicit provider-aware path
}
```

Implementation update, 2026-05-07: this hardening was implemented after provider IDs were propagated into reasoning payloads. The plugin now keys reasoning state by parent message plus provider item identity when available, keeps parent-only fallback behavior, and includes tests for metadata-routed interleaving and summaries for completed provider items.

### 5.8 Agent-mode plugin is intentionally app-local

Decision: agent mode is app-local web-chat glue. Shared plugins live under `pkg/chatapp/plugins`, while agent-mode plugin files intentionally remain under `cmd/web-chat`. Interns should not move this code into the reusable package unless the app-local product contract changes.

Where to look:

- `cmd/web-chat/agentmode_chat_feature.go`
- `cmd/web-chat/agentmode_sink.go`
- `pkg/middlewares/agentmode`
- `pkg/chatapp/plugins`

Why it matters:

- Shared reusable chat features and web-chat example features have different compatibility expectations.
- If third-party users want agent-mode timeline entities, they should not import `cmd/web-chat`.

Cleanup sketch:

Make the ownership explicit with comments near the plugin definition:

```go
// agentModePlugin is web-chat-app-local glue. Do not import from reusable packages.
// Promote to pkg/chatapp/plugins only if the product contract becomes reusable.
```

### 5.9 `cmd/web-chat/main.go` is still an example app and an integration harness

Problem: `main.go` handles static file serving, app config JS, profile API registration, debug route registration, Cobra/Glazed flags, runtime settings, stores, and server lifecycle.

Where to look:

- `cmd/web-chat/main.go`
- `runtimeConfigScript`
- `buildAppMux`
- `registerStaticUIHandlers`
- `Command.RunIntoGlazeProcessor` / command setup code

Why it matters:

- This is acceptable for a command, but it is not a reusable library surface.
- Prior docs already say legacy routes are removed and `cmd/web-chat` is canonical app wiring; keep it that way, but make the file easier to read.

Cleanup sketch:

```text
cmd/web-chat/
  main.go                   # cobra entrypoint only
  static_handlers.go         # app-config.js + static UI serving
  mux.go                     # buildAppMux route composition
  command_settings.go        # Glazed settings structs/sections decode
  runtime_wiring.go          # profile/runtime/debug/store construction
```

### 5.10 Generated and lockfile clutter can distract code review

Problem: `cmd/web-chat/web` contains both `package-lock.json` and `pnpm-lock.yaml`, and `node_modules` can be huge locally. The repo ignores `node_modules`, but multiple package manager locks can confuse contributors.

Where to look:

- `cmd/web-chat/web/package.json`
- `cmd/web-chat/web/package-lock.json`
- `cmd/web-chat/web/pnpm-lock.yaml`
- `cmd/web-chat/web/.gitignore`

Why it matters:

- Tooling drift makes frontend pre-commit failures harder to reproduce.
- The recent hook run installed many npm packages; a single package manager policy would reduce noise.

Cleanup sketch:

```text
Decision: pnpm or npm, not both.
If pnpm:
  - document corepack/pnpm version
  - remove package-lock.json
  - update Makefile/hooks to use pnpm consistently
If npm:
  - remove pnpm-lock.yaml
  - update docs/scripts to use npm consistently
```

Do this as a separate frontend-tooling cleanup because lockfile changes can be noisy.

### 5.11 Debug retention policy is count-based but not byte/stage-aware

Problem: `StreamDebugRecorder` uses a max record count. High-frequency Geppetto/provider and frontend debug records can still create large JSON/SQLite artifacts when each record carries payload JSON.

Where to look:

- `cmd/web-chat/app/debug_recorder.go`: `defaultDebugRecorderMaxRecords`, `append`
- `cmd/web-chat/app/debug_reconcile_db.go`: SQLite export of raw JSON/payload JSON fields

Why it matters:

- The browser smoke produced large SQLite artifacts even for simple arithmetic prompts.
- Count-based retention is simple but does not distinguish small transport lifecycle records from large provider/event payload records.

Cleanup sketch:

```go
type DebugRetentionPolicy struct {
    MaxRecords int
    MaxApproxBytes int
    PerKind map[DebugRecordKind]KindRetention
}

type KindRetention struct {
    MaxRecords int
    IncludePayloads bool
}
```

Keep Geppetto itself simple; enforce artifact-size policy at the Pinocchio recorder/export boundary.

### 5.12 Cross-repository dependency alignment remains a release risk

Problem: Pinocchio now imports local Geppetto observability and local Sessionstream observer APIs. Workspace tests can pass while `GOWORK=off` lint/release validation fails if pinned module versions do not include those APIs.

Where to look:

- `pinocchio/go.mod`
- `cmd/web-chat/app/debug_recorder.go`
- `cmd/web-chat/app/server.go`
- sibling `sessionstream/pkg/sessionstream/...` observer API work
- Geppetto `pkg/observability`

Why it matters:

- This is the most important operational cleanup before shipping.
- A code review can approve the architecture and still fail CI/release if module versions are not aligned.

Cleanup sketch:

```text
1. Finish Sessionstream observer API.
2. Tag Sessionstream.
3. Update Pinocchio go.mod to that Sessionstream version.
4. Tag Geppetto observability package.
5. Update Pinocchio go.mod to that Geppetto version.
6. Run:
   GOWORK=off go test ./...
   make lintmax
7. Remove any local replace directives before release.
```

## 6. Suggested cleanup roadmap

### Phase 1: Documentation and validation closure

- Keep this ticket and the recent GP/PINO diaries linked from review PRs.
- Run a fresh browser-backed SQLite export after the `ReasoningUpdate` provider fields and verify `backend_item_id` and `frontend_item_id` are populated.
- Resolve the `GOWORK=off` module alignment issue.

### Phase 2: Behavior-preserving file splits

- Split `pkg/chatapp/chat.go` without changing APIs.
- Split `debug_recorder.go` by record domain.
- Split `debug_reconcile_db.go` by schema/inserts/views.
- Split `wsManager.ts` by transport/hydration/mapping/debug.

### Phase 3: Typed frontend payload decoding

- Generate and use chatapp TypeScript protobuf types if not already wired.
- Replace generic UI-event payload mapping with typed event decoders.
- Add tests proving new fields such as `ReasoningUpdate.itemId` survive into debug/correlation paths.

### Phase 4: State-machine hardening

- Extract text segment state from `runtimeEventSink`.
- Add fixture tests for interleaved text/tool/reasoning events.
- Consider provider-item-aware reasoning state only if fixtures demonstrate a need.

### Phase 5: Reusable package boundary cleanup

- Decide whether agent-mode belongs in shared `pkg/chatapp/plugins` or remains web-chat-local.
- Keep `cmd/web-chat` as an app/example boundary; move only genuinely reusable pieces into `pkg`.

## 7. API reference cheat sheet

### Backend submission

```go
service.SubmitPromptRequest(ctx, sid, chatapp.PromptRequest{
    Prompt: prompt,
    Runtime: composedRuntime,
})
```

### ChatPlugin implementation

```go
type MyPlugin struct{}

func (p *MyPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error { ... }
func (p *MyPlugin) HandleRuntimeEvent(ctx context.Context, rt chatapp.RuntimeEventContext, ev gepevents.Event) (bool, error) { ... }
func (p *MyPlugin) ProjectUI(ctx context.Context, ev sessionstream.Event, sess *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) { ... }
func (p *MyPlugin) ProjectTimeline(ctx context.Context, ev sessionstream.Event, sess *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) { ... }
```

### Runtime composition

```go
runtime, err := composer.Compose(ctx, infruntime.ConversationRuntimeRequest{
    ConvID: sessionID,
    ProfileKey: profileKey,
    ResolvedProfileRuntime: runtimeConfig,
    ResolvedInferenceSettings: inferenceSettings,
})
```

### Frontend websocket flow

```ts
wsManager.connect({
  sessionId,
  basePrefix,
  dispatch,
  hydrate: true,
});
```

Frame handling shape:

```text
raw websocket bytes
  -> parseServerFrame
  -> normalizeServerFrame
  -> snapshot? applySnapshot
  -> ui-event? buffer until hydrated, then timelineMutationFromUIEvent
  -> Redux timelineSlice.upsertEntity
```

## 8. Review checklist for future PRs

Before approving a change in this area, ask:

- Does every new sessionstream event/UI event/timeline entity use a typed protobuf payload?
- Is the event name registered in `RegisterSchemas` before it can be emitted?
- Does the timeline projection preserve existing entity content when an update carries only status?
- Does websocket hydration handle late events deterministically?
- Does the frontend understand any new payload fields, or are they only debug/correlation evidence?
- Does the debug recorder capture enough evidence without becoming a second implementation of provider semantics?
- Does the change pass with `GOWORK=off` if it crosses module boundaries?
- Does the browser/SQLite proof cover the end-to-end path if the change affects streaming?

## 9. What not to do

- Do not move product-specific chat features into Sessionstream. Sessionstream should stay a generic session/event/projection/transport substrate.
- Do not reintroduce legacy `/chat`, `/ws`, or `/api/timeline` routes unless there is a deliberate compatibility ticket.
- Do not register top-level `google.protobuf.Struct` payloads for new sessionstream contracts.
- Do not make Geppetto observability records app-specific; Pinocchio can store and export them, but Geppetto should stay neutral.
- Do not mix release-alignment fixes with large file splits; module/version changes need clean validation.

## 10. Bottom line

The architecture is directionally sound. The codebase now has the right primitives: typed chat payloads, plugin-based feature projection, sessionstream-backed hydration/fanout, and strong debug evidence from backend through browser. The cleanup work should preserve those decisions while reducing file size, clarifying state machines, and making release validation reproducible outside the local workspace.
