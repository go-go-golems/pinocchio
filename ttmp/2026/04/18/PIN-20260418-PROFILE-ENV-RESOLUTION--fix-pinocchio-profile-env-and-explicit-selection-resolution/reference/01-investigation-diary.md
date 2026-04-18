---
Title: Investigation diary
Ticket: PIN-20260418-PROFILE-ENV-RESOLUTION
Status: active
Topics:
    - pinocchio
    - profiles
    - cli
    - bootstrap
    - configuration
    - runtime
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/cli/bootstrap/profile_registry.go
      Note: Validation site that explains the error message
    - Path: pinocchio/README.md
      Note: Documented expected registry and profile precedence
    - Path: pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md
      Note: Documentation that states the default registry should still be discovered
    - Path: pinocchio/pkg/cmds/profilebootstrap/profile_selection.go
      Note: Primary Pinocchio bootstrap wrapper inspected during the investigation
ExternalSources: []
Summary: ""
LastUpdated: 2026-04-18T13:28:05.722923283-04:00
WhatFor: ""
WhenToUse: ""
---


# Investigation Diary

## Goal

Capture the investigation that led to the Pinocchio profile-resolution ticket, including the failing command output, the current architecture boundaries, the docs/code mismatch, and the implementation direction for the fix.

## Step 1: Reproduce the profile-selection failure and map the current bootstrap flow

I started by recreating the issue from the user report and then tracing where Pinocchio resolves profile settings, registry sources, and final inference settings. The main discovery was that the failure is not in the model runtime itself; it happens earlier when the profile-selection code refuses to proceed without registry sources, even though the docs say the default `profiles.yaml` should be discoverable when present.

The work so far points to a shared bootstrap fix rather than a one-off command patch. The shared `profilebootstrap` wrapper, the command-level helpers, and the web-chat/JS entrypoints all participate in the same profile contract, so the safest direction is to centralize default registry discovery in the Pinocchio bootstrap layer and keep the per-command guards as sanity checks only.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket to fix the bug that pinocchio cannot apparently properly use the PINOCCHIO_PROFILE env variable or a profile is passed, despite it seemingly properly looking up the default profile (even if the key is invalid).

❯ pinocchio --profile gemini-2.5-pro code professional hello
Error: resolve engine profile settings for command run: validation error (profile-settings.profile-registries): must be configured when profile-settings.profile is set
2026-04-17--byok-host on  main 
❯ pinocchio code professional hello               
2026-04-18T13:25:45.9204777-04:00 ERR OpenAI streaming request failed error="error, status code: 401, status: 401 Unauthorized, message: You didn't provide an API key. You need to provide your API key in an Authorization header using Bearer auth (i.e. Authorization: Bearer YOUR_KEY), or as the password field (with blank username) if you're accessing the API from your browser and are prompted for a username and password. You can obtain an API key from https://platform.openai.com/account/api-keys."

[error] error, status code: 401, status: 401 Unauthorized, message: You didn't provide an API key. You need to provide your API key in an Authorization header using Bearer auth (i.e. Authorization: Bearer YOUR_KEY), or as the password field (with blank username) if you're accessing the API from your browser and are prompted for a username and password. You can obtain an API key from https://platform.openai.com/account/api-keys.
Error: inference failed: error, status code: 401, status: 401 Unauthorized, message: You didn't provide an API key. You need to provide your API key in an Authorization header using Bearer auth (i.e. Authorization: Bearer YOUR_KEY), or as the password field (with blank username) if you're accessing the API from your browser and are prompted for a username and password. You can obtain an API key from https://platform.openai.com/account/api-keys.
2026-04-17--byok-host on  main 
❯ PINOCCHIO_PROFILE=gemini-2.5-pro pinocchio code professional hello
Error: resolve engine profile settings for command run: validation error (profile-settings.profile-registries): must be configured when profile-settings.profile is set

reate a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file
  references.
  It should be very clear and detailed. Store in the ticket and the nupload to remarkable.

Keep a diary as you work."

**Assistant interpretation:** Create a new docmgr ticket, investigate the profile-resolution bug deeply, write a long-form design/implementation guide for an intern, store it in the ticket workspace, upload the bundle to reMarkable, and maintain a chronological diary while working.

