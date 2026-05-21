---
Title: Implementation Guide
Ticket: PIN-20260521-PROFILES-LIST-VERB
Status: active
Topics:
    - pinocchio
    - profiles
    - cli
    - glazed
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/profile_introspection.go
      Note: |-
        Existing Geppetto report structs and renderer that explain the current text layout
        Current profile report structs and no-header text renderer
    - Path: cmd/pinocchio/cmds/clip.go
      Note: |-
        Existing simple Glazed command registered from the Pinocchio root
        Simple existing Glazed command example
    - Path: cmd/pinocchio/cmds/tokens/helpers.go
      Note: |-
        Existing Cobra group wiring pattern with Glazed subcommands
        Existing static group command wiring with Glazed subcommands
    - Path: cmd/pinocchio/main.go
      Note: |-
        Root command wiring point for the new profiles command group
        Root command wiring point for future profiles group
    - Path: pkg/cmds/profilebootstrap/profile_introspection.go
      Note: |-
        Existing Pinocchio-aware profile report builder that can be reused or moved
        Pinocchio-aware profile report adapter to reuse or move
    - Path: pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md
      Note: |-
        User-facing profile-resolution documentation to update after the verb lands
        Docs to update after profiles list lands
ExternalSources: []
Summary: Design for replacing profile introspection flags with a Glazed `pinocchio profiles list` verb.
LastUpdated: 2026-05-21T18:05:00-04:00
WhatFor: Guide implementers adding a first-class profile listing command to Pinocchio.
WhenToUse: Use before modifying profile introspection CLI surfaces or adding profile discovery commands.
---


# Implementation Guide: `pinocchio profiles list`

## Executive Summary

Pinocchio should expose profile discovery as a first-class command:

```bash
pinocchio profiles list
pinocchio profiles list --verbosity detailed
pinocchio profiles list --verbosity full --output json
```

This should replace the recently explored global flag UX:

```bash
pinocchio run-command ./cmd.yaml --print-profiles
```

The command should be implemented as a Glazed command so table headers, field selection, JSON/YAML/CSV output, and script-friendly row processing are handled by the normal Glazed output stack.

The current text renderer in Geppetto prints profile rows without headers:

```text
Profiles
   * default       default                  model=gpt-5-mini ...
```

The first visible word is the **registry slug**, not a status column. It often says `default` because Pinocchio's inline `.pinocchio.yml` profile registry is normally named `default`. Without headers, this looks like a mysterious repeated default marker. The new verb should avoid that ambiguity by emitting named columns such as `registry`, `profile`, `is_default`, and `is_selected`.

## Problem Statement

The flag-based profile introspection UX has three problems:

1. **Discoverability:** Profile inspection is an action. Users naturally look for a noun/verb command like `pinocchio profiles list`, not an early-exit flag attached to unrelated commands.
2. **Output clarity:** The current text report is manually formatted and lacks headers for profile rows. This makes `registry=default` look like an unexplained first column.
3. **CLI semantics:** `--print-profiles` is a side-effecting early-exit flag. It must be mounted on root commands and dynamic command schemas, which adds complexity and makes command execution semantics less obvious.

The desired UX is a dedicated command group:

```text
pinocchio profiles
  list
```

The `list` verb should use Pinocchio's profile bootstrap path so it includes:

- imported profile registries from `--profile-registries`;
- inline profiles from layered `.pinocchio.yml` / `.pinocchio.override.yml` files;
- the active profile selection from config or `--profile`.

## Answer: why does the current first column often say `default`?

The current Geppetto text renderer prints profile rows like this:

```go
fmt.Fprintf(w, "  %s%s %-14s %-24s model=%-16s api=%-18s %s\n",
    selected,
    marker,
    profile.Registry,
    profile.Slug,
    ...,
)
```

The printed columns are:

```text
selected-marker default-marker registry profile model api description
```

There are no column headers. If `profile.Registry == "default"`, the first named value in the row is `default`. That does **not** mean the row is necessarily the default profile; it means the profile lives in a registry whose slug is `default`.

A dedicated Glazed command fixes this because the default table renderer prints column names, and JSON/YAML output names every field.

