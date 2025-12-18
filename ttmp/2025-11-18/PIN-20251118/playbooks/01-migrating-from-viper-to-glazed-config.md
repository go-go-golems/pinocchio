---
Title: Playbook - Migrating from Viper to Glazed Config System
Ticket: PIN-20251118
Status: complete
Topics:
  - pinocchio
  - glazed
  - migration
  - configuration
  - viper
DocType: playbook
Intent: long-term
Owners:
  - manuel
LastUpdated: 2025-11-18
---

# Playbook: Migrating from Viper to Glazed Config System

This playbook provides a step-by-step guide for migrating Glazed-based applications from the deprecated Viper configuration system to the new Glazed middleware-based config system. It is based on the actual migration of Pinocchio (PIN-20251118).

## Prerequisites

- Application using Glazed framework
- Currently using `clay.InitViper` or `logging.InitLoggerFromViper`
- Config files in flat format (top-level keys)
- Understanding of Go middleware patterns

## Overview

The migration involves:
1. Replacing Viper initialization with Glazed initialization
2. Updating logging initialization
3. Converting config file format from flat to layered structure
4. Replacing Viper-based middleware with env/config middlewares
5. Handling special cases (non-layer keys, flag reading order)

## Step-by-Step Migration

### Step 1: Replace Viper Initialization

**Before:**
```go
import (
    clay "github.com/go-go-golems/clay/pkg"
    "github.com/go-go-golems/glazed/pkg/cmds/logging"
)

func initRootCmd() error {
    err := clay.InitViper("myapp", rootCmd)
    if err != nil {
        return err
    }
    return nil
}
```

**After:**
```go
import (
    clay "github.com/go-go-golems/clay/pkg"
    "github.com/go-go-golems/glazed/pkg/cmds/logging"
)

func initRootCmd() error {
    err := clay.InitGlazed("myapp", rootCmd)
    if err != nil {
        return err
    }
    return nil
}
```

**Changes:**
- Replace `clay.InitViper("myapp", rootCmd)` with `clay.InitGlazed("myapp", rootCmd)`
- `InitGlazed` only adds logging flags, doesn't wire Viper

### Step 2: Update Logging Initialization

**Before:**
```go
var rootCmd = &cobra.Command{
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        err := logging.InitLoggerFromViper()
        if err != nil {
            return err
        }
        return nil
    },
}
```

**After:**
```go
var rootCmd = &cobra.Command{
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        err := logging.InitLoggerFromCobra(cmd)
        if err != nil {
            return err
        }
        return nil
    },
}
```

**Changes:**
- Replace `logging.InitLoggerFromViper()` with `logging.InitLoggerFromCobra(cmd)`
- Pass the `cmd` parameter so it can read flags directly

### Step 3: Replace Direct Viper Reads

If you're reading config values directly from Viper (e.g., during initialization), replace with direct config file reading.

**Before:**
```go
import "github.com/spf13/viper"

func initAllCommands() error {
    repositoryPaths := viper.GetStringSlice("repositories")
    // ...
}
```

**After:**
```go
import (
    "os"
    "gopkg.in/yaml.v3"
    glazedConfig "github.com/go-go-golems/glazed/pkg/config"
)

func loadRepositoriesFromConfig() []string {
    configPath, err := glazedConfig.ResolveAppConfigPath("myapp", "")
    if err != nil || configPath == "" {
        return []string{}
    }

    data, err := os.ReadFile(configPath)
    if err != nil {
        return []string{}
    }

    var config map[string]interface{}
    if err := yaml.Unmarshal(data, &config); err != nil {
        return []string{}
    }

    repos, ok := config["repositories"].([]interface{})
    if !ok {
        return []string{}
    }

    repositoryPaths := make([]string, 0, len(repos))
    for _, repo := range repos {
        if repoStr, ok := repo.(string); ok {
            repositoryPaths = append(repositoryPaths, repoStr)
        }
    }

    return repositoryPaths
}

func initAllCommands() error {
    repositoryPaths := loadRepositoriesFromConfig()
    // ...
}
```

**Note:** This is only needed for values read during initialization, not through the middleware chain.

### Step 4: Update Middleware Chain

Replace `GatherFlagsFromViper` with `UpdateFromEnv` and `LoadParametersFromFiles`.

**Before:**
```go
import (
    "github.com/go-go-golems/glazed/pkg/cmds/middlewares"
    "github.com/go-go-golems/glazed/pkg/cmds/parameters"
)

func GetMiddlewares(
    parsedCommandLayers *layers.ParsedLayers,
    cmd *cobra.Command,
    args []string,
) ([]middlewares.Middleware, error) {
    middlewares_ := []middlewares.Middleware{
        middlewares.ParseFromCobraCommand(cmd,
            parameters.WithParseStepSource("cobra"),
        ),
        middlewares.GatherArguments(args,
            parameters.WithParseStepSource("arguments"),
        ),
        middlewares.WrapWithWhitelistedLayers(
            []string{"layer1", "layer2"},
            middlewares.GatherFlagsFromViper(
                parameters.WithParseStepSource("viper"),
            ),
        ),
        middlewares.SetFromDefaults(
            parameters.WithParseStepSource("defaults"),
        ),
    }
    return middlewares_, nil
}
```

