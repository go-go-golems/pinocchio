Got it. Here’s a **mechanical mapping table** (old → new) and a **suggested commit sequence** that keeps the repo buildable most of the way, but still lands as a **hard cutover** (i.e., no long-lived compatibility layer).

I’m basing this on the current/refactored structure you’ve described (and the existing Pinocchio webchat layout: `router.go`, `conversation.go`, `conversation_service.go`, `stream_coordinator.go`, `timeline_*`, etc.). If a filename differs slightly in your tree, treat this as a “rename/move intent map”.

---

# A. Target package layout

You can do this in two ways:

## Option 1 (recommended for cutover): keep `pkg/webchat`, split into subpackages

This reduces external import churn while still clarifying the domains.

```
pinocchio/pkg/webchat/
  stream/
    backend.go
    backend_inmem.go
    backend_redis.go
    hub.go
    coordinator.go
    connection_pool.go
    sem_buffer.go
    sem_envelope.go
    topic.go

  chat/
    service.go
    state.go
    queue.go
    runtime_provider.go
    engine_builder.go   (optional; or fold into runtime provider)
    tools.go
    turns.go

  timeline/
    projector.go
    registry.go
    handlers_builtin.go
    service.go
    http.go

  http/
    chat_handler.go
    ws_handler.go
    timeline_handler.go
    ui_handler.go
    resolvers.go

  bootstrap/ (optional convenience)
    from_values.go
    open_stores.go
```

## Option 2: new root package name (`pkg/chatkit` / `pkg/webchatkit`)

Cleaner conceptually, but bigger ripple across imports.

For a hard cutover, Option 1 is usually the sweet spot: **clear decomposition without a package rename blast radius**.

---

# B. Mechanical mapping table (old file → new file + what moves)

Below is a “move list” you can literally execute.

> Legend: **Keep** = move with minimal edits; **Split** = break into multiple new files; **Replace** = delete old and reimplement.

---

## B1) Router and route ownership

### `pinocchio/pkg/webchat/router.go` → **Replace**

**Old responsibilities:**

* builds backend (redis/inmem)
* mounts `/chat`, `/ws`, `/api/timeline`, debug, UI
* owns `ConvManager` construction
* owns store initialization

**New home:**

* backend init → `webchat/stream/backend_*.go`
* store init → `webchat/bootstrap/open_stores.go`
* UI handler → `webchat/http/ui_handler.go`
* debug endpoints (if kept) → `webchat/http/debug_handler.go` or `webchat/bootstrap/debug.go`
* **no mux ownership**

✅ **Delete `Router`** in final cutover commit.
✅ Replace with optional `bootstrap.FromValues(...)` helpers.

---

### `pinocchio/pkg/webchat/router_timeline_api.go` → `pinocchio/pkg/webchat/timeline/http.go` (**Keep, move**)

Becomes an app-mounted handler:

* Old: `router.timelineSnapshotHandler`
* New: `timeline.NewHTTPHandler(timelineSvc, resolver)` or method `svc.HTTPHandler()`

---

## B2) Streaming subsystem (WS + stream consumption + fanout)

### `pinocchio/pkg/webchat/stream_coordinator.go` → `pinocchio/pkg/webchat/stream/coordinator.go` (**Keep + modify**)

**Change required:**

* accept two payload types:

  1. Geppetto event JSON → translate → SEM frames (existing)
  2. SEM envelope JSON (new) → patch cursor seq/stream_id → forward as-is

Add tests for SEM envelope pass-through.

---

### `pinocchio/pkg/webchat/connection_pool.go` → `pinocchio/pkg/webchat/stream/connection_pool.go` (**Keep**)

Minimal changes; it remains “WS fanout per conversation”.

But: don’t let pool be the only stream-stop trigger (see StreamHub maintenance below).

---

### (new) `pinocchio/pkg/webchat/stream/hub.go` (**New, extracted from conversation.go/ConvManager**)

This replaces the streaming half of `ConvManager` + `Conversation`:

* owns subscriber, coordinator, WS pool, sem buffer, projector
* starts/stops consumer loop based on activity
* offers:

  * `Ensure(convID)`
  * `AttachWebSocket(...)`
  * `PublishSEM(...)` (for background agents + internal app emits)

