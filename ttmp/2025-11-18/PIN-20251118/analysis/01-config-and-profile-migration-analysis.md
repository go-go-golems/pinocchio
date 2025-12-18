Title: Pinocchio config + profile migration analysis
Slug: config-profile-migration-analysis
Short: Findings from linting + profile loading investigation while upgrading to the env/config middleware flow.
Topics:
- pinocchio
- glazed
- profiles
- config
DocType: analysis
Intent: long-term
Owners:
- manuel
Status: draft

## TL;DR

- `make lint` currently fails because Pinocchio still wires every command through the deprecated Viper helpers (`clay.InitViper`, `logging.InitLoggerFromViper`, `GatherFlagsFromViper`, `GatherSpecificFlagsFromViper`, `InitViperInstanceWithAppName`). These calls live in `cmd/pinocchio/main.go`, `cmd/agents/simple-chat-agent/main.go`, `cmd/examples/*`, `cmd/pinocchio/cmds/catter/catter.go`, and `pkg/cmds/helpers/parse-helpers.go`.
- README and runtime still expect the legacy flat `~/.pinocchio/config.yaml` (e.g. the `openai-api-key` and `repositories` keys at README.md:47-55). The new Glazed middlewares require a layered structure (`layer-slug -> parameter -> value`) or a mapper shim.
- Profiles stopped applying because `geppetto/pkg/layers.GetCobraCommandGeppettoMiddlewares` reads `cli.ProfileSettings` **before** any flags/env/config are parsed, then bakes those default values into `GatherFlagsFromProfiles`. When we later export `PINOCCHIO_PROFILE=gemini-2.5-pro`, `profile-settings.profile` is updated inside the parsed layers, but the already-instantiated middleware still targets `profile="default"`, so the selected profile map is never merged.
- Migration will need two tracks: (1) adopt the new `LoadParametersFromFiles + UpdateFromEnv + InitGlazed/SetupLoggingFromParsedLayers` stack everywhere, and (2) refactor profile/config file loading to evaluate the active profile at middleware execution time (or move profile selection into a dedicated layer that runs before we resolve profile-specific files).

## Inputs reviewed

- `docmgr help how-to-use` and `docmgr help how-to-setup` (to bootstrap `.ttmp.yaml` + vocabulary for `pinocchio/ttmp`).
- `/tmp/01-migrating-from-viper-to-env-parsing.md` and `glazed/pkg/doc/tutorials/migrating-from-viper-to-config-files.md` (step-by-step instructions for replacing Viper with explicit config/env middlewares).
- `glazed/pkg/doc/topics/13-layers-and-parsed-layers.md`, `16-parsing-parameters.md`, and `21-cmds-middlewares.md` for reference on how layers + parsed layers interact with middlewares, and how `UpdateFromEnv` constructs env keys.
- `glaze help implementing-profile-middleware` (describes the intended profile-loading middleware chain, including chaining custom profile files and organization-wide profiles).
- `PINOCCHIO_PROFILE=gemini-2.5-pro pinocchio code unix hello --print-parsed-parameters` to confirm the bug (profile env var is logged, but downstream values remain on defaults).
- `make lint` at the repo root to see the current set of deprecated constructs flagged by staticcheck.

## Lint results and immediate hotspots

`make lint` surfaces six `SA1019` violations (full log saved in the shell history for 2025-11-18 22:02). The relevant files/lines:

1. `cmd/agents/simple-chat-agent/main.go:312` calls `clay.InitViper("pinocchio", root)` instead of the new `clay.InitGlazed/... + CobraParserConfig` setup.
2. `cmd/examples/simple-chat/main.go:172` and `cmd/examples/simple-redis-streaming-inference/main.go:215` do the same.
3. `cmd/pinocchio/cmds/catter/catter.go:48-52` still uses `middlewares.GatherSpecificFlagsFromViper`.
4. `cmd/pinocchio/cmds/prompto/prompto.go:21` uses `clay.InitViperInstanceWithAppName`.
5. `pkg/cmds/helpers/parse-helpers.go:77-92` relies on `middlewares.GatherFlagsFromViper` for the helper path.

Together with the warnings printed when running any command (`logging.InitLoggerFromViper is deprecated; use SetupLoggingFromParsedLayers`) it’s clear we must:

- Replace `clay.InitViper(...)` with `clay.InitGlazed(...)` and wire `cli.WithParserConfig(cli.CobraParserConfig{ AppName: "pinocchio", ConfigFilesFunc: ... })`.
- Convert every `GatherFlagsFromViper` / `GatherSpecificFlagsFromViper` use to a combination of `UpdateFromEnv`, `LoadParametersFromFile(s)`, and (when necessary) `LoadParametersFromProfiles` or `GatherFlagsFromCustomProfiles`.
- Switch logging initialization to `logging.InitLoggerFromCobra` (during `PersistentPreRunE`) or `logging.SetupLoggingFromParsedLayers` if we want config-driven logging.

## Current config loading picture

- `cmd/pinocchio/main.go:47-136` still binds Viper inside `initRootCmd()` and uses `logging.InitLoggerFromViper()` inside `rootCmd.PersistentPreRunE`. Config discovery therefore happens implicitly through `$HOME/.pinocchio` etc, and `viper.GetStringSlice("repositories")` is used at `cmd/pinocchio/main.go:138-168`.
- README.md:47-55 instructs users to write a flat `~/.pinocchio/config.yaml` with top-level keys (`openai-api-key`, `repositories`). This format won’t work once we move to `middlewares.LoadParametersFromFile` because that middleware expects `layer-slug -> parameter -> value`. We either need to (a) migrate the file format to the layered structure (`openai-chat.openai-api-key`, `pinocchio-prompts.repositories`, etc.) or (b) add a config mapper to translate the flat structure into the appropriate layers.
- `pkg/cmds/helpers/parse-helpers.go:45-102` is the utility used by `cmd/examples/*` to load Geppetto layers outside of Cobra. It repeats the Viper + profile middleware chain, so it will need to be updated alongside the main CLI.
- There is no code that currently resolves config file paths via the new `CobraParserConfig.ConfigFilesFunc`. Once we drop Viper we must decide how to locate `~/.pinocchio/config.yaml` (probably via `appconfig.ResolveAppConfigPath("pinocchio", "config.yaml")`) and how to merge override files.

