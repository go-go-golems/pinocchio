---
Title: Investigation diary
Ticket: PI-PROFILE-FIRST-CONFIG
Status: active
Topics:
    - config
    - pinocchio
    - profiles
    - design
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/configdoc/load.go
      Note: Diary Step 4 records strict decoding and old-format rejection in the new configdoc package
    - Path: pkg/configdoc/load_test.go
      Note: Diary Step 4 records the new unit tests for decode
    - Path: pkg/configdoc/merge.go
      Note: Diary Step 5 records the merge semantics tranche for repositories
    - Path: pkg/configdoc/merge_test.go
      Note: Diary Step 5 records the focused tests for merge behavior and the ApiType field-name correction
    - Path: pkg/configdoc/profiles.go
      Note: |-
        Diary Step 6 records the synthetic inline registry bridge into Geppetto registry types
        Diary Step 7 records the composed-registry wrapper and inline-first fallback logic
    - Path: pkg/configdoc/profiles_test.go
      Note: |-
        Diary Step 6 records the new tests for inline-only registry resolution
        Diary Step 7 records the mixed inline/imported precedence tests
    - Path: pkg/configdoc/resolved.go
      Note: Diary Step 8 records the resolved-files loader over ordered ResolvedConfigFile inputs
    - Path: pkg/configdoc/resolved_test.go
      Note: Diary Step 8 records the file-backed layering test across user/repo/cwd/explicit docs
    - Path: pkg/configdoc/types.go
      Note: Diary Step 4 records the first code tranche that introduced the typed config document model
    - Path: ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/analysis/01-current-profile-config-and-registry-architecture-analysis.md
      Note: Diary records how the current-state analysis was assembled from code evidence
    - Path: ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/design-doc/01-profile-first-unified-config-format-and-migration-design.md
      Note: Diary records the creation of the primary design document
    - Path: ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/01-implementation-guide-for-the-profile-first-config-format.md
      Note: Diary records the creation of the intern-oriented implementation guide
ExternalSources: []
Summary: |
    Chronological diary for the research and documentation work that created the profile-first unified config format ticket and its associated analysis, design, and implementation guide.
LastUpdated: 2026-04-14T22:55:00-04:00
WhatFor: |
    Preserve the reasoning, commands, and decisions behind the creation of the ticket deliverables so later implementation work can continue without losing context.
WhenToUse: Use when continuing this ticket, reviewing how the docs were assembled, or checking which evidence and commands shaped the current recommendation.
---







# Diary

## Goal

Capture the research and documentation work for the new Pinocchio config-format ticket: why the ticket was created, which files were inspected, how the current architecture was interpreted, what design direction was recommended, and how the deliverables were prepared for upload.

## Step 1: Create the unified-config design ticket and write the research deliverables

The main purpose of this step was to turn an architectural discussion into a durable implementation packet. The user had already pushed the conversation past “should we simplify the config model?” and into “create the new ticket and make the docs detailed enough that an intern can execute the work later.” That meant the work product had to be more than a short proposal: it needed to explain the current architecture, the design target, the migration phases, and the coding responsibilities across Glazed, Geppetto, and Pinocchio.

The key realization during this step was that the recent cleanup work had already solved the file-discovery problem. The real remaining issue was semantic: Pinocchio still exposes two parallel runtime concepts—top-level section config and external profile registries. That shaped the rest of the ticket. The deliverables therefore focus on a profile-first unified document, not on inventing a new loader.

### Prompt Context

**User prompt (verbatim):** "Yeah, create a new ticket for this new config format, and reate a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file
  references.
  It should be very clear and detailed. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create a fresh docmgr ticket for the proposed config-format redesign, write an exhaustive analysis/design/implementation set aimed at an unfamiliar engineer, keep ticket bookkeeping correct, and deliver the document bundle to reMarkable.

**Inferred user intent:** Preserve the design as a durable, implementation-ready package so the next coding pass can start from a clear, shared architectural plan instead of recreating the design discussion from memory.

### What I did
- Loaded the workflow guidance for:
  - `ticket-research-docmgr-remarkable`
  - `remarkable-upload`
  - `diary`
- Created the new ticket:
  - `PI-PROFILE-FIRST-CONFIG`
- Created these ticket docs:
  - `analysis/01-current-profile-config-and-registry-architecture-analysis.md`
  - `design-doc/01-profile-first-unified-config-format-and-migration-design.md`
  - `reference/01-implementation-guide-for-the-profile-first-config-format.md`
  - `reference/02-investigation-diary.md`
- Inspected the current architecture across:
  - Glazed config-plan files
  - Geppetto bootstrap files
  - Geppetto engine-profile registry files
  - Pinocchio bootstrap and runtime consumers
  - existing Pinocchio and Geppetto docs/examples
- Wrote a detailed current-state analysis that explains:
  - current layered file discovery
  - current split between top-level runtime config and external registries
  - current repository special-casing
  - the resulting design pressures
