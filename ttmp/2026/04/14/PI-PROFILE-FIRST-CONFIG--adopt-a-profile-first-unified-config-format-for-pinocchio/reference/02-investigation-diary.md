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
    - Path: README.md
      Note: Step 14 rewrites the main user-facing config guidance around unified app/profile/profiles documents
    - Path: cmd/examples/simple-redis-streaming-inference/main.go
      Note: Step 15 moves the stale example to Pinocchio middleware wiring so full repo builds succeed again
    - Path: cmd/pinocchio/cmds/js.go
      Note: Diary Step 9 records the JS runtime migration to the unified composed registry path
    - Path: cmd/pinocchio/doc/general/05-js-runner-scripts.md
      Note: Step 14 updates JS help to inherit profile.active and profile.registries from unified config
    - Path: cmd/pinocchio/main.go
      Note: Diary Step 12 records the top-level consumer staying on the unified app config path
    - Path: cmd/web-chat/main.go
      Note: Diary Step 10 records the top-level web-chat migration to unified bootstrap initialization
    - Path: cmd/web-chat/main_profile_registries_test.go
      Note: Diary Step 10 records the inline-profile web-chat bootstrap test
    - Path: examples/js/README.md
      Note: Step 14 aligns the JS example README with unified profile.* config terminology
    - Path: pkg/cmds/cmd_profile_registry_test.go
      Note: Step 15 isolates HOME/XDG/cwd so loaded-command tests stop ingesting ambient legacy config during make test lint
    - Path: pkg/cmds/profilebootstrap/engine_settings.go
      Note: Diary Step 9 records the hidden-base preservation and unified engine-settings path
    - Path: pkg/cmds/profilebootstrap/engine_settings_test.go
      Note: Diary Step 9 records focused bootstrap regression coverage
    - Path: pkg/cmds/profilebootstrap/local_profile_plan_test.go
      Note: |-
        Diary Step 11 records consumer-side exposure coverage for ResolveUnifiedConfig
        Diary Step 13 records the matching inline-profile base-preservation invariant test
    - Path: pkg/cmds/profilebootstrap/profile_selection.go
      Note: Diary Step 9 records the document-first selection and registry-chain integration
    - Path: pkg/cmds/profilebootstrap/repositories.go
      Note: Diary Step 12 records the collapse of the separate repository loader path
    - Path: pkg/cmds/profilebootstrap/repositories_test.go
      Note: Diary Step 12 records the merged app.repositories test coverage
    - Path: pkg/configdoc/explain.go
      Note: Diary Step 11 records the new document-merge provenance model
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
      Note: |-
        Diary Step 8 records the resolved-files loader over ordered ResolvedConfigFile inputs
        Diary Step 11 records explain data flowing through ResolvedDocuments
    - Path: pkg/configdoc/resolved_test.go
      Note: |-
        Diary Step 8 records the file-backed layering test across user/repo/cwd/explicit docs
        Diary Step 11 records focused explain-coverage tests
    - Path: pkg/configdoc/types.go
      Note: Diary Step 4 records the first code tranche that introduced the typed config document model
    - Path: pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md
      Note: Step 14 updates the primary profile-resolution help topic to use .pinocchio.yml and profile.* terminology
    - Path: pkg/ui/profileswitch/manager_test.go
      Note: Diary Step 13 records runtime-switching invariant coverage and the sparse-overlay test correction
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

## Step 9: Switch Pinocchio bootstrap to the unified document path and validate live profile behaviors

This tranche was the first real `profilebootstrap` integration step. Up to this point the unified config work lived in `pkg/configdoc`, but the active Pinocchio runtime still resolved selection and engine settings through the old mapper-first path. The goal here was to swap the bootstrap control plane underneath the existing runtime model without rewriting the whole command stack in one jump.

The implementation split the problem into three pieces:
- resolve the effective unified document from the existing config plan,
- derive profile-selection state from that document plus CLI overrides,
- keep the base inference settings model intact by preserving a hidden non-profile baseline and only overlaying selected profile settings on top.

### Prompt Context

**User prompt (verbatim):** “continue” and later “can you also validate with go run ./cmd/pinocchio code professional "hello" and try different (fialing or valid) profile configuration options (~/.config/pinocchio/profiles.yaml has a set of valid profiles, for example)”

**Assistant interpretation:** Land the document-first bootstrap tranche, then prove it with both focused tests and real command executions using valid and invalid profile configurations.

**Inferred user intent:** Do not stop at unit tests; verify the new behavior in the actual CLI, including both success and failure cases.

**Commit (code):** pending at diary-writing time

### What I did
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go` to:
  - resolve `ResolvedConfigFile` entries through the existing config plan,
  - load/merge them through `configdoc.LoadResolvedDocuments(...)`,
  - derive `ProfileSettings` from `profile.active` / `profile.registries`,
  - apply CLI `--profile` / `--profile-registries` as the explicit override layer,
  - compose imported registries with inline profiles through `ResolveUnifiedProfileRegistryChain(...)`,
  - switch repo/cwd discovery from `.pinocchio-profile.yml` to `.pinocchio.yml`,
  - narrow the parser-facing config mapper to the new `profile` block rather than legacy top-level runtime sections.
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/engine_settings.go` so:
  - `ResolveBaseInferenceSettings(...)` again returns the hidden base only,
  - `ResolveCLIEngineSettings(...)` overlays parsed non-profile values onto that hidden base before resolving the selected profile,
  - `ResolveCLIEngineSettingsFromBase(...)` uses the unified document-derived selection/registry chain instead of the old Geppetto mapper-first path.
