---
Title: Local Profile Loading - Code Analysis and Design Options
Ticket: PI-LOCAL-PROFILES
Status: active
Topics:
    - pinocchio
    - profiles
    - config
    - geppetto
    - glazed
    - local-config
    - git
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/config.go
      Note: Geppetto bootstrap config structure
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/profile_selection.go
      Note: Geppetto profile resolution - integrates with local config
    - Path: ../../../../../../../glazed/pkg/config/resolve.go
      Note: Glazed config resolution - needs local config extension
    - Path: pkg/cmds/cmd.go
      Note: Pinocchio command execution - uses resolved profiles
    - Path: pkg/cmds/profilebootstrap/profile_selection.go
      Note: Pinocchio bootstrap wrapper - enable local config here
    - Path: pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md
      Note: Profile resolution documentation - needs updating for local profiles
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---


# Local Profile Loading - Code Analysis and Design Options

## Goal

Add support for loading pinocchio profiles from local directory sources:
1. `.pinocchio-profile.yml` in the current working directory (PWD)
2. `.pinocchio-profile.yml` in the root of the git repository (if in a git repo)

This enables per-project profile configurations that travel with the codebase.

---

## Current Architecture Map

### Profile Resolution Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Profile Resolution Chain                             │
└─────────────────────────────────────────────────────────────────────────────┘

User Input
    │
    ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│  CLI Flags              │────▶│  --profile              │
│  --config-file          │     │  --profile-registries   │
└─────────────────────────┘     └─────────────────────────┘
                                        │
    ┌───────────────────────────────────┼───────────────────────────────────┐
    │                                   ▼                                   │
    │  ┌─────────────────────────────────────────────────────────────┐   │
    │  │  profilebootstrap.ResolveCLIConfigFiles()                    │   │
    │  │  └── bootstrap.ResolveCLIConfigFiles()                      │   │
    │  │       └── appconfig.ResolveAppConfigPath()                  │   │
    │  │            ├── $XDG_CONFIG_HOME/<app>/config.yaml            │   │
    │  │            ├── $HOME/.<app>/config.yaml                      │   │
    │  │            └── /etc/<app>/config.yaml                       │   │
    │  └─────────────────────────────────────────────────────────────┘   │
    │                                                                   │
    ▼                                                                   ▼
┌─────────────────────────┐                              ┌─────────────────────────┐
│  Config File Loading    │                              │  Profile Selection      │
│  (via sources.FromFiles)│                              │  (via geppetto registry)│
└─────────────────────────┘                              └─────────────────────────┘
            │                                                       │
            ▼                                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Final Resolution                                     │
│  BaseInferenceSettings + ResolvedProfileOverlay = FinalInferenceSettings   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Files and Their Roles

| File | Role | Purpose |
|------|------|---------|
| `geppetto/pkg/cli/bootstrap/profile_selection.go` | Bootstrap entry | Resolves CLI profile selection and config files |
| `geppetto/pkg/cli/bootstrap/config.go` | Config structure | Defines AppBootstrapConfig with app name, env prefix, mappers |
| `glazed/pkg/config/resolve.go` | Config discovery | `ResolveAppConfigPath()` - XDG/home/etc resolution |
| `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go` | Pinocchio wrapper | Wraps geppetto bootstrap with pinocchio-specific config |
| `pinocchio/pkg/cmds/cmd.go` | Command execution | Uses resolved settings to run commands |
| `pinocchio/pkg/cmds/profilebootstrap/engine_settings.go` | Engine settings | Resolves engine-specific settings from base + profile |
| `pinocchio/pkg/ui/profileswitch/manager.go` | Runtime switching | Preserves base, switches profiles at runtime |

### Config Resolution Order (Current)

```yaml
1. $XDG_CONFIG_HOME/pinocchio/config.yaml
2. $HOME/.pinocchio/config.yaml
3. /etc/pinocchio/config.yaml
4. Explicit --config-file path
```

### Profile Selection Precedence (Current)

```yaml
1. --profile flag
2. $PINOCCHIO_PROFILE env var
3. profile-settings.profile in config file
4. Registry default profile
```

---

## Proposed Local Profile Sources

### Option 1: Local File Discovery Pattern

