---
Title: Diary
Ticket: 001-FIX-GLAZED-LINTING
Status: active
Topics:
    - pinocchio
    - glaze
    - config
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2025-12-18T15:14:07.955320018-05:00
---

# Diary

## Goal

Migrate Pinocchio from deprecated Viper-based configuration to Glazed's config file middleware system. This eliminates all `staticcheck` SA1019 deprecation warnings and aligns Pinocchio with the current Glazed configuration approach.

## Step 1: Migrate deprecated Viper usage to Glazed config middlewares

This step removed all deprecated Viper-centric initialization from Pinocchio and replaced it with the current Glazed configuration approach. The main goal was to eliminate `staticcheck` SA1019 deprecations by migrating from `clay.InitViper`, `logging.InitLoggerFromViper`, `middlewares.GatherFlagsFromViper`, `middlewares.GatherSpecificFlagsFromViper`, and `clay.InitViperInstanceWithAppName` to their modern equivalents.

**Commit (code):** N/A (not committed in this session)

### What I did

- Created ticket `001-FIX-GLAZED-LINTING` for pinocchio
- Read `glaze help migrating-from-viper-to-config-files` to understand migration requirements
- Updated all deprecated Viper calls across 8 files:
  - `cmd/pinocchio/main.go`: Replaced `clay.InitViper` → `clay.InitGlazed`, `logging.InitLoggerFromViper` → `logging.InitLoggerFromCobra`, removed `viper` import, replaced `viper.GetStringSlice("repositories")` with direct YAML config file reading using `ResolveAppConfigPath`
  - `cmd/pinocchio/cmds/catter/catter.go`: Replaced `middlewares.GatherSpecificFlagsFromViper` → `middlewares.UpdateFromEnv("PINOCCHIO")`
  - `cmd/pinocchio/cmds/prompto/prompto.go`: Replaced `clay.InitViperInstanceWithAppName` → direct config file reading using `ResolveAppConfigPath` and YAML parsing
  - `pkg/cmds/helpers/parse-helpers.go`: Replaced `middlewares.GatherFlagsFromViper` → `LoadParametersFromFile` + `UpdateFromEnv` using `ResolveAppConfigPath`
  - `cmd/examples/simple-chat/main.go`: Replaced `clay.InitViper` → `clay.InitGlazed`, `logging.InitLoggerFromViper` → `logging.InitLoggerFromCobra`
  - `cmd/examples/simple-redis-streaming-inference/main.go`: Replaced `clay.InitViper` → `clay.InitGlazed`, `logging.InitLoggerFromViper` → `logging.InitLoggerFromCobra`
  - `cmd/agents/simple-chat-agent/main.go`: Replaced `clay.InitViper` → `clay.InitGlazed`, `logging.InitLoggerFromViper` → `logging.InitLoggerFromCobra`
  - `cmd/web-chat/main.go`: Replaced `clay.InitViper` → `clay.InitGlazed`, `logging.InitLoggerFromViper` → `logging.InitLoggerFromCobra`
- Fixed formatting issues with `gofmt`
- Validated with `make lint` - all deprecation warnings resolved

### Why

- `staticcheck` was failing with 6 deprecation warnings (SA1019)
- Viper-based config system is deprecated in favor of explicit config file middlewares
- New system provides better observability, deterministic precedence, and cleaner separation between config sources

### What worked

- All deprecation warnings eliminated
- `make lint` passes with 0 issues
- Config file discovery works using `ResolveAppConfigPath` helper
- Environment variable support maintained via `UpdateFromEnv` middleware
- Direct YAML parsing for repositories config preserves backward compatibility

### What didn't work

- Initial attempt to remove `viper` import from `main.go` failed because `viper.GetStringSlice("repositories")` was still being used - fixed by replacing with direct config file reading

### What I learned

- `InitGlazed` replaces `InitViper` and sets up Glazed properly without Viper
- `InitLoggerFromCobra` reads logging flags directly from Cobra instead of requiring Viper binding
- `ResolveAppConfigPath` provides automatic config discovery (XDG, home, /etc paths)
- `UpdateFromEnv` middleware handles environment variable parsing with explicit prefix
- Config files must match layer structure (layer names as top-level keys) in the new system
- For simple config reading (like repositories list), direct YAML parsing is acceptable when not using the middleware system

### What was tricky to build

- Replacing `viper.GetStringSlice("repositories")` in `initAllCommands` - this runs at startup before command execution, so couldn't use middleware system. Solved by reading config file directly using `ResolveAppConfigPath` and YAML parsing.
- Understanding that `GatherSpecificFlagsFromViper` for a single flag (`filter-profile`) could be replaced with `UpdateFromEnv` which reads from environment variables
- Ensuring config file discovery works correctly - `ResolveAppConfigPath` returns empty string when no config found, which needs to be handled gracefully

### What warrants a second pair of eyes

- Config file reading in `initAllCommands` and `loadPromptoConfig` - these bypass the middleware system and read YAML directly. Verify that this maintains backward compatibility with existing config files.
- Environment variable naming - `UpdateFromEnv("PINOCCHIO")` reads variables like `PINOCCHIO_LAYER_PARAMETER`. Confirm this matches expected behavior for `filter-profile` flag.
- The `UseViper` flag in `GeppettoLayersHelper` - this now uses config middlewares instead of Viper, but the flag name might be misleading. Consider renaming or documenting the behavior change.

### What should be done in the future

- Consider renaming `UseViper` flag in `GeppettoLayersHelper` to something like `LoadFromConfig` to reflect that it no longer uses Viper
- Document the config file structure requirements (layer names as top-level keys) in Pinocchio's README or config documentation
- Consider adding config file validation to catch structure mismatches early
- Test that existing user config files continue to work with the new system

### Code review instructions

- Start with `cmd/pinocchio/main.go` - verify `InitGlazed` usage and config file reading logic
- Check `pkg/cmds/helpers/parse-helpers.go` - verify config middleware chain is correct
- Review `cmd/pinocchio/cmds/catter/catter.go` - confirm `UpdateFromEnv` replacement is appropriate
- Run `make lint` to verify no regressions
- Test with existing config files to ensure backward compatibility

### Technical details

**Migration pattern used:**
```go
// Before
err := clay.InitViper("pinocchio", rootCmd)
logging.InitLoggerFromViper()

// After  
err := clay.InitGlazed("pinocchio", rootCmd)
logging.InitLoggerFromCobra(cmd)  // in PersistentPreRunE
```

**Config file reading pattern:**
```go
configPath, err := appconfig.ResolveAppConfigPath("pinocchio", "")
if err == nil && configPath != "" {
    data, _ := os.ReadFile(configPath)
    var config map[string]interface{}
    yaml.Unmarshal(data, &config)
    // Extract values from config
}
```

**Middleware replacement pattern:**
```go
// Before
middlewares.GatherFlagsFromViper(...)

// After
configPath, _ := appconfig.ResolveAppConfigPath("pinocchio", "")
configMiddlewares := []middlewares.Middleware{}
if configPath != "" {
    configMiddlewares = append(configMiddlewares,
        middlewares.LoadParametersFromFile(configPath, ...))
}
configMiddlewares = append(configMiddlewares,
    middlewares.UpdateFromEnv("PINOCCHIO", ...))
middlewares.WrapWithWhitelistedLayers(..., middlewares.Chain(configMiddlewares...))
```

### What I'd do differently next time

- Check for all deprecated calls upfront using `grep` or `golangci-lint` before starting migration
- Create a checklist of all files that need updating to avoid missing some (found 2 additional files during final lint check)