**Inferred user intent:** Produce a durable, reviewable research deliverable that explains the bug, the system architecture behind it, and the path to fixing it, while also preserving a work log for future reviewers.

**Commit (code):** N/A — research and documentation only so far

### What I did

- Created a new docmgr ticket workspace under Pinocchio’s `ttmp/` root.
- Added a design document and a diary/reference document.
- Traced the code path through the Pinocchio bootstrap wrapper, Geppetto profile selection, registry-chain loading, and command-level guards.
- Compared the implementation against the Pinocchio docs that promise default registry discovery.

### Why

- The error happens before inference, so the investigation needed to follow the bootstrap path instead of the runtime path.
- The bug appears to affect more than one entrypoint, so the fix needs to be shared and not duplicated per command.

### What worked

- `docmgr status --summary-only` confirmed the Pinocchio docs root and existing ticket structure.
- `docmgr ticket create-ticket ...` created a fresh workspace for the bug.
- `docmgr doc add ...` created the design and diary documents with the expected frontmatter.
- Reading the relevant files showed a consistent pattern: profile selection is resolved first, then registry loading, then merge-into-base.

### What didn't work

- The reported user command fails early with:
  - `validation error (profile-settings.profile-registries): must be configured when profile-settings.profile is set`
- The no-profile command path reaches inference but fails later with provider credentials, which initially made the issue look like a runtime bug even though the real problem is earlier in bootstrap.
- The current implementation and some tests still suggest “no implicit registry fallback,” while the docs say the default `profiles.yaml` should be discovered when present.

### What I learned

- `PINOCCHIO_PROFILE` and `--profile` are not enough by themselves in the current code because registry discovery is the missing piece.
- The shared `profilebootstrap` wrapper is the right place to fix the behavior because it is used across multiple Pinocchio entrypoints.
- The helper path in `pkg/cmds/helpers/parse-helpers.go` is another independent consumer that can drift unless it is aligned with the shared profile bootstrap behavior.

### What was tricky to build

- The confusing part was separating the profile-resolution failure from the later OpenAI 401. Those are two different layers of the system.
- Another sharp edge was the docs/code mismatch around default registry discovery: the docs say it exists, but the current code path and tests do not consistently implement it.
- It was also easy to conflate “default profile” with “default registry.” The bug is about discovering the registry source that contains the default profile, not about the model provider key itself.

### What warrants a second pair of eyes

- Whether the default registry discovery helper should live only in `pinocchio/pkg/cmds/profilebootstrap` or be shared more broadly.
- Whether any command-level validation should be simplified after the shared fix lands.
- Whether the tests that currently assert no fallback should be updated or removed, and what the new contract should be.

### What should be done in the future

- Implement the Pinocchio-local default registry discovery helper.
- Update shared profile-selection tests to prove `PINOCCHIO_PROFILE` and `--profile` work when the default `profiles.yaml` is present.
- Update helper consumers such as `ParseGeppettoLayers()` so they do not keep a second profile-selection contract.
- Run the affected command and bootstrap tests after the implementation lands.

### Code review instructions

- Start with `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go` and `geppetto/pkg/cli/bootstrap/profile_registry.go`.
- Then review `cmd/web-chat/main.go`, `cmd/pinocchio/cmds/js.go`, and `pkg/cmds/helpers/parse-helpers.go` for duplicated validation or secondary profile-selection logic.
- Validate by running the reported commands again and confirming that profile selection now reaches registry resolution instead of failing on empty `profile-registries`.

### Technical details

- Command that reproduces the issue:
  - `PINOCCHIO_PROFILE=gemini-2.5-pro pinocchio code professional hello`
- Current early failure:
  - `validation error (profile-settings.profile-registries): must be configured when profile-settings.profile is set`
- Later runtime failure on the no-profile path:
  - OpenAI `401 Unauthorized` due to missing API key