**After:**
```go
import (
    "github.com/go-go-golems/glazed/pkg/cmds/middlewares"
    "github.com/go-go-golems/glazed/pkg/cmds/parameters"
    glazedConfig "github.com/go-go-golems/glazed/pkg/config"
    "github.com/go-go-golems/glazed/pkg/cli"
)

func GetMiddlewares(
    parsedCommandLayers *layers.ParsedLayers,
    cmd *cobra.Command,
    args []string,
) ([]middlewares.Middleware, error) {
    // Read command settings from parsedCommandLayers (already parsed)
    commandSettings := &cli.CommandSettings{}
    err := parsedCommandLayers.InitializeStruct(cli.CommandSettingsSlug, commandSettings)
    if err != nil {
        return nil, err
    }

    middlewares_ := []middlewares.Middleware{
        // Highest precedence: command-line flags
        middlewares.ParseFromCobraCommand(cmd,
            parameters.WithParseStepSource("cobra"),
        ),
        // Positional arguments
        middlewares.GatherArguments(args,
            parameters.WithParseStepSource("arguments"),
        ),
        // Environment variables (MYAPP_*)
        middlewares.UpdateFromEnv("MYAPP",
            parameters.WithParseStepSource("env"),
        ),
    }

    // Config files resolver
    configFilesResolver := func(_ *layers.ParsedLayers, _ *cobra.Command, _ []string) ([]string, error) {
        var files []string
        
        // Load default config first (low precedence)
        configPath, err := glazedConfig.ResolveAppConfigPath("myapp", "")
        if err == nil && configPath != "" {
            files = append(files, configPath)
        }
        
        // Explicit config file (high precedence)
        if commandSettings.ConfigFile != "" {
            files = append(files, commandSettings.ConfigFile)
        }
        
        return files, nil
    }

    // Load config files with mapper if needed
    middlewares_ = append(middlewares_,
        func(next middlewares.HandlerFunc) middlewares.HandlerFunc {
            return func(layers_ *layers.ParameterLayers, parsedLayers *layers.ParsedLayers) error {
                if err := next(layers_, parsedLayers); err != nil {
                    return err
                }
                files, err := configFilesResolver(parsedLayers, cmd, args)
                if err != nil {
                    return err
                }
                return middlewares.LoadParametersFromFiles(files,
                    middlewares.WithParseOptions(
                        parameters.WithParseStepSource("config"),
                    ),
                )(func(_ *layers.ParameterLayers, _ *layers.ParsedLayers) error { return nil })(layers_, parsedLayers)
            }
        },
    )

    // Lowest precedence: defaults
    middlewares_ = append(middlewares_,
        middlewares.SetFromDefaults(
            parameters.WithParseStepSource("defaults"),
        ),
    )

    return middlewares_, nil
}
```

**Key Points:**
- Use `parsedCommandLayers` (function parameter) to read command settings, not `parsedLayers` from middleware
- Environment prefix should match your app name (uppercase)
- Config files are loaded in order: default first (low precedence), explicit last (high precedence)
- Middleware execution order: flags → args → env → config → defaults

### Step 5: Convert Config File Format

Convert your config file from flat format to layered format.

**Before (flat format):**
```yaml
# ~/.myapp/config.yaml
api-key: sk-1234567890
threshold: 42
log-level: debug
repositories:
  - /path/to/repo1
  - /path/to/repo2
```

**After (layered format):**
```yaml
# ~/.myapp/config.yaml
# Layer names as top-level keys
my-layer:
  api-key: sk-1234567890
  threshold: 42

logging:
  log-level: debug

# Special keys not part of layers (if needed)
repositories:
  - /path/to/repo1
  - /path/to/repo2
```

**Mapping Guide:**
1. Identify which layer each parameter belongs to
2. Group parameters under their layer slug
3. Keep non-layer keys (like `repositories`) at top level if they're handled separately
4. Use a config mapper to filter out non-layer keys (see Step 6)

### Step 6: Handle Non-Layer Keys

If your config file has keys that aren't part of any layer (e.g., `repositories`), create a mapper to exclude them.