## Profile loading failure (root cause)

- The Glazed/Geppetto middleware chain lives in `geppetto/pkg/layers/layers.go:93-170`. `GetCobraCommandGeppettoMiddlewares` reads `cli.CommandSettings` and `cli.ProfileSettings` from the `parsedCommandLayers` **before** we execute any middlewares (those layers only contain defaults at that point).
- We then compute:

  ```go
  if profileSettings.Profile == "" {
      profileSettings.Profile = "default"
  }
  middlewares_ = append(middlewares_,
      middlewares.GatherFlagsFromProfiles(
          defaultProfileFile,
          profileSettings.ProfileFile,
          profileSettings.Profile,
          ...
      ),
  )
  ```

  The profile name is captured as a string literal when we append the middleware.

- Later, when the middleware chain runs, `GatherFlagsFromProfiles` always loads whatever profile was captured at construction time (usually `"default"`). Even though `middlewares.GatherFlagsFromViper` (still in the same chain) eventually reads `PINOCCHIO_PROFILE` into the parsed layers, that value never propagates back to the already-instantiated profile middleware.
- Repro: `PINOCCHIO_PROFILE=gemini-2.5-pro pinocchio code unix hello --print-parsed-parameters` logs `profile-settings.profile` as `gemini-2.5-pro`, but the `ai-chat` layer still reports `ai-engine: gpt-4o-mini` and `ai-api-type: openai`.

### Fix direction

- Move profile resolution into a middleware that runs before we load profile files. For example, insert `middlewares.LoadParametersFromFiles` (for config) and `middlewares.UpdateFromEnv("PINOCCHIO")` so that `profile-settings` is populated, then use `middlewares.DynamicMiddleware`/`middlewares.WithConfigFilesFunc` (or simply inspect the parsed layers inside a custom middleware) to load the selected profile.
- Alternatively, defer the construction of the profile middleware until execution time by wrapping it in a closure that reads the up-to-date `profile-settings` from `parsedLayers` (e.g., implement a small middleware that calls `parsedLayers.InitializeStruct` after previous middlewares ran, then merges the relevant profile map).
- Consider merging the profile file with the rest of the config file and using `LoadParametersFromFiles` with overlays rather than a standalone `GatherFlagsFromProfiles`, which would also make precedence easier to reason about.

## Proposed migration plan (high-level)

1. **Modernize CLI bootstrap**
   - Replace `clay.InitViper`/`logging.InitLoggerFromViper` in `cmd/pinocchio/main.go` with `clay.InitGlazed` + `logging.InitLoggerFromCobra`.
   - Configure `cli.WithParserConfig(cli.CobraParserConfig{ AppName: "pinocchio", ConfigFilesFunc: resolveConfigFiles })` so every command automatically loads `~/.config/pinocchio/config.yaml` (and optional overrides) through `LoadParametersFromFiles`.
   - Ensure we still inject the helpers/profile layers via `cli.WithProfileSettingsLayer()` etc.

2. **Adopt env/config middlewares**
   - Update `geppetto/pkg/layers.GetCobraCommandGeppettoMiddlewares` to use:
     - `middlewares.LoadParametersFromFiles` for config discovery (app config + `--config-file`).
     - `middlewares.UpdateFromEnv("PINOCCHIO")` instead of `GatherFlagsFromViper`.
     - `logging.SetupLoggingFromParsedLayers` after the chain (so logging respects config/env).
   - Update `pkg/cmds/helpers/parse-helpers.go` and `cmd/pinocchio/cmds/catter/catter.go` to use the same pattern (no Viper).

3. **Rework profile loading**
   - Change the middleware chain so the selected profile is read from `parsedLayers` **after** env/config/flags were applied. Load the profile map via `middlewares.GatherFlagsFromProfiles` or `GatherFlagsFromCustomProfiles` using the latest `profile` + `profile-file`.
   - Consider splitting profile selection into its own middleware that merges `profiles.yaml` with config overlays, as described in `glaze help implementing-profile-middleware`.

4. **Config file format/mapping**
   - Decide whether to migrate `~/.pinocchio/config.yaml` to layered format (recommended for future compatibility) or to implement a mapper translating the flat keys into the correct layers. If we keep the old shape for now, we must document the mapper + eventual migration plan.
   - Update README and `pinocchio profiles init` docs accordingly so users know how to structure config + profile files in the new world.

5. **Examples + helpers**
   - Update sample commands (`cmd/examples/*`, `cmd/agents/simple-chat-agent`) to call the new helpers rather than re-binding Viper themselves.
   - Ensure the helper path (`helpers.ParseGeppettoLayers`) exposes an option to opt into the new env/config flow (or note that it is only used in tests/demos and can be simplified).

## Open questions / follow-ups

- Do we want to support legacy `~/.pinocchio/config.yaml` directly via a mapper, or can we announce a breaking change requiring users to move to the layered format?
- Should profile selection become part of the general config overlay (profiles file listed in `ConfigFilesFunc`) instead of a special-case middleware? This could simplify precedence and help with future features (profile inheritance, etc.).
- Once the middleware chain is realigned, we should add regression tests that cover `PINOCCHIO_PROFILE`, `--profile`, and default profile selection so the bug doesn’t return.