## Proposed Solution

Add a new Glazed command:

```bash
pinocchio profiles list [--verbosity default|detailed|full]
```

Recommended default output: Glazed table rows with explicit headers.

Recommended default columns:

| Column | Meaning |
|---|---|
| `selected` | `true` when this is the selected profile |
| `default` | `true` when this is the default profile for its registry/default chain |
| `registry` | Registry slug, e.g. `default`, `workspace`, `user` |
| `profile` | Profile slug |
| `display_name` | Optional display name |
| `model` | Effective or declared chat engine summary |
| `api_type` | Effective or declared API type summary |
| `description` | Optional description |

Detailed/full modes should add columns rather than changing the meaning of default columns.

## UX Contract

### Basic list

```bash
pinocchio profiles list
```

Shows a concise table with headers:

```text
selected  default  registry   profile       display_name  model       api_type
true      true     default    assistant     Assistant     gpt-5-mini  openai-responses
false     false    default    researcher    Researcher    gpt-5       openai-responses
```

### Select a profile while listing

```bash
pinocchio profiles list --profile researcher
```

This should mark `researcher` as selected and use the same profile resolution path as normal command execution.

### Use registry sources

```bash
pinocchio profiles list --profile-registries ./profiles.yaml
```

The command must support existing profile selection flags from Geppetto's profile settings section:

- `--profile`
- `--profile-registries`

### Verbosity

```bash
pinocchio profiles list --verbosity default
pinocchio profiles list --verbosity detailed
pinocchio profiles list --verbosity full
```

`--verbosity` values:

| Value | Intended audience | Columns |
|---|---|---|
| `default` | humans picking a profile | status, registry, profile, display, model/API, description |
| `detailed` | operators debugging profile sources | default columns plus source, version, registry default, stack length, has settings |
| `full` | implementers/debugging scripts | detailed columns plus resolved lineage and redacted merged settings metadata |

### Structured output

Because this is a Glazed command, users should get normal output controls:

```bash
pinocchio profiles list --output json
pinocchio profiles list --output yaml
pinocchio profiles list --fields registry,profile,model
```

Do not hand-roll JSON/YAML in this command.

## Design Decisions

### Decision 1: Add a verb instead of global early-exit flags

Use:

```bash
pinocchio profiles list
```

Do not keep expanding this flag model:

```bash
--print-profiles
--print-profile-resolution
--profile-output
```

Rationale:

- listing profiles is an independent action;
- Glazed already solves headers and structured output;
- command-local flags are easier to document than early-exit flags spread across unrelated commands;
- a command group leaves room for future verbs such as `profiles show`, `profiles resolve`, or `profiles validate`.

### Decision 2: Use Pinocchio profile bootstrap, not generic Geppetto-only bootstrap

The command must use:

```go
profilebootstrap.ResolveCLIProfileRuntime(ctx, parsed)
```

or a small wrapper around it.

Rationale:

- Pinocchio supports inline profiles in layered `.pinocchio.yml` files;
- imported registries and inline registries need to be represented in one chain;
- behavior must match normal Pinocchio runtime selection.

### Decision 3: Emit one Glazed row per profile

The `list` command should emit rows, not a manually formatted report.

Rationale:

- table output gets headers;
- JSON/YAML/CSV become natural;
- users can select fields with `--fields`;
- tests can assert rows rather than parsing pretty text.

### Decision 4: Keep resolution detail behind `--verbosity`

Default output should be compact. More expensive or verbose details should only be added for `detailed` or `full`.

Rationale:

- profile rows are often used interactively;
- full merged settings can be large;
- secret redaction must be carefully preserved for full output.

## Proposed File Layout

Recommended new files:

```text
cmd/pinocchio/cmds/profiles/root.go
cmd/pinocchio/cmds/profiles/list.go
cmd/pinocchio/cmds/profiles/list_test.go
```

Alternative if the project prefers fewer directories:

```text
cmd/pinocchio/cmds/profiles.go
cmd/pinocchio/cmds/profiles_list_test.go
```

Recommended package:

```go
package profiles
```

