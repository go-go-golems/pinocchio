---
Title: Pinocchio Profile Resolution and Runtime Switching
Slug: profile-resolution-runtime-switching
Short: How Pinocchio builds hidden base settings, merges engine profiles, and switches profiles at runtime without losing the underlying baseline.
Topics:
- pinocchio
- profiles
- bootstrap
- cli
- webchat
- runtime
- configuration
Commands:
- pinocchio
- web-chat
Flags:
- config-file
- profile
- profile-registries
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# Pinocchio Profile Resolution and Runtime Switching

## What This Page Covers

This page explains the lifecycle that sits between Geppetto's profile model and Pinocchio's runtime behavior.

The core idea is simple, but the implementation has two different "base settings" paths that are easy to confuse:

- a hidden base reconstructed from shared Geppetto sections plus config, environment, and defaults
- a profile-free base recovered from already parsed values by stripping parse steps whose source is `profiles`

You need this page when you are trying to understand:

- what `BaseInferenceSettings` really means
- why `FinalInferenceSettings` is separate
- how runtime profile switching avoids contaminating the baseline
- why cross-profile settings such as `ai-client.*` belong in the baseline rather than in profiles
- how local project config files such as `.pinocchio.yml` participate in bootstrap
- how to inspect parsed field history to see which config layer won

## Mental Model

Pinocchio treats profile resolution as:

```text
baseline + profile overlay = active runtime settings
```

The baseline is app-owned.

The profile overlay is Geppetto-owned.

The active runtime settings are what the current engine is actually built from.

When a user switches profiles later, Pinocchio does not mutate the existing active settings in place and treat that as the new baseline. It goes back to the preserved baseline, resolves a new profile overlay, and builds a new active settings object from scratch.

That is the whole reason profile switching stays deterministic.

## The Three Settings States

### 1. Hidden base inference settings

This is the internal baseline reconstructed through `profilebootstrap.ResolveBaseInferenceSettings(...)`.

It comes from:

- shared Geppetto sections
- config files
- environment variables
- defaults

### Hidden base config layers

Pinocchio now supports a layered config plan instead of only a single implicitly discovered config file.

The standard low-to-high precedence order is:

```text
system -> user -> repo -> cwd -> explicit
```

In concrete terms, Pinocchio can load from:

1. `/etc/pinocchio/config.yaml`
2. `$HOME/.pinocchio/config.yaml`
3. `${XDG_CONFIG_HOME}/pinocchio/config.yaml`
4. `.pinocchio.yml` at the git repository root
5. `.pinocchio.yml` in the current working directory
6. `--config-file <path>`

Later layers win.

That means:

- repo-local config can override user config
- cwd-local config can override repo-local config
- explicit `--config-file` can override everything else

This layered path is implemented through Glazed config-plan primitives and consumed by Geppetto bootstrap.

See:

- `pinocchio/pkg/cmds/profilebootstrap/engine_settings.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`

This path is especially important for commands such as `web-chat`, which intentionally expose a narrower visible CLI but still need a full AI baseline.

### 2. Profile-free base recovered from parsed values

This is the baseline Pinocchio can recover from already parsed command values by removing parse steps whose source is `profiles`.

It comes from:

- the actual parsed command values
- minus profile-derived parse steps

See:

- `pinocchio/pkg/cmds/profile_base_settings.go`

This path is especially important for commands that already parsed their real flag surface and for runtime profile switching.

### 3. Final inference settings

This is the result of merging the selected engine-profile overlay onto one of those bases:

```text
final = merge(base, resolved_profile.inference_settings)
```

This is the object used to build engines.

See:

- `geppetto/pkg/engineprofiles/inference_settings_merge.go`
- `pinocchio/pkg/cmds/cmd.go`

## Data Ownership Diagram

```text
Geppetto-owned profile overlay
  - resolved engine-profile inference settings
  - model/provider defaults

Pinocchio-owned baseline
  - config, env, defaults
  - command-level non-profile flags
  - shared transport settings such as ai-client.*

Runtime-owned active state
  - merged final InferenceSettings
  - selected profile metadata
  - live session builder / engine
```

The ownership rule behind this diagram is the main architectural takeaway:

- if a setting should survive profile changes, it belongs in the baseline
- if a setting describes model/profile behavior, it can belong in the profile overlay

## Startup Resolution Flow

The standard command path in `pinocchio/pkg/cmds/cmd.go` uses both a directly decoded settings object and a profile-safe baseline.

Sequence sketch:

```text
Glazed middleware parses values
  -> stepSettings from parsed values
  -> baseSettingsFromParsedValuesWithBase(...)
  -> profilebootstrap.ResolveCLIEngineSettingsFromBase(...)
  -> BaseInferenceSettings
  -> FinalInferenceSettings
  -> engine factory
```

