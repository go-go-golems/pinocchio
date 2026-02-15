Below is a **full hard-cutover design + implementation guide** to move from the current “app-owned /chat + /ws” refactor (GP‑025) to a **cleanly decomposed toolkit** where:

* **Streaming/WebSocket** is a reusable service (for chat *and* background agents).
* **Timeline projection + hydration** is reusable and transport-agnostic.
* **Chat/inference** is separate from transport concerns.
* The old “router-as-centerpiece” disappears (or becomes a tiny optional convenience wrapper).

I’m grounding this in what’s in your refactor right now under `pinocchio/pkg/webchat/`:

* `conversation.go` (`ConvManager`, `Conversation`, stream callback wiring)
* `conversation_service.go` (SubmitPrompt + WS attach + timeline user message upsert)
* `stream_coordinator.go` (Watermill subscriber → Geppetto event → SEM frames)
* `timeline_projector.go` + `timeline_registry.go`
* `router.go` + `router_timeline_api.go` (stores + API/UI handlers)

---

# 1) The end state you want

## 1.1 “Webchat” becomes a toolkit, not a router

**Transport ownership stays in apps**, but the toolkit provides:

* **Chat API** (submit prompt / queue / idempotency / toolloop)
* **Stream API** (attach websocket / publish progress events / fanout SEM frames)
* **Timeline API** (project SEM frames to store + serve timeline snapshot endpoint)
* Optional UI/static serving helper

The “router” stops being the orchestrator and becomes:

* either **deleted**,
* or replaced by a tiny **Bootstrap/Bundle builder** that only wires dependencies (no route ownership).

## 1.2 Rename the confusing pieces into explicit domains

Your current conceptual knot is: `ConvManager + runtime + Conversation + ConversationService` mixing three distinct domains.

Hard-cutover naming that clarifies the domains:

### Domain A — Streaming / Realtime (WS + event bus)

* `StreamBackend` (pub/sub transport: memory/redis)
* `StreamHub` (per-conversation stream state: subscriber, WS pool, SEM buffer, projector)
* `StreamPublisher` (publish SEM events into a conversation stream; used by background agents too)

### Domain B — Chat / Inference (turns, idempotency, tools)

* `ChatService` (submit prompt, queue/idempotency, start inference)
* `ChatRuntimeProvider` (resolve runtime key/fingerprint, build engine, allowed tools)
* `ChatState` (per-conversation: session ID, engine, queue records)

### Domain C — Timeline (projection + hydration serving)

* `TimelineProjector` (already exists)
* `TimelineService` (read-side “get snapshot” + HTTP handler helper)
* `TimelineStore` stays in `chatstore`

---

# 2) Core design decisions for the hard cutover

## 2.1 Timeline projection must be fed by the same stream pipeline

Right now, timeline projection is wired inside the stream callback in `conversation.go`:

```go
if conv.timelineProj != nil {
    _ = conv.timelineProj.ApplySemFrame(conv.baseCtx, frame)
}
```

That’s good: it makes timeline projection **event-driven** and **transport independent**.

The major remaining “bad coupling” is in `ConversationService.startInferenceForPrompt`:

* it manually writes user messages to `TimelineStore.Upsert(...)`
* using `version := uint64(time.Now().UnixMilli()) * 1_000_000`

That creates two problems:

1. **version collisions / ordering mismatch** with stream-derived seq (esp. redis xid vs local time)
2. timeline becomes “chat-owned” instead of “stream-owned projection”

### Hard-cutover fix

**User message goes into the same SEM stream as everything else**, and the projector handles it.

That means you add a **SEM event type for user message**, plus a **timeline handler** for it.

* ChatService emits a SEM frame: `type="chat.message"` (or `chat.user.message`)
* TimelineProjector projects it using `RegisterTimelineHandler("chat.message", ...)`

No more direct `TimelineStore.Upsert` from chat.

## 2.2 Stream pipeline must accept both

* Geppetto event JSON (existing)
* Pre-built SEM envelopes (new)