or, for the flat alternative:

```go
package cmds
```

The grouped package is cleaner because future verbs can live next to `list`.

## Command Wiring Plan

### 1. Create the command group

Add a Cobra group command:

```go
func NewProfilesCommand() (*cobra.Command, error) {
    root := &cobra.Command{
        Use:   "profiles",
        Short: "Inspect Pinocchio engine profiles",
    }

    listCmd, err := NewListCommand()
    if err != nil {
        return nil, err
    }

    cobraListCmd, err := cli.BuildCobraCommandFromCommand(listCmd,
        cli.WithParserConfig(cli.CobraParserConfig{
            ShortHelpSections: []string{schema.DefaultSlug},
            MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
        }),
    )
    if err != nil {
        return nil, err
    }

    root.AddCommand(cobraListCmd)
    return root, nil
}
```

Then register it in `cmd/pinocchio/main.go` near the other static command groups:

```go
profilesCmd, err := profiles.NewProfilesCommand()
cobra.CheckErr(err)
rootCmd.AddCommand(profilesCmd)
```

### 2. Include the right sections on `profiles list`

The command needs:

- Glazed output section;
- command settings section;
- Geppetto profile settings section (`--profile`, `--profile-registries`);
- default flags including `--verbosity`.

Skeleton:

```go
type ListCommand struct {
    *cmds.CommandDescription
}

type ListSettings struct {
    Verbosity string `glazed:"verbosity"`
}

func NewListCommand() (*ListCommand, error) {
    glazedSection, err := settings.NewGlazedSchema()
    if err != nil {
        return nil, err
    }
    commandSettingsSection, err := cli.NewCommandSettingsSection()
    if err != nil {
        return nil, err
    }
    profileSettingsSection, err := geppettosections.NewProfileSettingsSection()
    if err != nil {
        return nil, err
    }

    desc := cmds.NewCommandDescription(
        "list",
        cmds.WithShort("List Pinocchio engine profiles"),
        cmds.WithLong(`List profiles from Pinocchio's configured profile registry chain.

Examples:
  pinocchio profiles list
  pinocchio profiles list --profile researcher
  pinocchio profiles list --verbosity detailed --output json
`),
        cmds.WithFlags(
            fields.New(
                "verbosity",
                fields.TypeString,
                fields.WithDefault("default"),
                fields.WithHelp("Amount of profile detail to include: default, detailed, full"),
            ),
        ),
        cmds.WithSections(glazedSection, commandSettingsSection, profileSettingsSection),
    )

    return &ListCommand{CommandDescription: desc}, nil
}
```

If the field API supports enum choices in this repository version, add accepted values for `verbosity` there; otherwise validate manually in `RunIntoGlazeProcessor`.

### 3. Implement `RunIntoGlazeProcessor`

The command should decode settings and resolve the Pinocchio runtime:

```go
func (c *ListCommand) RunIntoGlazeProcessor(
    ctx context.Context,
    vals *values.Values,
    gp middlewares.Processor,
) error {
    s := &ListSettings{}
    if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
        return err
    }
    if err := validateVerbosity(s.Verbosity); err != nil {
        return err
    }

    report, cleanup, err := profilebootstrap.BuildProfileRegistryReport(ctx, vals, profilebootstrap.ProfileRegistryReportOptions{
        IncludeResolution:       s.Verbosity == "detailed" || s.Verbosity == "full",
        IncludeMergedSettings:   s.Verbosity == "full",
        RedactSecrets:           true,
    })
    if cleanup != nil {
        defer cleanup()
    }
    if err != nil {
        return err
    }

    for _, p := range report.Profiles {
        row := profileRow(report, p, s.Verbosity)
        if err := gp.AddRow(ctx, row); err != nil {
            return err
        }
    }
    return nil
}
```

### 4. Build rows with stable field names

Default row:

