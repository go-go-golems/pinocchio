---
Title: Profile System Interdependency Health Inspection
Ticket: IMPROVE-PROFILES-001
Status: active
Intent: long-term
Topics:
    - profiles
    - glazed
    - pinocchio
    - geppetto
    - middleware
DocType: analysis
Intent: long-term
Owners:
    - manuel
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2025-12-15T09:07:00.50811334-05:00
---

# Profile System Interdependency Health Inspection

## Purpose

This document provides a comprehensive health inspection of how profiles are defined, loaded, and used across Glazed, Pinocchio, and Geppetto. The goal is to identify and document the interdependency issues where profile selection must happen before middleware execution, but profile values are only available after middleware execution, creating a circular dependency.

## Executive Summary

**Critical Issue**: The profile system is broken due to a timing/interdependency problem. Profile settings (`--profile`, `--profile-file`, `PINOCCHIO_PROFILE`) are read from `parsedCommandLayers` **before** middlewares execute, meaning they only contain default values. The profile middleware (`GatherFlagsFromProfiles`) is then instantiated with these default values. Even when environment variables or CLI flags later update the profile settings in `parsedLayers`, the already-instantiated middleware continues to use the captured default values, causing profiles to never load correctly.

**Root Cause**: `GetCobraCommandGeppettoMiddlewares` reads `ProfileSettings` from `parsedCommandLayers` at construction time (lines 105-109 in `geppetto/pkg/layers/layers.go`), but these layers only contain defaults. The middleware chain hasn't executed yet, so env vars and CLI flags haven't been parsed.

**Impact**: Users cannot reliably use `--profile` flags or `PINOCCHIO_PROFILE` environment variables. Profiles silently fail to load, falling back to defaults.

## Architecture Overview

### Profile System Components

1. **ProfileSettings Layer** (`glazed/pkg/cli/cli.go`):
   - Defines `profile` and `profile-file` parameters
   - Created via `NewProfileSettingsLayer()`
   - Struct: `ProfileSettings` with `Profile` and `ProfileFile` fields

2. **Profile Middleware** (`glazed/pkg/cmds/middlewares/profiles.go`):
   - `GatherFlagsFromProfiles()`: Loads profile from YAML file
   - `GatherFlagsFromCustomProfiles()`: More flexible profile loading
   - Takes profile name and file path as constructor parameters

3. **Profile Files** (`clay/pkg/cmds/profiles/`):
   - YAML structure: `profile-name -> layer-slug -> parameter -> value`
   - Default location: `~/.config/{appName}/profiles.yaml`
   - Managed via `pinocchio profiles` commands

4. **Middleware Chain** (`geppetto/pkg/layers/layers.go`):
   - `GetCobraCommandGeppettoMiddlewares()` builds the chain
   - Order: Flags → Args → Env → Config → **Profiles** → Defaults

## The Problem: Timing and Interdependency

### Current Flow (Broken)

```
1. ParseCommandSettingsLayer() runs
   └─> Creates parsedCommandLayers with DEFAULTS ONLY
       └─> profile-settings.profile = "" (default)
       └─> profile-settings.profile-file = "" (default)

2. GetCobraCommandGeppettoMiddlewares() called
   └─> Reads ProfileSettings from parsedCommandLayers
       └─> profileSettings.Profile = "" (still default!)
       └─> profileSettings.ProfileFile = "" (still default!)
   └─> Sets defaults: Profile = "default", ProfileFile = defaultProfileFile
   └─> Instantiates GatherFlagsFromProfiles("default", defaultProfileFile, "default")
       └─> Middleware CAPTURES these values at construction time

3. Middleware chain executes (in reverse order):
   └─> SetFromDefaults() - sets defaults
   └─> GatherFlagsFromProfiles("default", ...) - loads "default" profile
   └─> LoadParametersFromFiles() - loads config files
   └─> UpdateFromEnv("PINOCCHIO") - reads PINOCCHIO_PROFILE=gemini
       └─> Updates parsedLayers["profile-settings"].profile = "gemini"
   └─> GatherArguments() - parses positional args
   └─> ParseFromCobraCommand() - parses --profile flag
       └─> Updates parsedLayers["profile-settings"].profile = "gemini" (if flag set)

4. Result: parsedLayers has correct profile="gemini", but
   GatherFlagsFromProfiles already loaded "default" profile!
```

### Why It Fails

The profile middleware is **instantiated** with values that are captured at construction time. When the middleware chain executes, the middleware uses those captured values, not the values that exist in `parsedLayers` at execution time.

