---
Title: Implementation Diary - Config and Profile Migration (Plan Points 1 & 2)
Ticket: PIN-20251118
Status: complete
Topics:
  - pinocchio
  - glazed
  - profiles
  - config
  - migration
DocType: analysis
Intent: long-term
Owners:
  - manuel
LastUpdated: 2025-11-18
---

# Implementation Diary: Config and Profile Migration (Plan Points 1 & 2)

## Overview

This diary documents the implementation of plan points 1 and 2 from the config and profile migration analysis. The goal was to modernize Pinocchio's CLI bootstrap and adopt the new Glazed env/config middleware system, replacing the deprecated Viper-based configuration.

## What I Did

### Phase 1: Understanding the Context

1. **Read workflow documentation**
   - Reviewed `/home/manuel/workspaces/2025-11-03/add-persistent-conversation-widget-state/go-go-mento/ttmp/how-to-work-on-any-ticket.md` to understand the docmgr workflow
   - Read `docmgr help how-to-use` to understand how to use docmgr for ticket management
   - Reviewed the analysis document `01-config-and-profile-migration-analysis.md` to understand the migration requirements

2. **Explored the codebase**
   - Read `cmd/pinocchio/main.go` to understand current Viper usage
   - Read `geppetto/pkg/layers/layers.go` to understand the middleware chain
   - Searched for examples of `InitGlazed` and `CobraParserConfig` usage
   - Searched for `LoadParametersFromFiles` and `UpdateFromEnv` examples

### Phase 2: Modernizing CLI Bootstrap

**Step 1: Replace Viper initialization**
- **What I tried**: Direct replacement of `clay.InitViper` with `clay.InitGlazed`
- **What worked**: Simple one-to-one replacement in `initRootCmd()`
- **Changes made**:
  ```go
  // Before
  err = clay.InitViper("pinocchio", rootCmd)
  
  // After
  err = clay.InitGlazed("pinocchio", rootCmd)
  ```

**Step 2: Replace logging initialization**
- **What I tried**: Replaced `logging.InitLoggerFromViper()` with `logging.InitLoggerFromCobra(cmd)` in `PersistentPreRunE`
- **What worked**: Direct replacement, no issues
- **Changes made**:
  ```go
  // Before
  PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    err := logging.InitLoggerFromViper()
    ...
  }
  
  // After
  PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    err := logging.InitLoggerFromCobra(cmd)
    ...
  }
  ```

**Step 3: Replace repositories config reading**
- **What I tried**: Initially considered keeping Viper just for repositories, but decided to read directly from config file
- **What worked**: Created `loadRepositoriesFromConfig()` helper function that:
  - Uses `glazedConfig.ResolveAppConfigPath("pinocchio", "")` to find config file
  - Reads and parses YAML directly
  - Extracts `repositories` key
- **What I learned**: The repositories key is special - it's not part of any layer and is read during initialization, not through the middleware chain
- **Changes made**:
  ```go
  // Before
  repositoryPaths := viper.GetStringSlice("repositories")
  
  // After
  repositoryPaths := loadRepositoriesFromConfig()
  ```

### Phase 3: Adopting Env/Config Middlewares

**Step 1: Update middleware chain**
- **What I tried**: Replaced `GatherFlagsFromViper` with `UpdateFromEnv` and `LoadParametersFromFiles`
- **What worked**: Added environment variable middleware and config file loading middleware
- **Changes made**:
  ```go
  // Added environment variables middleware
  middlewares_ = append(middlewares_,
    middlewares.UpdateFromEnv("PINOCCHIO",
      parameters.WithParseStepSource("env"),
    ),
  )
  
  // Added config files resolver and loader
  configFilesResolver := func(...) ([]string, error) {
    // Resolves config file paths
  }
  middlewares_ = append(middlewares_,
    middlewares.LoadParametersFromResolvedFilesForCobra(...)
  )
  ```

**Step 2: Config file resolver implementation**
- **What I tried**: Created a resolver function that:
  1. Checks for explicit config file from command settings
  2. Checks `LoadParametersFromFile` for backward compatibility
  3. Resolves app config path using `glazedConfig.ResolveAppConfigPath`
- **What worked**: The resolver correctly finds config files in order (low → high precedence)
- **What I learned**: The resolver is called at middleware execution time, not construction time, so it can access parsed layers

### Phase 4: Config File Format Migration

**Step 1: Understanding the problem**
- **What I discovered**: The old config file used flat format (`openai-api-key: value`), but the new system expects layered format (`openai-chat:\n  openai-api-key: value`)
- **What I tried**: Initially tried to run the command to see what would happen
- **What failed**: Got error `expected map[string]interface{} for layer kagi-api-key, got string` - the system was trying to parse flat keys as layer names

**Step 2: Converting the config file**
- **What I tried**: 
  1. Created backup: `cp ~/.pinocchio/config.yaml ~/.pinocchio/config.yaml.backup`
  2. Manually converted the config file to layered format
  3. Mapped keys to appropriate layers:
     - `ai-api-type`, `ai-engine`, `ai-max-response-tokens` → `ai-chat`
     - `openai-api-key` → `openai-chat`
     - `claude-api-key` → `claude-chat`
     - `autosave` → `geppetto-helpers`