```yaml
# New resolution order (proposal)
1. .pinocchio-profile.yml (in PWD)
2. .pinocchio-profile.yml (in git root, if different from PWD)
3. $XDG_CONFIG_HOME/pinocchio/config.yaml
4. $HOME/.pinocchio/config.yaml
5. /etc/pinocchio/config.yaml
6. Explicit --config-file path
```

### File Format

```yaml
# .pinocchio-profile.yml
# This is NOT a full config file - it's a profile overlay
profile: local-dev
profile-registries:
  - ./local-profiles.yaml
  
# OR inline profile definition (if we want to support it)
profile-slug: local-dev
inference_settings:
  chat:
    engine: gpt-4o-mini
    api_type: openai
  client:
    base_url: http://localhost:8080
```

---

## Design Options

### Option A: Extend Glazed Config Resolution (Recommended)

Add local directory config discovery to `glazed/pkg/config/resolve.go`:

```go
// In glazed/pkg/config/resolve.go

func ResolveAppConfigPathWithLocal(appName string, explicit string) ([]string, error) {
    files := make([]string, 0)
    
    // 1. Local directory config
    if localFile := findLocalConfig(appName); localFile != "" {
        files = append(files, localFile)
    }
    
    // 2. Git root config (if in git repo and different from local)
    if gitRootFile := findGitRootConfig(appName); gitRootFile != "" {
        // Only add if different from local
        if gitRootFile != localFile {
            files = append(files, gitRootFile)
        }
    }
    
    // 3. Standard XDG/home/etc configs
    if standard := ResolveAppConfigPath(appName, explicit); standard != "" {
        files = append(files, standard)
    }
    
    return files, nil
}

func findLocalConfig(appName string) string {
    pwd, _ := os.Getwd()
    candidates := []string{
        filepath.Join(pwd, fmt.Sprintf(".%s.yaml", appName)),
        filepath.Join(pwd, fmt.Sprintf(".%s.yml", appName)),
        filepath.Join(pwd, fmt.Sprintf(".%s-profile.yaml", appName)),
        filepath.Join(pwd, fmt.Sprintf(".%s-profile.yml", appName)),
    }
    for _, c := range candidates {
        if fileExists(c) {
            return c
        }
    }
    return ""
}

func findGitRootConfig(appName string) string {
    gitRoot, err := findGitRoot()
    if err != nil {
        return ""
    }
    pwd, _ := os.Getwd()
    if gitRoot == pwd {
        return "" // Already checked in local
    }
    
    candidates := []string{
        filepath.Join(gitRoot, fmt.Sprintf(".%s.yaml", appName)),
        filepath.Join(gitRoot, fmt.Sprintf(".%s.yml", appName)),
        filepath.Join(gitRoot, fmt.Sprintf(".%s-profile.yaml", appName)),
        filepath.Join(gitRoot, fmt.Sprintf(".%s-profile.yml", appName)),
    }
    for _, c := range candidates {
        if fileExists(c) {
            return c
        }
    }
    return ""
}

func findGitRoot() (string, error) {
    // Use git rev-parse --show-toplevel
    cmd := exec.Command("git", "rev-parse", "--show-toplevel")
    out, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(out)), nil
}
```

**Pros:**
- Generic solution for all go-go-golems apps
- Follows existing config resolution patterns
- Minimal changes to pinocchio

**Cons:**
- Adds git dependency to glazed
- Changes behavior for all glazed apps

### Option B: Geppetto Profile-Specific Extension

Add local profile discovery to `geppetto/pkg/cli/bootstrap/profile_selection.go`:

```go
// In geppetto/pkg/cli/bootstrap/profile_selection.go

func ResolveCLIConfigFilesWithLocal(cfg AppBootstrapConfig, parsed *values.Values) ([]string, error) {
    files, err := ResolveCLIConfigFiles(cfg, parsed)
    if err != nil {
        return nil, err
    }
    
    // Prepend local configs (highest precedence after explicit)
    localFiles, err := findLocalProfileFiles(cfg.normalizedAppName())
    if err != nil {
        return nil, err
    }
    
    // Insert local files at the beginning (they get merged first, lower precedence)
    // OR append them (they get merged last, higher precedence)
    // Need to decide on precedence order
    return append(localFiles, files...), nil
}
```

**Pros:**
- Profile-specific, doesn't affect other config
- Can have profile-specific file names (`.pinocchio-profile.yml`)

**Cons:**
- Only solves for geppetto-based apps
- Duplicates some config resolution logic

### Option C: Pinocchio-Specific Implementation