```go
// In GetCobraCommandGeppettoMiddlewares (geppetto/pkg/layers/layers.go:105-109)
profileSettings := &cli.ProfileSettings{}
err = parsedCommandLayers.InitializeStruct(cli.ProfileSettingsSlug, profileSettings)
// At this point, parsedCommandLayers only has DEFAULTS
// profileSettings.Profile = "" (empty string)

// Lines 232-234: Set defaults
if profileSettings.Profile == "" {
    profileSettings.Profile = "default"  // Hardcoded default!
}

// Lines 235-245: Instantiate middleware with captured values
middlewares_ = append(middlewares_,
    middlewares.GatherFlagsFromProfiles(
        defaultProfileFile,
        profileSettings.ProfileFile,  // Captured: "" or defaultProfileFile
        profileSettings.Profile,      // Captured: "default"
        ...
    ),
)
// The middleware is NOW instantiated with profile="default"
// Even if env/flag later sets profile="gemini", this middleware won't see it
```

### Evidence from Code

**File**: `geppetto/pkg/layers/layers.go:226-246`

```go
// Profile loading (NOTE: profile name is still read from defaults at construction time;
// this will be fixed in plan point 3 to read after env/config are applied)
defaultProfileFile := fmt.Sprintf("%s/pinocchio/profiles.yaml", xdgConfigPath)
if profileSettings.ProfileFile == "" {
    profileSettings.ProfileFile = defaultProfileFile
}
if profileSettings.Profile == "" {
    profileSettings.Profile = "default"  // ← Hardcoded default!
}
middlewares_ = append(middlewares_,
    middlewares.GatherFlagsFromProfiles(
        defaultProfileFile,
        profileSettings.ProfileFile,  // ← Captured at construction time
        profileSettings.Profile,      // ← Captured at construction time
        ...
    ),
)
```

The comment on line 226-227 explicitly acknowledges this is a known issue!

## Key Files and Symbols

### Profile Definition and Loading

**File**: `glazed/pkg/cli/cli.go`
- **Types**: `ProfileSettings` struct (lines 45-48)
- **Constants**: `ProfileSettingsSlug = "profile-settings"` (line 50)
- **Functions**: `NewProfileSettingsLayer()` (lines 52-74)
- **Key Fields**: `Profile string`, `ProfileFile string`

**File**: `glazed/pkg/cmds/middlewares/profiles.go`
- **Functions**: 
  - `GatherFlagsFromProfiles()` (lines 13-67): Main profile loading middleware
  - `GatherFlagsFromCustomProfiles()` (lines 92-138): Flexible profile loading
  - `loadProfileFromFile()` (lines 198-228): YAML parsing
  - `resolveProfileFilePath()` (lines 180-196): Path resolution
- **Types**: `ProfileConfig` (lines 140-147), `ProfileOption` (line 150)

**File**: `geppetto/pkg/layers/layers.go`
- **Function**: `GetCobraCommandGeppettoMiddlewares()` (lines 94-254)
- **Problem Location**: Lines 105-109 (reading ProfileSettings), Lines 226-246 (instantiating middleware)
- **Key Issue**: Profile settings read before middleware execution

**File**: `glazed/pkg/cli/cobra-parser.go`
- **Function**: `ParseCommandSettingsLayer()` (lines 305-344)
- **Purpose**: Pre-parses command-settings and profile-settings layers from Cobra flags
- **Limitation**: Only parses from Cobra flags, not env/config

**File**: `clay/pkg/cmds/profiles/cmds.go`
- **Function**: `NewProfilesCommand()` (lines 19-56)
- **Purpose**: CLI commands for managing profiles (`list`, `get`, `set`, `delete`, `edit`, `init`, `duplicate`)
- **Helper**: `GetProfilesPathForApp()` resolves default profile file path

**File**: `clay/pkg/cmds/profiles/paths.go`
- **Function**: `GetProfilesPathForApp()` (lines 11-26)
- **Purpose**: Returns `~/.config/{appName}/profiles.yaml`

### Profile Usage in Pinocchio

**File**: `pinocchio/cmd/pinocchio/main.go`
- **Lines 240-246**: Adds profiles command via `clay_profiles.NewProfilesCommand()`
- **Lines 284-326**: `pinocchioInitialProfilesContent()` provides default profile template

**File**: `pinocchio/pkg/cmds/helpers/parse-helpers.go`
- **Function**: `ParseGeppettoLayers()` (lines 47-102)
- **Issue**: Uses deprecated `GatherFlagsFromViper` and reads profile before env/config
- **Lines 62-74**: Manually constructs profile middleware with hardcoded values

