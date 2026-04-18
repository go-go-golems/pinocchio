---
Title: Pinocchio Profile Resolution and Runtime Switching
Slug: pinocchio-profile-resolution-runtime-switching
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

`web-chat` intentionally does not mount the full Geppetto sections on its public CLI. Its command surface currently mounts:

- `profile-settings`
- `redis`

See:

- `pinocchio/cmd/web-chat/main.go`

It still builds a hidden base through `ResolveBaseInferenceSettings(...)`, so shared baseline fields can still reach it through config and environment.

But there is an important consequence:

- simply adding an `ai-client` section to the `web-chat` command description would not be enough by itself to make new CLI flags effective if the runtime continues to rely only on the hidden-base path that rebuilds from env/config/defaults.

If `web-chat` ever wants explicit cross-profile `ai-client` CLI flags, it will need both:

1. a public `ai-client` section on the command
2. a base-resolution path that preserves those parsed CLI values when constructing the runtime baseline

## App-Local Repository Config Is A Separate Path

One subtlety that often confuses contributors is that Pinocchio's command repositories are **not** part of the shared Geppetto bootstrap sections.

The top-level config key:

- `repositories`

is intentionally excluded by `pinocchio/pkg/cmds/profilebootstrap/configFileMapper(...)` before the shared bootstrap middleware sees the config file.

That means there are two startup flows running side by side:

```text
shared bootstrap path
  config/env/defaults -> section values -> profile selection -> registry chain -> inference settings

root CLI repository path
  resolved config files -> top-level repositories[] -> prompt directories -> command discovery
```

The repository path currently lives in:

- `pinocchio/cmd/pinocchio/main.go`

More specifically, `loadRepositoriesFromConfig()` does the following:

1. calls `profilebootstrap.ResolveCLIConfigFiles(nil)` so repository discovery uses the same config-file plan as the shared bootstrap path
2. reads every resolved config file in that returned order
3. extracts the raw top-level `repositories` list from each file
4. de-duplicates exact repeated repository strings
5. appends `$HOME/.pinocchio/prompts`
6. mounts only directories that exist

This is important for architecture discussions because it explains why `repositories` should not be treated like a normal shared section:

- it is Pinocchio application metadata, not Geppetto runtime/profile data
- but it still follows the same config-file discovery stack so operator expectations stay consistent

So the clean split is:

- Geppetto/shared bootstrap owns profile/config/runtime resolution
- Pinocchio root startup owns repository harvesting and command discovery

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| Switching profiles leaks values from the previous profile | The runtime reused active settings instead of a preserved baseline | Re-merge from the preserved base every time |
| A shared setting disappears after profile changes | The setting was treated like profile data instead of baseline data | Move it into the shared baseline section and preserve it in base reconstruction |
| `web-chat` sees config/env settings but not equivalent CLI settings | Hidden base reconstruction currently rebuilds from env/config/defaults, not full parsed CLI values | Add a parsed-values-aware base path if widening `web-chat` CLI surface |
| A contributor puts transport config into engine profiles | Ownership boundary between baseline and overlay is unclear | Treat `ai-client.*` and similar operator settings as baseline-only |
| Repository changes in one config file do not behave like profile overrides | `repositories` is loaded as Pinocchio-local top-level app metadata across all resolved config files, not as a shared section merge | Inspect `cmd/pinocchio/main.go` and the resolved config-file stack, not just profile bootstrap |

## See Also

- [Pinocchio CLI Verb Migration Guide](../tutorials/07-migrating-cli-verbs-to-glazed-profile-bootstrap.md)
- [Webchat Engine Profile Guide](webchat-profile-registry.md)
- `geppetto/pkg/doc/topics/01-profiles.md`
- `geppetto/pkg/doc/tutorials/09-migrating-cli-commands-to-glazed-bootstrap-profile-resolution.md`
