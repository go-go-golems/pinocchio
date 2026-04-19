---
Title: Nested prompt alias warnings are caused by alias path resolution, not command load order
Ticket: PIN-20260418-PROMPT-ALIAS-RESOLUTION
Status: active
Topics:
    - pinocchio
    - glazed
    - aliases
    - bootstrap
    - cli
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../../../../go/pkg/mod/github.com/go-go-golems/clay@v0.4.3/pkg/repositories/repository.go
      Note: Repository alias resolution appends alias.Parents plus aliasFor and logs the warning
    - Path: ../../../../../../../glazed/pkg/cmds/loaders/loaders.go
      Note: |-
        Loader assigns alias parents from the directory path before the file is parsed
        Computes alias parents from directory paths
    - Path: cmd/pinocchio/prompts/code/go.yaml
      Note: |-
        Real target command lives at parents [code] name go
        Concrete target command that should exist before alias resolution
    - Path: cmd/pinocchio/prompts/code/go/concise-doc.yaml
      Note: |-
        Example nested alias file that currently declares aliasFor: go while living under code/go/
        Representative nested alias YAML that triggers the warning pattern
ExternalSources: []
Summary: The noisy startup warnings are not caused by commands being loaded after aliases. Commands are inserted first. The warning comes from nested alias files whose computed parent path already includes the target command name, so alias resolution searches for paths like code go go instead of code go.
LastUpdated: 2026-04-18T14:55:00-04:00
WhatFor: Quick root-cause note for the prompt alias warnings seen during pinocchio startup.
WhenToUse: 'Use when debugging alias warnings such as ''alias concise-doc (prefix: [code go], source ...) for go not found''.'
---


# Nested prompt alias warnings are caused by alias path resolution, not command load order

## Short answer

No: this does **not** look like a command-loading-order bug.

The current repository loader already does the safe thing:

1. load all commands and aliases from disk
2. separate concrete commands from aliases
3. insert all concrete commands first
4. only then try to resolve aliases

The warnings are coming from a **path-shape mismatch** between:

- how Glazed assigns alias parents from nested directories, and
- how Clay resolves `aliasFor` against those parents.

## Evidence

### 1. Commands are inserted before aliases

In Clay repository loading:

- `Repository.LoadCommands(...)` accumulates `commands` and `aliases` separately
- then executes:
  - `r.Add(commands...)`
  - `for _, alias_ := range aliases { r.Add(alias_) }`

Relevant file:

- `/home/manuel/go/pkg/mod/github.com/go-go-golems/clay@v0.4.3/pkg/repositories/repository.go`

Inside `Repository.Add(...)`, non-alias commands are inserted into the trie before alias resolution is attempted.

So the loader is **not** trying to resolve aliases before the real command exists.

### 2. Nested aliases inherit the nested directory as parents

Glazed's filesystem loader computes alias parents from the directory that contains the YAML file:

```go
fromDir := GetParentsFromDir(dir)
aliasOptions_ := append([]alias.Option{
    alias.WithSource(source + "/" + fileName),
    alias.WithParents(fromDir...),
}, aliasOptions...)
```

Relevant file:

- `/home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/glazed/pkg/cmds/loaders/loaders.go`

So for a file at:

- `cmd/pinocchio/prompts/code/go/concise-doc.yaml`

Glazed gives the alias these parents:

- `[code go]`

### 3. Clay resolves alias targets as `alias.Parents + aliasFor`

Clay currently builds the lookup path like this:

```go
prefix := alias_.Parents
aliasedCommandPath := append(append([]string{}, prefix...), alias_.AliasFor)
```

If the command is not found, it logs exactly the warning the user saw:

```go
log.Warn().Msgf("alias %s (prefix: %v, source %s) for %s not found", ...)
```

Relevant file:

- `/home/manuel/go/pkg/mod/github.com/go-go-golems/clay@v0.4.3/pkg/repositories/repository.go`

## Concrete failing example

### Real command

File:

- `/home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/prompts/code/go.yaml`

This defines the real command:

- parents: `[code]`
- name: `go`

So the real command path is:

- `code go`

### Alias file

File:

- `/home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/prompts/code/go/concise-doc.yaml`

This alias says:

```yaml
name: concise-doc
aliasFor: go
```

Because the alias file lives under `code/go/`, Glazed assigns it parents:

- `[code go]`

Clay then tries to resolve:

- `[code go] + go`
- final lookup path: `code go go`

But the real command lives at:

- `code go`

So the lookup fails, and the warning is emitted.

## Why the warnings look systematic

The same structural mismatch appears in multiple prompt packs:

- `code/go/*.yaml` with `aliasFor: go`
- `code/php/*.yaml` with `aliasFor: php`
- `code/typescript/*.yaml` with `aliasFor: typescript`
- `code/professional/suggest-unit-tests.yaml` with `aliasFor: professional`
- `examples/simple/example-driven/*.yaml` with `aliasFor: example-driven`
- `general/writing/ask-a-writer/1-3-1.yaml` with `aliasFor: ask-a-writer`

So this is not one broken file; it is a repeated authoring pattern that the current alias resolver does not interpret the way the prompt tree expects.

## Current conclusion

The root cause is:

- **nested alias files are being resolved relative to their own nested parent path**,
- while the target command is usually the **parent command one level up**, not a child command inside the alias directory.

So the likely mismatch is one of these:

1. **prompt layout convention is wrong**
   - nested alias files should not live inside a directory named after the target command unless `aliasFor` can express a parent-relative path
2. **alias resolution semantics are too limited**
   - `aliasFor: go` in `code/go/concise-doc.yaml` probably wants to mean “alias the command represented by the current parent directory” rather than “find child `go` under `[code go]`”
3. **both**
   - the prompt layout and alias resolver evolved with different assumptions

## Recommended next steps

### Option A: Fix the prompt layout

Move alias files so their computed parent path matches the target command's parent path.

Example:

- move `code/go/concise-doc.yaml` somewhere under `code/` if it is supposed to alias `code go`

Pros:

- no runtime semantics change
- keeps resolver simple

Cons:

- large content migration
- current nested organization is aesthetically useful and may be intentional

### Option B: Teach alias resolution about parent-command aliases

Possible shared fix ideas:

- allow `aliasFor` to be a path, not only a single slug
- support relative alias syntax such as:
  - `aliasFor: ../go`
  - `aliasFor: .`
- or add a fallback rule:
  - if `prefix + aliasFor` is not found, also try `parent(prefix) + aliasFor`

Pros:

- preserves the current prompt tree organization
- likely fixes all the current warnings at once

Cons:

- changes shared alias semantics in Clay/Glazed integration
- fallback behavior must be carefully specified to avoid ambiguity

### Option C: Keep semantics but suppress warnings

This would hide the symptom but not fix alias resolution.

Not recommended as the first move.

## Recommendation

Treat this as a **shared alias-resolution follow-up** with Pinocchio fixtures as the primary reproducer.

The first implementation task should be:

- add a focused regression test around a command at `code/go.yaml` plus an alias at `code/go/concise-doc.yaml`

That will make it clear whether the desired contract is:

- strict same-prefix aliasing, or
- parent-relative aliasing for nested alias files.

Once that contract is explicit, we can either:

- migrate prompt layout, or
- patch Clay/Glazed alias resolution cleanly.
