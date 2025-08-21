Title: Conversation-Scoped Topics with Shared Consumer Group — Design
Slug: conversation-topics-shared-group-design
Date: 2025-08-21

## Purpose

We want each WebSocket conversation to have its own Redis Stream topic so we can isolate traffic, enable targeted trimming/deletion, and avoid replaying unrelated history. At the same time, we want to avoid creating many consumer groups. This document proposes a design that uses per-conversation topics but a shared consumer group name, combined with a single server-side subscriber per conversation to fan out to all WebSocket clients attached to that conversation.

## TL;DR

- Topic per conversation: `chat:<conv_id>`
- A single shared consumer group name for UI: `ui`
- One server-side consumer per conversation stream (not per WebSocket) reads from group `ui` and broadcasts to all WebSocket connections attached to that conversation
- On connect: no new consumer group is created; we only add the WS connection to the broadcaster’s list
- On end: we can trim/delete `chat:<conv_id>` and destroy the `ui` group for that stream

This minimizes consumer group proliferation while ensuring broadcast semantics to multiple sockets.

## Background and Constraints

- Redis Streams consumer groups are defined per stream. Reusing the same group name across many streams still results in one group per stream. That’s okay, but we avoid one group per WebSocket connection.
- Consumer group semantics are queue-like (compete consumers), not broadcast. If we had multiple WS connections reading directly from the same group, events would be load-balanced across those connections. We want broadcast → single reader + server-side fan-out.
- We observed replay issues when groups were created without a starting offset and streams already had history. We’ll create consumer groups at `$` (tail) to avoid first-time replay unless explicitly requested.

## Proposed Architecture

### Topics

- Conversation stream key: `chat:<conv_id>`
  - Example: `chat:conv-1755812270387-e3pkgm`

### Consumer Groups

- UI group name: `ui` (constant)
  - Per Redis design, there will still be one group per stream, but a single group name keeps ops/simple
  - Create with offset `$` for new conversations: starts reading at the tail
  - If replay is requested, use a replay flow (see Replay below) rather than auto-replaying history to every socket

### Consumers

- Exactly one consumer per conversation stream per server instance, e.g., consumer name: `ws-forwarder:<server_id>:<conv_id>`
  - This consumer is the only reader from Redis for that conversation
  - It broadcasts every timeline message to all attached WebSockets (fan-out in memory)
  - When the last WebSocket disconnects, keep the consumer alive for a grace period (to allow quick reconnects) or stop it to reduce resources

### WebSocket Flow

1) Client connects to `/ws?conv_id=<conv_id>`
2) Server finds/creates a conversation record
3) Server ensures consumer group `ui` exists at `$` for topic `chat:<conv_id>` (first time only)
4) Server creates/starts the single reader (if not running) for `chat:<conv_id>` and registers this WS connection in the conversation’s connection set
5) Reader receives messages → converts to timeline JSON → broadcasts to all sockets in that conversation

### Publishing Events

- When a chat run starts for `conv_id`, the server’s sink publishes to `chat:<conv_id>` instead of the previous shared `chat` topic
  - All event producers (engine, tool loop) remain unchanged; only the sink topic changes per run

## Replay

Not implemented for now. We create consumer groups at `$` (tail) to avoid historical replays.

## Lifecycle and Cleanup

- On conversation idle or completion:
  - Option 1: Keep the stream for history; do nothing
  - Option 2: Trim with `XTRIM` to the last M entries
  - Option 3: Destroy the `ui` consumer group for `chat:<conv_id>` and delete the stream key (if history not needed)

We can add a janitor to clean up streams older than N hours/days.

## Redis Commands Cheat Sheet

- Create group at tail (first time):
  - `XGROUP CREATE chat:<conv_id> ui $ MKSTREAM`
- Set group to tail (existing group):
  - `XGROUP SETID chat:<conv_id> ui $`
- Read with consumer group (our reader):
  - `XREADGROUP GROUP ui ws-forwarder:<server_id>:<conv_id> COUNT 50 BLOCK 5000 STREAMS chat:<conv_id> >`
- Acknowledge processed message:
  - `XACK chat:<conv_id> ui <id>`

## Changes Required

1) Topic selection
   - Replace the hardcoded `"chat"` topic used by sinks with `fmt.Sprintf("chat:%s", conv.ID)` when starting a run