---

### `pinocchio/pkg/webchat/sem_translator.go` → `pinocchio/pkg/webchat/stream/sem_translator.go` (**Move**)

(or keep at root if used by multiple domains)
This is still used by the coordinator.

---

## B3) Timeline subsystem (projection + hydration)

### `pinocchio/pkg/webchat/timeline_projector.go` → `pinocchio/pkg/webchat/timeline/projector.go` (**Keep, move**)

### `pinocchio/pkg/webchat/timeline_registry.go` → `pinocchio/pkg/webchat/timeline/registry.go` (**Keep, move**)

### `pinocchio/pkg/webchat/timeline_upsert.go` → `pinocchio/pkg/webchat/timeline/upsert.go` (**Keep, move**)

**Change required:** add built-in handler for user messages (see chat section).

---

### (new) `pinocchio/pkg/webchat/timeline/service.go` (**New**)

Wraps `TimelineStore.GetSnapshot`.

---

### (new) `pinocchio/pkg/webchat/timeline/handlers_builtin.go` (**New**)

Registers builtin projections like:

* `chat.message` (user messages)
* optionally other app events if you want defaults

---

## B4) Chat subsystem (queue + session + runtime + inference)

### `pinocchio/pkg/webchat/send_queue.go` → `pinocchio/pkg/webchat/chat/queue.go` (**Move**)

### `pinocchio/pkg/webchat/conversation_service.go` → `pinocchio/pkg/webchat/chat/service.go` (**Split**)

### `pinocchio/pkg/webchat/conversation.go` → **Split heavily**

Right now this file mixes:

* streaming state (subscriber/coordinator/pool/projector)
* chat state (session/engine/sink/queue)

After cutover:

* streaming state moves to `stream/hub.go`
* chat state moves to `chat/state.go`

---

### Runtime builder files:

* `pinocchio/pkg/webchat/engine_builder.go` → `pinocchio/pkg/webchat/chat/engine_builder.go` (**Move or fold**)
* `pinocchio/pkg/webchat/engine_config.go` → `pinocchio/pkg/webchat/chat/runtime_config.go` (**Move or fold**)

But note: if you fully embrace `ChatRuntimeProvider`, you may **fold** engine builder + profiles into the app layer. The toolkit only needs the interface.

---

## B5) HTTP helpers (app-owned routing)

You already have chat/ws handler helpers in your GP‑025 refactor. Hard cutover makes them target the new services.

### `pinocchio/pkg/webchat/chat_handler.go` (or wherever) → `pinocchio/pkg/webchat/http/chat_handler.go` (**Adjust**)

* Input: `*chat.Service` + resolver
* No longer depends on ConversationService.

### `pinocchio/pkg/webchat/ws_handler.go` → `pinocchio/pkg/webchat/http/ws_handler.go` (**Adjust**)

* Input: `*stream.Hub` + resolver + upgrader

### Add:

* `pinocchio/pkg/webchat/http/timeline_handler.go`

---

# C. Symbol rename map (what developers will see)

These renames remove the “what is ConvManager vs ConversationService” confusion:

| Old Symbol               | New Symbol                                         | Notes                                                     |
| ------------------------ | -------------------------------------------------- | --------------------------------------------------------- |
| `ConvManager`            | `stream.Hub`                                       | owns per-conv stream: sub/coordinator/ws/projector/buffer |
| `Conversation`           | `stream.state` + `chat.state`                      | no longer a single mega-type                              |
| `ConversationService`    | `chat.Service`                                     | only inference / queue / idempotency; no WS               |
| `WSPublisher`            | `stream.Hub.PublishSEM` (and internal broadcaster) | publishing goes through stream pipeline                   |
| `router_timeline_api.go` | `timeline.Service` + `timeline.HTTPHandler`        | app mounts it                                             |
| `Router`                 | *(deleted)* or `bootstrap.Bundle`                  | no mux ownership                                          |

---

# D. Suggested commit sequence (buildable most of the way)

This sequence avoids “mass delete then rewrite”. You introduce new modules in parallel, switch callsites, then delete old.