```go
func profileRow(report *profilebootstrap.ProfileRegistryReport, p bootstrap.ProfileSummaryReport, verbosity string) types.Row {
    row := types.NewRow(
        types.MRP("selected", isSelected(report, p)),
        types.MRP("default", p.IsDefault),
        types.MRP("registry", p.Registry),
        types.MRP("profile", p.Slug),
        types.MRP("display_name", p.DisplayName),
        types.MRP("model", p.Model),
        types.MRP("api_type", p.APIType),
        types.MRP("description", p.Description),
    )

    if verbosity == "detailed" || verbosity == "full" {
        row.Set("source", p.Source)
        row.Set("version", p.Version)
        row.Set("registry_default", registryDefault(report, p.Registry))
        row.Set("has_settings", p.Model != "" || p.APIType != "")
    }

    if verbosity == "full" {
        row.Set("resolution_lineage", lineageForProfile(report, p))
        row.Set("selected_merged_settings", mergedSettingsForSelectedProfile(report, p))
    }

    return row
}
```

Adjust helper names/types to the actual `types.Row` API available in this repository.

### 5. Decide how to represent full details

For `--verbosity full`, prefer script-friendly scalar/string fields unless Glazed nested values are known to render cleanly in all output modes.

Recommended full columns:

- `resolution_lineage`: JSON-encoded compact string or `[]string` if Glazed handles it cleanly;
- `merged_settings_json`: redacted JSON string for the selected profile only;
- `registry_source`: source path/DSN if available;
- `profile_source`: profile source if available.

Do not emit unredacted API keys or DSNs.

## Interaction With Existing Flag Work

The current branch contains an experimental flag-based implementation:

- `cmd/pinocchio/main.go` mounts `NewProfileIntrospectionSection()` on the root command;
- `pkg/cmds/cmd.go` prepends profile introspection flags to each dynamic command and exits early;
- `pkg/cmds/profile_introspection_test.go` tests the flag path;
- `pkg/cmds/profilebootstrap/profile_introspection.go` contains a useful Pinocchio-aware report builder.

For the new UX, do this:

1. **Keep or move** `pkg/cmds/profilebootstrap/profile_introspection.go` if it remains the best report-building adapter.
2. **Remove** dynamic `--print-profiles` early-exit behavior from `pkg/cmds/cmd.go`.
3. **Remove** root `NewProfileIntrospectionSection()` flag mounting from `cmd/pinocchio/main.go`.
4. **Replace** flag tests with `profiles list` command tests.
5. **Update** docs to describe `pinocchio profiles list` instead of `--print-profiles`.

This preserves the correct profile resolution logic while replacing the user-facing surface.

## Field Catalog

The command can expose fields from four layers: profile rows, registry summaries, source summaries, and selected-profile resolution. The Glazed `profiles list` verb should keep default output small, but these are the available candidate fields for `--fields` and verbosity modes.

### Profile row fields

These are the natural one-row-per-profile fields.

| Field | Source | Suggested verbosity | Meaning |
|---|---|---|---|
| `selected` | `ProfileSummaryReport.IsSelected` plus selected fallback logic | default | Whether this row is the active selected profile. |
| `default` | `ProfileSummaryReport.IsDefault` | default | Whether this profile is the default profile for its registry/default resolution. |
| `registry` | `ProfileSummaryReport.Registry` | default | Registry slug containing the profile, often `default` for inline Pinocchio profiles. |
| `profile` | `ProfileSummaryReport.Slug` | default | Engine profile slug. |
| `display_name` | `ProfileSummaryReport.DisplayName` | default | Human-readable profile name. |
| `description` | `ProfileSummaryReport.Description` | default | Human-readable profile description. |
| `model` | `ProfileSummaryReport.Model` | default | Declared chat engine/model summary. |
| `api_type` | `ProfileSummaryReport.APIType` | default | Declared chat API/provider type summary. |
| `version` | `ProfileSummaryReport.Version` | detailed | Profile metadata version. |
| `source` | `ProfileSummaryReport.Source` | detailed | Profile metadata source path/name, when available. |
| `profile_ref` | derived | detailed | Stable `registry/profile` string for copy/paste. |
| `is_selected` | alias of `selected` if preferred | default or detailed | Machine-friendly selected boolean; avoid exposing both unless aliases are useful. |
| `is_default` | alias of `default` if preferred | default or detailed | Machine-friendly default boolean; avoid exposing both unless aliases are useful. |