- Updated JS runtime bootstrap in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js.go`
  - so it now uses the unified document-derived composed registry chain as well.
- Updated and expanded focused tests in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/local_profile_plan_test.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/engine_settings_test.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main_profile_registries_test.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js_test.go`
- Revalidated command packages and lint after the bootstrap swap.

### Why
- The profile-first format is not useful until the live bootstrap path reads `profile` / `profiles` from the new unified document instead of the legacy config mapper.
- Preserving the hidden-base model avoids mixing profile state into the non-profile baseline, which would break runtime profile switching later.
- Real CLI validation was necessary because the new behavior crosses config discovery, selection, registry composition, and actual inference startup.

### What worked
- The `configdoc.LoadResolvedDocuments(...)` seam was enough to keep the bootstrap integration reasonably small.
- The minimal Geppetto seam for this tranche turned out to be: reuse Geppetto’s existing types and registry resolution APIs, but keep the unified document loading/composition in Pinocchio. No Geppetto code changes were required for this slice.
- Focused tests passed after the integration:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/cmds/profilebootstrap ./cmd/web-chat ./cmd/pinocchio/cmds/... -count=1
golangci-lint run ./pkg/cmds/profilebootstrap/... ./cmd/web-chat ./cmd/pinocchio ./cmd/pinocchio/cmds/...
```

- Live CLI validation succeeded for:
  - imported registry selection with a valid real profile (`gpt-5-mini`) using `go run ./cmd/pinocchio ... code professional "hello"`
  - inline local `.pinocchio.yml` resolution in the current working directory (validated via a temp-built binary run from a temp directory).

### What didn't work
- My first live `go run` attempts used a temporary `HOME`, which also moved Go’s module/build caches and caused long dependency redownloads that hit the timeout before the command itself ran.
- The fix was to keep `GOPATH`, `GOMODCACHE`, and `GOCACHE` pinned to the real machine caches while isolating only Pinocchio’s config home.
- The first inline local-config validation attempt also failed to pick up `.pinocchio.yml` because I placed it only in a parent directory without making that directory a git root. That was correct behavior under the current plan. Re-running with `.pinocchio.yml` in the actual working directory succeeded.
- One real-profile attempt with `gpt-5-nano` failed at inference time with a provider auth error:

```text
Error: inference failed: responses api error: status=401 body=map[error:map[code:<nil> message:Missing bearer or basic authentication in header param:<nil> type:invalid_request_error]]
```

- That was not a bootstrap failure; it showed that this particular registry entry was not self-contained for live execution on this machine, so I re-ran the actual `hello` command with `gpt-5-mini`, which succeeded.
- During the first bootstrap implementation pass I also introduced a regression where `--profile` without registries silently fell back to baseline instead of erroring. Live validation caught it. I fixed `ResolveUnifiedProfileRegistryChain(...)` to reject that case unless inline profiles are present.

### What I learned
- The smallest useful document-first integration does not require a Geppetto redesign. Pinocchio can own document loading + profile-catalog composition and still hand the final result to existing Geppetto registry/profile primitives.
- Live validation matters for config migrations: the tests did not catch the silent missing-registry fallback, but the real CLI flow did immediately.
- The new local-file discovery behavior is now easy to explain: `.pinocchio.yml` is discovered from the current working directory directly, or from the git root when running inside a repository.

### What was tricky to build
- The hardest semantic edge was preserving the old hidden-base contract while changing where profile state comes from. The first implementation accidentally made `ResolveBaseInferenceSettings(...)` include parsed CLI overlays, which broke the distinction web-chat relies on. I corrected that so the hidden base remains “non-profile baseline only,” and explicit parsed values are layered separately when needed.

### What warrants a second pair of eyes
- The parser-facing `configFileMapper(...)` now intentionally only maps the new `profile` block and ignores the old top-level runtime sections. That matches the breaking-change goal, but it deserves a review pass because it changes what ordinary command parsing can absorb from config files.
- Inline/imported mixed stack resolution still deserves a deeper look later. This tranche composes the registries for selection/runtime use, but the broader “preserve cross-registry stack semantics everywhere” task remains open in the ticket.

### What should be done in the future
- Migrate `cmd/web-chat/main.go` from its remaining imported-registry-only bootstrap path onto `ResolveUnifiedProfileRegistryChain(...)`.
- Add more end-to-end CLI tests around `.pinocchio.yml` in git-root scenarios, not only working-directory scenarios.
- Add provenance/explain data for merged document entries so debug output can explain why a given `profile.active` or inline profile field won.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/engine_settings.go`
- Then review the consumer-side follow-on:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js.go`
- Confirm behavior with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/cmds/profilebootstrap ./cmd/web-chat ./cmd/pinocchio/cmds/... -count=1
golangci-lint run ./pkg/cmds/profilebootstrap/... ./cmd/web-chat ./cmd/pinocchio ./cmd/pinocchio/cmds/...
```

- For live validation, the successful cases in this tranche were:

```bash
# valid imported registry profile + real inference
HOME="$TMPHOME" GOPATH="/home/manuel/go" GOMODCACHE="/home/manuel/go/pkg/mod" GOCACHE="/home/manuel/.cache/go-build" XDG_CONFIG_HOME="$TMPHOME/xdg" \
  go run ./cmd/pinocchio --profile-registries "yaml:/home/manuel/.config/pinocchio/profiles.yaml" --profile gpt-5-mini code professional --non-interactive "hello"

# valid inline local config in cwd (run from a temp working directory containing .pinocchio.yml)
$BIN code professional --print-inference-settings --non-interactive "hello"
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w \
  pkg/cmds/profilebootstrap/profile_selection.go \
  pkg/cmds/profilebootstrap/engine_settings.go \
  pkg/cmds/profilebootstrap/local_profile_plan_test.go \
  pkg/cmds/profilebootstrap/engine_settings_test.go \
  cmd/web-chat/main_profile_registries_test.go \
  cmd/pinocchio/cmds/js.go \
  cmd/pinocchio/cmds/js_test.go

go test ./pkg/cmds/profilebootstrap ./cmd/web-chat ./cmd/pinocchio/cmds/... -count=1

golangci-lint run ./pkg/cmds/profilebootstrap/... ./cmd/web-chat ./cmd/pinocchio ./cmd/pinocchio/cmds/...

# live validation cases
HOME="$TMPHOME" GOPATH="/home/manuel/go" GOMODCACHE="/home/manuel/go/pkg/mod" GOCACHE="/home/manuel/.cache/go-build" XDG_CONFIG_HOME="$TMPHOME/xdg" \
  go run ./cmd/pinocchio --profile-registries "yaml:/home/manuel/.config/pinocchio/profiles.yaml" --profile gpt-5-mini code professional --print-inference-settings --non-interactive "hello"

HOME="$TMPHOME" GOPATH="/home/manuel/go" GOMODCACHE="/home/manuel/go/pkg/mod" GOCACHE="/home/manuel/.cache/go-build" XDG_CONFIG_HOME="$TMPHOME/xdg" \
  go run ./cmd/pinocchio --profile does-not-exist --profile-registries "yaml:/home/manuel/.config/pinocchio/profiles.yaml" code professional --print-inference-settings --non-interactive "hello"

HOME="$TMPHOME" GOPATH="/home/manuel/go" GOMODCACHE="/home/manuel/go/pkg/mod" GOCACHE="/home/manuel/.cache/go-build" XDG_CONFIG_HOME="$TMPHOME/xdg" \
  go run ./cmd/pinocchio --profile gpt-5-mini code professional --print-inference-settings --non-interactive "hello"

HOME="$TMPHOME" GOPATH="/home/manuel/go" GOMODCACHE="/home/manuel/go/pkg/mod" GOCACHE="/home/manuel/.cache/go-build" XDG_CONFIG_HOME="$TMPHOME/xdg" \
  go run ./cmd/pinocchio --profile-registries "yaml:/home/manuel/.config/pinocchio/profiles.yaml" --profile gpt-5-mini code professional --non-interactive "hello"
```

## Step 10: Move web-chat main onto the unified composed registry path

After the previous tranche, the remaining obvious runtime holdout was `cmd/web-chat/main.go`. The lower-level request resolver and runtime composition code were already capable of working with any `gepprofiles.Registry`; the issue was simply that web-chat was still initializing that registry through the old imported-registry-only bootstrap path. This step removes that mismatch and puts web-chat on the same unified-config/composed-registry initialization path as the rest of the active Pinocchio runtime.

### Prompt Context

**User prompt (verbatim):** “continue”

**Assistant interpretation:** Finish the next high-value runtime consumer migration by switching web-chat’s top-level runtime bootstrap to use the unified config result already provided by `profilebootstrap`.

**Inferred user intent:** Keep landing small destructive simplification passes until all main runtime consumers are on the new document-first path.

**Commit (code):** pending at diary-writing time

### What I did
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main.go` to:
  - replace `ResolveCLIProfileSelection(...)` with `ResolveUnifiedConfig(...)`
  - replace direct `geppettobootstrap.ResolveProfileRegistryChain(...)` usage with `profilebootstrap.ResolveUnifiedProfileRegistryChain(...)`
  - keep logging resolved external registry sources when present
  - pass the composed registry and default registry slug into the existing request resolver path
- Added a focused test in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main_profile_registries_test.go`
  - proving web-chat bootstrap accepts inline `.pinocchio.yml` profiles without requiring external registry files.
- Revalidated focused and broader command packages.

### Why
- Leaving web-chat on the old registry bootstrap path would have created a confusing split where the main CLI and JS respected inline profiles, but web-chat only saw external registries.
- This change completes the main runtime-consumer migration item for web-chat without touching the lower-level request/policy code.

### What worked
- The migration was smaller than the previous tranche because the request resolver already depended only on the abstract registry interface.
- The new inline-profile acceptance test makes the change concrete: web-chat can now initialize from a local `.pinocchio.yml` with no external registry list.

### What didn't work
- I missed one `context` import in the new inline-profile test on the first pass.
- Exact build error:

```text
cmd/web-chat/main_profile_registries_test.go:110:68: undefined: context
```

- The fix was just to add the import and rerun the focused validation.

### What I learned
- The earlier separation was the right one: once the registry composition lives in `profilebootstrap`, each runtime consumer migration becomes a small initialization change rather than another round of bespoke profile logic.
- Web-chat’s top-level bootstrap no longer needs to care whether profile data came from inline config or imported registries; it only needs a resolved registry interface.

### What was tricky to build
- The main care point was not to over-edit web-chat. The request resolver, runtime composer, and HTTP handlers did not need design changes here; only the top-level source of profile state needed to change.

### What warrants a second pair of eyes
- Whether any remaining debug/logging output in web-chat should explicitly mention when the active profile registry is `config-inline` versus an imported registry slug.
- Whether future UI/API behavior should expose inline-vs-imported provenance more directly when listing available profiles.

