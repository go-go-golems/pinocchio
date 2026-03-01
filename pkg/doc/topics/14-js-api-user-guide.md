---
Title: Pinocchio JavaScript API User Guide
Slug: pinocchio-js-api-user-guide
Short: Practical guide for building timeline projections with Pinocchio JavaScript SEM scripts.
Topics:
- pinocchio
- webchat
- javascript
- sem
- timeline
- user-guide
Commands:
- web-chat
Flags:
- timeline-js-script
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Application
---

This guide shows how to build and iterate on Pinocchio JavaScript timeline scripts.
If you need exact API signatures and return contracts, use [Pinocchio JavaScript API Reference](13-js-api-reference.md).

## Workflow Overview

Use this loop when developing scripts:

1. Start with one script file and one reducer.
2. Run `web-chat` with `--timeline-js-script`.
3. Trigger one known event flow (for example a short `llm.delta` response).
4. Verify timeline output via UI and `/api/timeline`.
5. Add handlers/reducers incrementally.

Recommended layout:

```text
your-app/
  scripts/
    timeline/
      01-observe.js
      02-llm-delta-side-projection.js
      03-custom-replacement.js
```

## Step 1: Start with an Observer

Begin with a read-only wildcard observer so you can inspect event shapes safely.

```javascript
const p = require("pinocchio");

p.timeline.onSem("*", function (ev, ctx) {
  // Keep logs bounded in production; this is for initial inspection.
  console.log("[sem]", ev.type, ev.id, ev.seq, ctx.now_ms);
});
```

Run:

```bash
go run ./cmd/web-chat web-chat \
  --addr :8080 \
  --profile-registries ./profiles.yaml \
  --timeline-js-script ./scripts/timeline/01-observe.js
```

Why this first: you validate event names and payload shape before changing projection behavior.

## Step 2: Add a Non-Consuming Reducer

Create an additive projection for `llm.delta` so built-in message projection still works.

```javascript
const p = require("pinocchio");

p.timeline.registerSemReducer("llm.delta", function (ev) {
  return {
    consume: false,
    upserts: [{
      id: ev.id + "-delta-side",
      kind: "llm.delta.side",
      props: {
        cumulative: ev.data && ev.data.cumulative
      }
    }]
  };
});
```

Expected result:

- built-in `message` entity continues updating.
- custom `llm.delta.side` entity appears beside it.

## Step 3: Replace Built-in Behavior Deliberately

Only use this when you intentionally own the projection for a given event type.

```javascript
const p = require("pinocchio");

p.timeline.registerSemReducer("llm.delta", function (ev) {
  return {
    consume: true,
    upserts: [{
      id: ev.id + "-custom",
      kind: "llm.delta.custom",
      props: { cumulative: ev.data && ev.data.cumulative }
    }]
  };
});
```

With `consume: true`, the built-in `llm.delta` projector does not run for that frame.

## Step 4: Compose Multiple Scripts

You can pass multiple script files to `web-chat`:

```bash
go run ./cmd/web-chat web-chat \
  --addr :8080 \
  --profile-registries ./profiles.yaml \
  --timeline-js-script ./scripts/timeline/01-observe.js \
  --timeline-js-script ./scripts/timeline/02-llm-delta-side-projection.js
```

Or comma-separated:

```bash
go run ./cmd/web-chat web-chat \
  --profile-registries ./profiles.yaml \
  --timeline-js-script "./scripts/timeline/01-observe.js,./scripts/timeline/02-llm-delta-side-projection.js"
```

## Step 5: Use the Alias Module Name (Optional)

If your host stack standardizes on the alias, this is supported:

```javascript
const p = require("pnocchio");
p.timeline.onSem("*", function (ev) {
  console.log(ev.type);
});
```

Prefer one import name per codebase to avoid unnecessary variation.

## Validation Checklist

Before shipping a script:

1. Script loads at startup with no syntax/import errors.
2. Reducers that should be additive return `consume: false`.
3. Each emitted entity has stable `id` and meaningful `kind`.
4. `props` payload is JSON-friendly.
5. Wildcard handlers do not produce excessive logs in production.

## Migration Note (Global API to Module API)

Use module APIs, not global functions.

Old pattern:

```javascript
registerSemReducer("llm.delta", fn);
onSem("*", fn);
```

Current pattern:

```javascript
const p = require("pinocchio");
p.timeline.registerSemReducer("llm.delta", fn);
p.timeline.onSem("*", fn);
```

## Testing Patterns

For fast regression checks, run targeted tests:

```bash
go test ./pkg/webchat -count=1
go test ./cmd/web-chat -count=1 -run 'TestConfigureTimelineJSScripts|TestLLMDeltaProjectionHarness'
```

These tests validate real runtime loading and `llm.delta` projection semantics.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| custom entity never appears | reducer did not match event type | verify exact `event.type` from observer logs |
| message stream stopped updating | reducer consumed event unexpectedly | return `consume: false` unless replacing built-in behavior |
| startup fails after adding script | invalid script path or syntax | verify path and run script linter/syntax check |
| `require("pinocchio")` fails in script | script not loaded by timeline runtime | ensure script is passed via `--timeline-js-script` |
| wildcard handler is noisy | logging every event in production | narrow to specific event types or sample logs |

## See Also

- [Pinocchio JavaScript API Reference](13-js-api-reference.md) — complete contract and return type details.
- [Webchat Framework Guide](webchat-framework-guide.md) — end-to-end backend and frontend integration.
- [Webchat SEM and UI](webchat-sem-and-ui.md) — SEM event routing and timeline rendering model.
- [Webchat Adding Event Types](webchat-adding-event-types.md) — extending event families and projection behavior.