Background agents want to stream progress **without** knowing WS details. The cleanest way is:

* agents publish a SEM envelope into the conversation topic
* the stream consumer assigns `seq` and fanouts to WS + projector

So `StreamCoordinator.consume` changes from:

* “always parse as geppetto event JSON”
  to:
* “if payload is SEM envelope → patch seq/stream_id → forward”
* else parse as geppetto event → translate → forward

This also means:

* `StreamHub` becomes the **one place** where “what gets projected and streamed” happens.

## 2.3 Router is not needed (but “Bootstrap” is nice)

What `router.go` still does for you today:

* build event backend (redis/inmem)
* open timeline/turn sqlite stores
* serve UI static assets
* mount timeline/debug APIs

All of that can be decomposed into:

* `NewStreamBackendFromValues(...)`
* `OpenTimelineStoreFromValues(...)`
* `OpenTurnStoreFromValues(...)`
* `UIHandler(staticFS)`
* `TimelineService.Handler()`

So: **you do not need `Router`**.
But a small optional “bundle builder” can help `cmd/web-chat` stay small.

---

# 3) The target developer APIs

This is the “nice to integrate” surface for apps.

## 3.1 Stream backend

```go
type StreamBackend interface {
    Publisher() message.Publisher
    UISubscriber(convID string) (sub message.Subscriber, close bool, err error)
    Close() error
}
```

* Redis backend uses your existing `redisstream.BuildRouter` and `BuildGroupSubscriber` logic.
* In-memory backend uses `gochannel`.

## 3.2 Stream hub

```go
type StreamHubConfig struct {
    BaseCtx context.Context
    Backend StreamBackend

    // Optional: used to enable projection + hydration.
    TimelineStore chatstore.TimelineStore

    // How many SEM frames to keep in memory for reconnect/offline debug.
    SemBufferSize int

    // Maintenance policy
    StopStreamAfter time.Duration   // stop subscriber loop if idle (even if never had WS)
    EvictStateAfter time.Duration   // remove in-memory state entirely
    SweepEvery      time.Duration

    // Optional hook to emit timeline.upsert notifications to WS
    TimelineUpsertHook func(convID string, entity *timelinepb.TimelineEntityV1, version uint64)
}

type StreamHub struct{ ... }

func NewStreamHub(cfg StreamHubConfig) (*StreamHub, error)

// Ensures stream state exists; starts subscriber loop if needed.
func (h *StreamHub) Ensure(ctx context.Context, convID string) error

// WebSocket attach: adds conn to pool + ping/pong loop + optional ws.hello.
func (h *StreamHub) AttachWebSocket(ctx context.Context, convID string, conn *websocket.Conn, opts WebSocketAttachOptions) error

// Publish a SEM envelope into the conversation stream (background agents use this).
func (h *StreamHub) PublishSEM(ctx context.Context, convID string, ev SEMEvent) error

// Optional: start sweep loop for stop+evict policy.
func (h *StreamHub) StartMaintenance(ctx context.Context)
```

`SEMEvent` is a lightweight event builder struct (not the full envelope):

```go
type SEMEvent struct {
    Type string
    ID   string
    Data json.RawMessage
    // optional
    Fields map[string]any
}
```

The hub will wrap it as `{sem:true, event:{type,id,data}}` and publish.

## 3.3 Timeline service

```go
type TimelineService struct {
    store chatstore.TimelineStore
}

func NewTimelineService(store chatstore.TimelineStore) *TimelineService

func (s *TimelineService) Snapshot(ctx context.Context, convID string, since uint64, limit int) (*timelinepb.TimelineSnapshotV1, error)

// Optional HTTP helper (replaces router_timeline_api.go)
func (s *TimelineService) HTTPHandler() http.HandlerFunc
```

Timeline projection is not part of TimelineService; it stays inside StreamHub via `TimelineProjector`.

## 3.4 Chat runtime provider

Hard-cutover: clarify “runtime” as “how to build engine + metadata”.

