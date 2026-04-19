# Tasks

## Phase 0: Ticket scaffolding and decision capture

- [x] Create a dedicated ticket workspace for the canonical runtime API cleanup.
- [x] Add a design document that specifies the clean target API and explicitly rejects backward compatibility wrappers.
- [x] Add an implementation diary for chronological work tracking.
- [x] Relate the key Geppetto and Pinocchio bootstrap files to the ticket docs.

## Phase 1: Geppetto bootstrap API cleanup

- [x] Remove the public selection-only bootstrap contract:
  - [x] delete `ResolvedCLIProfileSelection`
  - [x] delete `ResolveCLIProfileSelection(...)`
  - [x] delete `ResolveEngineProfileSettings(...)`
- [x] Refactor `ResolveCLIProfileRuntime(...)` so it directly resolves profile settings, config files, implicit registry fallback, and the registry chain.
- [x] Update `ResolvedCLIProfileRuntime` to carry `ProfileSettings` directly instead of nesting a selection object.
- [x] Update `ResolvedCLIEngineSettings` to carry only `ProfileRuntime`, not a duplicate `ProfileSelection` field.
- [x] Replace inference-debug’s dependency on the concrete engine-settings wrapper with a smaller resolved-inference payload contract.
- [x] Update Geppetto unit tests to validate the new runtime contract directly.
- [x] Update any Geppetto tutorial/doc references that still mention `ResolveCLIProfileSelection(...)` or `resolved.ProfileSelection`.

## Phase 2: Pinocchio bootstrap API cleanup

- [x] Remove the public split unified-config helper surface:
  - [x] remove `ResolveCLIProfileSelection(...)`
  - [x] remove `ResolveUnifiedConfig(...)` from the public API
  - [x] remove `ResolveUnifiedProfileRegistryChain(...)` from the public API
  - [x] remove `ResolveEngineProfileSettings(...)`
- [x] Introduce one canonical Pinocchio `ResolveCLIProfileRuntime(...)` that returns config documents, effective config, effective profile settings, and the composed registry chain.
- [x] Replace Pinocchio wrapper aliases over Geppetto bootstrap result structs with Pinocchio-owned clean structs.
- [x] Update `ResolveCLIEngineSettings(...)` and `ResolveCLIEngineSettingsFromBase(...)` to consume the canonical runtime object.
- [x] Update repository-path resolution to use the new config/runtime path without the old exported helper split.

## Phase 3: Call-site migration

- [x] Update `pinocchio/pkg/cmds/cmd.go` to consume `ProfileRuntime` rather than `ProfileSelection`.
- [x] Update `pinocchio/cmd/web-chat/main.go` to use the canonical runtime API.
- [x] Update `pinocchio/cmd/pinocchio/cmds/js.go` to use the canonical runtime API.
- [x] Update any remaining tests or helpers that reference removed selection APIs.

## Phase 4: Validation

- [x] Run focused Geppetto bootstrap tests.
- [x] Run focused Pinocchio bootstrap/cmd tests.
- [x] Run broad Pinocchio tests.
- [x] Build the Pinocchio CLI.
- [x] Re-run a safe `PINOCCHIO_PROFILE=... --print-inference-settings` smoke path.
- [x] Re-run a real runtime smoke path if credentials/config are available.

## Phase 5: Bookkeeping and final review

- [x] Update the diary with the implementation sequence, failures, and validation results.
- [x] Update the ticket changelog with the final API changes and validation summary.
- [x] Run `docmgr doctor --ticket PIN-20260418-CANONICAL-PROFILE-RUNTIME-API --stale-after 30`.
- [x] Commit the code changes.
- [x] Commit the ticket bookkeeping changes.
