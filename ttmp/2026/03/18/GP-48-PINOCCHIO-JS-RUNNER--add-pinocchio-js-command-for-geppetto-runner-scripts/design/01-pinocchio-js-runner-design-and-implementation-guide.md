---
Title: Pinocchio JS Runner Design And Implementation Guide
Ticket: GP-48-PINOCCHIO-JS-RUNNER
DocType: design
Summary: "Detailed intern-facing guide for implementing `pinocchio js` on top of Geppetto's JS runner API."
LastUpdated: 2026-03-18T11:55:00-04:00
---

# Pinocchio JS Runner Design And Implementation Guide

## Purpose

This document explains how to add a first-class `pinocchio js` command. The command should let a user run a JavaScript file directly from the Pinocchio CLI while reusing:

- Pinocchio config and environment loading
- Pinocchio profile-registry resolution
- Geppetto's JavaScript API
- Geppetto's new opinionated JS runner API

The main product goal is simple:

- a user should be able to run `pinocchio js --script script.js`
- the script should be able to call `require("geppetto")`
- the script should be able to resolve profile runtime from Pinocchio profile registries
- the script should be able to create an engine from Pinocchio defaults instead of manually rebuilding all provider settings inside JavaScript

## Why This Exists

Before this ticket, there was a demo binary in Geppetto:

- [main.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/cmd/examples/geppetto-js-lab/main.go)

That binary is useful for experimentation, but it is not a native Pinocchio entrypoint. It has three limitations:

- it lives in Geppetto examples, not in the main Pinocchio CLI
- it uses demo bootstrap code instead of Pinocchio config defaults
- it does not provide a Pinocchio-owned helper surface for "engine from Pinocchio defaults"

Pinocchio already has the hidden configuration machinery we want:

- [profile_runtime.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime.go)
- [parse-helpers.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/parse-helpers.go)

So the right design is to put a small shell around those existing helpers instead of inventing a second config story.

## System Overview

The command sits at the seam between two systems.

### Pinocchio owns

- CLI entrypoint
- config-file discovery
- env/default loading
- default profile-registry discovery
- any Pinocchio-specific helper module exposed to JavaScript

### Geppetto owns

- the `require("geppetto")` JavaScript module
- `gp.engines.*`
- `gp.profiles.*`
- `gp.runner.*`
- tool-loop/session execution internals

### Architecture diagram

```text
pinocchio js
  -> parse CLI args
  -> resolve hidden Pinocchio base StepSettings
  -> resolve Pinocchio profile registries
  -> construct JS runtime
       -> require("geppetto")
       -> require("pinocchio")   (new small helper module)
       -> console / ENV helpers
  -> run script
       -> gp.runner.resolveRuntime(...)
       -> pinocchio.engines.fromDefaults(...)
       -> gp.runner.run(...) or gp.runner.start(...)
```

## Existing Building Blocks

### 1. Root CLI wiring

Pinocchio root command already has a persistent profile-registry flag:

- [main.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/main.go)

Important detail:

- `rootCmd.PersistentFlags().String("profile-registries", "", "...")`

That means `pinocchio js` should not invent a second registry flag name. It should read the same root flag from the command context or use the same resolution rules.

### 2. Hidden base StepSettings resolution

Pinocchio already knows how to produce base AI settings from config/env/defaults:

- [ResolveBaseStepSettings](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime.go)

This is critical. It means the JS command does not need to expose the full Geppetto flag surface publicly just to create an engine.

The hidden bootstrap already loads:

- Pinocchio config files
- `PINOCCHIO_*` environment variables
- Geppetto default section values

### 3. Profile registry resolution

Pinocchio already has a default profile-registry discovery rule:

- [defaultPinocchioProfileRegistriesIfPresent](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/parse-helpers.go)

Current behavior:

- use explicit `--profile-registries` if provided
- otherwise use `PINOCCHIO_PROFILE_REGISTRIES`
- otherwise use `~/.config/pinocchio/profiles.yaml` when present

That behavior should carry into `pinocchio js`.

### 4. Geppetto JS runtime bootstrap

Geppetto already exposes a runtime wrapper that installs the Geppetto JS module:

- [runtime.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/pkg/js/runtime/runtime.go)

That wrapper gives us:

- `require("geppetto")`
- Go-owned runtime lifecycle
- default-module support
- runtime initializers

### 5. Geppetto JS runner API

The actual high-level JS API we want users to call already exists:

- [api_runner.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/pkg/js/modules/geppetto/api_runner.go)

Important public functions:

- `gp.runner.resolveRuntime(...)`
- `gp.runner.prepare(...)`
- `gp.runner.run(...)`
- `gp.runner.start(...)`

So this ticket does not need to design a new runner API. It needs to make it easy to use from Pinocchio.

## Recommended User Experience

### Common user command

```bash
pinocchio js --script script.js
```

### Alternative positional form

```bash
pinocchio js script.js
```

Support both if it is cheap to implement. This keeps the command ergonomic for one-off scripts while still allowing an explicit flag in automation.

### Common script shape

```javascript
const gp = require("geppetto");
const pinocchio = require("pinocchio");

const engine = pinocchio.engines.fromDefaults();
const runtime = gp.runner.resolveRuntime({
  profile: { profileSlug: "assistant" },
});

const turn = gp.runner.run({
  engine,
  runtime,
  prompt: "Summarize the current repo structure.",
});

console.log(turn.messages[0].content);
```

### Streaming script shape

```javascript
const gp = require("geppetto");
const pinocchio = require("pinocchio");

const events = gp.events.collector();
const engine = pinocchio.engines.fromDefaults();
const runtime = gp.runner.resolveRuntime({
  profile: { profileSlug: "assistant" },
});

const handle = gp.runner.start({
  engine,
  runtime,
  prompt: "Stream a response step by step.",
  eventSinks: [events.sink],
});

handle.wait();
console.log(events.items());
```

## What The New Pinocchio Module Should Do

The simplest useful module surface is small.

### Proposed JS module contract

```text
require("pinocchio")
  .engines.fromDefaults(options?)
  .config.baseSettings()
  .runtime.defaults()
```

The minimum implementation needed for the first cut is:

- `pinocchio.engines.fromDefaults(options?)`

That single helper is enough to make the command useful because profile runtime resolution is already handled by `gp.runner.resolveRuntime(...)`.

### Why not build everything into the Geppetto module?

Because Pinocchio-specific config defaults are application concerns.

Geppetto should not know:

- where Pinocchio config files live
- how Pinocchio discovers profile registries
- which app-specific defaults Pinocchio wants

So the clean split is:

- Geppetto stays generic
- Pinocchio adds a small app-owned JS helper surface

## Detailed API Recommendation

### Go command API

The Cobra command should look roughly like this:

```go
func NewJSCommand() *cobra.Command
```

Flags:

- `--script`
- `--print-result`
- `--list-go-tools`

Optional:

- positional script path

The command should also respect inherited root flags:

- `--profile-registries`

### Internal Go runtime bootstrap

```go
type jsCommandOptions struct {
    ScriptPath   string
    PrintResult  bool
    ListGoTools  bool
}
```

### Pinocchio JS helper module

```go
type pinocchioModuleOptions struct {
    BaseStepSettings *settings.StepSettings
}
```

```go
require("pinocchio").engines.fromDefaults({
  model: "gemini-2.5-flash",
  apiType: "gemini",
});
```

Recommended override policy:

- start from hidden base `StepSettings`
- optionally allow a narrow override set:
  - `model`
  - `apiType`
  - `baseURL`
  - `timeoutMs`
- do not expose full StepSettings mutation in the first cut

This keeps the API small and predictable.

## Implementation Steps

### Step 1. Add the ticket-local design and task docs

This is already the current document set. The reason is practical:

- the command touches two repos conceptually
- there is a risk of building a demo shell instead of a product command
- we want the implementation slices to stay reviewable

### Step 2. Add a new Cobra command

Files likely involved:

- [main.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/main.go)
- `cmd/pinocchio/cmds/js.go` (new)

Pseudocode:

```go
func NewJSCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "js [script.js]",
        Short: "Run JavaScript against Pinocchio's Geppetto runtime",
        RunE:  runJSCommand,
    }
    cmd.Flags().String("script", "", "Path to JavaScript file")
    cmd.Flags().Bool("print-result", false, "Print top-level return value as JSON")
    cmd.Flags().Bool("list-go-tools", false, "List Go tools exposed to JS and exit")
    return cmd
}
```

