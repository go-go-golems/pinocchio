---
Title: 'Catter File Filter Unification: Analysis, Design and Implementation Guide'
Ticket: PIN-20260212-CATTER-FILEFILTER-UNIFY
Status: active
Topics:
    - pinocchio
    - backend
    - refactoring
DocType: analysis
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: abs:///Users/manuel.odendahl/code/go-go-golems/clay/pkg/filefilter/filefilter.go
      Note: Canonical upstream copy; Phase 2 edit site
    - Path: cmd/pinocchio/cmds/catter/cmds/print.go
      Note: Primary consumer; createFileFilter assembly and constructor rename site
    - Path: cmd/pinocchio/cmds/catter/pkg/fileprocessor.go
    - Path: pkg/filefilter/filefilter.go
      Note: Duplicated filter core to be replaced by clay copy; contains substring-matching bug
    - Path: pkg/filefilter/section.go
ExternalSources: []
Summary: ""
LastUpdated: 2026-09-16T10:32:36.08285-04:00
WhatFor: ""
WhenToUse: ""
---
# Catter File Filter Unification: Analysis, Design and Implementation Guide

## Audience and goal

This document is written for a new intern joining the go-go-golems org who is asked to
"unify the catter file filter with clay's `pkg/filefilter`". It explains every part of the
system needed to understand the problem:

- what the `catter print` command does and how it filters files,
- how the `pkg/filefilter` package is structured and where its defaults live,
- the duplication between `pinocchio/pkg/filefilter` and `clay/pkg/filefilter`,
- an evidence-based answer to "would plain out replacing our copy with clay's work?",
- a recommended design and a phased, testable implementation plan,
- the upstream bug that motivates the whole exercise (the `builder-api` vs `build`
  directory-exclusion problem).

After reading this, you should be able to execute the plan without further context, and
you should understand *why* each step exists.

## TL;DR

- `pinocchio/pkg/filefilter` is a **fork copy** of `clay/pkg/filefilter`. The two
  `filefilter.go` files are byte-for-byte identical today.
- The only functional difference is a renamed constructor
  (`NewFileFilterParameterLayer` in pinocchio vs `NewFileFilterSection` in clay) and a
  different logcopter package name string.
- Pinocchio already depends on clay (`go.mod`: `github.com/go-go-golems/clay v0.4.12`),
  and clay v0.4.12 already ships `layer.go` with the identical section API.
- **A plain replacement works.** It is a mechanical change: delete the local copy,
  switch imports to `github.com/go-go-golems/clay/pkg/filefilter`, and rename the
  constructor at exactly two call sites.
- The *real* work is fixing the upstream substring-matching bug
  (`builder-api` excluded because it contains the substring `build`). That fix
  belongs in clay, must be released as a new clay version, and pinocchio then bumps
  its clay dependency. Doing the unification *first* means the fix lands in one place
  instead of two.

## 1. The system: what `catter print` actually does

`catter` is a subcommand tree inside the pinocchio CLI. Its `print` command walks a set
of paths, applies file filters, and prints (or archives) the surviving files with
optional token counting — think of it as a smarter `cat` for preparing LLM context.

### 1.1 The command surface

The command is defined in:

- `cmd/pinocchio/cmds/catter/cmds/print.go` — `NewCatterPrintCommand()` builds the
  Cobra-style command description (flags, arguments, sections) and
  `RunIntoGlazeProcessor` executes it.
- `cmd/pinocchio/cmds/catter/cmds/stats.go` — a sibling `stats` command that only
  computes statistics (file counts, sizes, tokens) without printing contents.
- `cmd/pinocchio/cmds/catter/pkg/fileprocessor.go` — the `FileProcessor` doing the
  actual walk/print/archive work.
- `cmd/pinocchio/cmds/catter/pkg/stats.go` — the `Stats` helper used for `--glazed`
  structured output.

The `print` command has its own flags (`max-total-size`, `list`, `delimiter`,
`max-lines`, `max-tokens`, `print-filters`, `filter-yaml`, `filter-profile`, `glazed`,
`archive-file`, `archive-prefix`) and attaches a shared **file filter parameter layer**
(a Glazed `schema.Section` of flags) to itself.

### 1.2 Runtime flow

In `print.go`, `RunIntoGlazeProcessor` does the following, in order:

1. Decode flag values into `CatterPrintSettings`.
2. Decide the output format (text, or zip/tar.gz when `--archive-file` is given).
3. Build the filter: `createFileFilter(...)` →
   - start from the CLI flag layer via
     `filefilter.CreateFileFilterFromSettings(layer)`,
   - if `--filter-yaml` is set, replace with `filefilter.LoadFromFile(yaml, profile)`,
   - otherwise, if a `.catter-filter.yaml` exists in the current directory, load that.
4. Construct `pkg.NewFileProcessor(...)` with the filter attached.
5. `fp.ProcessPaths(s.Paths)`.

Conceptually:

```text
                 ┌────────────────────────────────────────────┐
                 │ catter print [paths] [flags]              │
                 └───────────────┬────────────────────────────┘
                                 │
                 ┌───────────────▼───────────────┐
                 │ createFileFilter()            │
                 │  1. flags → FileFilter        │
                 │  2. --filter-yaml override?   │
                 │  3. .catter-filter.yaml?      │
                 └───────────────┬───────────────┘
                                 │ *filefilter.FileFilter
              ┌──────────────────┼───────────────────────┐
              ▼                  ▼                       ▼
     FileProcessor        Stats.ComputeStats      print filters
     (own recursion,      (filewalker +           (--print-filters)
      FilterPath)           FilterNode)
```

### 1.3 Two walk paths, one filter

Important detail for testing and reasoning: the filter instance is used on **two
different walk paths**.

- **Stats path:** `Stats.ComputeStats` uses
  `filewalker.NewWalker(..., filewalker.WithFilter(filter.FilterNode))` from
  `github.com/go-go-golems/clay/pkg/filewalker`. Here a rejected directory node causes
  the walker to prune the entire subtree (`buildFSNode` returns `nil, nil` and never
  descends).
- **Processor path:** `FileProcessor.processPath` / `processDirectory` do their own
  `os.ReadDir` recursion and call `filter.FilterPath(...)` per entry. Excluded
  directories return before recursion, so the subtree is skipped here too.

Both entry points (`FilterNode` for walker nodes, `FilterPath` for plain paths) end up
in the same core logic of the `FileFilter` struct — so any fix inside the package
automatically applies to both.

## 2. The `FileFilter` package: anatomy

The package lives (twice — see §3) at `pkg/filefilter/` with four Go files:

| File | Contents |
|---|---|
| `filefilter.go` | The `FileFilter` struct, default lists, matching logic, YAML (de)serialization |
| `section.go` (pinocchio) / `layer.go` (clay) | Glazed parameter-layer (flag) definitions and `CreateFileFilterFromSettings` |
| `logcopter.go` | Structured logging setup (`logcopter.Package(...)`) |
| `doc/` (clay only) | Developer documentation for the package |

### 2.1 The struct and its defaults

`FileFilter` mixes user-facing configuration with three "default" lists that are not
part of the YAML contract:

- `DefaultExcludedExts` — binary/media/document extensions (`.png`, `.mp4`, `.zip`,
  `.pdf`, `.woff2`, `.lock`, ...).
- `DefaultExcludedDirs` — `.git`, `.svn`, `node_modules`, `vendor`, `.history`,
  `.idea`, `.vscode`, `.yardoc`, `build`, `dist`, `sorbet`.
- `DefaultExcludedMatchFilenames` — regexes `.*-lock\.json$`, `go\.sum$`,
  `yarn\.lock$`, `package-lock\.json$`.

`NewFileFilter(options ...FileFilterOption)` seeds the struct with these lists and a
1 MiB `MaxFileSize`, then applies functional options (each `With...` function in the
file is one option).

### 2.2 Matching flow (pseudocode)

`FilterNode(node)` routes by node type; `FilterPath(path)` filters any path:

```text
FilterNode(node):
    if node is a directory:  return !isExcludedDir(node.path)
    else:                   return shouldProcessFile(node.path)

isExcludedDir(dirPath):
    for d in DefaultExcludedDirs (unless DisableDefaultFilters):
        if strings.Contains(dirPath, d): return true     # <-- substring, not segment!
    for d in ExcludeDirs (user-supplied):
        if strings.Contains(dirPath, d): return true     # <-- same problem
    return false

shouldProcessFile(path):
    stat(path); on error: exclude
    if gitignore enabled and match says ignore: exclude
    ext = lowercase extension
    if is a directory: return !isExcludedDir(path)
    for e in DefaultExcludedExts (unless disabled): if ext == e: exclude
    if size > MaxFileSize: exclude
    if IncludeExts non-empty and ext not in IncludeExts: exclude
    if ext in ExcludeExts: exclude
    if MatchFilenames/MatchPaths non-empty and nothing matches: exclude
    for re in DefaultExcludedMatchFilenames (unless disabled): if re matches base name: exclude
    for re in ExcludeMatchFilenames / ExcludeMatchPaths: if matches: exclude
    gitignore re-check
    if FilterBinaryFiles and first 512 bytes contain a NUL byte: exclude
    return include
```

Two consequences to internalize:

1. **Defaults are exclude-only and cannot be overridden.** User configuration can add
   exclusions but can never rescue a file the defaults rejected, except by globally
   disabling the defaults (`--disable-default-filters` / `disable-default-filters: true`).
2. **Directory exclusion is substring-based on the whole path.** A directory named
   `builder-api` is excluded by the default entry `build`, because
   `"builder-api"` contains `"build"`. Worse, any *path* that contains the substring
   matches — so a file `src/building-notes.md` lives under no excluded directory, but a
   directory `mybuild/` anywhere in the tree is pruned. This is the bug that
   motivated this ticket (observed with a real `builder-api` directory).

### 2.3 Where defaults are applied

Both default and user lists are checked inside the *same* functions
(`isExcludedDir`, `shouldProcessFile`), gated by the same `DisableDefaultFilters`
flag. There is no separate "default filter" code path — this is a common
misreading. The defaults are package variables consulted inline, interleaved with
user filters in a fixed order.

### 2.4 Configuration sources

The effective filter is assembled from three sources, in this precedence:

1. **CLI flag layer** (`section.go`): `--max-file-size`, `--disable-gitignore`,
   `--disable-default-filters`, `--include`, `--exclude`, `--match-filename`,
   `--match-path`, `--exclude-dirs`, `--exclude-match-filename`,
   `--exclude-match-path`, `--filter-binary`, `--verbose`.
2. **YAML override** (`--filter-yaml`, or auto-detected `.catter-filter.yaml` in the
   working directory). YAML loading *replaces* the flag-built filter entirely
   (`LoadFromFile` → `FromYAML` → unmarshal onto a freshly defaulted `FileFilter`),
   including per-profile sub-filters via the `profiles:` map and `--filter-profile`.
3. **Built-in defaults** (§2.1) applied inside the matching code unless disabled.

Note the YAML fields are `include-exts`, `exclude-exts`, `match-filenames`,
`match-paths`, `exclude-dirs`, `exclude-match-filenames`, `exclude-match-paths`
(yaml tags on the struct) — slightly different names from the CLI flags; interns
frequently trip on this.

## 3. The duplication: pinocchio's copy vs clay's original

### 3.1 Evidence

Clay is a shared library repo (`github.com/go-go-golems/clay`) that already provides
`pkg/filefilter`. Pinocchio's `pkg/filefilter` is a copy of it. Current diff inventory
(verified 2026-02-12):

| File | Pinocchio | Clay | Difference |
|---|---|---|---|
| `filefilter.go` | yes | yes | **byte-for-byte identical** (verified with `diff`) |
| `section.go` / `layer.go` | `section.go` | `layer.go` | identical except constructor name: `NewFileFilterParameterLayer` vs `NewFileFilterSection` |
| `logcopter.go` | yes | yes | identical except package name string: `go-go-golems.pinocchio.pkg.filefilter` vs `go-go-golems.clay.pkg.filefilter` |
| `doc/` | absent | present | documentation only |

Additionally, the module cache copy at
`$GOMODCACHE/github.com/go-go-golems/clay@v0.4.12/pkg/filefilter/` is identical to the
local clay checkout (clay's latest tag is `v0.4.13`; the filefilter package did not
change between v0.4.12 and v0.4.13).

### 3.2 Version compatibility