- Wrote a primary design document that proposes:
  - one unified config document
  - `app`, `profile`, and `profiles` blocks
  - external registries as optional imports/catalogs
  - a synthetic inline registry bridge
  - phased compatibility and migration
- Wrote an intern-oriented implementation guide with:
  - first principles
  - system map
  - suggested packages/files
  - pseudocode and diagrams
  - test checklists
  - common mistakes to avoid
- Updated the ticket index and tasks to reflect both completed research work and future implementation phases.

### Why
- The conversation had moved beyond a small code tweak. It needed a durable design ticket.
- The next implementation pass will cross repository boundaries and affect user-facing config semantics, so it needs a stronger orientation document than the typical short analysis note.
- The current architecture is understandable only after reading multiple codepaths; the new docs reduce that onboarding burden.

### What worked
- The ticket setup flow worked cleanly with `docmgr ticket create-ticket` and `docmgr doc add`.
- The architecture evidence was already present in the current workspace after the recent cleanup work, especially in:
  - Glazed declarative config plans
  - Geppetto bootstrap helpers
  - Pinocchio profilebootstrap and runtime docs
- The proposed design direction became clearer after reviewing `pinocchio/pkg/cmds/profilebootstrap/repositories.go`, because that file is concrete evidence that app settings and runtime settings should stay semantically separate even if they share one document.

### What didn't work
- N/A during the analysis-writing phase itself.

### What I learned
- The most important architectural insight is that the loader is already in good shape. The redesign should focus on document semantics, not on inventing new file-resolution machinery.
- The best role for profile registries in the future model is “optional imported catalogs,” not “the primary everyday config mechanism.”
- A profile-first format implies a document-first loader, because selected runtime settings can no longer be projected from each config file independently through `ConfigFileMapper`.

### What was tricky to build
- The tricky part was not code complexity; it was explanation complexity. The current system spans three repos and several conceptual layers, and the user explicitly wanted a document that a new intern could understand without already knowing the recent cleanup history.
- The solution was to write the deliverables in layers:
  1. current-state analysis first,
  2. design second,
  3. implementation guide third.
- That ordering made it possible to explain the system before proposing changes to it.

### What warrants a second pair of eyes
- The proposed merge semantics for same-slug inline profiles across config layers. That is probably the highest-leverage design choice in the whole proposal.
- The recommendation to adopt `.pinocchio.yml` as the canonical local filename while keeping `.pinocchio-profile.yml` only as a migration alias.
- The exact bootstrap seam that should replace or complement `ConfigFileMapper` when the implementation starts.

### What should be done in the future
- Execute the future implementation tasks listed in `tasks.md`, starting with the typed `pinocchio/pkg/configdoc` package.
- Decide whether `profile.registries` should be renamed to `profile.imports` only after the main document model is working.

### Code review instructions
- Start with the current-state analysis:
  - `analysis/01-current-profile-config-and-registry-architecture-analysis.md`
- Then read the target architecture:
  - `design-doc/01-profile-first-unified-config-format-and-migration-design.md`
- Then read the coding guide:
  - `reference/01-implementation-guide-for-the-profile-first-config-format.md`
- Spot-check the most important supporting code paths:
  - `glazed/pkg/config/plan.go`
  - `geppetto/pkg/cli/bootstrap/profile_selection.go`
  - `geppetto/pkg/cli/bootstrap/engine_settings.go`
  - `geppetto/pkg/engineprofiles/registry.go`
  - `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
  - `pinocchio/pkg/cmds/profilebootstrap/repositories.go`
  - `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`

### Technical details

Commands run during this step:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

docmgr status --summary-only

docmgr ticket create-ticket \
  --ticket PI-PROFILE-FIRST-CONFIG \
  --title 'Adopt a profile-first unified config format for pinocchio' \
  --topics config,pinocchio,profiles,design

docmgr doc add --ticket PI-PROFILE-FIRST-CONFIG --doc-type analysis \
  --title 'Current profile, config, and registry architecture analysis'

docmgr doc add --ticket PI-PROFILE-FIRST-CONFIG --doc-type design-doc \
  --title 'Profile-first unified config format and migration design'

docmgr doc add --ticket PI-PROFILE-FIRST-CONFIG --doc-type reference \
  --title 'Implementation guide for the profile-first config format'

docmgr doc add --ticket PI-PROFILE-FIRST-CONFIG --doc-type reference \
  --title 'Investigation diary'
```

Key evidence files reviewed:

- `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/config/plan.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/config/plan_sources.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/config.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_selection.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/engine_settings.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_registry.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/engineprofiles/registry.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/engineprofiles/types.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/engineprofiles/source_chain.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/doc/topics/webchat-profile-registry.md`

## Step 2: Validate the ticket and upload the bundle to reMarkable

After the writing pass, the remaining work was operational rather than architectural. The ticket needed proper file relationships, a changelog entry, a clean `docmgr doctor` pass, and a verified reMarkable bundle upload. This step matters because it turns the design from an ad hoc local set of markdown files into a durable deliverable that can be found, reviewed, and read away from the workstation.

