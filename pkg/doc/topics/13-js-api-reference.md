---
Title: Pinocchio JavaScript API Reference
Slug: pinocchio-js-api-reference
Short: Contract reference for JavaScript SEM handlers and reducers loaded by web-chat.
Topics:
- pinocchio
- webchat
- javascript
- sem
- timeline
- api-reference
Commands:
- web-chat
Flags:
- timeline-js-script
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

This page defines the JavaScript contract used by the web-chat timeline runtime.
Use it when writing scripts loaded through `--timeline-js-script`.

## Runtime Entry Points

The runtime registers a native module with two names:

| Module | Status | Notes |
|---|---|---|
| `require("pinocchio")` | canonical | Preferred import in new scripts |
| `require("pnocchio")` | alias | Supported alias for unified naming |

Module exports:

| Export | Type | Notes |
|---|---|---|
| `timeline` | namespace | Primary API namespace |
| `timeline.registerSemReducer(eventType, fn)` | function | Registers reducer callback(s) for a SEM event type |
| `timeline.onSem(eventType, fn)` | function | Registers observer callback(s) for a SEM event type |
| `registerSemReducer` | function | Top-level shortcut to `timeline.registerSemReducer` |
| `onSem` | function | Top-level shortcut to `timeline.onSem` |

Canonical usage:

```javascript
const p = require("pinocchio");
p.timeline.onSem("*", (ev, ctx) => {
  // observe SEM stream
});
```

## Callback Signatures

Both callbacks receive `(event, ctx)` arguments.

| Callback | Signature | Purpose |
|---|---|---|
| Handler | `fn(event, ctx)` | Side-effect observer; does not directly write timeline entities |
| Reducer | `fn(event, ctx)` | Returns projection instructions and optional consume decision |

### `event` payload

| Field | Type | Description |
|---|---|---|
| `type` | string | SEM event type, for example `llm.delta` |
| `id` | string | Event id used for projection grouping |
| `seq` | number | Monotonic stream sequence |
| `stream_id` | string | Stream identifier |
| `data` | object \| undefined | Decoded SEM event data payload |
| `now_ms` | number | Dispatch timestamp from runtime |

### `ctx` payload

| Field | Type | Description |
|---|---|---|
| `now_ms` | number | Dispatch timestamp from runtime |

## Registration Semantics

| API | Rule |
|---|---|
| `registerSemReducer(eventType, fn)` | `eventType` must be non-empty; `fn` must be a function |
| `onSem(eventType, fn)` | `eventType` empty string is normalized to `"*"` |
| `eventType = "*"` | Wildcard subscription for all SEM events |
| Multiple registrations | Allowed; callbacks run in registration order |

Invalid registration throws a JS `TypeError` during script load.

## Reducer Return Contract

Reducers may return one of the following:

| Return value | Effect |
|---|---|
| `undefined` / `null` | No upsert, no consume |
| `true` / `false` | Consume decision only |
| `entity` object | One timeline upsert |
| `entity[]` | Multiple timeline upserts |
| `{ consume, upserts }` | Explicit consume + one or many upserts |

Entity shape:

| Field | Type | Default |
|---|---|---|
| `id` | string | `event.id` |
| `kind` | string | `js.timeline.entity` |
| `props` | object | `{}` |
| `meta` | object | `{}` (values stringified) |
| `created_at_ms` / `createdAtMs` | number | `now_ms` |
| `updated_at_ms` / `updatedAtMs` | number | `now_ms` |

Notes:

- If `id` is empty after fallback to `event.id`, the entity is skipped.
- Invalid `props` object conversion is downgraded to `{}` and logged as warning.
- `meta` values are converted to strings.

## Projection Control (`consume`)

`consume` controls whether built-in projection runs after JS callbacks.

| Case | Result |
|---|---|
| `consume: false` (or unset) | JS upserts apply, then built-in projector still runs |
| `consume: true` | JS upserts apply, built-in projector is skipped for that event |

Use `consume: false` for additive projections.
Use `consume: true` only when intentionally replacing built-in behavior.

## Runtime Execution Order

For each SEM frame:

1. Matching handlers for exact type and wildcard (`*`) run first.
2. Matching reducers for exact type and wildcard (`*`) run next.
3. Reducer outputs are converted to timeline upserts.
4. If any reducer sets `consume=true`, built-in projection is skipped.

Errors:

- Handler/reducer runtime exceptions are logged and processing continues.
- Upsert failures are logged and processing continues.
- Script load failures are startup errors.

## Loading Scripts with `web-chat`

Scripts are loaded at startup using `--timeline-js-script`.

```bash
go run ./cmd/web-chat web-chat \
  --addr :8080 \
  --profile-registries ./profiles.yaml \
  --timeline-js-script ./scripts/timeline-projections.js
```

You may repeat the flag or pass comma-separated values.

Runtime resolves module paths by adding each script directory and `<script-dir>/node_modules` to `require` global folders.

## Built-in Event Types Commonly Extended in JS

These are frequent reducer targets:

| Event type | Built-in behavior (when not consumed) |
|---|---|
| `llm.start` / `llm.thinking.start` | Creates message entity in streaming state |
| `llm.delta` / `llm.thinking.delta` | Updates message content incrementally |
| `llm.final` / `llm.thinking.final` | Marks message streaming false |
| `tool.start` | Creates tool call entity |
| `tool.done` | Marks tool call completed |
| `tool.result` | Creates tool result entity |
| `chat.message` | Projects message snapshot via built-in handler |

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `module pinocchio not found` | runtime not booted via web-chat timeline runtime | load script through `--timeline-js-script` in `web-chat` |
| `registerSemReducer: eventType must be non-empty` | empty event type passed | pass concrete type or `*` |
| reducer runs but builtin message content disappears | reducer returned `consume: true` | set `consume: false` for additive projection |
| script fails at startup | syntax/import error in script | validate file path and script syntax first |
| wildcard observer not firing | registered on wrong module/global | use `const p = require("pinocchio"); p.timeline.onSem("*", fn)` |

## See Also

- [Pinocchio JavaScript API User Guide](14-js-api-user-guide.md) — practical workflow and patterns.
- [Webchat User Guide](webchat-user-guide.md) — backend route and integration contract.
- [Webchat SEM and UI](webchat-sem-and-ui.md) — frontend event routing and projection behavior.
- [Webchat Debugging and Ops](webchat-debugging-and-ops.md) — runtime and operational troubleshooting.