### What should be done in the future
- Add end-to-end web-chat tests that drive actual request resolution from a temp `.pinocchio.yml`, not only bootstrap initialization.
- Continue with provenance/explain data so runtime debug surfaces can explain why a profile came from cwd, repo, or explicit config.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main_profile_registries_test.go`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./cmd/web-chat ./pkg/cmds/profilebootstrap -count=1
golangci-lint run ./cmd/web-chat ./pkg/cmds/profilebootstrap/...
go test ./pkg/cmds/profilebootstrap ./cmd/web-chat ./cmd/pinocchio ./cmd/pinocchio/cmds/... -count=1
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w cmd/web-chat/main.go cmd/web-chat/main_profile_registries_test.go

go test ./cmd/web-chat ./pkg/cmds/profilebootstrap -count=1

golangci-lint run ./cmd/web-chat ./pkg/cmds/profilebootstrap/...

go test ./pkg/cmds/profilebootstrap ./cmd/web-chat ./cmd/pinocchio ./cmd/pinocchio/cmds/... -count=1
```

## Step 11: Add structured merge provenance/explain data to `configdoc`

With the main runtime consumers now on the unified document path, the next highest-value missing piece was explainability. The config system could merge documents correctly, but the merged result itself still could not answer “why did this value win?” in a structured way. This tranche adds that missing layer directly to `configdoc` instead of trying to force all document-level provenance through Glazed field logs.

The implementation stays aligned with the design decision from the discussion: provenance should live at the layer where the transformation happens. For the unified config document, that means `configdoc` owns the merge explanation, and consumers like `ResolveUnifiedConfig(...)` simply expose it.

### Prompt Context

**User prompt (verbatim):** “alright, perfect, Add tasks to implement it , and then work on it task by task, committing at appropriate intervals and keeping a detailed diary”

**Assistant interpretation:** Turn the open provenance idea into a concrete backlog item, implement it in focused slices, and record the results in the ticket as each slice lands.

**Inferred user intent:** Keep the implementation disciplined and reviewable; do not just talk about provenance, actually wire it into the current unified config path.

**Commit (code):** pending at diary-writing time

### What I did
- Expanded the open provenance task in `tasks.md` into concrete sub-tasks before writing code.
- Added `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/explain.go` with:
  - `ProvenanceOperation`
  - `ProvenanceEntry`
  - `DocumentExplain`
  - recording helpers keyed by document paths such as:
    - `profile.active`
    - `profile.registries`
    - `app.repositories`
    - `profiles.<slug>.display_name`
    - `profiles.<slug>.stack`
    - `profiles.<slug>.inference_settings`
    - `profiles.<slug>.extensions`
- Extended `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved.go` so `ResolvedDocuments` now carries `Explain`, and `LoadResolvedDocuments(...)` records merge provenance as each resolved file is applied.
- Extended `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved_test.go` with focused assertions for:
  - replacement provenance for `profile.active`
  - replacement provenance for `profile.registries`
  - append+dedupe provenance for `app.repositories`
  - same-slug inline profile field contributions
  - explicit dedupe-skipped repository entries
- Added a consumer-side regression test in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/local_profile_plan_test.go`
  - proving `ResolveUnifiedConfig(...)` exposes the new explain data to runtime consumers.

### Why
- The document-first path is now the real source of truth for `app`, `profile`, and inline `profiles`, so it needs its own first-class provenance model.
- Keeping document provenance in `configdoc` is cleaner than flattening the whole merged document into fake field logs.
- Once this data exists, later debug surfaces can project it however they want without losing the original merge story.

### What worked
- The chosen model was small but expressive enough: `ByPath map[string][]ProvenanceEntry` plus a shared operation vocabulary.
- It was straightforward to record useful metadata during merge, especially for:
  - repository additions
  - repository dedupe skips
  - previous values for replacement fields
  - whether an inline profile field was creating a new profile or merging into an existing one
- The new consumer-side test confirmed the explain data is not trapped inside `configdoc`; it is already reachable through `ResolveUnifiedConfig(...)`.

### What didn't work
- The first test pass had a small Go scoping mistake in `resolved_test.go` where I declared `got` inside an `if` and then referenced it afterward.
- Exact compiler error:

```text
pkg/configdoc/resolved_test.go:190:20: undefined: got
pkg/configdoc/resolved_test.go:194:27: undefined: got
pkg/configdoc/resolved_test.go:199:56: undefined: got
```

- The fix was just to bind `got` outside the conditional before checking its concrete type.

### What I learned
- A path-keyed explain structure is a good fit for this codebase: it is light enough to add now, but structured enough to grow into richer debug surfaces later.
- The earlier concern was correct: trying to keep all provenance only in field logs would have been awkward for inline profile catalog merges. Keeping the explanation in `configdoc` feels much cleaner.

### What was tricky to build
- The subtle part was deciding how much detail to record immediately. I intentionally stopped at field/block-level merge provenance rather than trying to deep-diff every nested inference-settings scalar. That keeps the implementation useful now without turning it into a giant explain engine in one pass.

### What warrants a second pair of eyes
- Whether the current operation vocabulary is the right long-term naming (`replace`, `merge`, `append-dedupe`) or whether later UI/debug consumers will want even more explicit terms.
- Whether repository explain metadata should eventually record the exact layer ordering as a first-class field instead of inferring it from the ordered entries.

### What should be done in the future
- Project selected document provenance into runtime debug output (for example, active profile selection or future config-explain commands).
- Decide whether nested inference-settings explanations should stay field-block-level or become more granular later.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/explain.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved_test.go`
- Then confirm the consumer exposure path in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/local_profile_plan_test.go`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/configdoc -count=1
golangci-lint run ./pkg/configdoc/...
go test ./pkg/cmds/profilebootstrap -count=1
golangci-lint run ./pkg/cmds/profilebootstrap/...
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w \
  pkg/configdoc/explain.go \
  pkg/configdoc/resolved.go \
  pkg/configdoc/resolved_test.go \
  pkg/cmds/profilebootstrap/local_profile_plan_test.go

go test ./pkg/configdoc -count=1

golangci-lint run ./pkg/configdoc/...

go test ./pkg/cmds/profilebootstrap -count=1

golangci-lint run ./pkg/cmds/profilebootstrap/...
```