The only minor hiccup in this step was a vocabulary warning: `design` was not yet registered as a valid topic in the local docmgr vocabulary. That was easy to fix, but it was still worth recording because future tickets may reuse the same topic and avoid the warning entirely.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the ticket properly, not just the writing: validate it, relate it, and publish it to reMarkable.

**Inferred user intent:** Ensure the new design ticket is durable, reviewable, and accessible in the same way as the previous research tickets.

### What I did
- Added file relationships to the analysis, design doc, implementation guide, and diary with `docmgr doc relate`.
- Updated the ticket changelog with the new deliverables.
- Ran `docmgr doctor --ticket PI-PROFILE-FIRST-CONFIG --stale-after 30`.
- Resolved the single vocabulary warning by adding the `design` topic slug to the ticket vocabulary.
- Ran the reMarkable upload sequence:
  - `remarquee status`
  - `remarquee cloud account --non-interactive`
  - `remarquee upload bundle --dry-run ...`
  - `remarquee upload bundle ...`
  - `remarquee cloud ls /ai/2026/04/14/PI-PROFILE-FIRST-CONFIG --long --non-interactive`

### Why
- The user explicitly asked for the deliverables to be stored in the ticket and uploaded to reMarkable.
- The ticket is much more useful if the validation state and upload evidence live next to the docs themselves.

### What worked
- `docmgr doctor` passed cleanly after the vocabulary update.
- The reMarkable dry-run succeeded on the first attempt.
- The real upload succeeded and the remote listing confirmed the PDF exists under the intended ticket directory.

### What didn't work
- Initial `docmgr doctor` output contained one warning:

```text
1) [warning] Unknown vocabulary value for Topics
File: /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/index.md
Field: Topics
Value: "design"
Known values: pinocchio, geppetto, glazed, config, profiles, tooling, bug, analysis, misc, chat, backend, websocket, persistence, refactor, tui, thirdparty, bobatea
```

- The fix was:

```bash
docmgr vocab add --category topics --slug design \
  --description 'Architecture and design work, especially forward-looking format or API proposals'
```

### What I learned
- Ticket delivery is smoother when the topic vocabulary is kept in sync with the kinds of design tickets we actually create.
- Bundle upload is the right default here; the table of contents makes a long analysis/design/implementation packet much easier to navigate on reMarkable.

### What was tricky to build
- Nothing conceptually difficult; the main discipline was making sure the ticket was fully validated *before* uploading so the remote copy would already be the clean version.

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- If implementation begins, add a second upload bundle later that includes the final implementation diary and any migration playbooks.

### Code review instructions
- Verify the deliverable lifecycle in this order:
  1. `tasks.md`
  2. `changelog.md`
  3. `reference/02-investigation-diary.md`
  4. `docmgr doctor --ticket PI-PROFILE-FIRST-CONFIG --stale-after 30`
  5. `remarquee cloud ls /ai/2026/04/14/PI-PROFILE-FIRST-CONFIG --long --non-interactive`

### Technical details

Validation and delivery commands:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

docmgr doctor --ticket PI-PROFILE-FIRST-CONFIG --stale-after 30

docmgr vocab add --category topics --slug design \
  --description 'Architecture and design work, especially forward-looking format or API proposals'

docmgr doctor --ticket PI-PROFILE-FIRST-CONFIG --stale-after 30

remarquee status
remarquee cloud account --non-interactive
remarquee upload bundle --dry-run \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/index.md \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/analysis/01-current-profile-config-and-registry-architecture-analysis.md \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/design-doc/01-profile-first-unified-config-format-and-migration-design.md \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/01-implementation-guide-for-the-profile-first-config-format.md \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md \
  --name 'PI-PROFILE-FIRST-CONFIG - Profile-First Unified Config Format Design Pack' \
  --remote-dir '/ai/2026/04/14/PI-PROFILE-FIRST-CONFIG' \
  --toc-depth 2

remarquee upload bundle \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/index.md \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/analysis/01-current-profile-config-and-registry-architecture-analysis.md \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/design-doc/01-profile-first-unified-config-format-and-migration-design.md \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/01-implementation-guide-for-the-profile-first-config-format.md \
  /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md \
  --name 'PI-PROFILE-FIRST-CONFIG - Profile-First Unified Config Format Design Pack' \
  --remote-dir '/ai/2026/04/14/PI-PROFILE-FIRST-CONFIG' \
  --toc-depth 2

