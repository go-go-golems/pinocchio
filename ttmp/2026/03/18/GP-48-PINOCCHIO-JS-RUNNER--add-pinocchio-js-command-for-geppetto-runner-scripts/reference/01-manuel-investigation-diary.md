---
Title: Manuel Investigation Diary
Ticket: GP-48-PINOCCHIO-JS-RUNNER
DocType: reference
Summary: "Chronological implementation diary for the Pinocchio JS command work."
LastUpdated: 2026-03-18T11:55:00-04:00
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