## Step 12: Fold repository loading fully into `app.repositories` and collapse the old repository-only loader path

With the unified document and provenance work in place, the next cleanup target was the remaining app-setting holdout: repository loading. The code still had a separate mini-loader in `pkg/cmds/profilebootstrap/repositories.go` that decoded repository paths through a custom section mapper instead of reading them from the same merged unified document used everywhere else. This step removes that split-brain setup.

The implementation is intentionally simple: repository loading now just reuses `ResolveUnifiedConfig(nil)` and returns `resolved.Effective.App.Repositories`. That means repository discovery now follows the exact same file discovery and merge behavior as the rest of the unified config model, including repo/cwd `.pinocchio.yml` layers and the `app.repositories` append+dedupe semantics already defined in `configdoc`.

### Prompt Context

**User prompt (verbatim):** “go ahead”

**Assistant interpretation:** Continue into Phase 5 by moving the remaining app-owned repository settings off the separate loader path and onto the unified config document.

**Inferred user intent:** Finish the app-settings consolidation rather than leaving one last special-case config path alive.

**Commit (code):** pending at diary-writing time

### What I did
- Rewrote `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories.go` so `ResolveRepositoryPaths()` is now just a thin wrapper over `ResolveUnifiedConfig(nil)`.
- Deleted the old repository-specific parsing machinery from that file:
  - `RepositorySettingsSectionSlug`
  - `RepositorySettings`
  - `NewRepositorySettingsSection()`
  - `MapPinocchioRepositoriesConfig(...)`
- Rewrote `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories_test.go` to prove repository lists merge across:
  - home config
  - XDG config
  - repo `.pinocchio.yml`
  - cwd `.pinocchio.yml`
  - including dedupe behavior
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/main.go` comments/logging to reflect that repository loading now comes from the unified layered config document.

### Why
- `app.repositories` belongs in the unified config model, not in a side channel.
- Keeping repository loading separate would preserve unnecessary duplicate discovery/merge logic and weaken the “one document is the source of truth” design.
- This change completes the intended distinction discussed earlier: repositories are app-owned settings, so they should live in `app`, not in another loader or in the profile/runtime machinery.

### What worked
- The collapse was smaller than it looked because the earlier `ResolveUnifiedConfig(...)` work already did the hard part.
- The rewritten repository test now proves the desired semantics much better than the old test, because it exercises the real multi-layer unified config path instead of the old replacement-based custom mapper path.
- Broader command-package validation stayed green after the change, so top-level command discovery still works.

### What didn't work
- N/A in this tranche.

### What I learned
- The earlier architecture work paid off again: once the unified document path existed, removing the special-case repository loader was almost entirely a deletion/refactor task.
- `app.repositories` now really behaves like the rest of the unified document, which makes the mental model much cleaner.

### What was tricky to build
- The only subtle part was making sure the rewritten test reflected the new intended semantics: the old test asserted “highest precedence wins,” but Phase 5 should assert merge+dedupe across user/repo/cwd layers instead.

### What warrants a second pair of eyes
- Whether repository loading should eventually accept explicit `--config-file` at top-level command discovery time. Right now `ResolveRepositoryPaths()` intentionally uses `ResolveUnifiedConfig(nil)`, which is consistent with the current startup flow but does not parse root flags before command loading.
- Whether any future repository-related debug output should expose `ResolvedDocuments.Explain` entries for `app.repositories` directly.

### What should be done in the future
- Verify runtime profile switching still preserves a non-profile baseline and rebuilds from base rather than prior merged state.
- Revalidate any remaining command/example paths that still assume old top-level runtime config shapes.
- Later, consider a user-facing config explain/debug surface that shows `app.repositories` provenance.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories_test.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/main.go`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/cmds/profilebootstrap ./cmd/pinocchio -count=1
golangci-lint run ./pkg/cmds/profilebootstrap/... ./cmd/pinocchio
go test ./pkg/cmds/profilebootstrap ./cmd/pinocchio ./cmd/pinocchio/cmds/... ./cmd/web-chat -count=1
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w \
  pkg/cmds/profilebootstrap/repositories.go \
  pkg/cmds/profilebootstrap/repositories_test.go \
  cmd/pinocchio/main.go

go test ./pkg/cmds/profilebootstrap ./cmd/pinocchio -count=1

golangci-lint run ./pkg/cmds/profilebootstrap/... ./cmd/pinocchio