2) Group creation at tail
   - Call `EnsureGroupAtTail(ctx, addr, fmt.Sprintf("chat:%s", conv.ID), "ui")` before subscribing

3) Single reader per conversation
   - For each conversation, maintain a goroutine that subscribes via Watermill to `chat:<conv_id>` (Subscriber bound to `ui` group) and fans out messages to all WebSockets attached to that conversation
   - Do NOT create WS-level consumers

4) Trimming (later)
   - Add a background janitor or manual endpoint to trim or delete `chat:<conv_id>` when a conversation is done

## Pseudocode Sketches

### Sink Topic

```go
runTopic := fmt.Sprintf("chat:%s", conv.ID)
sink := middleware.NewWatermillSink(router.Publisher, runTopic)
runCtx := events.WithEventSinks(runCtx, sink)
```

### Ensure Group at Tail

```go
_ = rediscfg.EnsureGroupAtTail(ctx, rs.Addr, fmt.Sprintf("chat:%s", conv.ID), "ui")
```

### Reader (One per Conversation)

```go
// Build a subscriber bound to the conversation topic and group ui
sub := rediscfg.BuildGroupSubscriber(rs.Addr, "ui", fmt.Sprintf("ws-forwarder:%s:%s", serverID, conv.ID))
// Subscribe to the topic for this conversation
ch, _ := sub.Subscribe(ctx, fmt.Sprintf("chat:%s", conv.ID))
go func(){
  for msg := range ch {
    e := events.NewEventFromJson(msg.Payload)
    // Convert to timeline JSON messages (forwarder)
    for _, b := range TimelineEventsFromEvent(e) {
      broadcastToWS(conv.ID, b) // fan-out to all sockets for this conv
    }
    msg.Ack()
  }
}()
```

### WebSocket Attach

```go
// No group creation here; just attach the socket to the existing conversation broadcaster
conv := getOrCreateConv(convID)
attachWebSocket(conv, ws)
```

## Observability

- Log when a reader starts/stops for `chat:<conv_id>`, number of attached sockets
- Metrics: per-topic read rate, broadcast rate, lag (stream ID vs last-delivered-id)
- Alarms: pending entries per group > 0 for prolonged time

## Migration Plan

1) Switch all sinks and readers to per-conversation topics `chat:<conv_id>` immediately
2) Remove all legacy usage of the shared `chat` topic
3) Add optional replay endpoint to preserve user experience
4) Implement trimming/janitor later

## Tradeoffs

- Per-conversation topics increase key count, but make lifecycle management easier (delete one key vs coordinating offsets across shared streams)
- One reader per conversation increases server goroutines slightly, but avoids multiplying consumer groups or competing consumers per WebSocket
- Broadcast semantics are implemented at the server (simple and predictable)

## Appendix: Key Decisions

- Topic: `chat:<conv_id>`
- Group: `ui` (shared name, one per topic)
- Consumer: `ws-forwarder:<server_id>:<conv_id>` (one per topic per server)
- Create groups at `$` to avoid historical replay unless explicitly requested


## Backend Forwarder + Engine/Turn/Conversation Lifetimes

### Entities

- Conversation: logical session identified by `conv_id`. Holds:
  - Topic: `chat:<conv_id>`
  - Single WS reader/broadcaster goroutine
  - Set of attached WebSockets
  - Engine instance (per conversation)
  - Current/last `run_id` and bookkeeping for turns
- Turn: a single user→assistant exchange (may span multiple engine steps). Turns live entirely inside a conversation.
- Engine: one instance bound to the conversation. It publishes events to `chat:<conv_id>` via the sink.
- Forwarder: stateless mapping that converts typed events into UI timeline messages. It does not track lifetimes beyond the incoming event metadata.

### Lifetimes and Flows

- WebSocket connect (first client for conv_id):
  - Create Conversation if absent
  - Ensure group `ui` at `$` for `chat:<conv_id>`
  - Start reader goroutine if not running
  - Attach WS to conversation
  - Optionally XRANGE last N for replay

- WebSocket reconnect (same conv_id):
  - Reuse existing Conversation and reader
  - Attach WS to conversation; no new consumer or group
  - Optionally XRANGE last N to backfill if desired

- WebSocket disconnect:
  - Detach WS from conversation
  - If zero sockets remain: either
    - Keep reader + engine alive for a grace period to allow quick reconnect (recommended), or
    - Stop reader and (optionally) engine to save resources