### Previous Analysis

**File**: `pinocchio/ttmp/2025-11-18/PIN-20251118/analysis/01-config-and-profile-migration-analysis.md`
- **TL;DR** (line 19): Documents the exact same issue
- **Root Cause** (lines 54-76): Explains the timing problem
- **Fix Direction** (lines 78-82): Proposes solutions

**File**: `pinocchio/ttmp/2025-11-18/PIN-20251118/design/01-profile-loading-plan.md`
- **Plan**: Proposes `resolveProfileSettings` helper with mini middleware chain
- **Approach**: Pre-parse profile-settings layer before building full middleware chain

## How Profiles Are Supposed to Work

### Intended Flow

1. User sets profile via:
   - CLI flag: `--profile gemini`
   - Environment variable: `PINOCCHIO_PROFILE=gemini`
   - Config file: `profile: gemini` in `~/.config/pinocchio/config.yaml`

2. Profile selection should be resolved with proper precedence:
   - CLI flags (highest)
   - Environment variables
   - Config files
   - Defaults (lowest)

3. Profile middleware should load the selected profile:
   - Read `profiles.yaml` file
   - Extract profile map for selected profile
   - Merge profile values into parsedLayers

4. Profile values should override defaults but be overridden by CLI flags

### Current Precedence (Broken)

The middleware chain order is correct:
1. Flags (highest priority)
2. Arguments
3. Environment variables
4. Config files
5. **Profiles** ← Should load based on resolved profile name
6. Defaults (lowest priority)

But profiles are loaded with the wrong profile name because it's captured before env/config/flags are parsed!

## Workarounds and Past Solutions

### Workaround 1: Direct Cobra Flag Lookup

**Location**: Mentioned in user query - "looking up the profiles flags or so directly from cobra"

**Approach**: Read `--profile` and `--profile-file` flags directly from Cobra command before building middleware chain

**Limitation**: Doesn't handle environment variables or config files

**Example**:
```go
// Hypothetical workaround
profileFlag := cmd.Flag("profile")
if profileFlag != nil && profileFlag.Changed {
    profileName = profileFlag.Value.String()
} else {
    // Check env var manually
    profileName = os.Getenv("PINOCCHIO_PROFILE")
}
```

### Workaround 2: Two-Phase Execution

**Location**: `pinocchio/ttmp/2025-11-18/PIN-20251118/analysis/02-profile-preparse-options.md`

**Approach**: Run a mini middleware chain first to resolve profile-settings, then build the full chain

**Status**: Proposed but not implemented

**Code Sketch**:
```go
// Phase 1: Resolve profile settings
seedLayers := layers.NewParameterLayers(
    layers.WithLayers(commandSettingsLayer, profileSettingsLayer),
)
seedParsed := layers.NewParsedLayers()
miniChain := []middlewares.Middleware{
    middlewares.ParseFromCobraCommand(cmd),
    middlewares.UpdateFromEnv("PINOCCHIO"),
    middlewares.LoadParametersFromFiles(configFiles),
    middlewares.SetFromDefaults(),
}
middlewares.ExecuteMiddlewares(seedLayers, seedParsed, miniChain...)

// Phase 2: Use resolved profile settings
profileSettings := &cli.ProfileSettings{}
seedParsed.InitializeStruct(cli.ProfileSettingsSlug, profileSettings)
// Now profileSettings.Profile has the correct value!
```

## Proposed Solutions

### Solution 1: Mini Middleware Chain (Recommended)

**Approach**: Pre-parse profile-settings layer before building full middleware chain

**Pros**:
- Self-contained, no Glazed changes needed
- Deterministic and easy to test
- Reuses existing middleware infrastructure

**Cons**:
- Slight duplication (mini chain re-parses CLI/env for two layers)
- Requires helper function

**Implementation**: Create `resolveProfileSettings()` helper that:
1. Creates temporary layers with only `command-settings` and `profile-settings`
2. Executes mini middleware chain: config → env → flags → args → defaults
3. Extracts resolved `ProfileSettings` struct
4. Returns resolved values for use in full middleware chain

**Location**: `geppetto/pkg/layers/layers.go` (new function)

### Solution 2: Dynamic Middleware

**Approach**: Wrap profile loader in middleware that reads profile-settings at execution time

**Pros**:
- Single pass, no reordering
- No duplication

**Cons**:
- Requires "dynamic middleware" helper or closure per call site
- More complex implementation