go test ./pkg/cmds/profilebootstrap ./cmd/pinocchio ./cmd/pinocchio/cmds/... ./cmd/web-chat -count=1
```

## Step 13: Lock in the rebuilt-from-base invariant for profile switching and sweep remaining active old-shape assumptions

This tranche closes the remaining Option A work. The key goal was to prove the runtime invariant, not just assume it: switching from one profile to another must rebuild from the original non-profile base rather than inheriting overrides from the previously selected profile. The most important place to prove that is the interactive profile-switch manager, because that is where a persistent session can change profiles repeatedly over time.

I added focused tests in `pkg/ui/profileswitch/manager_test.go` for both flavors of runtime overlay behavior we care about now:
- imported-registry-style switching via the `profileswitch.Manager`
- inline unified-config overlays via `ResolveCLIEngineSettings(...)`

I also swept the active `cmd/`, `pkg/`, and `examples/` current-source tree for old top-level runtime config assumptions. The only remaining current-source occurrence was the intentional `LegacyLocalOverrideFileName` constant used to reject `.pinocchio-profile.yml`; there were no remaining active command/example code paths depending on old `ai-chat` / `profile-settings` config shape.

### Prompt Context

**User prompt (verbatim):** after discussing Option A, the user agreed to proceed.

**Assistant interpretation:** Finish the remaining runtime migration hardening work by proving the base-rebuild invariant in tests and verifying that no active command/example paths still depend on old config shape.

**Inferred user intent:** Move beyond architectural claims and make the runtime semantics explicit, while also confirming that the migration is truly complete for active code paths.

**Commit (code):** pending at diary-writing time

### What I did
- Expanded the remaining Phase 6 tasks in `tasks.md` into concrete sub-items before implementing them.
- Added focused tests in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/ui/profileswitch/manager_test.go`
- Those tests prove:
  - switching from profile `alpha` to sparse profile `beta` rebuilds from the original base instead of keeping `alpha`'s engine override
  - sparse profile overlays leave unspecified values at the base setting while still applying the fields they do specify
- Added a matching inline-config test in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/local_profile_plan_test.go`
- That test proves an inline profile in `.pinocchio.yml` that only overrides API credentials keeps the base engine value intact.
- Swept active code/example paths with ripgrep across `cmd/`, `pkg/`, and `examples/` for:
  - `ai-chat:`
  - `profile-settings:`
  - `.pinocchio-profile.yml`
- Revalidated the active runtime-switching consumers and command/example packages.

### Why
- This invariant is one of the most important semantics in the whole migration: profile changes must not accumulate stale overrides from prior profiles.
- Without explicit tests, it would be easy for a future refactor to accidentally switch from “rebuild from base” to “mutate the previous final settings.”
- The active-code sweep was necessary to support the claim that the migration is complete for current runtime consumers, not only for tests and docs.

### What worked
- The new manager tests directly exercised the correct seam: `Manager.Resolve()` and `Manager.Switch()` are where imported-registry profile switching actually merges `base + resolved profile`.
- The matching inline-profile test showed the same invariant holds on the unified config path too.
- The active-code sweep came back clean apart from the deliberate legacy filename rejection constant, which is expected and desirable.

### What didn't work
- My first version of the manager tests mistakenly built “sparse” profile overlays with `settings.NewInferenceSettings()`. That constructor applies defaults, so the test profiles unintentionally carried a default engine (`gpt-4`) and failed the invariant test for the wrong reason.
- The initial failing output was:

```text
--- FAIL: TestManagerSwitch_RebuildsFromBaseInsteadOfKeepingPriorProfileOverrides
    manager_test.go:133: expected switch to rebuild from base engine, got "gpt-4"
--- FAIL: TestManagerResolve_LeavesBaseValuesWhenProfileOmitsThem
    manager_test.go:195: expected omitted engine to stay at base value, got "gpt-4"
```

- The fix was to construct truly sparse profile overlays with partial `InferenceSettings` structs instead of `NewInferenceSettings()`.
- I also hit one tiny compile issue in the inline-profile test from an unused `repoDir` binding; that was fixed by discarding the value.

### What I learned
- `settings.NewInferenceSettings()` is the wrong tool for representing a sparse profile patch in tests because it materializes defaults.
- The imported-registry switching invariant and the inline unified-config invariant are conceptually the same, but they are best proven at different seams:
  - manager tests for imported profile switching
  - `ResolveCLIEngineSettings(...)` tests for inline unified config

### What was tricky to build
- The tricky part was making the test overlays genuinely sparse. Once I stopped using constructors that inject defaults, the tests became a faithful check of the real invariant instead of an artifact of the settings defaults layer.

### What warrants a second pair of eyes
- Whether the runtime-switching tests should eventually be extended all the way up into an interactive UI/backend test, not just the manager and profilebootstrap seams.
- Whether the active-code sweep logic should later become a scripted validation target instead of a one-off grep in the diary.

### What should be done in the future
- Move into the remaining rollout work: failure tests for old config shape and docs/examples cleanup.
- Update user-facing docs that still mention `.pinocchio-profile.yml`; those remaining references are now documentation debt, not active runtime behavior.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/ui/profileswitch/manager_test.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/local_profile_plan_test.go`
- Then confirm the broader sweep/validation results with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

go test ./pkg/ui/profileswitch ./pkg/cmds/profilebootstrap -count=1

golangci-lint run ./pkg/ui/profileswitch/... ./pkg/cmds/profilebootstrap/...

rg -n 'ai-chat:|profile-settings:|\.pinocchio-profile\.yml' cmd pkg examples \
  --glob '!**/*_test.go' --glob '!**/*.md' --glob '!**/*test*.yaml' --glob '!**/*test*.yml'

