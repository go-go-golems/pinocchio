---
Title: Explicit alias paths for nested prompt aliases - analysis and implementation plan
Ticket: PIN-20260418-PROMPT-ALIAS-RESOLUTION
Status: active
Topics:
    - pinocchio
    - glazed
    - aliases
    - bootstrap
    - cli
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/glazed/pkg/cmds/alias/alias.go
      Note: Alias model currently stores aliasFor as a single string slug
    - Path: /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/glazed/pkg/cli/cobra.go
      Note: Direct alias resolution path for Cobra command building also assumes parents + single aliasFor slug
    - Path: /home/manuel/code/wesen/go-go-golems/clay/pkg/repositories/repository.go
      Note: Repository trie alias resolution currently appends alias parents plus aliasFor
    - Path: /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/prompts/code/go/concise-doc.yaml
      Note: Representative nested alias fixture to migrate to explicit path form
ExternalSources: []
Summary: "Add explicit alias path support so aliasFor can be either a simple same-prefix slug or a full command path. Use the new path form for nested Pinocchio prompt aliases that currently resolve to paths like code go go."
LastUpdated: 2026-04-18T15:05:00-04:00
WhatFor: "Detailed implementation plan for fixing nested prompt alias warnings without changing generic load order semantics."
WhenToUse: "Use when implementing or reviewing the alias-path follow-up ticket."
---

# Explicit alias paths for nested prompt aliases - analysis and implementation plan

## Executive summary

The warning storm is caused by a mismatch between:

- **relative alias semantics**: a scalar `aliasFor: go` means “resolve `go` under my current parent prefix”, and
- **nested alias authoring**: many Pinocchio aliases live inside a directory already named after the target command, such as `code/go/concise-doc.yaml`.

The clean fix is:

1. preserve existing scalar `aliasFor` behavior for same-prefix aliases,
2. add a new **explicit path** form for `aliasFor`, and
3. migrate the current nested Pinocchio aliases to the explicit path form.

That gives authors an unambiguous way to say:

```yaml
aliasFor: [code, go]
```

instead of relying on path inference from the alias file’s directory.

## Desired UX

### Existing behavior remains valid

For an alias in the same parent scope:

```yaml
name: old-explorer
aliasFor: test
```

this should continue to mean:

- alias target path = `alias.Parents + ["test"]`

### New explicit path form

For a nested alias that wants to point at a full command path:

```yaml
name: concise-doc
aliasFor: [code, go]
```

this should mean:

- alias target path = `["code", "go"]`
- do **not** prepend the alias file’s own parents

### Compatibility nicety

Because users may write `aliasFor: [code go]` without a comma, the parser should normalize scalar tokens inside the sequence by splitting whitespace as well. That makes all of these equivalent:

```yaml
aliasFor: [code, go]
aliasFor: [code go]
aliasFor:
  - code
  - go
```

## Responsibility split

### Glazed owns alias file decoding and alias-target semantics

Glazed should define the alias target representation and the rule that maps it to an actual command path.

### Clay owns repository trie lookup using the Glazed-provided semantics

Clay should stop hard-coding `alias.Parents + aliasFor` and instead ask the alias object for the resolved target path.

### Pinocchio owns fixture migration

Pinocchio should update its nested prompt aliases to the new explicit path form so startup warnings disappear and intent becomes obvious in the YAML.

## Proposed API shape

## 1. Represent alias targets as path segments

In Glazed, replace the single-string assumption with an alias target type that can decode from YAML as either:

- one scalar slug, or
- a path / sequence of segments

Sketch:

```go
type AliasTarget []string

func (t AliasTarget) IsZero() bool
func (t AliasTarget) Segments() []string
func (t AliasTarget) String() string
func (t *AliasTarget) UnmarshalYAML(node *yaml.Node) error
func (t AliasTarget) MarshalYAML() (interface{}, error)
```

`CommandAlias` then becomes:

```go
type CommandAlias struct {
    Name     string      `yaml:"name"`
    AliasFor AliasTarget `yaml:"aliasFor"`
    ...
}
```

## 2. Centralize target-path resolution on the alias object

Add a helper on `CommandAlias`:

```go
func (a *CommandAlias) ResolveAliasedCommandPath() []string {
    if len(a.AliasFor) == 1 {
        return append(append([]string{}, a.Parents...), a.AliasFor[0])
    }
    return a.AliasFor.Segments()
}
```

Contract:

- **one segment** => legacy relative resolution
- **multiple segments** => explicit absolute command path

This keeps old YAML working while making the nested case explicit.

## 3. Update all alias resolvers to use the helper

Places to update:

