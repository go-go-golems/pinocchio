---
Title: Run JavaScript Runner Scripts
Slug: js-runner-scripts
Short: Use `pinocchio js` to run JavaScript scripts with Pinocchio config defaults and Geppetto's session-centered JS API.
Topics:
- javascript
- pinocchio
- geppetto
- profiles
- runner
Commands:
- pinocchio js
Flags:
- script
- profile-registries
- print-result
- list-go-tools
- turns-dsn
- turns-db
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

## Overview

`pinocchio js` runs JavaScript against Geppetto's wrapper-first JS API while keeping Pinocchio's config/profile bootstrap behavior.

The command creates a JS runtime that exposes:

- `require("geppetto")`
- `require("pinocchio")`
- `console.log`
- `console.error`
- `ENV`
- `sleep(ms)`
- `assert(cond, msg)`

Use Geppetto's current public execution model:

```javascript
const gp = require("geppetto");
const settings = gp.inferenceProfiles.resolve();
const agent = gp.agent().inference(settings).build();
const session = agent.session().id("chat-123").build();
const result = session.next().user("Say hello.").run();
console.log(result.text());
```

## Pinocchio bootstrap behavior

Pinocchio supplies the runtime defaults:

- profile registries from `--profile-registries`, config, env, or the default Pinocchio profile registry
- profile selection from `--profile`, config, env, or registry defaults
- hidden base inference settings for Pinocchio-owned helpers
- optional durable turn storage from `--turns-dsn` / `--turns-db`

## Durable turn storage

Pass `--turns-dsn` or `--turns-db` to install a Pinocchio SQLite turn store into `require("geppetto")`:

```bash
pinocchio js \
  --script session-script.js \
  --profile-registries "$HOME/.config/pinocchio/profiles.yaml" \
  --turns-db /tmp/pinocchio-js-turns.db
```

Inside JavaScript, the store is available as `gp.turnStores.default()` and as the default session persister:

```javascript
const gp = require("geppetto");
const store = gp.turnStores.default();
const settings = gp.inferenceProfiles.resolve();
const agent = gp.agent().inference(settings).build();

const session = agent.session()
  .id("durable-chat")
  .defaultStore()
  .resumeLatest()
  .build();

const result = session.next()
  .user("Continue this durable conversation.")
  .run();

const latest = store.loadLatest({ sessionId: "durable-chat", phase: "final" });
console.log(latest.turnId, result.text());
```

`resumeLatest()` is non-strict by default. Use `resumeLatest({ required: true })` if missing history should be an error.

## Example scripts

The repo includes:

- `examples/js/runner-profile-demo.js` — real profile-driven inference through `session.next().run()`.
- `examples/js/runner-profile-smoke.js` — deterministic profile/session bootstrap smoke without a provider call.
- `examples/js/profiles/basic.yaml` — small local engine-profile registry used by the examples.

Run the real inference example:

```bash
pinocchio js \
  --script examples/js/runner-profile-demo.js \
  --profile-registries examples/js/profiles/basic.yaml
```

Pick an explicit profile:

```bash
pinocchio js \
  examples/js/runner-profile-demo.js \
  --profile assistant \
  --profile-registries examples/js/profiles/basic.yaml
```

Run the deterministic smoke script:

```bash
pinocchio js \
  --script examples/js/runner-profile-smoke.js \
  --profile-registries examples/js/profiles/basic.yaml
```

## Flags

### `--script`

Path to the JavaScript file. You may also pass the script as the positional argument.

### `--profile-registries`

Engine profile registry source list. The Geppetto JS module sees this through `gp.inferenceProfiles.resolve()`.

### `--profile`

Selects the default profile used by `gp.inferenceProfiles.resolve()`.

### `--turns-dsn`

SQLite DSN for durable JS turn snapshots. Preferred over `--turns-db`.

### `--turns-db`

SQLite database file path for durable JS turn snapshots. Pinocchio derives the DSN and creates the parent directory when needed.

### `--print-result`

Prints the top-level JavaScript return value as JSON.

### `--list-go-tools`

Lists built-in Go tools exposed to JavaScript and exits.

## Removed legacy Geppetto APIs

Older scripts may use removed names such as `gp.profiles`, `gp.engines`, `gp.runner`, `gp.turns`, `gp.turn(...)`, or `agent.run(turn)`. Update those scripts to `gp.inferenceProfiles`, `gp.engine()`, `gp.agent()`, and `agent.session().next().run()`.