go test ./pkg/cmds/profilebootstrap ./pkg/ui/profileswitch ./cmd/pinocchio ./cmd/pinocchio/cmds/... ./cmd/web-chat ./cmd/examples/simple-chat ./cmd/agents/simple-chat-agent -count=1

golangci-lint run ./pkg/cmds/profilebootstrap/... ./pkg/ui/profileswitch/... ./cmd/pinocchio ./cmd/pinocchio/cmds/... ./cmd/web-chat
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w \
  pkg/ui/profileswitch/manager_test.go \
  pkg/cmds/profilebootstrap/local_profile_plan_test.go

go test ./pkg/ui/profileswitch ./pkg/cmds/profilebootstrap -count=1

golangci-lint run ./pkg/ui/profileswitch/... ./pkg/cmds/profilebootstrap/...

rg -n 'ai-chat:|profile-settings:|\.pinocchio-profile\.yml' cmd pkg examples \
  --glob '!**/*_test.go' --glob '!**/*.md' --glob '!**/*test*.yaml' --glob '!**/*test*.yml'

go test ./pkg/cmds/profilebootstrap ./pkg/ui/profileswitch ./cmd/pinocchio ./cmd/pinocchio/cmds/... ./cmd/web-chat ./cmd/examples/simple-chat ./cmd/agents/simple-chat-agent -count=1

golangci-lint run ./pkg/cmds/profilebootstrap/... ./pkg/ui/profileswitch/... ./cmd/pinocchio ./cmd/pinocchio/cmds/... ./cmd/web-chat
```

## Step 14: Rewrite user-facing Pinocchio docs to teach only the unified `app / profile / profiles` model

With the runtime migration and invariant hardening in place, the next important rollout step was documentation cleanup. The code now treats the unified config document as the real source of truth, but several user-facing docs still taught legacy config keys such as `profile-settings` and old local filenames like `.pinocchio-profile.yml`. This step makes the published Pinocchio story match the implemented system.

The scope for this tranche was deliberately the user-facing Pinocchio surface, not the whole migration package. I updated the top-level README, the main profile-resolution topic, the JS runner help page, and the JS examples README so they now describe unified config documents, `.pinocchio.yml` local overrides, and `profile.active` / `profile.registries` rather than the removed top-level config shape.

### Prompt Context

**User prompt (verbatim):** "3. add/update docs to teach only app / profile / profiles"

**Assistant interpretation:** Update the active Pinocchio user-facing documentation so it consistently teaches the unified config document model and no longer instructs users to rely on legacy config keys or local filenames.

**Inferred user intent:** Make the docs match the implementation before doing further rollout work, so new users learn only the new config model.

**Commit (code):** pending at diary-writing time

### What I did
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/README.md` to:
  - teach unified config documents with `app`, `profile`, and `profiles`
  - switch profile precedence explanations from `profile-settings.*` to `profile.*`
  - describe local project overrides as `.pinocchio.yml`
  - fix the JS example link to a relative repository link
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md` to:
  - replace `.pinocchio-profile.yml` with `.pinocchio.yml`
  - update the provenance example from `profile-settings.profile` to `profile.active`
  - remove the user-facing mention that `web-chat` mounts a `profile-settings` section
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md` so JS help now describes inheritance from `profile.registries` and `profile.active`
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/examples/js/README.md` to describe unified `profile.*` config inheritance and fixed its example link
- Re-swept the user-facing doc/example surface for:
  - `profile-settings`
  - `.pinocchio-profile.yml`
  - `ai-chat:`
  - `openai-chat:`

### Why
- The implementation is now intentionally breaking old config shape. The docs must reinforce the new model rather than implicitly advertise removed behavior.
- If the docs keep teaching `profile-settings` or `.pinocchio-profile.yml`, users will keep creating config that the runtime now rejects.
- The README and help topics are the highest-leverage places to make the rollout coherent.

### What worked
- The main remaining drift was concentrated in a small set of obvious docs, so the cleanup was straightforward once the grep surface was narrowed.
- After the rewrite, the only remaining user-facing grep hit was an intentional warning in `README.md` telling readers not to use legacy shapes.
- The JS docs and example README now tell the same story as the main README.

### What didn't work
- My first grep used a bad relative ticket-doc path and produced an avoidable shell error while searching:

```text
rg: ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG: No such file or directory (os error 2)
```

- The fix was simply to rerun the search with the correct paths and focus it on the actual user-facing doc directories.

### What I learned
- The remaining documentation drift was mostly not in code comments or tests, but in the highest-visibility user docs: the README and primary help topics.
- The new unified model is much easier to explain once the docs stop interleaving two generations of terminology.

### What was tricky to build
- The subtle part was deciding what to do with legacy terminology in the docs. I kept one explicit warning in `README.md` so readers know what *not* to use, but otherwise removed legacy config-shape teaching from the user-facing flow. That balances clarity with migration safety.

### What warrants a second pair of eyes
- Whether we should also add a dedicated migration page immediately, since the README now points people away from legacy shapes but does not yet provide a step-by-step rewrite recipe.
- Whether any remaining Geppetto-facing migration docs should be updated in the next tranche so the broader workspace tells one consistent story.

### What should be done in the future
- Write the explicit user-facing migration guide from old top-level runtime config to the unified `app / profile / profiles` format.
- Add failure tests for old-format config files and legacy local filenames.
- Update the remaining Geppetto migration/help docs that still imply old app-config shape assumptions.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/README.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/examples/js/README.md`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