Add local profile discovery only in pinocchio:

```go
// In pinocchio/pkg/cmds/profilebootstrap/

func ResolveCLIConfigFilesWithLocal(parsed *values.Values) ([]string, error) {
    // Get standard files from geppetto
    files, err := ResolveCLIConfigFiles(parsed)
    if err != nil {
        return nil, err
    }
    
    // Find local pinocchio profile files
    localFiles := findPinocchioLocalProfiles()
    
    // Prepend local files for higher precedence
    return append(localFiles, files...), nil
}
```

**Pros:**
- Fastest to implement
- Full control over behavior
- No upstream dependencies

**Cons:**
- Code duplication
- Other go-go-golems apps don't benefit
- Technical debt

---

## Recommended Approach: Option A (Glazed Extension)

### Why Glazed?

1. **Generic utility**: Local directory config is useful beyond just profiles
2. **Existing patterns**: Glazed already handles config resolution
3. **File naming**: Can support both generic (`.pinocchio.yaml`) and profile-specific (`.pinocchio-profile.yaml`)
4. **Precedent**: `glazed/pkg/cmds/loaders/loaders.go` already traverses directories

### Implementation Plan

#### Phase 1: Glazed Core Changes

```
glazed/pkg/config/resolve.go
├── Add ResolveAppConfigPathWithLocal(appName, explicit string, options ...ResolveOption) ([]string, error)
├── Add ResolveOption for configuring behavior
│   ├── WithLocalDirectory(bool)      // Enable PWD lookup
│   ├── WithGitRoot(bool)             // Enable git root lookup
│   ├── WithLocalFileName(string)     // Custom filename
│   └── WithLocalFileNames([]string)  // Multiple candidates
└── Add Git root detection (exec or go-git library)
```

#### Phase 2: Geppetto Integration

```
geppetto/pkg/cli/bootstrap/profile_selection.go
├── Add AppBootstrapConfig.LocalConfigMode enum
│   ├── LocalConfigDisabled
│   ├── LocalConfigPWDOnly
│   ├── LocalConfigGitRootOnly
│   └── LocalConfigBoth
└── Modify ResolveCLIConfigFiles to check LocalConfigMode
```

#### Phase 3: Pinocchio Wiring

```
pinocchio/pkg/cmds/profilebootstrap/profile_selection.go
├── Update pinocchioBootstrapConfig() to enable local config
│   return bootstrap.AppBootstrapConfig{
│       AppName:          "pinocchio",
│       EnvPrefix:        "PINOCCHIO",
│       ConfigFileMapper: configFileMapper,
│       LocalConfigMode:  bootstrap.LocalConfigBoth,  // NEW
│       ...
│   }
└── No other changes needed (transparent upgrade)
```

---

## Precedence and Merge Order

### Critical Decision: When Should Local Config Apply?

**Option 1: Local config as BASE (merged first)**
```
Merge order:
1. Local config (PWD or git root)
2. XDG config
3. Home config
4. System config
5. Explicit --config-file

Result: Later configs OVERRIDE local config
Use case: System admin sets defaults, user can override
```

**Option 2: Local config as OVERRIDE (merged last, before explicit)**
```
Merge order:
1. System config
2. XDG config
3. Home config
4. Local config (PWD or git root)
5. Explicit --config-file

Result: Local config OVERRIDES user/system configs
Use case: Project-specific settings take precedence
```

**Recommendation: Option 2 (Local as OVERRIDE)**

Rationale: The use case is "per-project profiles". The project config should win over the user's global config, similar to how `.env` files work.

### Precedence Table

| Source | Precedence | Use Case |
|--------|------------|----------|
| `--config-file` explicit | 1 (Highest) | Debug/override everything |
| `.pinocchio-profile.yml` (PWD) | 2 | Project-specific settings |
| `.pinocchio-profile.yml` (git root) | 3 | Repo-wide settings |
| `$XDG_CONFIG_HOME/pinocchio/config.yaml` | 4 | User preferences |
| `$HOME/.pinocchio/config.yaml` | 5 | Legacy user config |
| `/etc/pinocchio/config.yaml` | 6 (Lowest) | System defaults |

---

## File Format Options

### Format A: Full Config File (Same as ~/.pinocchio/config.yaml)