```go
type ChatRuntimeRequest struct {
    ConvID     string
    RuntimeKey string
    Overrides  map[string]any
}

type ChatRuntime struct {
    Engine engine.Engine

    RuntimeKey         string
    RuntimeFingerprint string

    SeedSystemPrompt string
    AllowedTools     []string
}

type ChatRuntimeProvider interface {
    Resolve(ctx context.Context, req ChatRuntimeRequest) (ChatRuntime, error)
}
```

No sink in runtime output. The sink is always “publish events to stream backend”, optionally wrapped.

## 3.5 Chat service

```go
type ChatServiceConfig struct {
    BaseCtx context.Context

    Streams *StreamHub
    Runtime ChatRuntimeProvider

    StepController *toolloop.StepController
    TurnStore      chatstore.TurnStore

    ToolFactories map[string]ToolFactory

    // Optional wrappers for event sinks (web-agent-example style)
    EventSinkWrapper func(convID string, req ChatRuntimeRequest, sink events.EventSink) (events.EventSink, error)

    // Queue/idempotency policy knobs if you want them.
}

type ChatService struct{ ... }

func NewChatService(cfg ChatServiceConfig) (*ChatService, error)

// Ensure chat state exists; returns handle for UI (conv_id/runtime/session etc).
func (s *ChatService) EnsureConversation(ctx context.Context, req ChatRuntimeRequest) (*ConversationHandle, error)

// Submit prompt: idempotency + queue + run inference.
// Does NOT handle websockets.
func (s *ChatService) SubmitPrompt(ctx context.Context, in SubmitPromptInput) (SubmitPromptResult, error)
```

**Key hard-cutover rule**: ChatService never touches websockets and never directly writes timeline store.
It only:

* ensures stream exists (`Streams.Ensure`)
* appends user turn
* emits user-message SEM event into the stream (so the projector can project it)

## 3.6 HTTP helpers (optional)

These become thin glue and can live in `pkg/webchat/httphelpers.go`:

```go
func NewChatHandler(chat *ChatService, resolver ConversationRequestResolver) http.HandlerFunc
func NewWSHandler(streams *StreamHub, resolver ConversationRequestResolver, upgrader websocket.Upgrader) http.HandlerFunc
func NewTimelineHandler(timeline *TimelineService) http.HandlerFunc
func UIHandler(staticFS fs.FS) http.Handler
```

---

# 4) Hard-cutover implementation guide

This is a “do it once, break everything, land clean” plan. It’s structured so you can implement in a few commits, but **the merge is a single cutover** (no compatibility shims).

## Step 0 — Declare the removal list

At the end of the cutover, these are **gone** (or replaced):

* `Router` (`router.go`, `router_options.go`, `router_*`)
* `ConversationService` (replaced by `ChatService`)
* `ConvManager` (replaced by internal store + `StreamHub` + `ChatService`)
* `WSPublisher` (replaced by `StreamHub.PublishSEM` and internal WS broadcaster)
* `NewFromRouter(...)` server wrapper becomes `NewServer(...)` or just app code

You keep:

* `ConnectionPool`
* `StreamCoordinator` (modified)
* `sem_translator.go`
* `TimelineProjector`, timeline registry
* queue/idempotency machinery (moved under chat service)

## Step 1 — Extract StreamBackend

### 1.1 Create `stream_backend.go`

Move logic from:

* `router.go` (redis flags, usesRedis, redisAddr, BuildSubscriber)
* `redisstream.BuildRouter` usage stays, but you don’t need `Router` anymore

Implement:

* `NewInMemoryStreamBackend()`

  * uses `events.NewEventRouter(...)` or directly `gochannel.NewGoChannel(...)`
  * returns Publisher/Subscriber pair

* `NewRedisStreamBackend(settings rediscfg.Settings)`

  * uses `redisstream.BuildRouter(settings, ...)` to get publisher
  * subscriber creation: replicate `buildSubscriberDefault` (EnsureGroupAtTail + BuildGroupSubscriber)

### 1.2 Decide “UI consumer group” policy

Right now it’s hardcoded:

* group `"ui"`
* consumer `"ws-forwarder:"+convID`

Keep that for cutover (don’t redesign scaling now). Just move it to the backend.

## Step 2 — Make StreamCoordinator accept SEM envelope messages

Modify `stream_coordinator.go`:

### 2.1 Add a fast SEM detection path

Pseudo-code inside the loop:

```go
for msg := range ch {
    cur := cursorFromMessage(msg)

    if isSEMEnvelope(msg.Payload) {
        frame := patchSEMEnvelope(msg.Payload, cur)
        sc.onFrame(nil, cur, frame)
        msg.Ack()
        continue
    }

    ev, err := events.NewEventFromJson(msg.Payload)
    ...
    for _, frame := range SemanticEventsFromEventWithCursor(ev, cur) {
        sc.onFrame(ev, cur, frame)
    }
    msg.Ack()
}
```

Where `patchSEMEnvelope` sets:

* `event.seq = cur.Seq`
* `event.stream_id = cur.StreamID` if present

**This one change is what makes background agents and internal publishers “first-class”.**

## Step 3 — Build StreamHub

Create `stream_hub.go`.

### 3.1 Stream state structure

Internal per-conv state should look like (roughly):

```go
type streamState struct {
    convID string

    pool   *ConnectionPool
    semBuf *semFrameBuffer
    proj   *TimelineProjector

    sub       message.Subscriber
    subClose  bool
    coord     *StreamCoordinator

    lastActivity time.Time
}
```

### 3.2 Creation path

`Ensure(convID)`:

* create state if missing
* build subscriber via backend
* create coordinator:

  * onFrame callback:

    * broadcast to pool
    * buffer
    * projector.ApplySemFrame
* start coordinator (or lazy-start at attach/publish)

### 3.3 Timeline projection integration

If `TimelineStore != nil`:

* create `TimelineProjector(convID, store, onUpsert)`

`onUpsert` should *not* depend on connection pool directly.
Implement one of these:

Option A (keep current behavior, simplest):

* `StreamHub` provides an internal `broadcastTimelineUpsert(convID, entity, version)` that sends to pool.

Option B (more “pure”):

* publish `timeline.upsert` SEM event into stream
* (timeline projector ignores it)
* clients receive it through the normal pipeline

For cutover stability, **Option A** is closer to your existing refactor.

### 3.4 Fix the “stream never stops if no WS ever connected” issue

In the new hub, do **not** rely on `ConnectionPool` idle timer for stream lifecycle.

Instead:

* keep `StopStreamAfter` in StreamHub config
* in the maintenance sweep:

  * if pool empty and coordinator running and `now-lastActivity > StopStreamAfter`: `coord.Stop()`

Update `lastActivity` when:

* WS attach/detach
* frame processed
* PublishSEM called

This makes eviction actually work for headless conversations.

## Step 4 — Make Timeline projection cover user messages

### 4.1 Register a timeline handler for user messages

In a new file `timeline_handlers_builtin.go` (or similar), register:

```go
RegisterTimelineHandler("chat.message", func(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, now int64) error {
    var snap timelinepb.MessageSnapshotV1
    if err := protojson.Unmarshal(ev.Data, &snap); err != nil {
        return nil
    }
    entity := &timelinepb.TimelineEntityV1{
        Id:   ev.ID,
        Kind: "message",
        Snapshot: &timelinepb.TimelineEntityV1_Message{
            Message: &snap,
        },
    }
    return p.Upsert(ctx, ev.Seq, entity)
})
```

Now the projector can store user messages as timeline entities.

## Step 5 — Build ChatService (replacing ConversationService)

Create `chat_service.go` by refactoring `conversation_service.go`.

### 5.1 Remove websocket attach and WSPublisher surface

* WS attach is now StreamHub’s job.

### 5.2 Remove direct TimelineStore writes from chat

Replace:

```go
s.timelineStore.Upsert(...)
s.emitTimelineUpsert(...)
```

With:

* emit a SEM event into the stream via StreamHub:

```go
snapRaw, _ := protoToRaw(&timelinepb.MessageSnapshotV1{ ... role:"user", content: prompt ... })
_ = s.streams.PublishSEM(ctx, convID, SEMEvent{
    Type: "chat.message",
    ID:   "user-" + turnID,
    Data: snapRaw,
})
```

### 5.3 Ensure stream exists before starting inference

At the beginning of `SubmitPrompt` or `startInferenceForPrompt`:

* `s.streams.Ensure(ctx, convID)`

This guarantees:

* stream consumer is alive
* events emitted by inference will be consumed/projected even without WS

### 5.4 Runtime provider integration

Move the runtime composer logic out of `ConvManager.GetOrCreate` and into ChatService:

ChatService per-conv chat state stores:

* `RuntimeKey`, `RuntimeFingerprint`, `SeedSystemPrompt`, `AllowedTools`
* `SessionID`
* `Sess *session.Session`
* `Eng engine.Engine`
* `Sink events.EventSink`

On `EnsureConversation`:

* call runtime provider `Resolve(...)`
* if fingerprint changed, rebuild engine and reset session builder state as needed

For sink:

* base sink always: `middleware.NewWatermillSink(streamBackend.Publisher(), topicForConv(convID))`
* then apply `EventSinkWrapper` if provided

### 5.5 Keep queue/idempotency as-is

Your existing idempotency and queue semantics in `PrepareSessionInference(...)`, `activeRequestKey`, `requests` map, etc can be moved into the chat state struct unchanged.

(That’s core correctness; don’t change it during cutover.)

## Step 6 — TimelineService and HTTP handler extraction

Replace `router_timeline_api.go` with `timeline_service.go` + `timeline_http.go`:

* `TimelineService.Snapshot(...)` wraps `store.GetSnapshot(...)`
* `HTTPHandler()` basically becomes the old `timelineSnapshotHandler`

This makes timeline serving reusable without Router.

## Step 7 — Replace Router with small helpers

Delete:

* `router.go`, `router_options.go`, `router_mount_test.go`, debug route registration, etc.

Replace with small helper functions (optional):

* `UIHandler(staticFS fs.FS) http.Handler`
* `OpenTimelineStoreFromValues(...)`
* `OpenTurnStoreFromValues(...)`
* `NewStreamBackendFromValues(parsed *values.Values)`

No mux ownership.

## Step 8 — Update `cmd/web-chat` (hard cutover)

### 8.1 New wiring flow (conceptual)

Instead of:

```go
r, _ := webchat.NewRouter(...)
chatHandler := webchat.NewChatHandler(r.ConversationService(), resolver)
wsHandler := webchat.NewWSHandler(r.ConversationService(), resolver, upgrader)
mux.Handle("/api/", r.APIHandler())
mux.Handle("/", r.UIHandler())
```

You do:

```go
backend := webchat.NewStreamBackendFromValues(parsed)
timelineStore := webchat.OpenTimelineStoreFromValues(parsed)
turnStore := webchat.OpenTurnStoreFromValues(parsed)

streams := webchat.NewStreamHub(webchat.StreamHubConfig{
  BaseCtx: ctx,
  Backend: backend,
  TimelineStore: timelineStore,
  SemBufferSize: 1000,
  StopStreamAfter: time.Duration(idleTimeoutSeconds)*time.Second,
  EvictStateAfter: time.Duration(evictIdleSeconds)*time.Second,
  SweepEvery: time.Duration(evictIntervalSeconds)*time.Second,
})

chat := webchat.NewChatService(webchat.ChatServiceConfig{
  BaseCtx: ctx,
  Streams: streams,
  Runtime: runtimeProvider,
  StepController: stepCtrl,
  TurnStore: turnStore,
  ToolFactories: toolFactories,
  EventSinkWrapper: optionalWrapper,
})

timeline := webchat.NewTimelineService(timelineStore)

mux.HandleFunc("/chat", webchat.NewChatHandler(chat, resolver))
mux.HandleFunc("/ws", webchat.NewWSHandler(streams, resolver, upgrader))
mux.HandleFunc("/api/timeline", timeline.HTTPHandler())
mux.Handle("/", webchat.UIHandler(staticFS))

streams.StartMaintenance(ctx)
```