The important subtlety is that `stepSettings` can already reflect profile effects. That makes it useful for ordinary command execution but unsafe as the baseline for later profile switching unless profile-derived values are removed first.

## Why Stripping `profiles` Parse Steps Matters

Pinocchio stores parse provenance in `values.Values`. Each field keeps a log of where its value came from.

`baseSettingsFromParsedValuesWithBase(...)` walks those logs and keeps the last non-profile parse step for each field. That means:

- CLI flags still count
- config/env/defaults still count
- profile middleware contributions are removed

Conceptually:

```text
parsed values
  - remove source == "profiles"
  = profile-free parsed baseline
```

That is the key trick that lets Pinocchio rebase runtime profile changes onto the original launch-time settings instead of onto whatever profile happened to be active last.

## Config Provenance In Parsed Field History

The parsed field history is now also the main debugging surface for layered config resolution.

Config-derived parse steps carry metadata such as:

- `config_file`
- `config_index`
- `config_layer`
- `config_source_name`
- `config_source_kind`

That means you can inspect parsed fields or inference debug output and answer questions like:

- did this value come from user config or repo config?
- did cwd-local config override the git-root file?
- did an explicit `--config-file` win last?

A simplified example looks like this:

```yaml
profile.active:
  value: explicit-profile
  log:
    - source: config
      value: repo-profile
      metadata:
        config_layer: repo
        config_source_name: git-root-local-profile
    - source: config
      value: cwd-profile
      metadata:
        config_layer: cwd
        config_source_name: cwd-local-profile
    - source: config
      value: explicit-profile
      metadata:
        config_layer: explicit
        config_source_name: explicit-config-file
```

This provenance is especially useful when reviewing bug reports or unexpected profile selection in nested repositories.

## Runtime Profile Switching

The runtime switching implementation lives in:

- `pinocchio/pkg/ui/profileswitch/manager.go`
- `pinocchio/pkg/ui/profileswitch/backend.go`

The manager keeps a preserved `base *settings.InferenceSettings`. When a switch happens:

1. it resolves the requested profile from the registry
2. it merges that profile onto the preserved base
3. it returns a new `Resolved` object
4. the backend rebuilds the engine/session builder from the new final settings

Sequence sketch:

```text
SwitchProfile(profileSlug)
  -> manager.Resolve(profileSlug)
  -> merge(preserved base, resolved overlay)
  -> backend.applyResolved(...)
  -> new engine + new session builder
```

The session builder changes. The preserved base does not.

## Why This Matters For `ai-client`

`ai-client` settings are cross-profile transport settings.

That means they are baseline settings, not profile settings.

Examples:

- timeout
- organization
- user agent
- proxy configuration

If you put these settings in engine profiles, you are mixing operator/app infrastructure into profile overlays. If you keep them in the baseline, they naturally survive profile changes and stay consistent across runtime switches.

## `web-chat` Specific Caveat

`web-chat` intentionally does not mount the full Geppetto sections on its public CLI. Its command surface currently exposes profile-selection controls plus `redis`, rather than the entire shared runtime flag surface.

See:

- `pinocchio/cmd/web-chat/main.go`

It still builds a hidden base through `ResolveBaseInferenceSettings(...)`, so shared baseline fields can still reach it through config and environment.

But there is an important consequence:

- simply adding an `ai-client` section to the `web-chat` command description would not be enough by itself to make new CLI flags effective if the runtime continues to rely only on the hidden-base path that rebuilds from env/config/defaults.

If `web-chat` ever wants explicit cross-profile `ai-client` CLI flags, it will need both:

1. a public `ai-client` section on the command
2. a base-resolution path that preserves those parsed CLI values when constructing the runtime baseline

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| Switching profiles leaks values from the previous profile | The runtime reused active settings instead of a preserved baseline | Re-merge from the preserved base every time |
| A shared setting disappears after profile changes | The setting was treated like profile data instead of baseline data | Move it into the shared baseline section and preserve it in base reconstruction |
| `web-chat` sees config/env settings but not equivalent CLI settings | Hidden base reconstruction currently rebuilds from env/config/defaults, not full parsed CLI values | Add a parsed-values-aware base path if widening `web-chat` CLI surface |
| A contributor puts transport config into engine profiles | Ownership boundary between baseline and overlay is unclear | Treat `ai-client.*` and similar operator settings as baseline-only |

## See Also

- [Migrating Legacy Pinocchio Config to Unified Profile Documents](../tutorials/08-migrating-legacy-pinocchio-config-to-unified-profile-documents.md)
- [Pinocchio CLI Verb Migration Guide](../tutorials/07-migrating-cli-verbs-to-glazed-profile-bootstrap.md)
- [Webchat Engine Profile Guide](webchat-profile-registry.md)
- `geppetto/pkg/doc/topics/01-profiles.md`
- `geppetto/pkg/doc/tutorials/09-migrating-cli-commands-to-glazed-bootstrap-profile-resolution.md`