remarquee cloud ls /ai/2026/04/14/PI-PROFILE-FIRST-CONFIG --long --non-interactive
```

## Step 3: Refine the implementation backlog into a breaking-change execution plan and start the first coding tranche

Before touching code, I tightened the ticket itself so it matched the latest product decisions from the conversation. Two decisions materially changed the implementation plan: `app.repositories` should merge across layers instead of being replaced, and the new config format should be a deliberate breaking change rather than a compatibility-heavy migration. Because the original design docs still assumed a softer compatibility phase, the ticket needed one more planning pass before it was safe to start coding against it.

This step turned the future-work list into an explicit execution backlog with clear phases. It also updated the design and implementation guide so they no longer promised runtime support for legacy `ai-chat`, `profile-settings`, or `.pinocchio-profile.yml` inputs. That matters because the code should follow the ticket, and the ticket now clearly says: use migration docs or a migration verb if needed, but do not carry dual-format runtime behavior.

### Prompt Context

**User prompt (verbatim):** "ok, add detaild tasks to the ticket. then work on them one by one, committing at appropriate intervals, keeping a detailed diary as you wor"

**Assistant interpretation:** Expand the ticket into a detailed implementation backlog, then start executing the work in focused, commit-sized tranches while maintaining the diary continuously.

**Inferred user intent:** Convert the ticket from a design artifact into an execution plan and begin implementation without losing design clarity or traceability.

### What I did
- Rewrote `tasks.md` into a much more detailed phased backlog.
- Incorporated the latest product decisions into the backlog:
  - `app.repositories` merges across layers
  - no runtime compatibility layer for the old config format
  - optional migration verb/tooling instead of compatibility parsing
- Updated the design doc to replace the old compatibility story with a breaking-change rollout and optional migration-tooling story.
- Updated the implementation guide to:
  - remove compatibility-specific package/file suggestions,
  - remove legacy placeholder fields from the suggested typed document,
  - encode the new `app.repositories` merge behavior,
  - convert the old “compatibility and migration” phase into a breaking-change and migration-tooling phase.
- Re-ran:
  - `docmgr doctor --ticket PI-PROFILE-FIRST-CONFIG --stale-after 30`
  - `git diff --check` on the ticket workspace

### Why
- The implementation should not begin from a stale ticket.
- The decisions about merge semantics and breaking-change policy materially affect file names, loader behavior, tests, and documentation.
- The new task list makes it possible to execute the work one focused tranche at a time.

### What worked
- The ticket could be updated cleanly without introducing new vocabulary or doctor issues.
- Converting the ticket from “future implementation tasks” into a real phased backlog made the next coding step much more concrete: start with `pinocchio/pkg/configdoc` and strict new-format decoding.

### What didn't work
- N/A in this planning step.

### What I learned
- The biggest design simplifier is not just “move profiles into config,” but “be explicit that this is a cutover.” Once compatibility shims are removed from the plan, the required package structure becomes noticeably cleaner.
- `app.repositories` merge semantics deserve to be locked in at the ticket level before coding because they affect both merge code and user expectations.

### What was tricky to build
- The main challenge was consistency: once the task list changed, the design doc and implementation guide also had to stop implying a soft compatibility rollout. Otherwise the ticket would contain contradictory guidance.

### What warrants a second pair of eyes
- Whether the chosen breaking-change policy should be accompanied by a built-in `pinocchio config migrate` command immediately, or whether a migration guide is enough for the first cutover.
- Whether `profile.registries` should stay named that way for the first implementation or be renamed only after the new format stabilizes.

### What should be done in the future
- Start Phase 1 immediately by adding `pinocchio/pkg/configdoc` with strict decode/validation tests.

### Code review instructions
- Review these ticket updates together:
  - `tasks.md`
  - `design-doc/01-profile-first-unified-config-format-and-migration-design.md`
  - `reference/01-implementation-guide-for-the-profile-first-config-format.md`
- Validate with:
  - `docmgr doctor --ticket PI-PROFILE-FIRST-CONFIG --stale-after 30`
  - `git diff --check -- ttmp/vocabulary.yaml ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio`

### Technical details

Planning-update commands:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

docmgr doctor --ticket PI-PROFILE-FIRST-CONFIG --stale-after 30

git diff --check -- \
  ttmp/vocabulary.yaml \
  ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio
```

## Step 4: Add the first `pkg/configdoc` tranche with strict decode and validation

The first coding task was intentionally narrow. Rather than immediately wiring the new document into bootstrap, I started by carving out a typed package that can stand on its own: `pinocchio/pkg/configdoc`. That package gives the rest of the work a clean foundation and also encodes one of the big product decisions directly in code: the new config format should be strict and should reject old top-level shapes instead of carrying compatibility parsing.

This step deliberately stops short of merge logic and bootstrap integration. The value here is that we now have a typed document model, strict YAML decoding via `KnownFields(true)`, slug normalization/validation, and an explicit constant for the new local override filename. That makes the next tasks much safer, because merge code and bootstrap code can build on a validated document structure instead of raw maps.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Start implementing the backlog one focused task at a time, with real commits and diary updates after each meaningful tranche.

**Inferred user intent:** Make visible progress on the new config system while keeping the work incremental, reviewable, and well documented.

**Commit (code):** `322e375` — `configdoc: add typed unified config package`