- Key files consulted:
  - `pinocchio/README.md`
  - `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
  - `geppetto/pkg/cli/bootstrap/profile_registry.go`
  - `geppetto/pkg/cli/bootstrap/engine_settings.go`
  - `geppetto/pkg/sections/profile_sections.go`
  - `cmd/web-chat/main.go`
  - `cmd/pinocchio/cmds/js.go`
  - `pkg/cmds/helpers/parse-helpers.go`
  - `pkg/cmds/cmd.go`
  - `cmd/examples/simple-chat/main.go`

## Step 2: Reassess the ownership boundary and trace likely shared-regression history

After the first design pass, I revisited the architectural ownership boundary with the user’s feedback in mind: if Geppetto is supposed to own profile selection and registry-chain loading, then a Pinocchio-only fix would just preserve duplication in a different shape. I therefore re-read the shared bootstrap/config layers and looked for historical evidence of where the default `profiles.yaml` fallback used to live.

That second pass changed my recommendation. I now think the immediate bug should be fixed in Geppetto bootstrap, with Pinocchio only supplying app-specific identity and config mapping. Glazed is part of the story because it owns the generic config-plan infrastructure, but I do not think Glazed should gain profile-registry semantics just to solve this bug.

### Prompt Context

**User prompt (verbatim):** "OUr little brother did this first analysis, but I don't think I agree. I think the fix should be in geppetto if possible so that every tool that uses that logic can use it to handle config + profile + profile registry work. The only thing pinocchio should have to configure is the PINOCCHIO_ / appname configuration. ALso, validation and loading should not indeed be left to individual commands. Can you make a second document with your own assessment, and how we can avoid having duplicated / multiple registry related parts of the codebase. In fact, it is entirely possible that glazed might be implicated as well. This config override + repositories is a common pattern in our ecosystem that we recently refactored."

**Assistant interpretation:** Re-evaluate the ownership boundary, write a second design/assessment document, and explain how to centralize registry discovery/validation/loading so individual tools and commands stop duplicating that logic.

**Inferred user intent:** Ensure the fix lands at the correct abstraction layer and use this bug as an opportunity to reduce repeated registry/config plumbing across the ecosystem.

**Commit (code):** N/A — documentation and architecture assessment only

### What I did
- Re-read the shared Geppetto bootstrap path:
  - `geppetto/pkg/cli/bootstrap/config.go`
  - `geppetto/pkg/cli/bootstrap/profile_selection.go`
  - `geppetto/pkg/cli/bootstrap/profile_registry.go`
- Re-read Glazed config-plan and parser plumbing:
  - `glazed/pkg/cli/cobra-parser.go`
  - `glazed/pkg/config/plan.go`
  - `glazed/pkg/config/plan_sources.go`
- Traced duplicate registry logic still present in Pinocchio callers:
  - `pinocchio/cmd/web-chat/main.go`
  - `pinocchio/cmd/pinocchio/cmds/js.go`
  - `pinocchio/pkg/cmds/helpers/parse-helpers.go`
- Looked up historical Geppetto commits and old ticket notes.
- Found direct evidence that Geppetto previously had a shared default-XDG `profiles.yaml` fallback, and that the current bootstrap tests now assert the opposite.
- Wrote a second design doc with a Geppetto-first recommendation.

### Why
- The user’s objection was correct: profile selection and registry loading are already defined as shared Geppetto responsibilities, so a Pinocchio-only fix would be an awkward ownership split.
- The existence of old shared behavior strongly suggests a regression or semantic loss during the bootstrap extraction/refactor.
- The code currently duplicates validation/loading in multiple commands, which is exactly what should be eliminated.

### What worked
- `git log -S "~/.config/pinocchio/profiles.yaml" -- pkg/sections pkg/cli/bootstrap pkg/doc` quickly found the older shared fallback commit.
- `git show c6ec017 -- pkg/sections pkg/cli/bootstrap` confirmed that Geppetto previously auto-used the default XDG `profiles.yaml` in shared code.
- Reading current Geppetto bootstrap tests exposed the contradiction:
  - current test name: `TestResolveCLIProfileSelection_DoesNotUseImplicitProfilesFallback`
- The second design document now has a clearer responsibility split:
  - Glazed for generic config discovery
  - Geppetto for profile-registry semantics
  - Pinocchio for app identity and mapping only

### What didn't work
- The first design document was too Pinocchio-local. It would have solved the user-visible symptom, but it would not have removed duplication across callers.
- There is still no shared Geppetto helper that returns “selection + registry chain” together, so callers keep reconstructing or validating pieces themselves.

### What I learned
- This looks much more like a **shared regression** than a missing Pinocchio-only feature.
- The old section-based Geppetto implementation already had the exact behavior the user wants.
- Glazed’s recent config-plan refactor is relevant as infrastructure, but the profile-registry policy itself still belongs in Geppetto.

### What was tricky to build
- The subtle part was separating “where the fallback path should be discovered” from “where generic config file discovery belongs.” Those are not the same question.
- Another tricky point was that the current tests and docs disagree with each other, while historical docs and commits support the shared fallback model. That required looking at both current code and historical ticket/commit evidence.

### What warrants a second pair of eyes
- Whether `AppBootstrapConfig` should gain a simple `DefaultProfileRegistrySources` hook or a more general source-builder callback.
- Whether we should also add a higher-level Geppetto helper that returns `ResolvedCLIProfileSelection + ResolvedProfileRegistryChain` so commands stop loading chains themselves.
- Whether any future generic “secondary resource discovery” abstraction is worth pushing down into Glazed, or whether that would over-generalize too early.

### What should be done in the future
- Implement the Geppetto-first fix.
- Remove duplicated registry validation/loading from Pinocchio commands after the shared helper exists.
- Update the current Geppetto bootstrap tests so they enforce the intended shared behavior rather than the current no-fallback behavior.

### Code review instructions
- Start with the second design doc:
  - `../design-doc/02-shared-assessment-centralize-profile-registry-discovery-and-loading-in-geppetto-bootstrap.md`
- Then compare:
  - current Geppetto bootstrap files
  - current Pinocchio duplicate callers
  - historical commit `c6ec017`
- Validate the argument by checking that the old shared fallback existed, the current shared bootstrap no longer has it, and command-level duplication is still present.

### Technical details
- Historical shared fallback evidence:
  - commit `c6ec017` — `profiles: default to XDG profiles.yaml and refresh docs`
- Likely regression/extraction window:
  - `63d56ad` — `bootstrap: share config and registry helpers`
  - `095f056` — `bootstrap: drop path list config wrappers`
- Current contradictory shared test:
  - `geppetto/pkg/cli/bootstrap/bootstrap_test.go:126-158`
- Generic config-plan ownership in Glazed:
  - `glazed/pkg/cli/cobra-parser.go:91-185`
  - `glazed/pkg/config/plan.go:11-220`
  - `glazed/pkg/config/plan_sources.go:18-149`

## Step 3: Restore the shared default-registry fallback in Geppetto bootstrap

With the ownership boundary settled, I implemented the first code phase in Geppetto itself. The key change was to restore implicit default-registry discovery in the shared bootstrap path, keyed off the app name, and to add a shared helper that resolves both profile selection and the registry chain together. That lets Geppetto own the “selection + registry loading” contract again instead of leaving pieces of it scattered across callers.

While validating that phase, I also hit a useful mismatch immediately: Pinocchio’s bootstrap wrapper was already out of sync with the extracted Geppetto API because it still lacked a `ConfigPlanBuilder`. That confirmed the repos had drifted at exactly the seam we were investigating.

### Prompt Context

**User prompt (verbatim):** "add detailed tasks to the tasks file. THen implement them in sequence, committing at appropriate intervals, keeping a diary."

**Assistant interpretation:** Break the work into explicit phases, implement them in order, commit after meaningful milestones, and record the implementation diary as the work progresses.

**Inferred user intent:** Turn the research ticket into a real implementation effort with disciplined sequencing, reproducible validation, and reviewable commit boundaries.

**Commit (code):** `5276b0b` (`geppetto`) — `bootstrap: restore implicit profile registry fallback`

### What I did
- Added shared default-registry discovery in Geppetto bootstrap based on `AppName` and `${XDG_CONFIG_HOME:-~/.config}/<app>/profiles.yaml`.
- Updated `ResolveCLIProfileSelection(...)` to inject implicit registry sources before validation.
- Added `ResolvedCLIProfileRuntime` plus `ResolveCLIProfileRuntime(...)` so callers can get both selection and registry chain from one shared helper.
- Refactored `ResolveCLIEngineSettings(...)` to use the new shared profile-runtime helper.
- Replaced the old negative bootstrap test with a positive fallback test and added coverage for the shared profile-runtime helper.

### Why
- This was the central shared fix point identified in the second design doc.
- Restoring the behavior in Geppetto first avoids re-implementing the same logic in Pinocchio.
- The new helper also creates the shared abstraction needed to remove duplicated registry loading from commands later.

### What worked
- Targeted validation passed after the shared bootstrap changes:
  - `go test ./geppetto/pkg/cli/bootstrap ./pinocchio/pkg/cmds/profilebootstrap -count=1`
- The Geppetto repo pre-commit hook passed full tests and linting during commit.
- The updated bootstrap tests now prove implicit fallback behavior instead of forbidding it.

### What didn't work
- The first validation attempt exposed a Pinocchio-side compatibility issue:
  - `NewCLISelectionValues failed: app bootstrap config: config plan builder is required`
- That error came from `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`, which confirmed the Pinocchio wrapper had not fully kept up with the extracted Geppetto bootstrap contract.

### What I learned
- The shared Geppetto-first approach is viable in code, not just in theory.
- Adding a shared `ResolveCLIProfileRuntime(...)` helper immediately makes the command-level duplication problem easier to remove.
- The current workspace had already drifted enough that fixing the shared layer also required restoring wrapper compatibility in Pinocchio.

### What was tricky to build
- The tricky part was deciding how much app-specific configuration should be required. I intentionally kept the implicit fallback generic and keyed off `AppName` so apps do not need an extra custom callback just to get the common XDG behavior.
- Another sharp edge was preserving existing engine-settings behavior while threading the new shared profile-runtime helper through it.

### What warrants a second pair of eyes
- Whether the `AppName`-based default-registry rule is exactly the right amount of implicit behavior for all current Geppetto bootstrap consumers.
- Whether `ResolvedCLIEngineSettings` should continue to expose the embedded `ProfileRuntime` object long-term, or whether a narrower exported surface would be better.

### What should be done in the future
- Use the new shared helper in Pinocchio callers and remove duplicated registry validation/loading.
- Update the remaining docs/tests to match the restored shared behavior.

### Code review instructions
- Start with:
  - `geppetto/pkg/cli/bootstrap/profile_selection.go`
  - `geppetto/pkg/cli/bootstrap/profile_runtime.go`
  - `geppetto/pkg/cli/bootstrap/profile_registry_defaults.go`
  - `geppetto/pkg/cli/bootstrap/engine_settings.go`
- Then review the updated tests in:
  - `geppetto/pkg/cli/bootstrap/bootstrap_test.go`
- Validate with:
  - `go test ./geppetto/pkg/cli/bootstrap -count=1`

### Technical details
- Commands run:
  - `go test ./geppetto/pkg/cli/bootstrap ./pinocchio/pkg/cmds/profilebootstrap -count=1`
  - `git -C geppetto commit -m "bootstrap: restore implicit profile registry fallback"`
- Key new shared types/helpers:
  - `ResolvedCLIProfileRuntime`
  - `ResolveCLIProfileRuntime(...)`
- Immediate compatibility error discovered during validation:
  - `app bootstrap config: config plan builder is required`

## Step 4: Remove duplicated profile-registry loading from Pinocchio callers and fix stale parser call sites

After the Geppetto commit, I switched to the Pinocchio repo and used the new shared bootstrap helper to collapse several duplicated code paths. This phase also turned into a broader compatibility cleanup because the repo still contained older parser/config APIs like `ConfigFilesFunc`, `ResolveAppConfigPath`, and references to the removed section-based Geppetto middleware builder.

The end result is that Pinocchio callers now rely on the shared bootstrap path much more directly, and the repo-level test suite is green again.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Apply the new shared Geppetto bootstrap path in Pinocchio, remove duplicated registry logic, and keep the repo in a commit-worthy state.

**Inferred user intent:** Make the shared fix real for the actual Pinocchio entrypoints instead of leaving the commands on legacy code paths.

**Commit (code):** `6d2c944` (`pinocchio`) — `bootstrap: remove duplicated profile registry loading`

### What I did
- Added `ConfigPlanBuilder` wiring to `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go` so the Pinocchio wrapper is compatible with the extracted Geppetto bootstrap contract.
- Refactored `cmd/web-chat/main.go` to use `profilebootstrap.ResolveCLIProfileRuntime(...)` instead of validating/loading profile registries locally.
- Refactored `cmd/pinocchio/cmds/js.go` so JS runtime bootstrap reuses the resolved shared profile runtime instead of reopening registry chains from raw settings.
- Refactored `pkg/cmds/helpers/parse-helpers.go` to stop re-reading `PINOCCHIO_PROFILE` and manually enforcing the validation rule.
- Updated tests in:
  - `pkg/cmds/helpers/profile_selection_test.go`
  - `cmd/web-chat/main_profile_registries_test.go`
  - `cmd/pinocchio/cmds/js_test.go`
- Replaced stale parser/config API usage across the repo:
  - `ConfigFilesFunc` -> `ConfigPlanBuilder`
  - removed old `GetCobraCommandGeppettoMiddlewares` references
  - replaced `ResolveAppConfigPath` usage in `cmd/pinocchio/main.go`

### Why
- The Geppetto fix only matters if the real Pinocchio entrypoints stop bypassing or duplicating it.
- The stale parser/config symbols were preventing a clean repo-wide build and would have broken the commit hook.
- Cleaning these up now reduces the chances of reintroducing a split profile contract later.

### What worked
- Targeted packages passed:
  - `go test ./pinocchio/pkg/cmds/profilebootstrap ./pinocchio/pkg/cmds/helpers ./pinocchio/cmd/web-chat ./pinocchio/cmd/pinocchio/cmds -count=1`
- Full Pinocchio repo validation passed after the compatibility cleanup:
  - `cd pinocchio && go test ./... -count=1`
- The Pinocchio pre-commit hook passed both lint and full tests when creating the commit.

### What didn't work
- The first broader `go test ./...` in Pinocchio failed with multiple stale symbols:
  - `undefined: geppettosections.GetCobraCommandGeppettoMiddlewares`
  - `undefined: glazedConfig.ResolveAppConfigPath`
- That failure showed the repo still had several latent assumptions from the older parser/bootstrap surface.

### What I learned
- The duplicated profile-registry behavior was only part of the drift; parser/config call sites had also fallen behind the current Glazed/Geppetto bootstrap API.
- Once the shared bootstrap path existed, the Pinocchio cleanup became mostly a matter of deleting local special cases and routing everything through the shared helper.

### What was tricky to build
- The tricky part was avoiding a partial migration. Fixing only `web-chat` or only `js` would have left enough stale call sites around that repo-wide validation still would have failed.
- Another tricky edge was `loadRepositoriesFromConfig()`: the old helper expected a single resolved config path, but the current config-plan world naturally returns a stack of files. I converted that logic to read all resolved config files and dedupe repository entries so the repo still builds cleanly.

### What warrants a second pair of eyes
- The semantics of repository loading from multiple resolved config files in `cmd/pinocchio/main.go`.
- Whether `loadPinocchioProfileRegistryStack(...)` in JS should continue to exist as a thin compatibility helper or be removed entirely once no tests rely on it.

### What should be done in the future
- Update the remaining docs and ticket bookkeeping to reflect the implemented shared behavior.
- Re-run the original `PINOCCHIO_PROFILE=...` repro once the runtime environment has the expected registry file and provider credentials.

### Code review instructions
- Start with:
  - `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
  - `pinocchio/cmd/web-chat/main.go`
  - `pinocchio/cmd/pinocchio/cmds/js.go`
  - `pinocchio/pkg/cmds/helpers/parse-helpers.go`