```yaml
# .pinocchio-profile.yml - can contain anything
repositories:
  - ./prompts
profile-settings:
  profile: local-dev
  profile-registries:
    - ./profiles.yaml
openai-chat:
  openai-api-key: ${LOCAL_API_KEY}
```

**Pros:** Maximum flexibility
**Cons:** Can affect repositories, other settings unexpectedly

### Format B: Profile-Only File (Safer)

```yaml
# .pinocchio-profile.yml - only profile settings
profile: local-dev
profile-registries:
  - ./profiles.yaml
```

**Pros:** Clear purpose, limited blast radius
**Cons:** Can't set API keys or other settings locally

### Format C: Sectioned File (Best of both)

```yaml
# .pinocchio-profile.yml
version: 1

profile-settings:
  profile: local-dev
  profile-registries:
    - ./profiles.yaml

# Optional: Allow some baseline settings
baseline-settings:
  openai-chat:
    base_url: http://localhost:8080
```

**Recommendation: Start with Format B, consider Format C later**

---

## Related Files to Modify

### Glazed
- `glazed/pkg/config/resolve.go` - Add local config resolution
- `glazed/pkg/config/` - May need new file for git utilities

### Geppetto
- `geppetto/pkg/cli/bootstrap/config.go` - Add LocalConfigMode to AppBootstrapConfig
- `geppetto/pkg/cli/bootstrap/profile_selection.go` - Use local config resolution

### Pinocchio
- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go` - Enable local config mode
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md` - Document new behavior

---

## Testing Strategy

### Unit Tests
- Git root detection (with/without git, nested repos, worktrees)
- Local file discovery (file exists/not exists, multiple candidates)
- Precedence ordering (merge order verification)

### Integration Tests
- End-to-end profile resolution with local files
- Verify profile switching still works with local base

### Manual Test Cases
```bash
# Test 1: PWD local profile
cd /tmp/test-project
# Create .pinocchio-profile.yml
pinocchio examples test --print-inference-settings
# Should show local profile settings

# Test 2: Git root profile
cd /tmp/test-project/subdir
# Profile at git root, not in subdir
pinocchio examples test --print-inference-settings
# Should find profile from git root

# Test 3: Precedence
# Both PWD and git root have profiles
# PWD profile should win

# Test 4: No local profile
# Falls back to XDG config as before
```

---

## Open Questions

1. **Git dependency**: Should we shell out to `git` command or use go-git library?
2. **Git worktrees**: How should worktrees be handled?
3. **Monorepos**: In a monorepo, should sub-packages have their own profiles?
4. **Security**: Should local profiles be allowed to set API keys, or only select profiles?
5. **Migration**: Do we need a migration guide for users who might have existing `.pinocchio.yaml` files?

---

## Summary

| Aspect | Recommendation |
|--------|----------------|
| **Where to implement** | Glazed (generic) + Geppetto (profile integration) |
| **File name** | `.pinocchio-profile.yml` (PWD) and `.pinocchio-profile.yml` (git root) |
| **Precedence** | Local config overrides XDG/home, but explicit --config-file wins |
| **Format** | Start with profile-only, expand if needed |
| **Git detection** | Shell out to `git` command (simplest, no new dependencies) |

---

## Related Files Map

```
~/workspaces/2026-04-10/pinocchiorc/
├── glazed/
│   └── pkg/config/
│       └── resolve.go              # ADD: Local config resolution
│
├── geppetto/
│   └── pkg/cli/bootstrap/
│       ├── config.go               # MODIFY: Add LocalConfigMode
│       └── profile_selection.go    # MODIFY: Use local config
│
└── pinocchio/
    ├── pkg/cmds/profilebootstrap/
    │   └── profile_selection.go  # MODIFY: Enable local config
    ├── pkg/doc/topics/
    │   └── pinocchio-profile-resolution-and-runtime-switching.md  # UPDATE: Docs
    └── ttmp/
        └── 2026/04/10/PI-LOCAL-PROFILES--add-local-directory-profile-support-for-pinocchio/
            ├── analysis/01-local-profile-loading-code-analysis-and-design-options.md  # THIS FILE
            └── reference/01-diary.md  # Implementation diary
```

---

## See Also

- `geppetto/pkg/doc/topics/01-profiles.md` - Geppetto profile documentation
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md` - Current resolution flow
- `pinocchio/pkg/doc/tutorials/07-migrating-cli-verbs-to-glazed-profile-bootstrap.md` - Migration patterns