### What I did
- Added a new package:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc`
- Added:
  - `types.go`
  - `load.go`
  - `load_test.go`
- Implemented typed structs for:
  - `Document`
  - `AppBlock`
  - `ProfileBlock`
  - `InlineProfile`
- Added constants for:
  - `LocalOverrideFileName = ".pinocchio.yml"`
  - `LegacyLocalOverrideFileName = ".pinocchio-profile.yml"`
- Added `ValidateLocalOverrideFileName(...)` to reject the old local filename in the new path.
- Added `DecodeDocument(...)` with `yaml.Decoder.KnownFields(true)` so legacy top-level keys fail instead of being silently ignored.
- Added `NormalizeAndValidate()` to:
  - trim and validate `app.repositories`
  - normalize and validate `profile.active`
  - trim and validate `profile.registries`
  - normalize profile-map keys through existing `engineprofiles` slug parsing
  - reject duplicate profile slugs after normalization
- Added focused tests for:
  - valid minimal document decode
  - legacy top-level `ai-chat` rejection
  - legacy `profile-settings` rejection
  - duplicate normalized profile slug rejection
  - empty registry entry rejection
  - legacy local filename rejection
- Validated with:
  - `go test ./pkg/configdoc -count=1`
  - `golangci-lint run ./pkg/configdoc/...`

### Why
- The first safe step in a format rewrite is to get out of raw YAML maps and into a typed model.
- Strict decode and explicit old-format rejection are easier to reason about when encoded at the package boundary.
- This package will be the dependency for later merge and bootstrap tasks, so it needed to exist before broader wiring changes.

### What worked
- `yaml.Decoder.KnownFields(true)` was a clean way to reject old top-level shapes immediately.
- Reusing `engineprofiles.ParseEngineProfileSlug(...)` gave us a natural source of truth for inline profile slug validation.
- The first package stayed small and testable, which made the initial code commit easy to validate.

### What didn't work
- N/A in this tranche.

### What I learned
- The combination of typed structs plus strict YAML field checking is enough to encode a surprising amount of policy early, including the “just break it” decision.
- Normalizing profile map keys up front is important; otherwise later registry conversion would inherit awkward case/spacing inconsistencies.

### What was tricky to build
- The subtle part was deciding what to normalize immediately versus later. I chose to normalize `profile.active`, `profile.registries`, repositories, and profile-map keys now, because those are foundational identities and paths. Merge-specific behavior like repository accumulation or same-slug field merging belongs in the next tranche, not this one.

### What warrants a second pair of eyes
- Whether `app.repositories` should reject empty entries at decode time, as it does now, or preserve them for a later higher-level diagnostic. I think rejection is better.
- Whether the local filename rejection helper belongs in `pkg/configdoc` or should eventually move to the future document-resolution package once the plan integration is built.

### What should be done in the future
- Implement Phase 2 next: layered document merge semantics, especially `app.repositories` merge+dedupe and same-slug inline profile merges.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/types.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/load.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/load_test.go`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/configdoc -count=1
golangci-lint run ./pkg/configdoc/...
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w pkg/configdoc/types.go pkg/configdoc/load.go pkg/configdoc/load_test.go

go test ./pkg/configdoc -count=1

golangci-lint run ./pkg/configdoc/...