- Then review the broader compatibility updates in:
  - `pinocchio/pkg/cmds/cobra.go`
  - `pinocchio/cmd/pinocchio/main.go`
  - `pinocchio/cmd/examples/simple-chat/main.go`
  - `pinocchio/cmd/agents/simple-chat-agent/main.go`
  - `pinocchio/cmd/examples/simple-redis-streaming-inference/main.go`
- Validate with:
  - `cd pinocchio && go test ./... -count=1`

### Technical details
- Commands run:
  - `go test ./pinocchio/pkg/cmds/profilebootstrap ./pinocchio/pkg/cmds/helpers ./pinocchio/cmd/web-chat ./pinocchio/cmd/pinocchio/cmds -count=1`
  - `cd pinocchio && go test ./... -count=1`
  - `git -C pinocchio commit -m "bootstrap: remove duplicated profile registry loading"`
- Stale-symbol failures encountered before the final cleanup:
  - `undefined: geppettosections.GetCobraCommandGeppettoMiddlewares`
  - `undefined: glazedConfig.ResolveAppConfigPath`

## Step 5: Update shared docs, ticket bookkeeping, and delivery artifacts

After the code phases were committed, I updated the current shared tutorial docs to describe the restored implicit profile-registry fallback model and then refreshed the ticket materials to match the implementation. This step closes the loop between research, code, and delivery.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Finish the implementation sequence by aligning the docs and ticket artifacts with the code that actually landed.