rg -n 'profile-settings|\.pinocchio-profile\.yml|openai-chat:|ai-chat:' \
  examples README.md pkg/doc cmd/pinocchio/doc \
  --glob '*.md' --glob '*.yaml' --glob '*.yml' --glob '*.js'

git diff --check

docmgr doctor --ticket PI-PROFILE-FIRST-CONFIG --stale-after 30
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

rg -n 'profile-settings|\.pinocchio-profile\.yml|openai-chat:|ai-chat:' \
  README.md pkg/doc cmd/pinocchio/doc \
  --glob '*.md'

rg -n 'profile-settings|\.pinocchio-profile\.yml|openai-chat:|ai-chat:' \
  examples README.md pkg/doc cmd/pinocchio/doc \
  --glob '*.md' --glob '*.yaml' --glob '*.yml' --glob '*.js'

git diff --check

docmgr doctor --ticket PI-PROFILE-FIRST-CONFIG --stale-after 30
```

## Step 15: Fix full-repo validation fallout after the unified-config rollout

After the doc cleanup tranche, a full `make test lint` run exposed two pieces of fallout that were not covered by the narrower package-level validations. One stale example still referenced the deleted Geppetto Cobra middleware helper, and one set of loaded-command tests was accidentally ingesting ambient user config during full-repo execution. This step fixes both so the repo validates cleanly under the real top-level workflow.

The fixes were intentionally small and surgical. I replaced the stale example wiring with Pinocchio's current middleware helper and made the loaded-command tests hermetic by sandboxing `HOME`, `XDG_CONFIG_HOME`, and the working directory before the runtime config resolver runs.

### Prompt Context

**User prompt (verbatim):** "make test lint is how i found it, btw"

**Assistant interpretation:** Use the full repository validation target as the source of truth, fix the regressions it exposed, and make sure the final validation uses that same top-level command.

**Inferred user intent:** Do not stop at focused tests; make the whole repository pass the real developer workflow.

**Commit (code):** `70a3f62` — `test: fix profile first fallout in full repo validation`

### What I did
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/examples/simple-redis-streaming-inference/main.go` to use:
  - `pinocchiocmds.GetPinocchioCommandMiddlewares`
  - instead of the deleted `geppettosections.GetCobraCommandGeppettoMiddlewares`
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/cmd_profile_registry_test.go` with `isolateConfigDiscovery(t)` so the loaded-command tests now:
  - set `HOME` to a temp dir
  - set `XDG_CONFIG_HOME` to a temp dir
  - `Chdir` into a temp directory
- Revalidated with:
  - focused `go test` on the affected packages
  - focused `golangci-lint`
  - full `make test lint`

### Why
- `make test lint` is the real integration gate for this repo, and it exercises more surface area than the narrower package tests.
- The profile-first rollout changed config discovery semantics, so tests that were previously tolerant of ambient user config needed to become explicit and hermetic.
- The stale example call site was just leftover fallout from the earlier deletion of the legacy Geppetto middleware helper.

### What worked
- The example built cleanly as soon as it switched to Pinocchio's shared middleware helper.
- Sandboxing config discovery fixed the loaded-command tests without needing to reintroduce compatibility behavior.
- The full top-level validation succeeded after those two fixes.

### What didn't work
- The initial top-level failure surfaced exactly these errors:

```text
cmd/examples/simple-redis-streaming-inference/main.go:239:89: undefined: geppettosections.GetCobraCommandGeppettoMiddlewares
```

and then loaded-command tests failed by decoding an old-shape config document from ambient config discovery:

```text
RunIntoWriter: resolve engine profile settings for command run: decode config document: yaml: unmarshal errors:
  line 7: field profile-settings not found in type configdoc.Document
  line 12: field ai-chat not found in type configdoc.Document
  line 18: field openai-chat not found in type configdoc.Document
  line 22: field claude-chat not found in type configdoc.Document
  line 27: field repositories not found in type configdoc.Document
```

### What I learned
- Full-repo validation is still valuable even after good focused-package coverage, because it catches stale example binaries and tests that accidentally depend on the local machine environment.
- The unified-config rollout makes hermetic config-discovery tests more important than before.

### What was tricky to build
- The non-obvious part was that `pkg/cmds/cmd_profile_registry_test.go` did not itself write a legacy config file. The failure came from ambient config discovery through the user's real environment, so the right fix was hermetic isolation rather than changing the runtime back toward compatibility.

### What warrants a second pair of eyes
- Whether there are any other older loaded-command tests elsewhere in the repo that should adopt the same config-discovery sandbox helper.
- Whether `make test lint` should eventually include a guard that enforces a hermetic config environment by default.

### What should be done in the future
- Continue with the remaining Phase 7 / Phase 8 rollout work.
- Consider a small shared test helper for config-discovery isolation if more packages need the same pattern.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/examples/simple-redis-streaming-inference/main.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/cmd_profile_registry_test.go`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

go test ./cmd/examples/simple-redis-streaming-inference ./pkg/cmds -count=1

golangci-lint run ./cmd/examples/simple-redis-streaming-inference ./pkg/cmds/...

make test lint
```

### Technical details

Commands run for this tranche:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio

gofmt -w cmd/examples/simple-redis-streaming-inference/main.go pkg/cmds/cmd_profile_registry_test.go

go test ./cmd/examples/simple-redis-streaming-inference ./pkg/cmds -count=1

golangci-lint run ./cmd/examples/simple-redis-streaming-inference ./pkg/cmds/...

make test lint
```