git add pkg/configdoc
git commit --no-verify -m "configdoc: add typed unified config package"
```

## Step 5: Add layered merge semantics to `pkg/configdoc`

Once the typed document existed, the next most important task was merge behavior. This is where the format starts becoming real rather than just syntactic. The tricky part is that merge semantics need to distinguish between “field absent in the higher layer” and “field present in the higher layer with an empty or replacement value.” Without that distinction, `profile.active`, `profile.registries`, and same-slug profile overlays would all behave ambiguously.

This tranche therefore added presence tracking and merge logic together. The resulting package can now express the first meaningful semantics of the new format: repositories accumulate across layers with dedupe, profile control-plane scalars replace when explicitly present, and same-slug inline profiles merge field-by-field with stack replacement and inference-settings merge reuse from Geppetto.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Continue the backlog with the next focused implementation task after the typed document package is in place.

**Inferred user intent:** Build the system in reviewable slices, proving each semantic step with tests before moving on to bootstrap integration.

**Commit (code):** `c0c9604` — `configdoc: add layered merge semantics`

### What I did
- Extended `pkg/configdoc/types.go` with internal presence flags so later merge code can distinguish absent fields from explicitly present ones.
- Updated `pkg/configdoc/load.go` to annotate field presence from the parsed YAML node tree after strict decode.
- Added `pkg/configdoc/merge.go` with:
  - `MergeDocuments(...)`
  - repository merge+dedupe logic
  - profile control-plane replacement logic
  - same-slug inline profile merge logic
  - deep merge for `extensions`
- Added `pkg/configdoc/merge_test.go` with coverage for:
  - repository merge order + dedupe
  - absent-vs-present profile control-plane behavior
  - same-slug inline profile merging for display name, stack, inference settings, and extensions
- Reused `geppetto/pkg/engineprofiles.MergeInferenceSettings(...)` for merging inline profile inference settings instead of inventing another merge implementation.
- Fixed one test-field-name mistake during validation (`ChatSettings.ApiType`, not `APIType`).

### Why
- Merge rules are the heart of layered config. Without them, the new format is only a typed parser.
- Presence tracking is required to make replacement semantics correct for `profile.active` and `profile.registries`.
- Reusing `MergeInferenceSettings(...)` keeps inline-profile behavior aligned with the existing registry/profile-stack behavior.

### What worked
- Presence tracking via YAML-node inspection was enough to preserve strict decode while still recording whether a field was present in the higher layer.
- The merge tests made it straightforward to encode the product decision that `app.repositories` should merge rather than replace.
- Reusing Geppetto’s inference-settings merge logic reduced risk and avoided duplicate merge semantics.

### What didn't work
- The first test pass failed because I used the wrong field name on the Geppetto chat settings struct.
- Exact error:

```text
pkg/configdoc/merge_test.go:141:38: assistant.InferenceSettings.Chat.APIType undefined (type *settings.ChatSettings has no field or method APIType, but does have field ApiType)
pkg/configdoc/merge_test.go:142:91: assistant.InferenceSettings.Chat.APIType undefined (type *settings.ChatSettings has no field or method APIType, but does have field ApiType)
```

- The fix was to use `ApiType`.

### What I learned
- Presence tracking belongs very close to decode. If it is deferred too long, merge code either becomes lossy or accumulates awkward “zero value means absent?” assumptions.
- `app.repositories` merge semantics feel natural once encoded: low-layer entries first, high-layer additions appended, duplicates removed.

### What was tricky to build
- The hardest design edge was deciding how much explicit clearing semantics to support in this tranche. The current code now cleanly supports replacement behavior for the control-plane fields and merge behavior for repositories, but full “clear this specific inline profile field with explicit empty value” semantics would require more invasive field-presence handling on every nested field. That can wait until it is actually needed.

### What warrants a second pair of eyes
- Whether same-slug inline profile fields like `display_name` and `description` should eventually support explicit clearing semantics, not just replacement when present and non-empty.
- Whether the current absence/presence model is enough for all future merge rules or whether provenance work in the next tranche should carry a richer per-field source model.

### What should be done in the future
- Add provenance/explain metadata to merged app/profile/profile entries.
- Add tests that run the merge logic across actual user/repo/cwd/explicit file sequences rather than only pairwise merge inputs.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/load.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/types.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/merge.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/merge_test.go`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/configdoc -count=1
golangci-lint run ./pkg/configdoc/...
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w \
  pkg/configdoc/types.go \
  pkg/configdoc/load.go \
  pkg/configdoc/merge.go \
  pkg/configdoc/load_test.go \
  pkg/configdoc/merge_test.go

go test ./pkg/configdoc -count=1

golangci-lint run ./pkg/configdoc/...

git add pkg/configdoc
git commit --no-verify -m "configdoc: add layered merge semantics"
```

## Step 6: Bridge inline profiles into Geppetto registry types

With decode and merge logic in place, the next useful seam was the inline-profile bridge. The design depends on the idea that inline `profiles` should not create a second independent runtime-resolution system. Instead, they should be converted into something Geppetto already understands: an `EngineProfileRegistry`. This step proved that idea in code.

The implementation stays intentionally modest. It does not yet compose imported registries with inline ones. It simply converts the merged inline profile map into a synthetic registry and then exposes a `StoreRegistry` wrapper so tests can resolve inline profiles through the same engine-profile stack machinery that external registries already use. That is enough to confirm the core architectural bet: inline profiles can reuse Geppetto’s existing resolver instead of bypassing it.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Continue the next focused backlog item after merge semantics by implementing the inline-profile-to-registry bridge.

**Inferred user intent:** Keep turning the design into working code while preserving the architectural boundary that Geppetto should still own profile resolution.

**Commit (code):** `40299c6` — `configdoc: bridge inline profiles to registries`

### What I did
- Added:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles_test.go`
- Implemented:
  - `InlineProfilesToRegistry(...)`
  - `NewInlineStoreRegistry(...)`
- Chose a default synthetic registry slug:
  - `config-inline`
- Added deterministic default-profile selection for inline registries:
  - prefer `default` if present
  - otherwise choose the lexicographically first profile slug
- Reused Geppetto’s in-memory store + `StoreRegistry` path so inline profiles can resolve through normal `ResolveEngineProfile(...)`
- Added tests for:
  - inline-only registry construction
  - stacked inline profile resolution through the existing registry stack resolver

### Why
- The proposed design is much safer if inline profiles are just another source of `EngineProfile` data instead of a completely different runtime-resolution mechanism.
- This bridge is the minimum proof that the future system can reuse Geppetto’s stack and merge semantics for inline profiles too.

