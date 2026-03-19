---
Title: Manuel Investigation Diary
Ticket: GP-48-PINOCCHIO-JS-RUNNER
DocType: reference
Summary: "Chronological implementation diary for the Pinocchio JS command work."
LastUpdated: 2026-03-18T16:32:00-04:00
---

# Manuel Investigation Diary

## 2026-03-18

### Initial findings

- Confirmed `pinocchio` repo worktree was clean before starting.
- Read [main.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/main.go) to verify the root command already defines `--profile-registries`.
- Read [profile_runtime.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime.go) and [parse-helpers.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/parse-helpers.go) to confirm Pinocchio already has hidden base `StepSettings` resolution and profile-registry default discovery.
- Read [geppetto-js-lab main](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/cmd/examples/geppetto-js-lab/main.go) as the baseline shell implementation.
- Confirmed Geppetto JS runtime bootstrap and the new JS runner API are already available in:
  - [runtime.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/pkg/js/runtime/runtime.go)
  - [api_runner.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/geppetto/pkg/js/modules/geppetto/api_runner.go)

### Design decision

The command should not be a straight copy of `geppetto-js-lab`.

The right architecture is:

- Pinocchio CLI owns config/default/profile-registry bootstrap
- Geppetto still owns the inference JS module
- a small Pinocchio-owned JS module should provide the missing ergonomic helper: "engine from Pinocchio defaults"

### Open implementation questions captured before coding

- whether to expose a positional script path as well as `--script`
- whether to move web-chat middleware definitions into a reusable package or keep the first cut with a smaller registry
- whether to expose just `pinocchio.engines.fromDefaults()` or also config/profile helper objects in the JS module

### Implementation slice 1

- Added a new Cobra command in [cmd/pinocchio/cmds/js.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/cmds/js.go) and registered it from [cmd/pinocchio/main.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/main.go).
- Added a reusable native JS module in [pkg/js/modules/pinocchio/module.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/js/modules/pinocchio/module.go).
- The first exported helper is `pinocchio.engines.fromDefaults(options?)`.
- The command now:
  - resolves hidden Pinocchio base `StepSettings`
  - resolves profile registry sources with Pinocchio defaults
  - builds a JS runtime exposing both `require("geppetto")` and `require("pinocchio")`
  - injects `console`, `ENV`, `sleep`, and `assert`
  - exposes a real Go calculator tool
- Added a smoke script and local sample registry:
  - [runner-profile-demo.js](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/runner-profile-demo.js)
  - [basic.yaml](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/profiles/basic.yaml)

### Validation notes

- `go test ./cmd/pinocchio/... ./pkg/js/... -count=1` passed.
- `go build ./cmd/pinocchio` passed.
- `go run ./cmd/pinocchio js --list-go-tools` printed `calc`.
- `go run ./cmd/pinocchio js --script examples/js/runner-profile-demo.js --profile-registries examples/js/profiles/basic.yaml` succeeded and printed the expected assistant string.

### Known follow-up from this slice

- `pinocchio.engines.fromDefaults({ model: ... })` still appears to need an explicit `apiType` when base Pinocchio config does not already set a provider. The command works, but the model-only fallback is not yet trustworthy enough to document as the primary path.

### Discoverability slice

- Updated [README.md](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/README.md) with a dedicated `pinocchio js` section and runnable example.
- Added [examples/js/README.md](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/README.md) so the example directory explains the intended script model.
- Added a Glazed help page at [05-js-runner-scripts.md](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md).
- Verified the help page renders correctly with `pinocchio help js-runner-scripts`.

### Follow-up: native profile/config behavior for `pinocchio js`

- Reproduced the reported failure: `pinocchio js ./examples/js/runner-profile-demo.js --profile gpt-5-mini` failed because the first cut of the command did not expose `--profile`/default profile handling the same way as the rest of Pinocchio.
- Confirmed a second mismatch at the same time: the command also did not expose `--config-file`, so it could not inherit `profile-settings.profile-registries` from the normal Pinocchio config path.
- Added app-owned default profile resolution plumbing to the Geppetto JS module so hosts can tell `gp.profiles.resolve({})` and `gp.runner.resolveRuntime({})` to use:
  - an explicitly selected profile when one is configured
  - otherwise the registry stack default profile
- Updated [cmd/pinocchio/cmds/js.go](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/cmds/js.go) so `pinocchio js` now accepts:
  - `--profile`
  - `--config-file`
  - `--profile-registries`
  and resolves them through the same config/env/default profile settings helper used by the rest of Pinocchio.
- Updated the local demo script and docs so the recommended script pattern is now:
  - `const runtime = gp.runner.resolveRuntime({});`
  instead of hard-coding a profile slug inside the script.

### Follow-up validation

- `go test ./pkg/js/modules/geppetto -count=1`
- `go test ./cmd/pinocchio/... ./pkg/cmds/helpers -count=1`
- `go run ./cmd/pinocchio js ./examples/js/runner-profile-demo.js --profile assistant --profile-registries examples/js/profiles/basic.yaml`
- `go run ./cmd/pinocchio js ./examples/js/runner-profile-demo.js --config-file <tmp-config>`
- `go run ./cmd/pinocchio js ./examples/js/runner-profile-demo.js --config-file <tmp-config> --profile assistant`

### Follow-up: split smoke example from real inference example

- Confirmed a usability bug in the first example layout: [runner-profile-demo.js](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/runner-profile-demo.js) still used a local `gp.engines.fromFunction(...)` engine for the final run, so the visible output looked like a model reply even though no live inference happened.
- Split the examples into two clear roles:
  - [runner-profile-demo.js](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/runner-profile-demo.js): real inference example
  - [runner-profile-smoke.js](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/runner-profile-smoke.js): deterministic local smoke script
- Moved the command-level regression coverage to the smoke script so tests stay fast and deterministic.
- Updated the docs to state explicitly that:
  - the smoke script is for local validation
  - the demo script is for real LLM calls
  - the demo script passes explicit `model` and `apiType` overrides so it can run even if base Pinocchio config does not already define a provider

### Smoke validation after example split

- `go test ./cmd/pinocchio/... ./pkg/cmds/helpers -count=1`
- `go run ./cmd/pinocchio js ./examples/js/runner-profile-smoke.js --profile assistant --profile-registries examples/js/profiles/basic.yaml`

### Follow-up: expose engine bootstrap inspection

- The live example still left one important debugging gap: the engine object itself is opaque in JS, so when inference failed there was no obvious way to see the selected provider/model/base URL without dropping into Go.
- Added [pinocchio.engines.inspectDefaults(...)](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/js/modules/pinocchio/module.go) as a JS-facing inspection helper.
- The helper follows the same base-settings-plus-overrides path as `pinocchio.engines.fromDefaults(...)`, but returns a plain object instead of constructing a live engine.
- Current fields exposed by the helper:
  - `apiType`
  - `model`
  - `baseURL`
  - `hasAPIKey`
  - `timeoutMs`
- Updated the live demo to print:
  - resolved runtime metadata (`runtimeKey`, `runtimeFingerprint`, `profileVersion`, `toolNames`)
  - inspected engine bootstrap settings
  before running live inference.

### Validation after inspection helper

- `go test ./cmd/pinocchio/... ./pkg/cmds/helpers -count=1`
- `go run ./cmd/pinocchio js ./examples/js/runner-profile-smoke.js --profile assistant --profile-registries examples/js/profiles/basic.yaml`