- **What worked**: The conversion worked, but encountered a new issue with `repositories`

**Step 3: Handling the repositories key**
- **What failed**: Got error `expected map[string]interface{} for layer repositories, got []interface {}` - the middleware was trying to parse `repositories` as a layer, but it's a list
- **What I tried**: 
  1. Initially tried to remove `repositories` from config file - but it's needed for initialization
  2. Tried to create a special layer for repositories - but it's not part of the layer system
  3. Added a config mapper to exclude `repositories` from layer parsing
- **What worked**: Created a `ConfigFileMapper` that filters out excluded keys:
  ```go
  configMapper := func(rawConfig interface{}) (map[string]map[string]interface{}, error) {
    // Filter out repositories and other non-layer keys
    excludedKeys := map[string]bool{"repositories": true}
    // Only include keys that are maps (layers)
  }
  ```
- **What I learned**: The config mapper is called for each config file, allowing us to transform or filter the structure before it's parsed into layers

**Step 4: Fixing the mapper integration**
- **What failed**: First attempt to use `WithConfigFileMapper` with `LoadParametersFromResolvedFilesForCobra` failed because it only accepts `ParseStepOption`, not `ConfigFileOption`
- **What I tried**: 
  1. Tried to pass mapper as `ParseStepOption` - type mismatch
  2. Created a custom middleware wrapper that calls `LoadParametersFromFiles` directly with the mapper
- **What worked**: Created an inline middleware that:
  1. Calls the resolver to get config files
  2. Calls `LoadParametersFromFiles` with the mapper
  3. Properly handles the middleware chain

## What Failed

1. **Initial config file format**: The flat format didn't work with the new system - had to convert to layered format
2. **Repositories key parsing**: The middleware tried to parse `repositories` as a layer - had to add mapper to exclude it
3. **Mapper integration**: First attempt to integrate mapper failed due to type mismatch - had to create custom middleware wrapper

## What Worked

1. **Direct Viper replacement**: `InitViper` → `InitGlazed` and `InitLoggerFromViper` → `InitLoggerFromCobra` were straightforward replacements
2. **Config file resolver**: The resolver pattern worked well for finding config files in the right order
3. **Config mapper**: The mapper successfully filtered out non-layer keys while preserving layer structure
4. **Build and test**: The code compiled and the command ran successfully after all changes

## What I Learned

1. **Middleware execution order matters**: Middlewares are executed in reverse order (last added = highest precedence), so flags → args → env → config → profiles → defaults

2. **Config file format is strict**: The new system expects `layer-slug: { parameter: value }` format, not flat keys. This is a breaking change for users.

3. **Some keys are special**: The `repositories` key is not part of any layer - it's read during initialization for command discovery, not through the middleware chain. This required special handling.

4. **Mappers are powerful**: Config mappers allow transforming arbitrary config structures into the layer format, which is useful for:
   - Filtering out non-layer keys
   - Supporting legacy formats
   - Custom transformations

5. **Resolver vs Mapper**: 
   - Resolver: Determines which config files to load (can be dynamic based on parsed layers)
   - Mapper: Transforms the structure of each config file (filters, transforms keys)

6. **Build system**: The codebase uses a workspace (`go.work`) with multiple modules, so building requires being in the right directory

## What I Would Do Differently

1. **Test config file format earlier**: I should have checked the expected config file format before making code changes. This would have saved time debugging the format issues.

2. **Create mapper first**: Instead of converting the config file and then discovering the repositories issue, I could have created a mapper that handles both flat and layered formats from the start.

3. **More incremental testing**: I made multiple changes before testing. It would have been better to:
   - Replace Viper → test
   - Add env middleware → test
   - Add config middleware → test
   - Convert config file → test

4. **Document layer mappings**: I should have documented which keys map to which layers before converting the config file. This would make the conversion more systematic.

5. **Handle all keys**: I left some keys unmapped (anyscale-token, bee-api-key, kagi-api-key, etc.). I should have either:
   - Mapped them to appropriate layers
   - Documented why they're not mapped
   - Created a migration guide for users

6. **Add validation**: I should have added validation to ensure the config file structure is correct, or at least better error messages when it's not.

## Key Files Modified

1. **`pinocchio/cmd/pinocchio/main.go`**:
   - Replaced `clay.InitViper` with `clay.InitGlazed`
   - Replaced `logging.InitLoggerFromViper()` with `logging.InitLoggerFromCobra(cmd)`
   - Added `loadRepositoriesFromConfig()` helper function
   - Removed `viper` import, added `glazedConfig` and `yaml` imports

2. **`geppetto/pkg/layers/layers.go`**:
   - Replaced `GatherFlagsFromViper` with `UpdateFromEnv("PINOCCHIO")`
   - Added config files resolver function
   - Added config mapper to exclude `repositories` key
   - Added custom middleware wrapper for config file loading
   - Added `glazedConfig` import

