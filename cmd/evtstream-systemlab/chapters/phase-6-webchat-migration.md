# Phase 6 — The Migration Console

By Phase 6, the interesting question is no longer whether `evtstream` can host a toy chat app. We already proved that in earlier labs. The question now is whether the **real** `cmd/web-chat` application can stand on the new substrate without quietly falling back to `pkg/webchat` assumptions.

That is why this page is a **migration console** rather than another synthetic widget demo. It reaches a live `cmd/web-chat` server through its public canonical routes, runs a real session, and reports what happened. If the migration is healthy, the console should show three things at once:

1. the canonical routes respond,
2. the legacy routes are gone,
3. the final snapshot contains both the **user** message and the **assistant** message.

## What this page is checking

The console intentionally stays outside the internals. It talks to the app the same way a frontend or another tool would:

```text
GET  /api/chat/profiles
POST /api/chat/sessions
POST /api/chat/sessions/:sessionId/messages
GET  /api/chat/sessions/:sessionId
```

It also probes the old routes that should be gone after the cutover:

```text
POST /chat                  -> should be 404
GET  /api/timeline?conv_id  -> should be 404
```

That public-boundary perspective matters. If this page had to reach into `pkg/webchat` internals or call private Go helpers, it would not really be validating the migration. It would be cheating.

## How to use it

1. Start a real `cmd/web-chat` server.
2. Point the **Web Chat Base URL** field at that server.
3. Pick a real profile such as `gpt-5-nano-low`.
4. Run the live probe.
5. Read the checks, the HTTP trace, and the final snapshot together.

## What to pay attention to

The most important check is not only “did the assistant answer?” It is whether the snapshot contains a **user** entity and an **assistant** entity at the same time. That tells you the migrated canonical model is no longer assistant-only.

The second important check is whether the assistant output is **not** the old demo echo form `Answer: <prompt>`. That is the simplest signal that the runtime-backed inference path is being exercised instead of the placeholder engine.

## Things to try

- Point the page at a broken or stale server and see which route checks fail first.
- Change the profile to one that is intentionally misconfigured and observe how the final snapshot stops or errors.
- Reload the Systemlab page after a successful run and confirm the last probe result is still visible through the local Phase 6 state endpoint.