Recommendation: expose `selected` and `default` as the public table column names. They are shorter and read well in table output. If JSON compatibility with Geppetto reports matters, also support `is_selected` and `is_default` through aliases or documented JSON mode.

### Registry summary fields

These can be joined onto each profile row by matching `profile.registry == registry.slug`, especially in `detailed` mode.

| Field | Source | Suggested verbosity | Meaning |
|---|---|---|---|
| `registry_slug` | `ProfileRegistrySummaryReport.Slug` | detailed/full | Same value as `registry`, included only if explicit disambiguation is desired. |
| `registry_display_name` | `ProfileRegistrySummaryReport.DisplayName` | detailed | Human-readable registry name. |
| `registry_description` | `ProfileRegistrySummaryReport.Description` | detailed | Registry description. |
| `registry_default_profile` | `ProfileRegistrySummaryReport.DefaultProfile` | detailed | Default profile slug for this registry. |
| `registry_profile_count` | `ProfileRegistrySummaryReport.ProfileCount` | detailed | Number of profiles in this registry. |
| `registry_is_default` | `ProfileRegistrySummaryReport.IsDefault` | detailed | Whether this registry is the default registry. |

### Source fields

These describe configured registry sources. They are not naturally one-to-one with profile rows, so use them carefully. For `profiles list`, prefer `source` from the profile metadata when available. Source reports are better for a future `pinocchio profiles sources` verb or a `--verbosity full` synthetic column.

| Field | Source | Suggested verbosity | Meaning |
|---|---|---|---|
| `source_raw` | `ProfileRegistrySourceReport.Raw` | full | Original registry source string. |
| `source_kind` | `ProfileRegistrySourceReport.Kind` | full | Source type, e.g. yaml/sqlite/sqlite-dsn. |
| `source_path` | `ProfileRegistrySourceReport.Path` | full | Filesystem path for file-backed registry sources. |
| `source_dsn` | `ProfileRegistrySourceReport.DSN` | full | DSN for DSN-backed sources; must be redacted if it can include secrets. |

### Global/default-selection fields

These fields are report-level values. They can be repeated on every row in `full` mode or omitted from row output and reserved for future summary commands.

| Field | Source | Suggested verbosity | Meaning |
|---|---|---|---|
| `default_registry` | `ProfileRegistryReport.DefaultRegistry` | full | Default registry slug. |
| `default_profile` | `ProfileRegistryReport.DefaultProfile` | full | Default profile slug. |
| `selected_registry` | `ProfileRegistryReport.SelectedRegistry` | full | Resolved selected registry slug. |
| `selected_profile` | `ProfileRegistryReport.SelectedProfile` | full | Resolved selected profile slug. |

### Resolution fields

These are available when the command asks the report builder for resolution details. Use them for `detailed`/`full`, not default output.

| Field | Source | Suggested verbosity | Meaning |
|---|---|---|---|
| `resolved_registry` | `ProfileResolutionReport.Registry` | detailed/full | Registry of the resolved selected profile. |
| `resolved_profile` | `ProfileResolutionReport.Profile` | detailed/full | Slug of the resolved selected profile. |
| `resolution_lineage` | `ProfileResolutionReport.Lineage` | full | Base-to-leaf profile stack lineage. Prefer compact JSON or a joined string in table mode. |
| `resolution_metadata` | `ProfileResolutionReport.Metadata` | full | Redacted selected-profile metadata map. |
| `merged_inference_settings` | `ProfileResolutionReport.InferenceSettings` | full | Redacted merged inference settings for the selected profile. Large; best in JSON/YAML output. |

Lineage entries have these subfields:

| Subfield | Source | Meaning |
|---|---|---|
| `lineage.registry_slug` | `ResolvedProfileStackEntry.RegistrySlug` | Registry slug for a stack entry. |
| `lineage.profile_slug` | `ResolvedProfileStackEntry.EngineProfileSlug` | Profile slug for a stack entry. |
| `lineage.version` | `ResolvedProfileStackEntry.Version` | Profile version for a stack entry. |
| `lineage.source` | `ResolvedProfileStackEntry.Source` | Source for a stack entry. |