## Commit 1 — Add new directory skeleton + topic helper

**Goal:** no behavior changes, compile-only.

* Add `pkg/webchat/stream/topic.go`:

  * `func TopicForConversation(convID string) string { return "chat:" + convID }`
* Add stub packages: `stream/`, `chat/`, `timeline/`, `http/`, `bootstrap/`
* No code moved yet.

✅ Build should pass.

---

## Commit 2 — Extract StreamBackend (in-memory + redis)

**Goal:** move backend construction out of router.

* Add:

  * `stream/backend.go` interface
  * `stream/backend_inmem.go`
  * `stream/backend_redis.go` using existing redisstream helpers
* Keep router untouched for now; router can call the new backend functions internally (temporary).

✅ Build passes.
✅ Add a small unit test that `backend.Sink/Subscribe` works in memory.

---

## Commit 3 — Teach StreamCoordinator to accept SEM envelopes

**Goal:** enable unified stream pipeline for agent/app emits.

* Modify coordinator:

  * detect SEM envelope payload
  * patch `seq` + `stream_id`
  * forward frame directly
* Add tests:

  * payload is SEM envelope → onFrame called exactly once
  * seq matches cursor seq

✅ Build passes.

---

## Commit 4 — Introduce TimelineService + move timeline HTTP handler

**Goal:** timeline read-side becomes reusable and router no longer “special”.

* Move logic from `router_timeline_api.go` into:

  * `timeline/service.go`
  * `timeline/http.go`
* Keep old router endpoint temporarily by delegating to timeline service (so no behavior change yet).

✅ Build passes.
✅ cmd/web-chat still works.

---

## Commit 5 — Add builtin timeline handler for user messages (`chat.message`)

**Goal:** projection supports user messages without Chat writing directly to store.

* Add `timeline/handlers_builtin.go`:

  * register handler for `"chat.message"` (or your chosen name)
* Ensure it calls projector upsert with `ev.Seq` and creates the right entity snapshot.

✅ Build passes.
✅ Add a unit test: projector applies a `chat.message` frame → store has message entity.

---

## Commit 6 — Implement StreamHub (new) using existing internals

**Goal:** consolidate streaming responsibilities, but don’t delete old ConvManager yet.

* Add `stream/hub.go`:

  * holds per-conv stream state:

    * `ConnectionPool`
    * `StreamCoordinator`
    * `TimelineProjector` (optional, if store present)
    * optional sem buffer
  * methods:

    * `Ensure(ctx, convID)`
    * `AttachWebSocket(...)`
    * `PublishSEM(...)` (publishes SEM envelope via backend publisher)
    * `StartMaintenance(...)` (stop/evict policy)
* **In this commit, you can internally re-use existing `ConnectionPool`, `StreamCoordinator`, `TimelineProjector` with minimal changes.**

✅ Build passes.
✅ Add one integration-ish test in memory:

* Ensure → AttachWS → PublishSEM → WS receives frame; projector called if enabled.

---

## Commit 7 — Implement chat.Service by extracting from ConversationService (but keep old APIs temporarily)

**Goal:** chat is now a dedicated service and calls StreamHub.

* Add `chat/service.go`, `chat/state.go`, `chat/queue.go`
* Move queue/idempotency logic from old `Conversation/ConversationService` into `chat.state`.
* Add `chat.RuntimeProvider` interface.
* In `chat.Service.SubmitPrompt`:

  * `streams.Ensure(convID)`
  * publish `"chat.message"` SEM event for the user prompt via `streams.PublishSEM(...)`
  * run inference as before (engine/session/sink)
  * **do not call TimelineStore.Upsert directly anymore**

For now, keep the old `ConversationService` in place but make it delegate internally to `chat.Service` (temporary).

✅ Build passes.

---

## Commit 8 — Update HTTP helpers: ChatHandler → uses chat.Service, WSHandler → uses stream.Hub

**Goal:** app-mounted endpoints now cleanly map to services.

* `NewChatHandler(chatSvc, resolver)` returns http.HandlerFunc
* `NewWSHandler(streamHub, resolver, upgrader)` returns http.HandlerFunc
* Timeline handler already exists from commit 4.

