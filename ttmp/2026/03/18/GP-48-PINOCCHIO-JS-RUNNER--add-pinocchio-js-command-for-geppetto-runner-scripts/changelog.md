# Changelog

## 2026-03-18

- Initial workspace created
# Changelog

## 2026-03-18

- Created GP-48 to track a first-class `pinocchio js` command.
- Added the initial design and implementation guide.
- Added a detailed task list and implementation diary.
- Implemented the first working slice of `pinocchio js`:
  - new Cobra verb
  - new `require("pinocchio")` JS module
  - Pinocchio config-backed base engine helper
  - profile-registry bootstrap
  - small real Go tool registry
  - middleware factories for Pinocchio profile runtime metadata
  - smoke script under `examples/js`
- Added discoverability and operator docs:
  - README section for `pinocchio js`
  - example-directory README
  - Glazed help page `js-runner-scripts`
- Validated:
  - `go test ./cmd/pinocchio/... ./pkg/doc/... -count=1`
  - `go run ./cmd/pinocchio help js-runner-scripts`
  - `docmgr doctor --ticket GP-48-PINOCCHIO-JS-RUNNER --stale-after 30`
- Follow-up hardening:
  - `pinocchio js` now exposes `--profile` and `--config-file`
  - the command resolves profile settings through the same config/env/default path as other Pinocchio commands
  - the Geppetto JS module now supports host-provided default profile resolution so scripts can call `gp.runner.resolveRuntime({})` and inherit the active/default Pinocchio profile
  - added regression tests for the host-default JS resolution path and for the `pinocchio js` config/profile flow
- Example cleanup:
  - split the old misleading local-engine script into:
    - `runner-profile-demo.js` for real inference
    - `runner-profile-smoke.js` for deterministic smoke coverage
  - moved automated command checks to the smoke script
  - clarified the README and help docs so operators know which script is live and which is local