### Recommended `--verbosity` mapping

Default:

```text
selected, default, registry, profile, display_name, model, api_type, description
```

Detailed:

```text
selected, default, registry, profile, display_name, model, api_type, description,
version, source, registry_default_profile, registry_is_default, registry_profile_count,
profile_ref
```

Full:

```text
all detailed fields,
default_registry, default_profile, selected_registry, selected_profile,
resolved_registry, resolved_profile, resolution_lineage,
resolution_metadata, merged_inference_settings
```

For full table output, consider omitting or compacting very large nested fields unless the user explicitly requests them with `--fields`. For `--output json` and `--output yaml`, nested values are acceptable if redacted.

## Tests

### Unit tests for command rows

Add tests under:

```text
cmd/pinocchio/cmds/profiles/list_test.go
```

Test cases:

1. Inline `.pinocchio.yml` profile appears.
2. Registry file profile appears via `--profile-registries`.
3. `--profile X` marks profile `X` as selected.
4. `--verbosity default` does not include full merged settings.
5. `--verbosity detailed` includes source/registry metadata.
6. `--verbosity full` includes redacted settings and no secrets.
7. Invalid `--verbosity noisy` returns a useful error.

### CLI smoke tests

Manual smoke commands:

```bash
go run ./cmd/pinocchio profiles list

go run ./cmd/pinocchio profiles list \
  --profile-registries ./examples/js/profiles/basic.yaml

go run ./cmd/pinocchio profiles list \
  --profile-registries ./examples/js/profiles/basic.yaml \
  --verbosity detailed \
  --output json | jq .
```

### Regression test for headers

Because Glazed owns table rendering, avoid testing pretty table spacing. Instead assert that the command emits fields named:

```text
selected
default
registry
profile
model
api_type
```

If an end-to-end CLI test is added, it may assert the table output contains `registry` and `profile` headers.

## Implementation Plan

1. Create `cmd/pinocchio/cmds/profiles/` package.
2. Implement `NewProfilesCommand()` group and `NewListCommand()` Glazed command.
3. Reuse `profilebootstrap.BuildProfileRegistryReport(...)` to resolve the registry chain.
4. Emit one Glazed row per profile with stable field names and explicit status booleans.
5. Register `profiles` in `cmd/pinocchio/main.go`.
6. Remove the flag-based early-exit UX from dynamic command schemas and root flags.
7. Replace flag tests with command tests.
8. Update profile docs with `pinocchio profiles list` examples.
9. Validate:
   - `go test ./cmd/pinocchio/cmds/... ./pkg/cmds ./pkg/cmds/profilebootstrap -count=1`
   - `go test ./... -count=1`
   - manual smoke with table and JSON output.
10. Update ticket diary/changelog and commit code/docs.

## Review Checklist

- [ ] `pinocchio profiles list` uses Pinocchio's full profile bootstrap path.
- [ ] Inline `.pinocchio.yml` profiles are visible.
- [ ] Registry file profiles are visible.
- [ ] Table output has clear headers.
- [ ] The first `default` users see is labeled as `registry`, not a mystery column.
- [ ] `--verbosity default|detailed|full` is validated.
- [ ] `--output json` and `--output yaml` work through Glazed.
- [ ] Full output redacts secrets.
- [ ] Old `--print-profiles` flags are removed or intentionally left undocumented only if compatibility is explicitly requested.

## Alternatives Considered

### Keep flags and add headers to Geppetto's text renderer

This would fix the immediate display confusion, but it keeps profile listing as an early-exit side effect on unrelated commands.

Rejected for Pinocchio because `profiles list` is clearer and composes better with Glazed output.

### Add `pinocchio profiles` without Glazed

A hand-written Cobra command could print a nice table, but it would recreate output features Glazed already provides.

Rejected because the user explicitly requested a Glazed command and because structured output is important for scripting.

### Add only `pinocchio profiles resolve`

A resolve verb may be useful later, but users first need a simple inventory command.

Deferred. The `profiles` group can grow future verbs.