Keep old helper constructors around only if needed for compilation, but ideally you switch usages immediately.

✅ Build passes.

---

## Commit 9 — Cut cmd/web-chat over (remove Router usage)

**Goal:** first-party app uses the new wiring fully.

In `cmd/web-chat/main.go`:

* create backend (from values)
* create timeline store
* create stream hub
* create runtime provider (app-owned)
* create chat service
* mount:

  * `/chat` (chat handler)
  * `/ws` (ws handler)
  * `/api/timeline` (timeline handler)
  * `/` (ui handler)
* start `streams.StartMaintenance(ctx)`

At this point `Router` should not be used by cmd/web-chat at all.

✅ Build passes.
✅ Manual test: chat + ws streaming + timeline hydration.

---

## Commit 10 — Cut web-agent-example over

**Goal:** the “advanced integration” example uses the same decomposition.

* Use `EventSinkWrapper` support in `chat.ServiceConfig` (keep your disco wrapper behavior).
* Mount routes the same way.

✅ Build passes.

---

## Commit 11 (hard cutover) — Delete Router + ConvManager + old ConversationService

**Goal:** remove the legacy centerpiece and any “double path”.

Delete:

* `router.go`, `router_options.go`, `router_*`
* `conversation.go` (or split: remove streaming parts, keep chat state only if still relevant)
* legacy `ConversationService` if it still exists

Make sure:

* all references replaced with `stream.Hub`, `chat.Service`, `timeline.Service`

✅ Build passes.
✅ Tests updated/removed accordingly.

---

## Commit 12 — Polish: package docs + “how to integrate” docs + rename cleanup

**Goal:** make the new decomposition easy to understand.

* Add docs at:

  * `pkg/webchat/README.md` (or Go doc in `doc.go`)
* Include 3 recipes:

  * web chat app
  * background agent with progress stream
  * chat-only API (no WS)

✅ Build passes.

---

# E. A practical “file move ledger” (quick copy/paste checklist)

Here’s the minimal move list you can follow while doing commits 6–11:

### Streaming

* move `connection_pool.go` → `stream/connection_pool.go`
* move `stream_coordinator.go` → `stream/coordinator.go` (modify in commit 3)
* move `sem_translator.go` → `stream/sem_translator.go` (or keep at root if shared)
* add `stream/hub.go`, `stream/backend*.go`, `stream/sem_envelope.go`, `stream/topic.go`

### Timeline

* move `timeline_projector.go` → `timeline/projector.go`
* move `timeline_registry.go` → `timeline/registry.go`
* move `timeline_upsert.go` → `timeline/upsert.go`
* add `timeline/service.go`, `timeline/http.go`, `timeline/handlers_builtin.go`

### Chat

* move/refactor `send_queue.go` → `chat/queue.go`
* refactor from `conversation_service.go` → `chat/service.go`
* refactor chat session bits out of `conversation.go` → `chat/state.go`
* runtime provider interface in `chat/runtime_provider.go`

### HTTP helpers

* move handler glue into `http/`:

  * `http/chat_handler.go`
  * `http/ws_handler.go`
  * `http/timeline_handler.go`
  * `http/ui_handler.go`
  * `http/resolvers.go` (interfaces)

### Bootstrap (optional)

* extract value-based store opening from router into:

  * `bootstrap/open_stores.go`
  * `bootstrap/from_values.go`

---

# F. Two “don’t get cutover-burned” warnings

These are the two places hard cutovers usually fail:

## 1) Ordering/version monotonicity

If you previously wrote user messages directly with `version := now_ms*1_000_000`, you can get collisions with stream-derived seq.

**In the new plan**, user messages go through the stream (`PublishSEM`), and the coordinator assigns/preserves ordering → projector uses `ev.Seq` consistently.

That’s the correct fix. Don’t keep the direct-upsert shortcut.

## 2) “No WS ever attached” leaks

Your previous stop condition likely relied on ConnectionPool’s idle timer firing on Remove(). If no WS ever connected, it never stops.

**StreamHub maintenance must stop streams even if no WS ever attached**, based on `lastActivity`.

---
