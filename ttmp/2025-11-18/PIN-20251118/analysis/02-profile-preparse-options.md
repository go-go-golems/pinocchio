Title: Pre-parsing profile settings before profile middleware
Slug: profile-preparse-options
Short: Options to ensure profile-selection flags/env are resolved before wiring the profile middleware.
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

## Context

- `geppetto/pkg/layers/layers.go:93-170` builds the middleware slice for every Pinocchio/Geppetto command. It currently reads `cli.ProfileSettings` from `parsedCommandLayers` (which still hold defaults) and immediately instantiates `middlewares.GatherFlagsFromProfiles(...)`. Because the middleware captures the profile string at construction time, any later env/flag overrides are ignored.
- We want the profile selection layer (`cli.ProfileSettingsSlug`) to go through the *same* config/env/flag resolution as everything else, and only then instantiate the profile middleware that loads `profiles.yaml`.
- We’re free to touch Glazed internals if needed, but there’s also a self-contained approach that reuses existing middlewares on a tiny subset of layers.

## Option matrix

| Option | Description | Pros | Cons / Changes |
| --- | --- | --- | --- |
| Two-phase execution | Run a first batch of middlewares (config/env/flags) to populate `parsedLayers`, inspect `profile-settings`, then run the rest (profile + defaults). | Keeps current API intact, minimal code changes in Pinocchio. | Needs a helper to execute the chain in stages. |
| Dynamic middleware | Wrap profile loader in a middleware that reads `profile-settings` from `parsedLayers` at runtime and then calls `GatherFlagsFromProfiles`. | Single pass, no reordering. | Requires a “dynamic middleware” helper or inlined closure per call site. |
| Config overlay | Treat profiles as config overlays via `ConfigFilesFunc`, applying a mapper that extracts just the selected profile. | Simplifies precedence to standard config semantics. | Requires mapper + changes to `ConfigFilesFunc` to peek at `profile-settings`. |
| Parser bootstrap | Modify `cli.BuildCobraCommand` to resolve command/profile settings before user middlewares run. | Centralized solution benefitting all commands. | Deeper change in Glazed CLI parser. |
| Mini chain (recommended) | Manually parse only the `command-settings` and `profile-settings` layers via a small middleware chain before we build the full chain; use the result to instantiate the profile middleware. | Self-contained, zero Glazed changes, deterministic, easy to test. | Slight duplication of middleware execution (the mini pass re-parses CLI/env just for two layers). |

## Detailed approach: self-contained mini chain

1. **Build the layer subset**  
   - Use the two existing layer definitions from the command description (they’re already injected via `cli.WithCommandSettingsLayer()` / `cli.WithProfileSettingsLayer()`).
   - Create a new `layers.ParameterLayers` containing only those two layers.

2. **Run a miniature middleware pass**  
   ```go
   seedLayers := layers.NewParameterLayers(layers.WithLayers(profileLayer, commandLayer))
   seedParsed := layers.NewParsedLayers()
   err := middlewares.ExecuteMiddlewares(seedLayers, seedParsed,
       middlewares.LoadParametersFromFiles(initialConfigPaths...), // optional—only if --config-file impacts profile defaults
       middlewares.UpdateFromEnv("PINOCCHIO"),                    // picks up PINOCCHIO_PROFILE(_FILE)
       middlewares.ParseFromCobraCommand(cmd),
       middlewares.GatherArguments(args),
       middlewares.SetFromDefaults(),
   )
   ```
   - This is near-instant because only two layers are involved.

3. **Read the settings**  
   ```go
   commandSettings := &cli.CommandSettings{}
   _ = seedParsed.InitializeStruct(cli.CommandSettingsSlug, commandSettings)
   profileSettings := &cli.ProfileSettings{}
   _ = seedParsed.InitializeStruct(cli.ProfileSettingsSlug, profileSettings)
   ```
   - Now `profileSettings.Profile`/`ProfileFile` reflect defaults overridden by config, env, and CLI flags.

4. **Instantiate the real profile middleware**  
   ```go
   profileMW := middlewares.GatherFlagsFromProfiles(
       defaultProfileFile,
       profileSettings.ProfileFile,
       profileSettings.Profile,
       parameters.WithParseStepSource("profiles"),
       parameters.WithParseStepMetadata(map[string]interface{}{
           "profileFile": profileSettings.ProfileFile,
           "profile":     profileSettings.Profile,
       }),
   )
   middlewares_ = append(middlewares_, profileMW)
   ```

5. **Run the standard chain**  
   - Build the normal middleware list (config files, `UpdateFromEnv`, CLI flags, defaults) for the *full* set of layers. Because this pass is unchanged, we keep deterministic precedence.

## Benefits

- Works today without touching Glazed internals.
- Mirrors how `glaze help implementing-profile-middleware` recommends layering profile-specific config: we simply evaluate the `--profile/--profile-file` args earlier.
- Avoids duplicating profile parsing logic across commands; this helper can live near `geppetto/pkg/layers` and be reused by standalone binaries (`cmd/examples/*`) that need profile support.
- The same pattern can resolve other “top-level” settings that influence later middleware wiring (e.g., `commandSettings.LoadParametersFromFile`).

## Required file touch points

- `geppetto/pkg/layers/layers.go`: insert the mini chain right before we append the profile middleware.
- Optional: utility in `glazed/pkg/cmds/middlewares` (e.g., `ExecuteMiddlewaresOnce`) if we want to avoid repeating boilerplate, but not strictly required because the small chain already uses public APIs.
- Tests: add coverage for `PINOCCHIO_PROFILE`, `--profile`, and fallback defaults so we detect regressions.