**Add Config Mapper:**
```go
// Mapper to filter out non-layer keys
configMapper := func(rawConfig interface{}) (map[string]map[string]interface{}, error) {
    configMap, ok := rawConfig.(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("expected map[string]interface{}, got %T", rawConfig)
    }
    
    result := make(map[string]map[string]interface{})
    
    // Keys to exclude from layer parsing (handled separately)
    excludedKeys := map[string]bool{
        "repositories": true,
        // Add other non-layer keys here
    }
    
    for key, value := range configMap {
        if excludedKeys[key] {
            continue // Skip excluded keys
        }
        
        // If the value is a map, treat the key as a layer slug
        if layerParams, ok := value.(map[string]interface{}); ok {
            result[key] = layerParams
        }
    }
    
    return result, nil
}

// Use mapper in LoadParametersFromFiles
middlewares.LoadParametersFromFiles(files,
    middlewares.WithConfigFileMapper(configMapper),
    middlewares.WithParseOptions(
        parameters.WithParseStepSource("config"),
    ),
)
```

### Step 7: Test the Migration

1. **Backup your config file:**
   ```bash
   cp ~/.myapp/config.yaml ~/.myapp/config.yaml.backup
   ```

2. **Convert config file** to layered format (see Step 5)

3. **Test basic functionality:**
   ```bash
   myapp --help
   myapp <command> --print-parsed-parameters
   ```

4. **Test config file loading:**
   ```bash
   # Test default config
   myapp <command> --print-parsed-parameters
   
   # Test explicit config file
   myapp <command> --config-file /path/to/test-config.yaml --print-parsed-parameters
   ```

5. **Test environment variables:**
   ```bash
   MYAPP_MY_LAYER_API_KEY=test-key myapp <command> --print-parsed-parameters
   ```

6. **Verify precedence:**
   - Default config values should be overridden by explicit config
   - Config values should be overridden by environment variables
   - Environment variables should be overridden by command-line flags

## Common Issues and Solutions

### Issue 1: Config File Not Found

**Symptom:** Config file isn't being loaded

**Solution:** Check that `ResolveAppConfigPath` is finding your config file. It searches:
1. `$XDG_CONFIG_HOME/myapp/config.yaml`
2. `$HOME/.myapp/config.yaml`
3. `/etc/myapp/config.yaml`

### Issue 2: Flags Not Available in Resolver

**Symptom:** `--config-file` flag value is empty in resolver

**Solution:** Use `parsedCommandLayers` (function parameter) instead of `parsedLayers` (middleware parameter). Command settings are parsed before the middleware chain runs.

### Issue 3: Config File Format Error

**Symptom:** `expected map[string]interface{} for layer X, got string`

**Solution:** Convert config file to layered format. Flat keys need to be under layer names.

### Issue 4: Non-Layer Keys Causing Errors

**Symptom:** Error parsing keys like `repositories` as layers

**Solution:** Add a config mapper to exclude non-layer keys from parsing.

### Issue 5: Wrong Precedence Order

**Symptom:** Values from lower precedence sources override higher precedence

**Solution:** Ensure middleware order is: flags → args → env → config → defaults. Config files should be loaded in order: default first, explicit last.

## Verification Checklist

- [ ] Replaced `clay.InitViper` with `clay.InitGlazed`
- [ ] Replaced `logging.InitLoggerFromViper` with `logging.InitLoggerFromCobra`
- [ ] Replaced `GatherFlagsFromViper` with `UpdateFromEnv` and `LoadParametersFromFiles`
- [ ] Converted config file to layered format
- [ ] Added config mapper for non-layer keys (if needed)
- [ ] Resolver uses `parsedCommandLayers` not `parsedLayers`
- [ ] Config files loaded in correct precedence order
- [ ] Tested default config loading
- [ ] Tested explicit config file (`--config-file`)
- [ ] Tested environment variables
- [ ] Tested command-line flags
- [ ] Verified precedence order (defaults < config < env < flags)
- [ ] Removed all Viper imports (except for special cases)
- [ ] Build succeeds
- [ ] All tests pass

## Migration Example: Pinocchio

See `pinocchio/ttmp/2025-11-18/PIN-20251118/various/02-implementation-diary-config-migration.md` for a detailed account of the Pinocchio migration, including:
- What was tried
- What failed
- What worked
- Lessons learned
- What would be done differently

## Additional Resources

- **Glazed Migration Tutorial**: `glazed/pkg/doc/tutorials/migrating-from-viper-to-config-files.md`
- **Config Files Topic**: `glazed/pkg/doc/topics/24-config-files.md`
- **Middlewares Topic**: `glazed/pkg/doc/topics/21-cmds-middlewares.md`
- **Layers Topic**: `glazed/pkg/doc/topics/13-layers-and-parsed-layers.md`

## Summary

The migration from Viper to Glazed config system involves:
1. Replacing initialization functions
2. Updating middleware chains
3. Converting config file format
4. Handling special cases

The key insight is that **middleware execution order matters** - flags are parsed after config files in the chain, so use `parsedCommandLayers` (pre-parsed) instead of `parsedLayers` (from middleware) when reading command settings in resolvers.