**Inferred user intent:** Leave behind a coherent, reviewable package: code, docs, tasks, changelog, diary, and reMarkable delivery all saying the same thing.

**Commit (code):** `3f5a4be` (`geppetto`) — `docs: describe implicit profile registry fallback`

### What I did
- Updated `geppetto/pkg/doc/tutorials/09-migrating-cli-commands-to-glazed-bootstrap-profile-resolution.md` so it now:
  - includes `ConfigPlanBuilder` in the `AppBootstrapConfig` example,
  - documents the implicit `${XDG_CONFIG_HOME:-~/.config}/<app>/profiles.yaml` fallback,
  - updates the troubleshooting guidance around `profile-settings.profile-registries`.
- Updated the ticket task list, diary, changelog, and index links.
- Re-ran `docmgr doctor` and refreshed the reMarkable bundle.

### Why
- The code now implements the shared fallback again, so the current docs needed to stop describing the stricter no-fallback model.
- The ticket should reflect what actually shipped, not just the initial research state.

### What worked
- The Geppetto docs-only commit was straightforward because the code shape was already settled.
- `docmgr doctor` passed cleanly after the ticket updates.
- The reMarkable bundle was refreshed successfully after forcing an overwrite of the existing PDF.

### What didn't work
- The first re-upload attempt skipped because the PDF already existed remotely:
  - `SKIP: PIN-20260418-PROFILE-ENV-RESOLUTION already exists ... (use --force to overwrite)`
