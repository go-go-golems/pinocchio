---
Title: Redis Streaming Integration Notes (Status, Findings, and Next Steps)
Slug: redis-streaming-integration-notes
Short: What we implemented, how it’s wired, the symptoms we’re seeing with Redis Streams, and concrete steps to fix consumer-group competition for start/partial events.
Topics:
- pinocchio
- geppetto
- watermill
- redis
- consumer-groups
- events
- timeline-ui
Date: 2025-08-20
SectionType: StatusReport
---

## 1) Summary

We integrated Redis Streams as a transport for Geppetto inference events and wired the Pinocchio/Bobatea UI to consume from Redis instead of the in-memory bus. Streaming works and we can see published `start`/`partial`/`final` events via Redis (`XRANGE`/`MONITOR`); however, the timeline UI sometimes misses the initial entity creation (triggered by the `start` event), resulting in partials being applied without the entity “created” first. This is consistent with multiple handlers competing within the same Redis Streams consumer group, causing the `start` event to be received by a non-UI handler while the UI only sees later events.

## 2) What we built

- Added a reusable Redis Streams transport package:
  - `pinocchio/pkg/redisstream/redis_layer.go`: defines `redis` layer and `Settings`:
    - `redis-enabled`, `redis-addr`, `redis-group`, `redis-consumer`.
  - `pinocchio/pkg/redisstream/router.go`: `BuildRouter(Settings, verbose)` returns a Redis-backed `events.EventRouter` when enabled; defaults to in-memory otherwise.
- Updated agents/examples to support Redis:
  - `pinocchio/cmd/agents/simple-chat-agent/main.go`: adds the `redis` layer and uses `BuildRouter`.
  - `pinocchio/cmd/examples/simple-chat/main.go`: builds a router (from `redis` layer if present) and passes it into `RunWithOptions`.
- Ensured `geppetto/pkg/events/event-router.go` only defaults to in-memory when no external publisher/subscriber are provided.
- Added `pinocchio/scripts/log-filter.sh` to focus on critical streaming lines.

## 3) Current wiring (relevant files and symbols)

- Router and sink:
  - `geppetto/pkg/events/event-router.go`: `EventRouter`, `AddHandler`, `Run`
  - `geppetto/pkg/inference/middleware/sink_watermill.go`: `WatermillSink` → publishes JSON payload to topic (stream).
- Redis Streams transport:
  - `pinocchio/pkg/redisstream/router.go`: builds Watermill `Publisher`/`Subscriber` using `watermill-redisstream` and `go-redis/v9`.
  - All handlers added via `router.AddHandler(name, "chat", fn)` subscribe using the same `Subscriber` instance and therefore the same `ConsumerGroup`/`Consumer` configuration.
- Timeline/UI forwarders:
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go`: `MakeUIForwarder(p)` maps events to timeline `UIEntity*` messages (created/updated/completed).
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/xevents/events.go`: optional channel forwarder (disabled in Redis mode if needed).

## 4) Observed symptoms

- Redis `MONITOR`/`XRANGE` confirms the following sequence for a turn:
  1) `log` (agentmode prompt inserted)
  2) `start` (with metadata including message_id)
  3) one or more `partial`
- In the UI logs for Redis runs, we sometimes see `partial`/`final` applied, but the initial `start` → `UIEntityCreated` is missing. The same sequence works reliably without Redis (in-memory gochannel).

## 5) Root cause analysis

Watermill handler registration in our process adds multiple handlers to the same `EventRouter` (and therefore the same Redis `Subscriber`). With Redis Streams consumer groups, each call to `Subscribe(topic)` will create a consumer within the same group (depending on subscriber implementation), and the stream entries will be load-balanced across consumers. As a result, different handlers (UI forwarder vs logs/persistence) can each receive only a subset of events. When the `start` event goes to the non-UI handler, the UI forwarder receives only subsequent `partial` events, which cannot create the initial timeline entity.

This matches the symptom: the UI misses the entity creation step when `start` was consumed by another handler.

## 6) Verification steps

- Inspect Redis groups and consumers:
  - `redis-cli XINFO STREAM chat`
  - `redis-cli XINFO GROUPS chat`
- Run the agent with Redis enabled and a single group dedicated to the UI, and disable auxiliary handlers inside the same process:
  - `--redis-enabled=true --redis-group ui --redis-consumer ui-1`
  - In our current code, auxiliary handlers (logger/persist) can be conditionally disabled when Redis is enabled to avoid competing consumers.
- Observe logs using the filter:
  - `./pinocchio/scripts/log-filter.sh pinocchio/agent.log`
  - Look for `OpenAI publishing start event` → `agent forwarder: dispatch … type=start` → `Applying external entity event … lifecycle=created`.

## 7) Proposed fixes

Option A (Recommended): Single consumer group per concern
- Use one `EventRouter`/`Subscriber` with a unique consumer group for the UI only (e.g. `ui`).
- Move logging/persistence handlers to a separate process (or a separate `EventRouter` instance) configured with a different consumer group (e.g. `logs`).
- Result: Each group receives all messages independently; the UI reliably receives `start`.

Option B: Single handler fanout inside process
- Register only one Watermill handler (the UI forwarder) in the process; fan out internally to logging/persistence if desired.
- Result: No competition at the consumer-group level.

Option C: Distinct subscribers within the same process
- Create separate `Subscriber` instances for each handler, each configured with a different consumer group.
- Result: Similar to A but still single process; more plumbing.

Option D: Consumer name per handler (not ideal)
- If the library treats consumer name uniquely per `Subscribe`, set different consumer names per handler; this still competes on the same group and will load-balance rather than duplicate delivery, so it does not solve the core problem.

## 8) Short-term workaround applied

- We added a utility script to filter logs to the critical path: `pinocchio/scripts/log-filter.sh`.
- We also made it possible to disable auxiliary handlers when Redis is enabled (to reduce the chance of event starvation for the UI). The durable fix is still to separate groups or processes.

## 9) Next steps

- Implement Option A or B explicitly:
  1) For the UI process: ensure only the UI forwarder is registered on the `chat` topic and group `ui`.
  2) For logging/persistence: create a separate runner using group `logs` (either another process or a separate `EventRouter` instance in the same binary).
- Add a quick guardrail: when `redis-enabled`, print a warning if more than one handler is registered against the same `Subscriber`/topic.
- Provide flags to configure both groups (`--redis-ui-group`, `--redis-logs-group`) and wire them to separate routers when co-located.
- Extend the log filter with a selector to show only the UI sequence or only logger/persist sequences.

## 10) Useful commands

- See current groups/consumers:
```
redis-cli XINFO STREAM chat
redis-cli XINFO GROUPS chat
```

- Live-tail entries (like `tail -f`):
```
redis-cli --raw XREAD BLOCK 0 STREAMS chat $
```

- Filter logs:
```
./pinocchio/scripts/log-filter.sh pinocchio/agent.log
```

## 11) Open questions

- Should we keep non-UI handlers in the same process but force separate groups by spinning a second `EventRouter` with its own `Subscriber`? This adds complexity but keeps a single binary.
- Do we need additional idempotency in the UI forwarder to “create if not exists” on `partial` (as a safety net) when the `start` event is missing? It won’t fix the underlying delivery semantics but could improve resilience.


