Title: Plan - Profile Parsing rework
Slug: profile-parsing-plan
Short: Detailed steps to make `--profile` / `PINOCCHIO_PROFILE` work with the new Glazed config flow.
Topics:
- pinocchio
- profiles
- glazed
- config
DocType: design-doc
Intent: long-term
Owners:
- manuel
Status: draft

## Goals

1. Ensure `cli.ProfileSettings` (profile + profile-file) are fully resolved via config/env/CLI before we instantiate `GatherFlagsFromProfiles`.
2. Keep the rest of the middleware chain aligned with the “env/config first, defaults last” flow described in `01-config-and-profile-migration-analysis.md`, the implementation diary, and the migration playbook.
3. Update helper entry points (e.g., `pkg/cmds/helpers/parse-helpers.go`, `cmd/examples/*`) so they reuse the same profile-resolution logic.
4. Add regression tests for `PINOCCHIO_PROFILE`, `--profile`, `PINOCCHIO_PROFILE_FILE`, and default fallback.

## Reference inputs

- `various/02-implementation-diary-config-migration.md`: shows that modern Glazed config middlewares are already wired (InitGlazed, logging, resolver, env middleware).
- `playbooks/01-migrating-from-viper-to-glazed-config.md`: outlines how layered config files are loaded and how non-layer keys (like `repositories`) are mapped.
- `analysis/02-profile-preparse-options.md`: enumerates the options we can pick; we’ll implement the “mini middleware chain” approach here.

## Plan of record

1. **Introduce a profile-settings bootstrap helper (`resolveProfileSettings`)**
   - Location: `geppetto/pkg/layers` (next to `GetCobraCommandGeppettoMiddlewares`) or a new utility package.
   - Inputs: command description (for layers), Cobra command, args, already built config resolver.
   - Actions:
     1. Construct a temporary `layers.ParameterLayers` with only `command-settings` and `profile-settings`.
     2. Execute a short middleware chain:
        - `LoadParametersFromFiles` (using the same resolver as the real chain so `--config-file`/default config can set profile defaults)
        - `UpdateFromEnv("PINOCCHIO")`
        - `ParseFromCobraCommand(cmd)`
        - `GatherArguments(args)`
        - `SetFromDefaults()`
     3. Initialize `cli.CommandSettings` and `cli.ProfileSettings` structs from the resulting parsed layers.
     4. Return the resolved struct values plus any config paths (so the main chain can reuse them).
   - Notes: share resolver logic so we don’t resolve config paths twice; optionally cache the result on the command context to avoid re-running when the same command executes multiple times.

2. **Refactor `GetCobraCommandGeppettoMiddlewares`**
   - Replace the current `profileSettings := ...` block with a call to `resolveProfileSettings`.
   - Append `GatherFlagsFromProfiles` using the resolved `profileSettings`.
   - Keep the rest of the middleware list as-is (config loader -> env -> CLI -> args -> defaults), but now the profile middleware is wired with correct values.
   - Ensure `commandSettings.LoadParametersFromFile` still participates in both the mini chain and the full chain for backward compatibility.

3. **Update helper paths**
   - `pkg/cmds/helpers/parse-helpers.go`: replace its inline `GatherFlagsFromProfiles + GatherFlagsFromViper` chain with:
     - `resolveProfileSettings` (or an equivalent helper for non-Cobra contexts)
     - `LoadParametersFromFiles` + `UpdateFromEnv` + `SetFromDefaults` for the normal run
   - `cmd/examples/*` and other binaries that manually call `helpers.ParseGeppettoLayers` should continue to work once that helper is updated.

4. **Testing & verification**
   - Add unit/integration tests (likely in `geppetto/pkg/layers` or a new package) covering:
     - Default profile (`profile-settings.profile` empty -> `default`).
     - `PINOCCHIO_PROFILE=gemini` env var overriding defaults.
     - `--profile gemini` flag overriding env.
     - `--profile-file /tmp/profiles.yaml` and `PINOCCHIO_PROFILE_FILE` picking up non-default paths.
   - Add CLI-level smoke tests (maybe in `cmd/pinocchio` using `go test ./cmd/pinocchio/...`) to call `pinocchio code ... --print-parsed-parameters` with env vars via `exec.Command`.
   - Document the expected precedence in the README or a troubleshooting doc.

5. **Docs & migration notes**
   - Update README/config documentation to mention layered `profiles.yaml` structure and how to override via env or CLI.
   - Extend `playbooks/01-migrating-from-viper-to-glazed-config.md` (or a new playbook) with a “profile resolution” section referencing this helper so future repos can reuse it.

## Dependencies / sequencing

1. `resolveProfileSettings` helper (blocking item for the rest).
2. Wiring the helper into `GetCobraCommandGeppettoMiddlewares`.
3. Updating `helpers.ParseGeppettoLayers` and other entry points.
4. Adding regression tests + docs simultaneously once the new flow lands.

## Open questions

- Should we cache the resolved profile info per command execution to avoid re-running the mini chain when multiple middleware consumers inspect it? (Probably unnecessary right now.)
- Do we want to allow `--profile` to appear in layered config files too? If yes, ensure the mini chain’s `LoadParametersFromFiles` includes those same files.
- Does any other layer (e.g., repositories) need a similar early parse? If so, we can generalize the helper later.