### 8.2 What disappears

* no `RunEventRouter` step (unless you explicitly need it elsewhere)
* no `webchat.NewFromRouter(...)`

## Step 9 — Update web-agent-example similarly

The only special thing web-agent-example needed was:

* middleware registration
* `EventSinkWrapper` (disco dialogue wrapping)

That remains supported in `ChatServiceConfig.EventSinkWrapper`.

WS + timeline become identical to cmd/web-chat.

## Step 10 — Rewrite tests (you’ll delete router tests)

### Keep + adapt

* `connection_pool_test.go` unchanged
* `stream_coordinator_test.go` add cases for SEM envelope payload
* `timeline_projector_test.go` add projection for `"chat.message"`
* `send_queue_test.go` and chat service tests update for new names

### Remove

* `router_*_test.go` (router is gone)

### Add

* `stream_hub_test.go`: attach WS, publish sem event, ensure broadcast and projection hook called
* `chat_service_test.go`: submit prompt emits `"chat.message"` SEM event (can test by inspecting sem buffer / mock subscriber)

---

# 5) What you keep from `webchat/router` (and what you delete)

## Keep (as standalone helpers)

From `router.go` you keep the *useful utilities*:

* Opening timeline/turn stores from `Values` (`timeline-dsn`, `timeline-db`, `turns-dsn`, `turns-db`)
* UI handler that serves embedded static FS
* (Optional) debug handlers, but they should be a separate `DebugService`/`DebugHandler` builder

## Delete

* Any “router owns mux and mounts everything”
* Any “router options to customize behavior”
* Any “router owns runtime policy hooks”

In short: **router no longer exists as an abstraction boundary**.
Only pure builders remain.

---

# 6) Why this decomposition supports your integration scenarios

## Scenario A: Existing app wants chat + ws, but owns auth and routing

They import:

* `ChatService`, `StreamHub`, `TimelineService`
  They mount:
* `POST /chat` (their handler → `chat.SubmitPrompt`)
* `GET /ws` (their handler → `streams.AttachWebSocket`)
* `GET /api/timeline` (their handler → `timeline.Snapshot`)

They never touch:

* `websocket` fanout logic
* stream coordinator details
* timeline projector semantics

## Scenario B: Background agent wants realtime progress streaming

They only need:

* `streams.PublishSEM(ctx, convID, SEMEvent{Type:"agent.progress", ...})`

If they want hydration too:

* register a timeline handler `RegisterTimelineHandler("agent.progress", ...)`

No websockets code. No router code.

## Scenario C: Chat UI + background agent monitoring share the same view

Both chat inference events and agent progress events go into the same conversation stream.
The same stream consumer:

* broadcasts to clients
* projects to timeline store

So you get a single, consistent mechanism.

---

# 7) Practical cutover checklist

Use this as your “did we actually cut over” checklist:

1. **No code imports `webchat.Router`** anymore
2. `cmd/web-chat` mounts:

   * `/chat` → ChatService
   * `/ws` → StreamHub
   * `/api/timeline` → TimelineService
   * `/` → UIHandler
3. Timeline store is updated for:

   * assistant messages (existing llm.*)
   * **user messages** (new chat.message handler)
4. `StreamCoordinator` tests cover:

   * geppetto event JSON payload
   * SEM envelope payload
5. “No WS connected” still results in:

   * chat works
   * timeline snapshots persist
   * stream stops eventually (StopStreamAfter) and state evicts (EvictStateAfter)

---

# 8) One strong recommendation for minimizing risk

During the cutover, don’t change:

* idempotency / queue logic
* SEM translator registry
* existing timeline projection of llm/tool events

The only “semantic” change you *need* is:

* **user message projection moves from ChatService → StreamHub+TimelineProjector via SEM event**

Everything else is primarily mechanical decomposition.

---