- `glazed/pkg/cli/cobra.go`
- `clay/pkg/repositories/repository.go`

Both currently embed the old assumption.

## YAML normalization rules

### Scalar form

If `aliasFor` decodes from a scalar:

- trim whitespace
- if it contains slash-separated or whitespace-separated path segments, split into segments
- otherwise keep it as a single segment

Examples:

- `test` -> `["test"]`
- `code/go` -> `["code", "go"]`
- `code go` -> `["code", "go"]`

### Sequence form

If `aliasFor` decodes from a sequence:

- each sequence item must be scalar-like
- normalize each item the same way
- flatten the result

Examples:

- `[code, go]` -> `["code", "go"]`
- `[code go]` -> `["code", "go"]`
- `['code/go']` -> `["code", "go"]`

## Why this is the right fix

## It solves the real ambiguity directly

The current bug is not that the loader is too early. It is that the YAML cannot distinguish:

- “look up `go` under my current prefix”
- from
- “look up the absolute command path `code go`”

Explicit path targets add that missing expressiveness.

## It preserves backward compatibility

Most existing aliases probably expect current same-prefix behavior. One-segment `aliasFor` can keep that meaning unchanged.

## It keeps author intent obvious in YAML

A nested alias under `code/go/` that points to `[code, go]` is self-documenting and easy to grep.

## It avoids heuristic fallback logic

A fallback like “try `parents + aliasFor`, then `parent(parents) + aliasFor`” is less explicit and can introduce ambiguous resolution rules. Explicit paths are cleaner.

## Implementation plan

## Phase 1 - Glazed alias target model

Files:

- `glazed/pkg/cmds/alias/alias.go`
- new tests in `glazed/pkg/cmds/alias/alias_test.go`

Work:

1. add `AliasTarget`
2. update `CommandAlias` to use it
3. implement YAML decode / normalize behavior for scalar and sequence forms
4. add `ResolveAliasedCommandPath()` helper on `CommandAlias`
5. add tests for:
   - `aliasFor: test`
   - `aliasFor: [code, go]`
   - `aliasFor: [code go]`
   - invalid empty inputs

## Phase 2 - Shared resolver consumers

Files:

- `glazed/pkg/cli/cobra.go`
- `clay/pkg/repositories/repository.go`
- Clay repository tests

Work:

1. replace hard-coded `parents + aliasFor` path building with `ResolveAliasedCommandPath()`
2. update warning/error messages to print the resolved path nicely
3. add a Clay repository regression test:
   - command at parents `[code]`, name `go`
   - alias at parents `[code, go]`, name `concise-doc`, `aliasFor: [code, go]`
   - confirm alias resolves and is inserted

## Phase 3 - Pinocchio fixture migration

Files:

- `pinocchio/cmd/pinocchio/prompts/code/go/*.yaml`
- `pinocchio/cmd/pinocchio/prompts/code/php/*.yaml`
- `pinocchio/cmd/pinocchio/prompts/code/typescript/*.yaml`
- `pinocchio/cmd/pinocchio/prompts/code/professional/suggest-unit-tests.yaml`
- `pinocchio/cmd/pinocchio/prompts/examples/simple/example-driven/*.yaml`
- `pinocchio/cmd/pinocchio/prompts/general/writing/ask-a-writer/1-3-1.yaml`

Work:

1. migrate nested alias fixtures from scalar `aliasFor` to explicit path form
2. re-run a Pinocchio startup / help smoke test and confirm warnings disappear

## Validation plan

### Glazed

- `go test ./pkg/cmds/alias ./pkg/cli -count=1`

### Clay

- `go test ./pkg/repositories -count=1`

### Pinocchio

- `go test ./cmd/pinocchio/... -count=1`
- `go run ./cmd/pinocchio --help` or built-binary equivalent and confirm alias warnings are gone

## Risks

### Risk: sequence semantics are interpreted as absolute paths too broadly

Mitigation:

- document the rule clearly
- keep one-segment behavior unchanged
- add regression tests for both forms

### Risk: changing `CommandAlias.AliasFor` type breaks call sites

Mitigation:

- update the small set of compile points immediately (`glazed/pkg/cli/cobra.go`, `clay/pkg/repositories/repository.go`)
- keep helper constructors for the scalar case

### Risk: some nested aliases were intentionally same-prefix children

Mitigation:

- only migrate the currently noisy Pinocchio fixtures after verifying the intended target path for each set
- keep scalar behavior available where truly desired

## Recommendation

Implement the explicit path form now.

That gives us:

- a principled shared alias model,
- no heuristic resolution hacks,
- no prompt-tree flattening,
- and a straightforward Pinocchio fixture migration that removes the noisy warnings.