3. **`~/.pinocchio/config.yaml`**:
   - Converted from flat to layered format
   - Mapped keys to appropriate layers
   - Kept `repositories` at top level (excluded from layer parsing)

## Testing

- **Build**: `go build ./cmd/pinocchio` - successful
- **Command execution**: `pinocchio code unix hello --print-parsed-parameters` - successful
- **Config loading**: Verified that config file is loaded and parameters are parsed correctly
- **Repositories**: Verified that repositories are still loaded during initialization

## Phase 5: Fixing Config File Resolver Issue

**Discovery**: After initial implementation, we discovered that the `configFilesResolver` couldn't properly read the `--config-file` flag because of middleware execution order.

**The Problem**:
- The resolver was trying to read `--config-file` from `parsedLayers` (passed to the middleware)
- But at that point in the middleware chain, `ParseFromCobraCommand` hadn't run yet
- So the flag value wasn't available in `parsedLayers`

**What I tried**:
1. Initially tried reading from `parsed` parameter in the resolver - failed because flags weren't parsed yet
2. Checked middleware execution order - discovered that flags are parsed AFTER config files in the chain
3. Realized that `parsedCommandLayers` (passed to `GetCobraCommandGeppettoMiddlewares`) already has command settings parsed via `ParseCommandSettingsLayer`

**What worked**:
- Changed resolver to use `commandSettings` (read from `parsedCommandLayers` at function start) instead of reading from `parsed` parameter
- Fixed file loading order: default config first (low precedence), then explicit config (high precedence)
- Added comments explaining why we use `parsedCommandLayers` instead of `parsedLayers`

**What I learned**:
1. **Middleware execution order matters critically**: The resolver runs as part of the config loading middleware, but flags are parsed in a later middleware. We need to use pre-parsed command settings.
2. **ParseCommandSettingsLayer runs first**: Before the main middleware chain, command settings (including `--config-file`) are already parsed into `parsedCommandLayers`. This is available to the middleware constructor function.
3. **File precedence**: When loading multiple config files, order matters - default config should be loaded first (low precedence), then explicit config (high precedence) so explicit values override defaults.

**Changes made**:
```go
// Before: Tried to read from parsed parameter (flags not parsed yet)
configFilesResolver := func(parsed *cmdlayers.ParsedLayers, ...) {
    cs := &cli.CommandSettings{}
    if err := parsed.InitializeStruct(cli.CommandSettingsSlug, cs); err == nil {
        // This would fail because flags aren't parsed yet
    }
}

// After: Use commandSettings from parsedCommandLayers (already parsed)
configFilesResolver := func(_ *cmdlayers.ParsedLayers, ...) {
    // Use commandSettings read earlier from parsedCommandLayers
    if commandSettings.ConfigFile != "" {
        files = append(files, commandSettings.ConfigFile)
    }
    // Load default config first (low precedence)
    configPath, _ := glazedConfig.ResolveAppConfigPath("pinocchio", "")
    if configPath != "" {
        files = append(files, configPath)
    }
    // Explicit config loaded last (high precedence)
}
```

**Testing**:
- Created test config file `/tmp/test-pinocchio-config.yaml` with `openai-api-key: test-key-from-config-file`
- Tested: `pinocchio code unix hello --config-file /tmp/test-pinocchio-config.yaml --print-parsed-parameters`
- Verified: Test config file is loaded and `openai-api-key` value is `test-key-from-config-file`
- Verified: Without `--config-file`, default config (`~/.pinocchio/config.yaml`) is loaded

**Follow-up Fix**: Added whitelist to `UpdateFromEnv` middleware to restrict environment variable parsing to the same layers that were previously whitelisted for Viper parsing. This ensures only specific layers (ai-chat, ai-client, openai-chat, claude-chat, gemini-chat, embeddings, profile-settings) are affected by environment variables, matching the previous Viper behavior and shielding other layers from unintended env parsing.

## Next Steps (Not Done in This Session)

1. **Plan Point 3**: Rework profile loading to read profile name after env/config are applied
2. **Plan Point 4**: Document config file format migration for users
3. **Plan Point 5**: Update examples and helpers to use new pattern
4. **Add tests**: Regression tests for config loading and profile selection
5. **Handle unmapped keys**: Decide what to do with anyscale-token, bee-api-key, kagi-api-key, etc.

## Conclusion

The migration of plan points 1 and 2 was successful. The codebase now uses the modern Glazed middleware system instead of Viper. The main challenges were:
1. Understanding the new config file format requirements
2. Handling special keys like `repositories` that aren't part of the layer system
3. Integrating the config mapper with the middleware chain
4. **Fixing the config file resolver to properly read `--config-file` flag** (discovered and fixed after initial implementation)

The implementation is working correctly, including the `--config-file` flag support and environment variable whitelisting. There's still work to do on plan points 3-5, especially fixing the profile loading bug where the profile name is read before env/config are applied.