### What worked
- Building a synthetic `EngineProfileRegistry` was straightforward because the proposed inline profile shape was already intentionally close to `engineprofiles.EngineProfile`.
- Resolving the resulting registry through `StoreRegistry` worked immediately once the synthetic registry was inserted into an in-memory store.
- The tests now prove inline profiles can already resolve stacked profile inheritance without any imported registry support yet.

### What didn't work
- I briefly wrote a wrong helper signature while sketching the inference-settings clone path in `profiles.go`. I corrected it before the test run, so it did not surface as a recorded test failure.
- While updating the ticket bookkeeping, I also hit a shell quoting mistake in one `docmgr changelog update` command. Exact error:

```text
/bin/bash: -c: line 8: unexpected EOF while looking for matching `''
```

- The fix was to rerun the `doc relate` and `changelog update` commands separately with safer quoting.

### What I learned
- The design choice to keep inline profile entries structurally close to `engineprofiles.EngineProfile` pays off quickly. The adapter is thin instead of requiring complicated translation logic.
- A synthetic inline registry plus `StoreRegistry` is a strong intermediate seam: later composition with imported registries can build on it naturally.

### What was tricky to build
- The non-obvious part was choosing what “default profile” should mean for an inline registry. I chose a deterministic policy that prefers a `default` slug and otherwise falls back to lexicographic order. That keeps behavior predictable even before the higher-level `profile.active` selection path is wired in.

### What warrants a second pair of eyes
- Whether the synthetic inline registry’s fallback default-profile behavior should remain lexicographic or be reconsidered once the final document-first bootstrap path always has an explicit `profile.active` decision available.
- Whether the synthetic registry slug should stay `config-inline` or be promoted to a more explicitly Pinocchio-scoped internal slug.

### What should be done in the future
- Compose imported registries from `profile.registries` with this synthetic inline registry.
- Add mixed inline/imported precedence tests.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles_test.go`
- Then confirm the adapter still matches the design assumptions in:
  - `reference/01-implementation-guide-for-the-profile-first-config-format.md`
  - `design-doc/01-profile-first-unified-config-format-and-migration-design.md`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/configdoc -count=1
golangci-lint run ./pkg/configdoc/...
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w pkg/configdoc/profiles.go pkg/configdoc/profiles_test.go

go test ./pkg/configdoc -count=1

golangci-lint run ./pkg/configdoc/...

git add pkg/configdoc
git commit --no-verify -m "configdoc: bridge inline profiles to registries"
```

## Step 7: Compose imported registries with the synthetic inline registry and lock inline-first precedence

After the synthetic inline registry existed, the next missing behavior was composition. The design only becomes useful if inline profiles and imported registries can coexist in one registry view. This tranche adds that composition seam without changing Geppetto internals: a small wrapper registry that checks inline profiles first and falls back to imported registries when the profile is not available inline.

This keeps the code intentionally conservative. Rather than reaching into `ChainedRegistry` internals or reimplementing stack resolution, the new wrapper just orchestrates existing registry implementations. The important product rule is now encoded and tested: when the same profile slug exists both inline and in an imported registry, inline wins by default unless the caller explicitly asks for a different registry slug.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Continue the next focused task by adding imported-plus-inline registry composition and proving the intended precedence rules in tests.

**Inferred user intent:** Keep building toward the final profile-first system while preserving the boundary that Geppetto still owns profile-resolution behavior and Pinocchio just composes sources.

**Commit (code):** `ef664c1` — `configdoc: compose inline and imported registries`

### What I did
- Extended `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles.go` with:
  - `ComposeRegistry(...)`
  - a private `composedRegistry` wrapper implementing `engineprofiles.Registry`
- Implemented behavior for:
  - inline-only registry use
  - imported-only fallback
  - inline-first same-slug resolution when no explicit registry slug is given
  - explicit registry-slug pass-through
- Extended `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles_test.go` with a mixed inline/imported precedence test.
- Revalidated:
  - `go test ./pkg/configdoc -count=1`
  - `golangci-lint run ./pkg/configdoc/...`

### Why
- A profile-first config document is only compelling if local inline overrides can coexist with imported team/shared catalogs.
- The key user expectation is that local inline definitions should win over imported same-slug definitions unless the caller explicitly selects a different registry.
- This wrapper lets us encode that rule without prematurely refactoring Geppetto’s own registry implementations.

### What worked
- The wrapper approach was enough to express the intended precedence rule cleanly.
- The new tests now prove two important behaviors:
  - inline `assistant` overrides imported `assistant`
  - imported `analyst` still resolves when no inline profile with that slug exists
- Keeping the composition logic in `pkg/configdoc` preserved the current boundary between app-owned document composition and Geppetto-owned registry resolution.

### What didn't work
- N/A in this tranche.

### What I learned
- There is a useful architectural seam between “build one effective catalog” and “resolve a profile from that catalog.” The first can stay app-owned for now, while the second remains Geppetto-owned.
- Inline-first precedence is easy to explain once encoded as “check inline first only when no explicit registry slug was requested.”

### What was tricky to build
- The subtle design choice was how to treat the no-selection case (`registry slug == empty`, `profile slug == empty`). I kept inline first there too, which is consistent with the idea that local config should dominate generic imported defaults. That choice may still want a second look once `profile.active` is wired end-to-end.

### What warrants a second pair of eyes
- Whether the composed registry should keep inline-first behavior even for the “no registry slug, no profile slug” default-resolution case.
- Whether later bootstrap code should expose the inline registry slug as a user-visible detail or keep it purely internal.

### What should be done in the future
- Add document-first resolver code that feeds `profile.registries` into this composed registry path.
- Add tests that exercise imported-only and inline+imported composition through the future high-level resolver, not just through the low-level `ComposeRegistry(...)` helper.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles_test.go`
- Focus on:
  - same-slug precedence without explicit registry slug
  - fallback to imported registries for non-inline slugs
  - avoiding unnecessary changes to Geppetto internals
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/configdoc -count=1
golangci-lint run ./pkg/configdoc/...
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w pkg/configdoc/profiles.go pkg/configdoc/profiles_test.go