- Pinocchio `go.mod` pins `clay v0.4.12` and `glazed v1.4.3`.
- Clay v0.4.12 uses `glazed v1.4.2`.
- The parameter layer returns a `schema.Section` from glazed; the API used
  (`schema.NewSection`, `fields.New`, `cmds.WithSections`) is stable across those
  versions, and pinocchio already compiles against clay v0.4.12 elsewhere
  (`clay/pkg/filewalker`).

### 3.3 API surface pinocchio actually uses

Only four consumer files exist, all under `cmd/pinocchio/cmds/catter/`:

- `cmds/print.go`
- `cmds/stats.go`
- `pkg/fileprocessor.go`
- `pkg/stats.go`

Together they use exactly this surface:

| Symbol | Used for |
|---|---|
| `filefilter.NewFileFilterParameterLayer()` | build flag layer (2 call sites: `print.go`, `stats.go`) |
| `filefilter.CreateFileFilterFromSettings(layer)` | flags → `FileFilter` (`print.go`) |
| `filefilter.FileFilterSlug` | retrieve the parsed layer (`print.go`) |
| `filefilter.FileFilter` (type) | struct field types (`fileprocessor.go`, `stats.go`) |
| `filefilter.LoadFromFile(yaml, profile)` | YAML/profile loading (`print.go`) |
| methods `FilterPath`, `FilterNode`, `PrintConfiguredFilters` | matching + debug output |

All of these exist in clay's package with identical signatures; only the constructor
name differs.

### 3.4 Why this duplication is bad

- **Drift risk:** the `builder-api` substring bug must currently be fixed in two
  places. If someone fixes only one copy, behavior diverges silently between tools.
- **Wasted maintenance:** every improvement (e.g., an include-dirs feature) must be
  ported.
- **Confusing provenance:** tests, docs, and bug reports must say *which* filefilter
  they mean.
- **Release friction:** pinocchio cannot benefit from clay fixes without manual
  copying.

## 4. Analysis: would a plain replacement work?

**Yes.** The evidence chain:

1. The core file is byte-identical; there is no pinocchio-specific behavior in it.
2. The only code change required is renaming `NewFileFilterParameterLayer()` →
   `NewFileFilterSection()` at exactly two call sites.
3. Clay v0.4.12 — *the version pinocchio already pins* — contains all needed
   symbols, including `CreateFileFilterFromSettings` and `FileFilterSlug`.
4. Glazed version skew (v1.4.3 vs v1.4.2) is not a problem because the section API
   used is identical in both; pinocchio's own glazed version wins in the build anyway
   (Go minimal version selection picks v1.4.3).
5. The logcopter package-name difference is cosmetic: log lines will be tagged
   `go-go-golems.clay.pkg.filefilter` instead of the pinocchio variant. That is
   arguably *more correct* after unification.

**Caveats (the fine print):**

- **`--print-filters` output:** `FileProcessor.printConfiguredFilters` reads exported
  fields (`Filter.MaxFileSize`, `IncludeExts`, ...) — all exported, all present in
  clay's copy. No change.
- **Tests:** pinocchio's tests referencing `pinocchio/pkg/filefilter` must switch
  imports. Check for any test fixtures referencing the local package path.
- **The substring bug is NOT fixed by replacement.** Replacing removes duplication,
  but `builder-api` stays excluded because the identical logic — now in clay — still
  uses `strings.Contains`. The real fix must land in clay and flow back via a version
  bump (§6, Phase 2).
- **Behavior change ordering matters.** If we fix the bug in clay first and only then
  unify, pinocchio needs a clay bump anyway. If we unify first (mechanical, low-risk),
  the behavior fix afterwards is a single small clay PR plus a version bump. Recommended
  order: unify first.

## 5. Design: target state

### 5.1 Target dependency graph

```text
        ┌─────────────────────────┐          ┌──────────────┐
        │ pinocchio (catter)     │          │ clay         │
        │  cmds/print.go         │  imports │ pkg/filefilter│
        │  cmds/stats.go         ├─────────►│ (single copy) │
        │  pkg/fileprocessor.go  │          │ pkg/filewalker│
        │  pkg/stats.go          │          └──────────────┘
        └─────────────────────────┘
   pinocchio/pkg/filefilter: DELETED
```

### 5.2 Upstream fix design (in clay)

Two coordinated changes to `clay/pkg/filefilter`:

**A. Glob-based directory matching (per path segment).** Replace substring
matching with glob matching, one path segment at a time, using the stdlib
`filepath.Match`:

```text
segments = split(dirPath)   # on "/" and "\\", drop empty segments

# A pattern matches the path if it matches ANY single segment:
segmentGlobMatch(segments, pattern):
    for s in segments:
        if filepath.Match(pattern, s): return true
    return false

isExcludedDir(dirPath):
    for p in IncludeDirs:                              # include always wins
        if segmentGlobMatch(segments, p): return false
    for p in DefaultExcludedDirs (unless DisableDefaultFilters):
        if segmentGlobMatch(segments, p): return true
    for p in ExcludeDirs:
        if segmentGlobMatch(segments, p): return true
    return false
```

Why globs and not exact segment equality:

- `filepath.Match` treats `/` as a separator, and `*` does not cross segments —
  so matching per segment is natural and needs no third-party glob library.
- A plain pattern like `build` matches only a directory literally named `build`:
  `builder-api` is **included** again (the bug fix).
- Old substring behavior remains expressible when actually wanted:
  `build*` matches `builder-api`, `*build*` reproduces today's greedy semantics.
- All current default entries (`.git`, `node_modules`, `dist`, ...) are glob-free,
  so the default *lists* stay unchanged; only their matching semantics tighten.
- Case sensitivity follows the OS via `filepath.Match` (case-sensitive on Unix).
- Document in the flag help: escape `*?\\` literally is rarely needed; plain names
  and `*` cover all supported use cases.

`--exclude-dirs` user values switch to the same glob-per-segment semantics. Any
user who relied on substring collisions must switch to `*...*` — call this out in
release notes.

**B. `IncludeDirs` escape hatch.** Add a new field + flag so users can re-include
directories that defaults would exclude:

```yaml
FileFilter:
    ...
    IncludeDirs []string `yaml:"include-dirs,omitempty"`
```

- New flag `--include-dirs` in the section/layer (short flag `I` is free).
- Matching (pseudocode):

```text
isExcludedDir(dirPath):
    if any glob in IncludeDirs matches any segment of dirPath: return false  # include wins
    ... existing exclude logic (now glob-per-segment) ...
```

- `IncludeDirs` must beat both `DefaultExcludedDirs` and user `ExcludeDirs`, because
  the entire problem with `builder-api` is that defaults cannot currently be overridden
  per-directory. Glob semantics are identical to `ExcludeDirs` (see A).
- Keep `DisableDefaultFilters` semantics unchanged.

### 5.3 Compatibility and release strategy

- Segment matching is a behavior change (e.g., `dist-new/` and `rebuild/` are no
  longer excluded). This is the *point* of the fix, but it must be called out in the
  clay release notes and catter help docs.
- Clay change flow: PR to clay → tests → tag `v0.4.14` → pinocchio bumps
  `go.mod` (and `go.sum`) → pinocchio tests.
- Until the clay release exists, pinocchio can develop against a local clay checkout
  via a temporary `replace github.com/go-go-golems/clay => ../clay` directive.
  **Never commit the replace directive.**

## 6. Implementation guide (phased)

Work in small, verifiable phases. After each phase, `make build` (per repo guidelines,
do not run lint separately) and run the package tests touched.

### Phase 0 — Baseline (pinocchio)

- Record current behavior as tests before changing anything, especially for the
  substring bug, so the fix is provable:

```bash
go test ./pkg/filefilter/... ./cmd/pinocchio/cmds/catter/...
```

- Write a table-driven test in `pkg/filefilter/filefilter_test.go` covering:
  - `builder-api` under the *old* semantics (assert excluded, with a comment that
    Phase 2 flips this),
  - `build`, `dist`, `node_modules` excluded,
  - `--include-dirs` not yet existing (skip).
- Also add an end-to-end-ish test for `FilterNode` pruning via
  `filewalker.WithFilter` (the Stats path), since that is a separate code path from
  `FilterPath`.

### Phase 1 — Mechanical unification (pinocchio only)

Goal: pinocchio consumes clay's package; local copy deleted; zero behavior change.

1. In `cmd/pinocchio/cmds/catter/cmds/print.go` and `cmds/stats.go`:
   - change the import to `github.com/go-go-golems/clay/pkg/filefilter`,
   - rename `filefilter.NewFileFilterParameterLayer()` → `filefilter.NewFileFilterSection()`.