- Re-running with `--force` resolved that cleanly.

### What I learned
- The code/doc drift in this area was real and recent enough that the shared tutorial still encoded the pre-fix behavior.
- Refreshing the delivery bundle is worth treating as a first-class implementation step, not just a publishing afterthought.

### What was tricky to build
- The tricky part here was keeping the diary and ticket bookkeeping synchronized with commits that landed in **two repositories** while the docs workspace lives only in the Pinocchio repo.

### What warrants a second pair of eyes
- Whether any additional current Geppetto docs outside the migration tutorial should explicitly mention the restored implicit fallback behavior.

### What should be done in the future
- Optionally add a focused end-to-end smoke test that shells out to a built Pinocchio binary with only XDG `profiles.yaml` present plus `PINOCCHIO_PROFILE=...`, mirroring the older GP-50 validation style.

### Code review instructions
- Start with the three commits in order:
  - `5276b0b` (`geppetto`) — shared bootstrap fix
  - `6d2c944` (`pinocchio`) — caller cleanup
  - `3f5a4be` (`geppetto`) — doc alignment
- Then compare the ticket docs against the landed code and rerun the listed validation commands.

### Technical details
- Commands run:
  - `docmgr doctor --ticket PIN-20260418-PROFILE-ENV-RESOLUTION --stale-after 30`
  - `remarquee upload bundle --force ...`
  - `remarquee cloud ls /ai/2026/04/18/PIN-20260418-PROFILE-ENV-RESOLUTION --long --non-interactive`

## Related

- Design doc 1: `../design-doc/01-pinocchio-profile-env-and-explicit-profile-resolution-design.md`
- Design doc 2: `../design-doc/02-shared-assessment-centralize-profile-registry-discovery-and-loading-in-geppetto-bootstrap.md`
- README docs: `pinocchio/README.md`
- Tutorial docs: `pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md`
- Topic docs: `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