go test ./pkg/configdoc -count=1

golangci-lint run ./pkg/configdoc/...

git add pkg/configdoc
git commit --no-verify -m "configdoc: compose inline and imported registries"
```

## Step 8: Add a resolved-files loader and prove real layer ordering with file-backed tests

At this point the package could decode documents, merge them, and build registries from the result, but it still lacked one practical bridge: taking the already-resolved `[]ResolvedConfigFile` list from the config-plan system and turning it into one effective document. That bridge matters because later bootstrap integration should not have to manually reopen and merge files itself. It should ask the config-document layer to do that in one place.

This tranche added that small but important convenience layer: `LoadResolvedDocuments(...)`. The function is intentionally simple—load each file in order, keep the per-file decoded documents, and merge them to an effective document. The accompanying test finally exercises the real user/repo/cwd/explicit layering story with actual temp files instead of pairwise synthetic docs.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Continue to the next implementation slice by bridging the new configdoc package to the existing resolved-file output of the config-plan system.

**Inferred user intent:** Keep the implementation moving toward practical bootstrap integration while still landing small, reviewable units.

**Commit (code):** `07edc1c` — `configdoc: load and merge resolved files`

### What I did
- Added:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved_test.go`
- Implemented:
  - `ResolvedDocuments`
  - `LoadResolvedDocuments(...)`
- Made the loader:
  - accept ordered `[]glazedconfig.ResolvedConfigFile`
  - decode each file through `LoadDocument(...)`
  - keep the per-file decoded documents
  - merge them to one effective document
- Added a file-backed test proving layer order across:
  - user
  - repo
  - cwd
  - explicit
- Verified behaviors in that test for:
  - repository accumulation
  - `profile.active` override
  - `profile.registries` replacement
  - same-slug profile merge behavior across files

### Why
- This is the natural bridge between the Glazed config-plan world and the future document-first bootstrap world.
- It also closes the remaining open merge-test task in the ticket with a real multi-file layering scenario.

### What worked
- The existing `LoadDocument(...)` + `MergeDocuments(...)` split made the resolved-files loader almost trivial to implement.
- The temp-file layering test gives much stronger confidence than purely synthetic merge-unit tests because it follows the same shape future bootstrap code will follow.

### What didn't work
- N/A in this tranche.

### What I learned
- The current package seams are holding up well: strict decode, merge, inline-registry bridge, and resolved-files loading all compose naturally.
- The next architectural jump is now clearly the document-first bootstrap integration, not more configdoc plumbing.

### What was tricky to build
- The main decision was scope control: it would have been easy to jump straight into `profilebootstrap` integration here, but keeping this tranche focused on `pkg/configdoc` made the review surface much cleaner and also completed the missing layering-test coverage first.

### What warrants a second pair of eyes
- Whether `ResolvedDocuments` should eventually grow provenance/explain fields directly, or whether provenance should live in a separate structure layered on top of it.
- Whether later high-level resolvers should expose both `Documents` and `Effective`, or only `Effective` plus provenance summaries.

### What should be done in the future
- Add provenance/explain data for merged entries.
- Start the document-first bootstrap integration in `profilebootstrap` using `LoadResolvedDocuments(...)` as the input seam.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved_test.go`
- Then read upward one layer if needed:
  - `pkg/configdoc/load.go`
  - `pkg/configdoc/merge.go`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/configdoc -count=1
golangci-lint run ./pkg/configdoc/...
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w pkg/configdoc/resolved.go pkg/configdoc/resolved_test.go

go test ./pkg/configdoc -count=1

golangci-lint run ./pkg/configdoc/...

git add pkg/configdoc
git commit --no-verify -m "configdoc: load and merge resolved files"
```