2. In `cmd/pinocchio/cmds/catter/pkg/fileprocessor.go` and `pkg/stats.go`:
   - change the import only (no other edits; all symbols match).
3. Grep the repo for any remaining `pinocchio/pkg/filefilter` references:

```bash
rg -n "pinocchio/pkg/filefilter" --type go
rg -rn "pinocchio/pkg/filefilter" docs cmd pkg
```

4. Delete `pkg/filefilter/` (all files: `filefilter.go`, `section.go`, `logcopter.go`).
5. Move any pinocchio-specific tests for the package into `cmd/pinocchio/cmds/catter/`
   as consumer-side tests, or port them to clay in Phase 2.
6. Build and test:

```bash
make build
go test ./cmd/pinocchio/cmds/catter/...
```

7. Sanity-check runtime behavior manually:

```bash
./pinocchio catter print --print-filters
./pinocchio catter print --list .
```

8. Commit (single focused commit, e.g. `refactor(catter): use clay filefilter, drop local copy`).

Expected diff size: a handful of import lines, two constructor renames, deleted files.

### Phase 2 — Upstream fix (clay)

1. In `clay/pkg/filefilter/filefilter.go`:
   - implement `segmentGlobMatch` helper (split path on `/` and `\`, match each
     segment with `filepath.Match`),
   - rewrite `isExcludedDir` per §5.2 A and B (include-wins, glob-per-segment),
   - add `IncludeDirs` field + `WithIncludeDirs` option + YAML tag,
   - update `PrintConfiguredFilters` to print include dirs.
2. In `clay/pkg/filefilter/layer.go`:
   - add `--include-dirs` flag (TypeStringList, help text documenting segment
     semantics), wire it in `CreateFileFilterFromSettings`.
3. Port/move the Phase 0 tests to clay (they encode the contract); flip the
   `builder-api` expectation to *included* now, and add glob-specific cases:
   `build*` excludes `builder-api`, `*build*` reproduces substring behavior,
   `dist` does not match `distribution`.
4. Run clay's tests, get the PR reviewed and merged, and tag a release (e.g. `v0.4.14`).
   Ask a maintainer if you lack release permissions — do not tag others' repos
   unilaterally.

### Phase 3 — Consume the fix (pinocchio)

1. `go get github.com/go-go-golems/clay@vX.Y.Z && go mod tidy`.
2. Re-run catter tests; add one regression test asserting `builder-api` is included
   by default config, and that `--include-dirs` re-includes a directory excluded by
   `--exclude-dirs` (e.g. exclude `api`, include `builder-api`).
3. Update user-facing help/docs if they mention exclusion semantics:
   check `docs/` and any help-loader entries in `cmd/pinocchio/cmds/help_loader.go`.
4. Update `.catter-filter.yaml` documentation examples if present in the repo.

### Phase 4 — Verification checklist

- [ ] `make build` passes in pinocchio and clay.
- [ ] `go test ./...` green in both repos (at minimum: filefilter + catter packages).
- [ ] `rg "pinocchio/pkg/filefilter"` returns nothing.
- [ ] `catter print --print-filters` shows IncludeDirs.
- [ ] Directory named `builder-api` is included by defaults.
- [ ] Directory named `build` still excluded by defaults.
- [ ] Glob `--exclude-dirs "build*"` excludes `builder-api` again (opt-in greedy).
- [ ] Glob `--exclude-dirs "*build*"` reproduces old substring semantics.
- [ ] `--include-dirs build` re-includes it (include beats default exclude).
- [ ] Stats path (`--glazed`) and print path agree on which files survive
      (the two walk paths from §1.3).
- [ ] YAML `include-dirs:` works via `--filter-yaml` and `.catter-filter.yaml`.
- [ ] gitignore behavior unchanged.

## 7. Risks and open questions

- **Behavior change visibility:** glob-per-segment matching makes previously-excluded
  dirs (`builder-api`, but also e.g. `distribution/` no longer matching `dist`) appear
  in output. Sizes and token counts will grow for some users. Mitigation: release notes
  + this is arguably correct behavior. Users who relied on substring matching can
  restore it with explicit `*build*` globs; a compatibility flag was considered and
  rejected since globs cover the old semantics explicitly.
- **`strings.Contains` also affects files:** `isExcludedDir` is also called from
  `shouldProcessFile` for directory paths; the fix covers both since both go through
  the same helper.
- **Windows path separators:** segment splitting handles both `/` and `\`;
  `filepath.Match` patterns use `/` on all platforms, so per-segment matching is
  platform-safe. Add a unit test for both separator styles.
- **Glob metacharacter footguns:** `filepath.Match` treats `\` as an escape on Unix
  and returns `ErrBadPattern` for malformed patterns. Decide policy: surface the
  error (reject bad patterns at filter construction time) rather than silently
  never matching. Add a test.
- **Root-path filtering quirk:** `Walk` applies the filter to the *root* path too, so
  `catter print ./builder-api` currently prints nothing. After the fix it works;
  keep a test for explicitly-listed excluded-by-default paths plus `--include-dirs`
  as the sanctioned escape hatch.
- **Upstream ownership:** confirm with clay maintainers (same org) that taking the
  pinocchio-originated tests into clay is wanted. Precedent: filewalker already lives
  in clay.

## 8. Reference card

### 8.1 File reference

| Path | Role |
|---|---|
| `cmd/pinocchio/cmds/catter/cmds/print.go` | catter print command, `createFileFilter`, YAML auto-load |
| `cmd/pinocchio/cmds/catter/cmds/stats.go` | catter stats command |
| `cmd/pinocchio/cmds/catter/pkg/fileprocessor.go` | walk/print/archive engine, second filter consumer |
| `cmd/pinocchio/cmds/catter/pkg/stats.go` | Stats + filewalker integration |
| `pkg/filefilter/filefilter.go` | duplicated filter core (to be deleted in Phase 1) |
| `pkg/filefilter/section.go` | duplicated flag layer (to be deleted in Phase 1) |
| `../clay/pkg/filefilter/filefilter.go` | canonical filter core (Phase 2 edit site) |
| `../clay/pkg/filefilter/layer.go` | canonical flag layer (Phase 2 edit site) |
| `$GOMODCACHE/.../clay@v0.4.12/pkg/filefilter/` | published version pinocchio already depends on |

### 8.2 API reference (post-unification)

| Symbol | Signature | Notes |
|---|---|---|
| `filefilter.NewFileFilterSection()` | `func() (schema.Section, error)` | attach via `cmds.WithSections` |
| `filefilter.CreateFileFilterFromSettings` | `func(*values.Values) (*FileFilter, error)` | flags → filter |
| `filefilter.FileFilterSlug` | `const FileFilterSlug = "file-filter"` | parsed-layer key |
| `filefilter.NewFileFilter` | `func(...FileFilterOption) *FileFilter` | programmatic use |
| `filefilter.LoadFromFile` | `func(filename, profile string) (*FileFilter, error)` | YAML + profiles |
| `(*FileFilter).FilterNode` | `func(*filewalker.Node) bool` | walker hook |
| `(*FileFilter).FilterPath` | `func(string) bool` | processor hook |
| `(*FileFilter).SaveToFile` / `ToYAML` / `FromYAML` | serialization | unchanged |
| `FileFilter.IncludeDirs` *(new)* | `[]string`, yaml `include-dirs` | Phase 2, glob-per-segment semantics |
| Directory exclude matching *(changed)* | glob per path segment via `filepath.Match` | Phase 2; replaces `strings.Contains` |

### 8.3 Command reference

```bash
# effective filters (debug)
pinocchio catter print --print-filters

# defaults off, manual excludes
pinocchio catter print --disable-default-filters --exclude-dirs node_modules

# YAML config (auto-detected cwd file or explicit)
pinocchio catter print --filter-yaml filters.yaml --filter-profile go

# tests
go test ./cmd/pinocchio/cmds/catter/... ./pkg/filefilter/...   # before Phase 1
go test ./cmd/pinocchio/cmds/catter/...                       # after Phase 1
```

## 9. Summary for the impatient

1. The copy is identical except one function name → **replace it, it just works**
   (Phase 1, ~15 minutes of mechanical edits).
2. The real bug (substring dir matching, `builder-api`) is fixed **in clay**
   (Phase 2), released, and consumed via a version bump (Phase 3).
3. Write the expectations down as tests **before** flipping behavior (Phase 0).
4. One filter, one place, both walk paths fixed at once.