**Implementation**: Create middleware wrapper that:
1. Calls `next()` first
2. Reads `profile-settings` from `parsedLayers` after previous middlewares ran
3. Dynamically loads profile based on resolved values
4. Merges profile map into parsedLayers

### Solution 3: Config Overlay

**Approach**: Treat profiles as config overlays via `ConfigFilesFunc`

**Pros**:
- Simplifies precedence to standard config semantics
- Profiles become just another config source

**Cons**:
- Requires mapper + changes to `ConfigFilesFunc`
- Need to peek at `profile-settings` in resolver

**Implementation**: Modify `ConfigFilesFunc` to:
1. Read `profile-settings` from `parsedLayers`
2. Include `profiles.yaml` in config file list
3. Apply mapper that extracts only selected profile
4. Load via standard `LoadParametersFromFiles`

## Impact Analysis

### Affected Components

1. **Geppetto** (`geppetto/pkg/layers/layers.go`):
   - `GetCobraCommandGeppettoMiddlewares()` must be refactored
   - All commands using Geppetto layers are affected

2. **Pinocchio** (`pinocchio/cmd/pinocchio/main.go`):
   - Commands using `GetCobraCommandGeppettoMiddlewares` are affected
   - Profile CLI commands (`pinocchio profiles`) work correctly (they don't use middleware)

3. **Helper Functions** (`pinocchio/pkg/cmds/helpers/parse-helpers.go`):
   - `ParseGeppettoLayers()` needs similar fix
   - Used by examples and test code

4. **Examples** (`pinocchio/cmd/examples/*`):
   - All examples using Geppetto layers are affected
   - May need updates to use fixed helper

### User Impact

**Current State**: Profiles don't work reliably
- `--profile` flag may be ignored
- `PINOCCHIO_PROFILE` env var may be ignored
- Users fall back to defaults without knowing

**After Fix**: Profiles work correctly
- CLI flags override env vars
- Env vars override config files
- Config files override defaults
- Proper precedence chain

## Testing Requirements

### Test Cases Needed

1. **Default Profile**:
   - No profile specified → loads "default" profile
   - "default" profile doesn't exist → no error, uses defaults

2. **Environment Variable**:
   - `PINOCCHIO_PROFILE=gemini` → loads "gemini" profile
   - `PINOCCHIO_PROFILE_FILE=/custom/path.yaml` → uses custom file

3. **CLI Flag**:
   - `--profile gemini` → loads "gemini" profile
   - `--profile-file /custom/path.yaml` → uses custom file

4. **Precedence**:
   - `PINOCCHIO_PROFILE=gemini --profile claude` → loads "claude" (flag wins)
   - Config file sets `profile: gemini`, env sets `PINOCCHIO_PROFILE=claude` → loads "claude" (env wins)

5. **Error Cases**:
   - Profile doesn't exist → error (unless "default")
   - Profile file doesn't exist → error (unless default file)
   - Invalid YAML → error

### Regression Tests

- Ensure existing profile functionality still works
- Test that profile values override defaults
- Test that CLI flags override profile values
- Test that multiple profiles can be loaded sequentially

## Related Issues and Context

### Previous Work

- **Ticket PIN-20251118**: Config and profile migration analysis
- **Analysis**: `pinocchio/ttmp/2025-11-18/PIN-20251118/analysis/01-config-and-profile-migration-analysis.md`
- **Design**: `pinocchio/ttmp/2025-11-18/PIN-20251118/design/01-profile-loading-plan.md`
- **Status**: Analysis complete, plan proposed, implementation pending

### Dependencies

- Depends on Glazed middleware system
- Depends on `ParseCommandSettingsLayer` for pre-parsing
- May require changes to `CobraParserConfig` if using Solution 3

## Next Steps

1. **Choose Solution**: Decide between mini middleware chain, dynamic middleware, or config overlay
2. **Implement Helper**: Create `resolveProfileSettings()` or equivalent
3. **Refactor Middleware Chain**: Update `GetCobraCommandGeppettoMiddlewares()`
4. **Update Helpers**: Fix `ParseGeppettoLayers()` and other entry points
5. **Add Tests**: Comprehensive test coverage for all scenarios
6. **Update Documentation**: Document new behavior and precedence

## References

- `glazed/pkg/cmds/middlewares/profiles.go` - Profile middleware implementation
- `glazed/pkg/cli/cli.go` - ProfileSettings layer definition
- `geppetto/pkg/layers/layers.go` - Middleware chain builder (problem location)
- `clay/pkg/cmds/profiles/` - Profile management CLI commands
- `pinocchio/ttmp/2025-11-18/PIN-20251118/` - Previous analysis and design work