### Step 3. Resolve script path and runtime folders

The command should:

- accept `--script`
- accept one positional script path
- reject missing script
- add the script directory and `node_modules` directory to the Goja `require` global folders

This mirrors:

- [geppetto-js-lab main](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/cmd/examples/geppetto-js-lab/main.go)

### Step 4. Resolve base StepSettings from Pinocchio

This should use:

- [ResolveBaseStepSettings](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime.go)

That is the most important difference from `geppetto-js-lab`.

The command should not force users to reconstruct provider config manually in JavaScript if Pinocchio already knows how to resolve it.

### Step 5. Resolve profile registries the Pinocchio way

Use:

- [ResolveProfileSettings](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime.go)
- [ParseProfileRegistrySourceEntries](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/pkg/profiles/sources.go)
- [NewChainedRegistryFromSourceSpecs](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/pkg/profiles/chain_from_sources.go)

This is a good place to remember the design rule:

- the command owns how to load registries
- the Geppetto JS module only consumes the registry reader

### Step 6. Build a real Go tool registry

The first cut can stay small. Recommended:

- register a calculator tool

If that registry is empty, profile `tools` metadata becomes less useful. So the command should expose at least one real tool.

### Step 7. Provide middleware definitions when possible

There are two choices:

1. `nil` middleware definitions
2. a real Pinocchio middleware registry

The better product choice is a real registry, because Pinocchio profiles may reference middleware uses.

The likely implementation is:

- move or duplicate the web-chat middleware definition construction into a reusable helper

Existing source:

- [middleware_definitions.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/web-chat/middleware_definitions.go)

Recommended outcome:

- reusable helper in a non-`main` package

### Step 8. Add the Pinocchio native JS module

This is the core ergonomic improvement.

Module name:

- `pinocchio`

Minimum exports:

```text
pinocchio.engines.fromDefaults()
```

Maybe:

```text
pinocchio.config.baseSettings()
pinocchio.profile.connectedSources()
```

But those can come later. `engines.fromDefaults()` is the minimum needed to make the command feel valuable.

### Step 9. Install runtime helpers

Keep the bootstrap helpers small and practical:

- `console.log`
- `console.error`
- `ENV`
- `assert`
- `sleep`

That behavior can be borrowed from:

- [geppetto-js-lab main](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/cmd/examples/geppetto-js-lab/main.go)

### Step 10. Add example scripts or smoke paths

The best smoke test is not a synthetic unit test. It is a real script run.

Recommended smoke path:

```bash
pinocchio js --script /path/to/script.js --profile-registries /path/to/profiles.yaml
```

The initial smoke script should:

- require `geppetto`
- require `pinocchio`
- build engine from Pinocchio defaults
- resolve runtime from profile
- call `gp.runner.run(...)`

## Common Failure Modes

### Failure: script can resolve profile runtime but cannot build an engine

Cause:

- no Pinocchio-owned helper exists yet to create engine from hidden base settings

Fix:

- add `require("pinocchio").engines.fromDefaults()`

### Failure: profile middleware cannot resolve

Cause:

- command passed no middleware definition registry into the Geppetto JS module

Fix:

- provide real Pinocchio middleware definitions

### Failure: script sees no profile registry

Cause:

- command did not resolve the inherited `--profile-registries` value or default config fallback

Fix:

- route command bootstrap through Pinocchio helper resolution, not raw local flag parsing only

## Concrete Acceptance Criteria

- `pinocchio js --script script.js` exists
- the command boots `require("geppetto")`
- the command boots `require("pinocchio")`
- `gp.runner.resolveRuntime({ profile: ... })` works using Pinocchio registry configuration
- `pinocchio.engines.fromDefaults()` works using hidden Pinocchio base settings
- at least one real Go tool is exposed
- help text is clear enough that a user can discover the workflow
- command builds and runs a smoke script successfully

## Review Checklist

- Does the command reuse Pinocchio config/profile resolution instead of duplicating it?
- Is the new JS helper surface small and app-owned?
- Can a simple script run inference without reconstructing provider config manually?
- Are middleware definitions and tool registries real enough to make profile runtime meaningful?
- Does the command stay a thin bootstrap over Geppetto rather than creating a second inference stack?
