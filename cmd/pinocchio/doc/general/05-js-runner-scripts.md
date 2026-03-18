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

This page explains how to use `pinocchio js` to run JavaScript scripts against the Geppetto JS API while keeping Pinocchio's own config and profile-registry behavior.

This matters because there are two separate concerns:

- Geppetto provides the generic JavaScript inference API and runner API.
- Pinocchio provides the application bootstrap: config files, env defaults, and profile-registry loading.

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

This builds an engine starting from Pinocchio's hidden base `StepSettings`, which come from:

- config files
- `PINOCCHIO_*` environment variables
- built-in defaults

That means your script does not need to manually reconstruct provider settings from scratch.

## Basic Workflow

The most common workflow looks like this:

1. Build an engine from Pinocchio defaults.
2. Resolve runtime metadata from a profile registry.
3. Run inference through `gp.runner`.

In pseudocode:

```text
pinocchio js
  -> resolve base StepSettings from Pinocchio config
  -> resolve profile registry stack
  -> load JS runtime
  -> script calls pinocchio.engines.fromDefaults()
  -> script calls gp.runner.resolveRuntime()
  -> script calls gp.runner.run() or gp.runner.start()
```

## Example

This example uses:

- `require("pinocchio").engines.fromDefaults(...)`
- `gp.runner.resolveRuntime(...)`
- `gp.runner.run(...)`

```javascript
const gp = require("geppetto");
const pinocchio = require("pinocchio");

const engine = pinocchio.engines.fromDefaults({
  model: "gpt-4o-mini",
  apiType: "openai",
});

const runtime = gp.runner.resolveRuntime({
  profile: { profileSlug: "assistant" },
});

const out = gp.runner.run({
  engine,
  runtime,
  prompt: "Say hello in one line.",
});

console.log(out.blocks[0].payload.text);
```

## Running The Local Demo

The repo includes a local runnable example:

- `examples/js/runner-profile-demo.js`
- `examples/js/profiles/basic.yaml`

Run it from the repo root:

```bash
pinocchio js \
  --script examples/js/runner-profile-demo.js \
  --profile-registries examples/js/profiles/basic.yaml
```

This is a good first smoke test because it proves all of the following in one run:

- the command can execute a script
- the profile registry is loaded
- runtime metadata is resolved and stamped
- `pinocchio.engines.fromDefaults(...)` works
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

Use this when your script resolves runtime from profiles.

```bash
pinocchio js \
  --script examples/js/runner-profile-demo.js \
  --profile-registries examples/js/profiles/basic.yaml
```

If you do not pass the flag, Pinocchio still follows its normal discovery rules:

- `PINOCCHIO_PROFILE_REGISTRIES`
- `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` when present

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

This is the recommended path when:

- you want the same provider credentials and timeout defaults the rest of Pinocchio uses
- you want script setup to stay small

### Profile runtime resolution

`gp.runner.resolveRuntime(...)` is still the right place to resolve:

- system prompt
- middleware uses
- allowed tool names
- runtime metadata

That separation is intentional:

- Pinocchio owns engine bootstrap.
- Geppetto owns generic runtime resolution and runner execution.

## Troubleshooting

| Problem | Cause | Solution |
| --- | --- | --- |
| `--script is required` | No script path was given | Pass `--script path.js` or a positional file path |
| `profile registry ...` validation errors | The YAML registry format is wrong | Use single-registry runtime YAML, not legacy `registries:` or `default_profile_slug` |
| `unknown provider <nil>` from `pinocchio.engines.fromDefaults()` | Base config does not specify a provider and the script did not override it | Pass both `model` and `apiType` explicitly |
| `unknown go middleware` | Profile runtime references middleware not registered by the command | Check the profile middleware names and the Pinocchio JS bootstrap surface |
| `Cannot find module` | The script or its local `node_modules` directory is not reachable | Run with the correct script path so the command can add the script directory to require search paths |

## See Also

- `pinocchio help profiles`
- `pinocchio help webchat-profile-registry`
- `pinocchio help webchat-runner-migration-guide`
