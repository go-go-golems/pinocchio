---
Title: Run JavaScript Runner Scripts
Slug: js-runner-scripts
Short: Use `pinocchio js` to run JavaScript scripts with Pinocchio config defaults and Geppetto's JS runner API.
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
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

## Overview

This page explains how to use `pinocchio js` to run JavaScript scripts against the Geppetto JS API while keeping Pinocchio's own config and engine-profile behavior.

This matters because there are two separate concerns:

- Geppetto provides the generic JavaScript inference API and runner API.
- Pinocchio provides the application bootstrap: config files, env defaults, and engine-profile registry loading.

`pinocchio js` is the glue between those two layers.

## What The Command Provides

When you run:

```bash
pinocchio js --script script.js
```

the command creates a JS runtime that exposes:

- `require("geppetto")`
- `require("pinocchio")`
- `console.log`
- `console.error`
- `ENV`
- `sleep(ms)`
- `assert(cond, msg)`

The two modules have different jobs.

### `require("geppetto")`

Use this for generic runtime work:

- `gp.engines.*`
- `gp.profiles.*`
- `gp.runner.*`
- `gp.turns.*`
- `gp.tools.*`
- `gp.events.*`

### `require("pinocchio")`

Use this for Pinocchio-owned helpers.

Right now the main helper is:

```javascript
pinocchio.engines.fromDefaults(options?)
```

This builds an engine starting from Pinocchio's hidden base `InferenceSettings`, which come from:

- config files
- `PINOCCHIO_*` environment variables
- built-in defaults

That helper is intentionally base-config-only. It does not resolve engine profiles. For profile-driven engine selection, use `gp.profiles.resolve({})` and `gp.engines.fromResolvedProfile(...)`.

## Basic Workflow

The most common workflow looks like this:

1. Resolve an engine profile from the configured registry stack.
2. Build an engine from that resolved profile.
3. Run inference through `gp.runner`.

In pseudocode:

```text
pinocchio js
  -> resolve base InferenceSettings from Pinocchio config
  -> resolve engine profile registry stack
  -> load JS runtime
  -> script calls gp.profiles.resolve()
  -> script calls gp.engines.fromResolvedProfile()
  -> script calls gp.runner.run() or gp.runner.start()
```

## Example

This example uses:

- `gp.profiles.resolve(...)`
- `gp.engines.fromResolvedProfile(...)`
- `gp.runner.run(...)`

```javascript
const gp = require("geppetto");
const resolved = gp.profiles.resolve({});
console.log(JSON.stringify({
  profileSlug: resolved.profileSlug,
  model: resolved.inferenceSettings?.chat?.engine,
}, null, 2));

const engine = gp.engines.fromResolvedProfile(resolved);

const out = gp.runner.run({
  engine,
  prompt: "Say hello in one line.",
});

console.log(out.blocks[0].payload.text);
```

## Running The Example Scripts

The repo includes two example scripts:

- `examples/js/runner-profile-demo.js`
- `examples/js/runner-profile-smoke.js`
- `examples/js/profiles/basic.yaml`

Run the real inference example from the repo root:

```bash
pinocchio js \
  --script examples/js/runner-profile-demo.js \
  --profile-registries examples/js/profiles/basic.yaml
```

Pick an explicit profile from that registry:

```bash
pinocchio js \
  examples/js/runner-profile-demo.js \
  --profile assistant \
  --profile-registries examples/js/profiles/basic.yaml
```

This is the example to use when you want an actual LLM response.

Use the smoke script when you want deterministic local output without calling a live model:

```bash
pinocchio js \
  --script examples/js/runner-profile-smoke.js \
  --profile-registries examples/js/profiles/basic.yaml
```

The smoke script is a good first bootstrap test because it proves all of the following in one run:

- the command can execute a script
- the engine profile registry is loaded
- the selected engine profile changes the resolved model
- `gp.runner.run(...)` works

## Flags

### `--script`

Use `--script` to pass the JS file path explicitly.

```bash
pinocchio js --script examples/js/runner-profile-demo.js
```

You can also use a positional script path:

```bash
pinocchio js examples/js/runner-profile-demo.js
```

### `--profile-registries`

Use this when your script resolves engine profiles from registries.

```bash
pinocchio js \
  --script examples/js/runner-profile-demo.js \
  --profile-registries examples/js/profiles/basic.yaml
```

If you do not pass the flag, Pinocchio still follows its normal discovery rules:

- `PINOCCHIO_PROFILE_REGISTRIES`
- `profile.registries` from the merged unified config document (`--config-file` participates in that merge)
- `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` when present

### `--config-file`

Use this when the profile registry stack should come from the same Pinocchio config file that other commands use.

```bash
pinocchio js \
  examples/js/runner-profile-demo.js \
  --config-file ~/.config/pinocchio/config.yaml
```

The command reads `profile.registries` and `profile.active` from that unified config document before applying explicit CLI overrides.

If the default or configured `profiles.yaml` still uses the old mixed-runtime format, rewrite it first to the engine-only `inference_settings` format. Use [examples/js/profiles/basic.yaml](../../../examples/js/profiles/basic.yaml) as the reference shape.

### `--profile`

Use this when the script should follow the same selected-profile behavior as the rest of Pinocchio.

```bash
pinocchio js \
  examples/js/runner-profile-demo.js \
  --profile assistant \
  --profile-registries examples/js/profiles/basic.yaml
```

If you do not pass `--profile`, the script can still resolve the registry stack default engine profile.

### `--print-result`

Use this when you want the top-level JS return value printed as JSON.

```bash
pinocchio js --script script.js --print-result
```

### `--list-go-tools`

Use this to see which Go tools are exposed to the script runtime.

```bash
pinocchio js --list-go-tools
```

## Practical Notes

### Engine configuration

`pinocchio.engines.fromDefaults()` is the app-owned helper. It starts from Pinocchio defaults, not from a blank Geppetto config object.

It intentionally stays base-config-only. It does not consult the engine-profile registry stack.

`pinocchio.engines.inspectDefaults()` exposes the same bootstrap path without constructing a live engine, which is useful when debugging:

- selected `apiType`
- selected `model`
- resolved `baseURL`
- whether an API key is configured
- timeout in milliseconds

This is the recommended path when:

- you want the same provider credentials and timeout defaults the rest of Pinocchio uses
- you want script setup to stay small

You can call it in two styles:

- `pinocchio.engines.fromDefaults({})`
  when your base Pinocchio config already defines provider/model defaults
- `pinocchio.engines.fromDefaults({ model: "...", apiType: "..." })`
  when you want the script to force a concrete live engine regardless of the base config

### Engine profile resolution

`gp.profiles.resolve({})` is the profile-aware path for `pinocchio js`.

For `pinocchio js`, the command provides both the registry stack and the active/default profile context. That means a script can usually just do:

```javascript
const resolved = gp.profiles.resolve({});
const engine = gp.engines.fromResolvedProfile(resolved);
```

and let the command apply:

- `--profile` when provided
- config-driven profile selection
- registry-default engine profile selection when no explicit profile is set

That separation is intentional:

- Pinocchio owns base config and registry discovery.
- Geppetto owns engine-profile resolution and runner execution.

## Troubleshooting

| Problem | Cause | Solution |
| --- | --- | --- |
| `--script is required` | No script path was given | Pass `--script path.js` or a positional file path |
| `profile registry ...` validation errors | The YAML engine-profile registry format is wrong | Use `profiles:` with `inference_settings`, not legacy mixed `runtime:` fields |
| `unknown provider <nil>` from `pinocchio.engines.fromDefaults()` | Base config does not specify a provider and the script did not override it | Pass both `model` and `apiType` explicitly, or use `gp.engines.fromResolvedProfile(...)` |
| `Cannot find module` | The script or its local `node_modules` directory is not reachable | Run with the correct script path so the command can add the script directory to require search paths |

## See Also

- `pinocchio help config-migration-guide`
- `pinocchio help profiles`
- `pinocchio help webchat-profile-registry`
- `pinocchio help webchat-runner-migration-guide`