- Starting a run (POST /chat):
  - Lookup Conversation by `conv_id` (create if absent)
  - Use (or create) engine instance dedicated to this conversation
  - Create a new `run_id` in the Conversation context; seed initial Turn
  - Build sink to `chat:<conv_id>`; run engine/tool loop, publishing to the conversation topic
  - Forwarder maps published events to TL messages received by the reader and broadcast to WS clients

- Ending a run:
  - Mark Conversation not running; retain engine for next run (hot reuse) or shutdown if idle policy dictates

### Why engine-per-conversation

- Isolation: independent model settings per conversation without cross-talk
- Future features: “one model per conversation” becomes straightforward
- Simpler replay: a single topic per conversation simplifies slicing and replaying recent messages

### When to create vs reuse Conversation

- Create: first `/chat` or `/ws` for a new `conv_id`
- Reuse: subsequent `/chat` or reconnect `/ws` with the same `conv_id`
- Expiration: conversations may be GC’d after inactivity timeout; WS reconnect after GC will create anew

### Handling empty run_id events

- The reader forwards events with either matching `run_id` or empty `run_id` (some tool events may lack `run_id`); this ensures forwarder sees all relevant events for the conversation topic without dropping tool_exec/result messages

### Failure and Resilience

- If the reader crashes, it’s restarted and resumes from the group offset for `chat:<conv_id>`
- If the engine crashes mid-run, the server marks Conversation as not running and allows a new run
- Backpressure: since we broadcast to sockets, slow clients only affect their WS link; the reader continues and does not compete with other sockets

## Step-by-Step Implementation Plan

1) Wiring topics per conversation (server write path)
   - [x] Add helper `topicForConv(convID) -> "chat:<conv_id>"`.
   - [x] Replace sink creation in `/chat` handler to use `topicForConv(conv.ID)` instead of `"chat"`.
   - [x] Remove legacy `"chat"` usage entirely.

2) Reader per conversation (server read path)
   - [x] On `getOrCreateConv`, ensure Redis group at tail for `topicForConv(convID)` via `EnsureGroupAtTail`.
   - [x] Create one Watermill subscriber bound to group `ui` and consumer name `ws-forwarder:<server_id>:<conv_id>`.
   - [x] Subscribe to `topicForConv(convID)` and start a goroutine that converts events via forwarder and broadcasts to WS clients.
   - [x] Reuse the running reader on additional WS connects (no new subscriber).

3) Conversation manager enhancements
   - [ ] Track `attachedSockets` and implement `attach/detach` helpers.
   - [ ] Add idle timer: when `attachedSockets == 0`, start a grace timeout (configurable); on expiry, stop the reader and optionally engine.
   - [ ] Expose metrics: current conversations, readers running, sockets attached.

4) WebSocket API updates
   - [ ] Document reconnect semantics and recommend clients preserve `conv_id` in local storage.

5) Forwarder compatibility
   - [ ] No changes needed to mapping; verify events still include metadata for UI.
   - [ ] Keep zero-UUID fallbacks and custom `<tool>_result` emission.

6) Observability and logs
   - [ ] Add logs when reader starts/stops for a conversation.
   - [ ] Log group creation at `$` and when skipping history.
   - [ ] Add counters: read rate per `chat:<conv_id>`, broadcast rate, and lag.

7) Migration and rollout
   - [x] Switch all codepaths to `chat:<conv_id>` and delete legacy `chat` usages.

8) Cleanup/janitor (later)
   - [ ] Background job to destroy `ui` group and delete `chat:<conv_id>` for conversations idle beyond TTL, or run `XTRIM` to a size/time threshold.

9) Testing plan
   - [ ] Unit: topic selection helper, EnsureGroupAtTail no-op when BUSYGROUP.
   - [ ] Integration (with Redis):
     - [ ] On first connect, assert no replay when group created at `$`.
     - [ ] Multiple WS on same conv receive identical broadcast frames.
     - [ ] Idle timeout stops reader; reconnect restarts reader at tail.
   - [ ] Manual: use `redis-cli XINFO GROUPS chat:<conv_id>` to confirm `last-delivered-id` movement.

10) Risks and mitigations
   - **Too many streams**: implement janitor and TTL-based trimming.
   - **Reader leaks**: enforce idle timeout and ensure stop on server shutdown.
   - **Replay gaps**: prefer XRANGE-based replay buffer; consider in-memory ring buffer for hot sessions.

