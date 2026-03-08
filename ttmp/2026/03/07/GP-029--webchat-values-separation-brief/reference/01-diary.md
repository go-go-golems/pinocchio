---
Title: Diary
Ticket: GP-029
Status: active
Topics:
    - webchat
    - backend
    - pinocchio
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/doc/topics/webchat-framework-guide.md
      Note: Main webchat framework guide that will need the new constructor guidance
    - Path: pinocchio/pkg/doc/topics/webchat-values-separation-migration-guide.md
      Note: New migration guide describing the explicit constructor path
    - Path: pinocchio/pkg/webchat/router.go
      Note: Main constructor currently mixing parsed-values decoding with core router composition
    - Path: pinocchio/pkg/webchat/router_deps.go
      Note: New dependency-injected router construction helpers and parsed-values adapter
    - Path: pinocchio/pkg/webchat/server.go
      Note: Server constructor delegates through the router path that also needs the split
    - Path: pinocchio/pkg/webchat/stream_backend.go
      Note: Current stream backend constructor still decodes Redis settings from parsed values
ExternalSources: []
Summary: 'Implementation diary for GP-029: separating parsed values from Pinocchio webchat core construction.'
LastUpdated: 2026-03-07T15:07:15-05:00
WhatFor: Use this diary to capture the concrete implementation steps, commits, tests, and migration work for GP-029.
WhenToUse: Use when reviewing how the Values-separation refactor was implemented, validated, and documented.
---



# Diary

## Goal

Capture the implementation of GP-029 step by step so a reviewer can see what changed, why it changed, what failed, what was verified, and which commits map to which milestones.

## Context

GP-029 starts from a design brief, not from code. The job is to separate parsed Glazed values from `pkg/webchat` core construction, keep the compatibility path for current CLI callers, update docs, and leave a migration trail for embedders.

## Quick Reference

Implementation steps will be recorded here as the work proceeds.

## Step 1: Ticket setup and implementation plan

I started by converting the brief-only ticket into an implementation workspace. The main work in this step was not code; it was turning the handoff note into a runnable plan and creating the diary that will track the code and documentation changes.

This step establishes the execution order for the refactor: split constructors first, preserve adapters second, then update tests and docs. That avoids changing public guidance before the new API shape exists.

### Prompt Context

**User prompt (verbatim):** "Ok, let's implement all of GP-029, including writing a migration guide in pkg/doc, task by task, commit appropriately, keep a diary. If there are no tasks yet, create them."

**Assistant interpretation:** Implement the entire GP-029 refactor, create any missing ticket tasks, maintain a detailed diary during the work, and include a migration guide in `pkg/doc`.

**Inferred user intent:** Turn the design brief into finished, reviewed work with execution tracking and migration guidance, rather than leaving the refactor at the proposal stage.

**Commit (code):** pending

### What I did

- Inspected the current GP-029 ticket contents and confirmed it only had the brief and a minimal completed task list.
- Read the current webchat constructor surfaces:
  - `pkg/webchat/router.go`
  - `pkg/webchat/server.go`
  - `pkg/webchat/stream_backend.go`
- Read the current webchat docs in `pkg/doc/topics/` to identify where migration guidance needs to land.
- Created this diary document.
- Expanded `tasks.md`, updated the ticket index, and recorded the new execution backlog in the changelog.

### Why

- The user explicitly asked to work task by task and keep a diary.
- The original ticket did not contain actionable tasks, so implementation would have drifted without a backlog.
- Mapping the touched code and docs up front reduces the chance of missing a constructor path or migration reference later.

### What worked

- The ticket structure already existed and only needed to be expanded.
- The code seams identified in the brief still match the current repo state.

### What didn't work

- There was no pre-existing implementation backlog in `GP-029`; I had to create it before starting the refactor.

### What I learned

- `NewRouter(...)`, `NewServer(...)`, and `NewStreamBackendFromValues(...)` are the critical constructor surfaces.
- `BuildHTTPServer()` currently depends on `Router.parsed`, which means Values separation has to include HTTP-server settings ownership, not only initial router construction.

### What was tricky to build

- The subtle part is that the router stores `parsed` for later use in `BuildHTTPServer()`. Separating Values cleanly therefore requires introducing explicit retained settings on the router, not just moving the initial decode.

### What warrants a second pair of eyes

- The eventual constructor API naming: whether the new explicit constructor becomes the canonical name immediately or remains the new sibling alongside compatibility wrappers.

### What should be done in the future

- Implement the stream backend, router, and server constructor split described in the new task breakdown.

### Code review instructions

- Start with the updated ticket files in `pinocchio/ttmp/.../GP-029--webchat-values-separation-brief/`.
- Validate that the backlog matches the design brief before reviewing code changes.

### Technical details

- Key commands run:
  - `rg -n "NewRouter\\(|NewStreamBackendFromValues|DecodeSectionInto\\(|parsed \\*values.Values|BuildHTTPServer\\(" ...`
  - `docmgr doc add --root pinocchio/ttmp --ticket GP-029 --doc-type reference --title "Diary" --summary "Implementation diary for GP-029: separating parsed values from Pinocchio webchat core construction."`

## Step 2: Split stream backend, router, and server constructors

This step implemented the actual code refactor. I introduced explicit constructor layers for the stream backend, router, and server, then rewired the old parsed-values entry points into thin adapters over those new paths.

The critical design decision in this step was to keep the compatibility names in place. Existing callers such as `cmd/web-chat` continue to call `NewServer(...)`, but the real construction path is now `BuildRouterDepsFromValues(...)` into `NewServerFromDeps(...)` and `NewRouterFromDeps(...)`.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Implement the Values-separation refactor in the core webchat constructors, preserve compatibility wrappers, and verify the result with tests and commits.

**Inferred user intent:** Land the refactor in code without forcing an immediate migration on existing callers, while making the explicit dependency-injected API available for embedders.

**Commit (code):** `1bceb2e` — `refactor: separate webchat values parsing from core constructors`

### What I did

- Added `NewStreamBackend(ctx, rediscfg.Settings)` and kept `NewStreamBackendFromValues(...)` as an adapter.
- Added `RouterDeps`, `BuildRouterDepsFromValues(...)`, `NewDefaultTimelineStore(...)`, `NewTimelineStoreFromSettings(...)`, and `NewTurnStoreFromSettings(...)` in `pkg/webchat/router_deps.go`.
- Added `NewRouterFromDeps(...)` and changed `NewRouter(...)` to delegate through `BuildRouterDepsFromValues(...)`.
- Removed the router’s retained `parsed` field and replaced it with retained `RouterSettings`.
- Changed `BuildHTTPServer()` to use retained `RouterSettings` instead of decoding from parsed values.
- Added `NewServerFromDeps(...)` and changed `NewServer(...)` to delegate through the new dependency path.
- Exposed `stream.NewBackend` in `pkg/webchat/stream/api.go`.
- Added tests covering:
  - explicit stream backend construction;
  - parsed-values adapter dependency construction;
  - dependency-injected router construction;
  - dependency-injected server construction.
- Ran `go test ./pkg/webchat/...` during development, then the repo pre-commit hook ran `go test ./...`, `go build ./...`, `go generate ./...`, `golangci-lint`, and `go vet`.

### Why

- The brief explicitly called for explicit dependency-injected construction in core with parsed-values handling moved into adapter code.
- `Router.parsed` was the last structural reason `BuildHTTPServer()` still depended on Glazed values after initial construction.
- Keeping the old entry points avoids unnecessary churn in callers and docs while still exposing the cleaner API.

### What worked

- The constructor split fit cleanly into a new `router_deps.go` file without forcing a larger redesign.
- Existing callers did not need code changes because the compatibility wrappers preserved the old signatures.
- The targeted `pkg/webchat` test suite passed once the new tests were corrected.
- The repo’s pre-commit pipeline also passed after the refactor, which confirmed there was no broader breakage in `cmd/web-chat` or other packages.

### What didn't work

- The first focused test run failed because I removed `middleware` and `message` imports from `router.go` while those symbols were still used later in the file.
- The first version of `router_deps_test.go` used the wrong `EventSink` stub and guessed the `values.Values` mutation API incorrectly.
- An additional quick fix was needed because I duplicated a `noopSink` test helper already present in the package.

Exact failures:

- `pkg/webchat/router.go:330:21: undefined: middleware`
- `pkg/webchat/router.go:368:50: undefined: message`
- `pkg/webchat/router_deps_test.go:25:10: cannot use events.NewPublisherManager() ... as events.EventSink`
- `pkg/webchat/router_deps_test.go:40:21: multiple-value parsed.Set("addr", ":4242") ... in single-value context`
- `pkg/webchat/router_deps_test.go:16:6: noopSink redeclared in this block`

### What I learned

- The cleanest migration seam is not only constructor naming. It is explicit retention of resolved settings on `Router`.
- `BuildRouterDepsFromValues(...)` is a useful public API in its own right because it lets apps start from Glazed but still intercept and override dependencies before server construction.
- The existing webchat package tests were already close to what was needed; the new tests mainly had to prove explicit constructor use.

### What was tricky to build

- The main tricky point was preserving current behavior while moving object creation around. Timeline store creation, turn store creation, and HTTP server settings all used to live inside `NewRouter(...)`; splitting them without changing behavior required a clear distinction between:
  - parsed-values adapter work;
  - explicit dependency construction;
  - retained runtime settings.

### What warrants a second pair of eyes

- Whether `RouterDeps` should stay minimal or grow further as more constructor work moves out of `pkg/webchat`.
- Whether the package should eventually rename `NewRouterFromDeps(...)` back to `NewRouter(...)` once downstream callers migrate.

### What should be done in the future

- Finish the docs and migration work so embedders see the new constructor layering in the public guidance.
- Run final ticket hygiene checks and record the documentation step in this diary.

### Code review instructions

- Start with:
  - `pkg/webchat/router_deps.go`
  - `pkg/webchat/router.go`
  - `pkg/webchat/server.go`
  - `pkg/webchat/stream_backend.go`
- Then inspect the new tests in `pkg/webchat/router_deps_test.go` and the adjusted tests in `pkg/webchat/stream_backend_test.go` and `pkg/webchat/router_handlers_test.go`.
- Validation:
  - `go test ./pkg/webchat/...`
  - note that the code commit also passed the repo pre-commit checks: `go test ./...`, `go build ./...`, `go generate ./...`, `golangci-lint`, and `go vet`

### Technical details

- New API surface added in this step:
  - `NewStreamBackend(...)`
  - `BuildRouterDepsFromValues(...)`
  - `NewRouterFromDeps(...)`
  - `NewServerFromDeps(...)`
- Compatibility wrappers preserved:
  - `NewStreamBackendFromValues(...)`
  - `NewRouter(...)`
  - `NewServer(...)`

## Step 3: Publish migration guidance and validate the public docs path

After the code refactor was in place, I updated the public webchat documentation so new embedders see the explicit constructor layering first instead of discovering it only from the code. I also added the dedicated migration guide requested in the prompt.

This step matters because GP-029 is partly an API-boundary cleanup and partly a migration story. If the docs still present `NewServer(parsed, ...)` as the primary pattern without explanation, the refactor is technically done but practically invisible.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Finish GP-029 by documenting how embedders should migrate to the explicit constructor flow and validate that the updated docs still integrate cleanly with the repo.

**Inferred user intent:** Make the API cleanup adoptable by other engineers rather than leaving it as an internal-only improvement.

**Commit (code):** pending

### What I did

- Updated:
  - `pkg/doc/topics/webchat-framework-guide.md`
  - `pkg/doc/topics/webchat-user-guide.md`
  - `pkg/doc/topics/webchat-http-chat-setup.md`
- Added:
  - `pkg/doc/topics/webchat-values-separation-migration-guide.md`
- Documented the preferred layering:
  - `BuildRouterDepsFromValues(...)`
  - `NewServerFromDeps(...)`
  - `NewRouterFromDeps(...)`
- Preserved the compatibility story in the docs by explicitly calling out `NewServer(...)` and `NewRouter(...)` as wrappers.
- Related the migration guide and touched files back to the GP-029 ticket.
- Ran validation commands:
  - `go test ./pkg/doc ./cmd/web-chat ./cmd/pinocchio`
  - `docmgr doctor --root pinocchio/ttmp --ticket GP-029 --stale-after 30`

### Why

- The user explicitly asked for a migration guide in `pkg/doc`.
- The framework and user guides are the main discovery path for future embedders, so they need to prefer the new constructor layering.
- Updating the HTTP setup page keeps the route documentation aligned with the new constructor recommendations.

### What worked

- The existing docs were already structured around a handler-first architecture, so the Values-separation update fit cleanly as a constructor-layering clarification rather than a rewrite.
- The migration guide could stay narrowly scoped to constructor changes without reopening the `/chat` ownership discussion.
- Validation stayed green:
  - `go test ./pkg/doc ./cmd/web-chat ./cmd/pinocchio`
  - `docmgr doctor --root pinocchio/ttmp --ticket GP-029 --stale-after 30`

### What didn't work

- N/A

### What I learned

- The most important doc change was not just adding the migration page. It was changing the "recommended baseline" language in the framework/user guides so the explicit constructor path is visible immediately.
- `BuildRouterDepsFromValues(...)` is the critical bridge concept to explain to existing Glazed-based apps.

### What was tricky to build

- The tricky part was balancing two truths at once:
  - the explicit constructor path is now the preferred API;
  - the parsed-values wrappers still remain supported and should not be described as broken or deprecated.

### What warrants a second pair of eyes

- Whether the docs should go even further and update third-party tutorial examples to the new explicit constructor path immediately, or whether the migration guide plus framework/user-guide updates are sufficient for now.

### What should be done in the future

- If more embedders adopt the explicit constructor path, update any remaining tutorials that still default to `NewServer(...)`.

### Code review instructions

- Review the new migration guide first:
  - `pkg/doc/topics/webchat-values-separation-migration-guide.md`
- Then compare the framing changes in:
  - `pkg/doc/topics/webchat-framework-guide.md`
  - `pkg/doc/topics/webchat-user-guide.md`
  - `pkg/doc/topics/webchat-http-chat-setup.md`
- Validate with:
  - `go test ./pkg/doc ./cmd/web-chat ./cmd/pinocchio`
  - `docmgr doctor --root pinocchio/ttmp --ticket GP-029 --stale-after 30`

### Technical details

- The migration guide documents three supported paths:
  - stay on compatibility wrappers for now;
  - use `BuildRouterDepsFromValues(...) + NewServerFromDeps(...)`;
  - construct `RouterDeps` fully explicitly.

## Step 4: Make `cmd/web-chat` runtime composition profile-driven only

This step removed the last practical place where `cmd/web-chat` still treated parsed AI settings as a runtime source of truth. The command now exposes server/profile/transport flags only, while engine/provider settings come from the resolved profile runtime patch and nothing else.

This work closes the architectural loop from the earlier discussion: Values separation is not very meaningful for `web-chat` if the command still lets `--ai-engine` and related flags sneak back into the runtime composer. Tightening the command and the composer together makes the registry contract explicit instead of conventional.

### Prompt Context

**User prompt (verbatim):** "Ok, add tasks to the ticket and then implement these changes too. 

the parsed step settings the runtime composer takes now should come exclusively from the registry, right?"

**Assistant interpretation:** Extend GP-029 with the stricter `cmd/web-chat` cleanup, then implement the change so runtime step settings come from the profile registry runtime rather than direct parsed AI flags.

**Inferred user intent:** Make profile registries the single source of truth for `web-chat` runtime selection and remove mixed configuration paths that would undermine that design.

**Commit (code):** `d4286ed` — `refactor: make web-chat runtime settings profile-driven`

### What I did

- Added a local `profile-settings` section in `cmd/web-chat/main.go` and stopped calling `geppettosections.CreateGeppettoSections()` for that command.
- Switched `cmd/web-chat` from the Geppetto-specific Cobra middleware stack to the default Glazed parser path with `cli.CobraParserConfig{AppName: "pinocchio"}` so env/config/default handling stays intact without injecting profile step patches into parsed values.
- Refactored `ProfileRuntimeComposer` in `cmd/web-chat/runtime_composer.go` to remove `*values.Values` entirely and build its base settings from `settings.NewStepSettings()`.
- Kept profile runtime application intact by applying `ResolvedProfileRuntime.StepSettingsPatch` on top of those defaults.
- Updated `cmd/web-chat/runtime_composer_test.go` so test fixtures provide provider credentials via the profile runtime patch instead of via parsed values.
- Added a command regression test in `cmd/web-chat/main_profile_registries_test.go` that asserts `--ai-engine` and `--ai-api-type` are no longer exposed while `--profile` and `--profile-registries` remain present.
- Updated `cmd/web-chat/README.md` and `pkg/doc/topics/webchat-http-chat-setup.md` to describe the stricter profile-driven contract.
- Ran:
  - `go test ./cmd/web-chat -count=1`
  - `go test ./pkg/webchat/... -count=1`
  - `go test ./pkg/doc -count=1`
  - `docmgr doctor --root pinocchio/ttmp --ticket GP-029 --stale-after 30`

### Why

- The user explicitly confirmed that `web-chat` should no longer care about direct `ai-engine` style CLI flags because runtime selection lives in profiles now.
- Leaving parsed AI sections on the command would preserve a second runtime authority and make the registry-first design easy to bypass accidentally.
- Removing `*values.Values` from the composer is the cleanest way to prove the runtime builder no longer depends on command parsing for engine/provider policy.

### What worked

- The command parser could be simplified instead of replaced with a custom loader by using Glazed’s default env/config/default path with `AppName: "pinocchio"`.
- The composer change was mechanically small: replacing `NewStepSettingsFromParsedValues(...)` with `NewStepSettings()` and removing the unused parsed state.
- The new regression test makes the CLI-surface change explicit and stable.
- Validation passed after the test fixtures were updated to match the new contract.

### What didn't work

- The first `go test ./cmd/web-chat -count=1` run failed because several composer tests were still implicitly relying on parsed values to inject an OpenAI API key.
- The first attempt to clone `RuntimeSpec` in the test helper failed because `RuntimeSpec` has no `Clone()` method; only `Profile` does.
- The normal pre-commit hook failed again on the repo-wide lint step due the existing environment mismatch:
  - `golangci-lint` was built with Go 1.25
  - the repo target is Go 1.26.1

Exact failures:

- `engine init failed: invalid settings for provider openai: missing API key openai-api-key`
- `cmd/web-chat/runtime_composer_test.go:20:16: spec.Clone undefined (type *profiles.RuntimeSpec has no field or method Clone)`
- `can't load config: the Go language version (go1.25) used to build golangci-lint is lower than the targeted Go version (1.26.1)`

### What I learned

- The parsed-values dependency in `ProfileRuntimeComposer` was the last hidden bridge keeping CLI/runtime concerns coupled in `web-chat`.
- The real semantic change is not “remove flags”; it is “credentials and engine/provider settings must now be modeled as profile runtime data”.
- Glazed’s default parser path is sufficient for `web-chat` once the Geppetto profile-patch middleware is no longer required.

### What was tricky to build

- The subtle point was keeping `profile-settings.profile-registries` working while removing the broader Geppetto section stack. The command still needs env/config/default resolution for profile source selection, just not for AI runtime sections.
- The tests needed a contract update, not just fixture churn. Once runtime settings are profile-owned, the tests have to express credentials and engine policy through `StepSettingsPatch` rather than through parsed command values.

### What warrants a second pair of eyes

- Whether other apps besides `cmd/web-chat` should follow the same strict profile-driven cleanup soon, or whether this command should remain the main proving ground first.
- Whether future profile validation should explicitly require provider credentials when a runtime selects a provider that cannot run without them, to fail earlier than engine initialization.

### What should be done in the future

- If the team wants an even harder cutover, consider removing any remaining documentation or examples elsewhere in the repo that suggest `web-chat` supports direct AI runtime flags.
- Re-run the full hook suite without `--no-verify` once the local `golangci-lint` toolchain matches Go 1.26.1.

### Code review instructions

- Start with:
  - `cmd/web-chat/main.go`
  - `cmd/web-chat/runtime_composer.go`
- Then inspect the contract coverage in:
  - `cmd/web-chat/main_profile_registries_test.go`
  - `cmd/web-chat/runtime_composer_test.go`
- Finally review the updated user-facing guidance in:
  - `cmd/web-chat/README.md`
  - `pkg/doc/topics/webchat-http-chat-setup.md`
- Validation:
  - `go test ./cmd/web-chat -count=1`
  - `go test ./pkg/webchat/... -count=1`
  - `go test ./pkg/doc -count=1`
  - `docmgr doctor --root pinocchio/ttmp --ticket GP-029 --stale-after 30`

### Technical details

- `cmd/web-chat` no longer pulls in `geppetto/pkg/sections.CreateGeppettoSections()` for command registration.
- `ProfileRuntimeComposer` now derives its base settings exclusively from `settings.NewStepSettings()`.
- The remaining parsed-values use in `cmd/web-chat` is operational:
  - server flags
  - profile registry source resolution
  - timeline JS script configuration
  - generic command/env/config handling
  It no longer influences runtime engine/provider selection.

## Step 5: Debug the post-cutover webchat regression and restore provider defaults

This step investigated the live `/chat` failures reported immediately after the GP-029/GP-030 work and traced them back to a subtle defaulting regression introduced by the registry-only runtime change. The key finding was that `settings.NewStepSettings()` preserved provider-specific struct defaults, but it did not seed the shared `API.BaseUrls` map with provider base URL defaults that previously arrived through parsed Glazed values.

I reproduced the issue against the running `http://localhost:8082` server with Playwright and the live `/tmp/webchat.log`, then patched the settings bootstrap path so profile-driven runtime composition still inherits provider base URL defaults. I also added request-level `/chat` error logging so immediate startup failures stop disappearing behind a bare `500`.

### Prompt Context

**User prompt (verbatim):** "So we've been working on pinocchio/ttmp (docmgr --root pinocchio/ttmp) on ticket GP-030 and GP-029, and it seems we actually broke webchat. I get a 500 on

await fetch(\"http://localhost:8082/chat\", {
    \"credentials\": \"include\",
    \"headers\": {
        \"User-Agent\": \"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:148.0) Gecko/20100101 Firefox/148.0\",
        \"Accept\": \"*/*\",
        \"Accept-Language\": \"en-US,en;q=0.9\",
        \"Content-Type\": \"application/json\",
        \"Sec-Fetch-Dest\": \"empty\",
        \"Sec-Fetch-Mode\": \"cors\",
        \"Sec-Fetch-Site\": \"same-origin\",
        \"Priority\": \"u=0\"
    },
    \"referrer\": \"http://localhost:8082/?conv_id=3542c2a3-841f-4850-9bbf-624a3e18f179\",
    \"body\": \"{\\\"conv_id\\\":\\\"3542c2a3-841f-4850-9bbf-624a3e18f179\\\",\\\"prompt\\\":\\\"hello\\\"}\",
    \"method\": \"POST\",
    \"mode\": \"cors\"
});

start session inference failed


----

But no logs, and it seemed also that I got one conversation going earlier, but it was not streaming I think.

Please look at what is going on. It's in /tmp/webchat.log 

❯ go run ./cmd/web-chat web-chat --timeline-db /tmp/timeline5.db --turns-db /tmp/turns5.db --log-level debug --with-caller --log-file /tmp/webchat.log --log-to-stdout --addr :8082
{\"level\":\"info\",\"file\":\"/tmp/webchat.log\",\"time\":\"2026-03-08T12:23:32.734463088-04:00\",\"caller\":\"/home/manuel/workspaces/2026-03-02/deliver-mento-1/glazed/pkg/cmds/logging/init.go:50\",\"message\":\"Logging to file\"}

etc..."

**Assistant interpretation:** Reproduce the live webchat regression, determine whether it is an immediate `/chat` startup failure or a later streaming/runtime failure, and keep the ticket diary updated while fixing the underlying regression.

**Inferred user intent:** Restore working webchat behavior after the profile/runtime cutover and make future failures diagnosable from logs instead of opaque browser errors.

**Commit (code):** uncommitted workspace changes

### What I did

- Read `/tmp/webchat.log` and confirmed it contained startup lines only, with no request-level diagnostics for failing `/chat` calls.
- Traced the `start session inference failed` response text to `pinocchio/pkg/webchat/http/api.go`, then followed the submission path through `ChatService`, `ConversationService`, `StreamHub`, `ConvManager`, and `LLMLoopRunner`.
- Used Playwright against `http://localhost:8082` to inspect the live app state:
  - confirmed the default browser context loaded with profile `default`,
  - confirmed websocket attachment succeeded,
  - submitted a prompt on `default` and observed `POST /chat => 200` while the background run failed with `error="no base URL for openai"`,
  - switched the live UI profile to `gpt-5` and reproduced `POST /chat => 500`, which matches the user’s `credentials: "include"` fetch path where the `chat_profile` cookie controls runtime selection.
- Inspected `geppetto/pkg/steps/ai/settings/settings-step.go`, `geppetto/pkg/steps/ai/settings/openai/chat.yaml`, and `geppetto/pkg/steps/ai/openai/helpers.go` to verify that:
  - provider base URL defaults still exist in the section YAML,
  - `settings.NewStepSettings()` was no longer seeding them into `API.BaseUrls`,
  - `MakeClient(...)` hard-fails if `API.BaseUrls["openai-base-url"]` is absent.
- Patched `settings.NewStepSettings()` to initialize `APISettings` with provider default base URLs from the OpenAI, Claude, and Gemini section definitions.
- Added `geppetto/pkg/steps/ai/settings/settings-step_test.go` to lock the provider base URL defaults in place.
- Added structured `/chat` request logging in `pinocchio/pkg/webchat/http/api.go` for both request-resolution failures and prompt-submission failures.
- Ran:
  - `gofmt -w geppetto/pkg/steps/ai/settings/settings-step.go geppetto/pkg/steps/ai/settings/settings-step_test.go pinocchio/pkg/webchat/http/api.go`
  - `go test ./geppetto/pkg/steps/ai/settings -count=1`
  - `go test ./pinocchio/cmd/web-chat ./pinocchio/pkg/webchat/... -count=1`

### Why

- The runtime-only profile cutover was correct in principle, but it accidentally removed a second responsibility from parsed values: seeding provider base URL defaults into the shared API settings map.
- Fixing the default source in `settings.NewStepSettings()` preserves the registry-first runtime contract while restoring the provider defaults that engines still expect.
- Adding request logging is necessary because profile-driven runtime failures can now happen at request time based on cookies and selected profiles, and the previous handler provided no diagnostic evidence when startup failed before background execution.

### What worked

- Playwright was the fastest way to separate the two observed behaviors:
  - `default` profile: `/chat` returned `200`, then inference failed asynchronously.
  - `gpt-5` profile: `/chat` returned `500` immediately.
- The live `default` profile reproduced the core regression without needing to guess from code alone; the log line `error="no base URL for openai"` was decisive.
- The fix belongs in shared settings bootstrap, not in `cmd/web-chat` only, because the missing defaults were introduced at `settings.NewStepSettings()` construction time.
- The new settings test passed and the existing `cmd/web-chat` / `pkg/webchat` suites stayed green after the change.

### What didn't work

- The original `/tmp/webchat.log` gave no visibility into the failing `/chat` request path, which made the first pass mostly a code trace instead of evidence-driven debugging.
- The initial live run on the `default` profile did not reproduce the user’s reported `500`; it returned `200` and only later exposed the underlying regression:
  - `2026-03-08T12:29:10.82313257-04:00 ERR pkg/webchat/chat_service.go:199 > runner completed with error error="no base URL for openai" component=webchat conv_id=3542c2a3-841f-4850-9bbf-624a3e18f179 run_id=5cb0357f-f268-4609-bcde-047ad55ee8c6`
- The user’s exact `500` path only became reproducible after switching the live UI profile to `gpt-5`, which confirmed that cookie-selected profiles can fail immediately while the `default` profile failed later:
  - `POST http://localhost:8082/chat => [500] Internal Server Error`

### What I learned

- GP-029 removed more than CLI/runtime authority overlap; it also removed an implicit defaulting path that engines were still depending on.
- There are currently two classes of failures in webchat:
  - immediate startup failures that surface as `500` from `/chat`,
  - asynchronous run failures that still return `200` to `/chat` but leave the UI with a stuck user message and no assistant output.
- The `chat_profile` cookie is operationally important for debugging because the browser can reproduce a failure mode that a hand-written JSON request body does not make obvious.

### What was tricky to build

- The tricky part was not locating the `/chat` handler; it was separating “request start failed” from “request started but the run died immediately afterward.” The HTTP layer hides that distinction unless you inspect network traffic, websocket state, and server logs together.
- The other subtlety was that the missing default lived in `APISettings`, not in the provider-specific `OpenAI` struct. That meant `openai.NewSettings()` still looked healthy while `MakeClient(...)` later failed because `API.BaseUrls` had been left empty.

### What warrants a second pair of eyes

- Whether `gpt-5`, `gpt-5-mini`, and `gpt-5-nano` should carry explicit API key/base URL profile data, or whether webchat should validate and reject profile selection earlier with a clearer UI/API error.
- Whether asynchronous run failures should also emit a visible SEM/timeline/UI error state so a `200` followed by immediate backend failure does not look like a hung chat.
- Whether other call sites that rely on `settings.NewStepSettings()` were implicitly depending on parsed-value default seeding in the same way.

### What should be done in the future

- Add validation or readiness checks for selected profiles so obviously incomplete runtimes fail with an actionable error before the user presses Send.
- Consider logging successful `/chat` submission context at debug level as well, including resolved runtime key and profile version, so cookie-driven profile mismatches are easier to spot.
- Consider adding an integration regression test that exercises a real composed OpenAI runtime far enough to prove `API.BaseUrls` are present before network I/O begins.

### Code review instructions

- Start with:
  - `geppetto/pkg/steps/ai/settings/settings-step.go`
  - `pinocchio/pkg/webchat/http/api.go`
- Then inspect the new regression coverage in:
  - `geppetto/pkg/steps/ai/settings/settings-step_test.go`
- Re-read the GP-029 Step 4 entry immediately above this step to see how the regression was introduced by the runtime-composer cutover.
- Validation:
  - `go test ./geppetto/pkg/steps/ai/settings -count=1`
  - `go test ./pinocchio/cmd/web-chat ./pinocchio/pkg/webchat/... -count=1`
  - Live repro:
    - load `http://localhost:8082/?conv_id=...`
    - send a prompt on `default`
    - inspect `/tmp/webchat.log`
    - switch profile to `gpt-5`
    - send again and confirm the current server returns `500`

### Technical details

- `settings.NewStepSettings()` now calls a helper that initializes `APISettings` defaults from the OpenAI, Claude, and Gemini section definitions.
- The regression existed because `settings.NewStepSettings()` previously created an empty `APISettings` map and GP-029 stopped using `NewStepSettingsFromParsedValues(...)`, which had been backfilling those defaults through section decoding.
- `MakeClient(...)` in `geppetto/pkg/steps/ai/openai/helpers.go` requires:
  - `APIKeys["<api-type>-api-key"]`
  - `BaseUrls["<api-type>-base-url"]`
  so missing base URL defaults are fatal even when the profile carries a valid API key.
- `pinocchio/pkg/webchat/http/api.go` now logs:
  - request resolution failures,
  - prompt submission failures with `conv_id`, `runtime_key`, `runtime_fingerprint`, `profile_version`, `idempotency_key`, and prompt length.

## Step 6: Remove the default backfill and enforce registry-owned runtime fields

After the first regression fix, the user clarified the intended contract: `web-chat` should not reintroduce provider URL defaults or any other hidden runtime defaults once the profile-registry cutover is in place. The registry has to own the runtime values, and incomplete profiles should fail early instead of inheriting defaults from section schemas or provider engines.

I responded by backing out the API base URL default backfill and moving the strictness into `cmd/web-chat` runtime composition itself. The composer now strips the relevant AI/provider fallback fields from `settings.NewStepSettings()` before applying the resolved profile patch, then validates that the selected profile supplied the provider type, model, and required URL/auth fields.

### Prompt Context

**User prompt (verbatim):** "The settings should be built from the profile registry, which itself contains all the uis. don't do the defaults thing. If the profile is not available in the registry, it needs to fail. 

2026-03-08T12:35:40.765828536-04:00 DBG ../geppetto/pkg/inference/middleware/systemprompt_middleware.go:91 > systemprompt: middleware end block_count=2 inference_id=ec99bbc9-551b-4119-bd3c-643c22213138 session_id=4dfa0075-c05f-428a-98e4-43fa0f96afd2 turn_id=25538212-25dc-4be8-8472-78375c12e553
2026-03-08T12:35:40.76589234-04:00 DBG ../geppetto/pkg/steps/ai/openai_responses/engine.go:975 > Responses: tools attached to request tool_choice=null tool_count=1
2026-03-08T12:35:40.765935257-04:00 DBG ../geppetto/pkg/steps/ai/openai_responses/engine.go:78 > Responses: Turn tool blocks present tool_call_blocks=0 tool_use_blocks=0
2026-03-08T12:35:40.765991936-04:00 DBG ../geppetto/pkg/steps/ai/openai_responses/engine.go:93 > Responses: request input summary input_items=2 input_preview=[{\"parts\":[{\"len\":20,\"text\":\"You are an assistant\",\"type\":\"input_text\"}],\"role\":\"system\"},{\"parts\":[{\"len\":5,\"text\":\"hello\",\"type\":\"input_text\"}],\"role\":\"user\"}]
2026-03-08T12:35:40.766339763-04:00 DBG ../geppetto/pkg/steps/ai/openai_responses/engine.go:106 > Responses: built request include_len=1 input_items=2 model=gpt-5-mini stream=true
2026-03-08T12:35:40.766424973-04:00 DBG ../geppetto/pkg/steps/ai/openai_responses/engine.go:156 > Responses: sending request body_len=911 stream=true url=https://api.openai.com/v1/responses
2026-03-08T12:35:40.766639523-04:00 DBG pkg/webchat/sem_translator.go:152 > received event (SEM) component=web_forwarder event_id=c94e37e6-8cb5-4939-ac1b-96f5f44d6c67 event_type=*events.EventPartialCompletionStart inference_id=ec99bbc9-551b-4119-bd3c-643c22213138 session_id=4dfa0075-c05f-428a-98e4-43fa0f96afd2 turn_id=25538212-25dc-4be8-8472-78375c12e553
2026-03-08T12:35:41.062322717-04:00 DBG ../geppetto/pkg/steps/ai/openai_responses/engine.go:191 > Responses: HTTP response received content_type=application/json status=401
2026-03-08T12:35:41.062694985-04:00 DBG ../geppetto/pkg/steps/ai/openai_responses/engine.go:195 > Responses: HTTP error error_body={\"error\":{\"code\":null,\"message\":\"Missing bearer or basic authentication in header\",\"param\":null,\"type\":\"invalid_request_error\"}} status=401
2026-03-08T12:35:41.062808346-04:00 ERR pkg/webchat/chat_service.go:199 > runner completed with error error=\"responses api error: status=401 body=map[error:map[code:<nil> message:Missing bearer or basic authentication in header param:<nil> type:invalid_request_error]]\" component=webchat conv_id=8f03d5d3-63c5-438b-8b6f-b3396ccff5ff run_id=ec99bbc9-551b-4119-bd3c-643c22213138"

**Assistant interpretation:** Remove the temporary default-backfill fix, make runtime composition depend only on registry-resolved profile data, and fail fast when a selected profile is incomplete.

**Inferred user intent:** Preserve the architectural contract from GP-029: profile registry data is authoritative, and missing provider config should be treated as a profile error rather than patched over by defaults.

**Commit (code):** uncommitted workspace changes

### What I did

- Reverted the temporary `APISettings` default backfill in `geppetto/pkg/steps/ai/settings/settings-step.go`.
- Left the `/chat` request logging in place from Step 5 because it is still useful independent of the defaulting strategy.
- Updated `cmd/web-chat/runtime_composer.go` to:
  - construct `StepSettings`,
  - strip the fallback AI/provider fields that would otherwise come from section defaults (`ai-engine`, `ai-api-type`, token/temperature defaults, API key map, base URL map),
  - apply only the registry-resolved `runtime.step_settings_patch`,
  - validate that the resulting runtime contains the required provider fields before engine construction.
- Added validation for profile-owned runtime completeness:
  - all profiles must provide `ai-api-type` and `ai-engine`,
  - OpenAI/OpenAI Responses profiles must provide URL + auth fields from the registry patch,
  - Claude profiles must provide `claude-api-key` + `claude-base-url`,
  - Gemini profiles must provide `gemini-api-key` + `gemini-base-url`.
- Updated `cmd/web-chat/runtime_composer_test.go` to reflect the new contract:
  - empty resolved runtime now fails,
  - missing provider base URL now fails,
  - successful test fixtures explicitly supply the provider URL in the profile patch.
- Ran:
  - `gofmt -w geppetto/pkg/steps/ai/settings/settings-step.go pinocchio/cmd/web-chat/runtime_composer.go pinocchio/cmd/web-chat/runtime_composer_test.go pinocchio/pkg/webchat/http/api.go`
  - `go test ./pinocchio/cmd/web-chat ./pinocchio/pkg/webchat/... -count=1`

### Why

- The user explicitly rejected the backfill approach because it hid incomplete profile data behind schema defaults.
- The runtime composer is the right place to enforce the contract because it is where profile resolution turns into concrete engine settings for `web-chat`.
- Scrubbing the fallback fields before patch application removes the accidental `gpt-4/openai` escape hatch while preserving the rest of the object initialization code.

### What worked

- The log snippet the user provided confirmed that OpenAI Responses still had its own default URL path (`https://api.openai.com/v1/responses`) and therefore could still mask missing registry data unless `web-chat` rejected the profile first.
- Localizing the strict validation to `cmd/web-chat` avoided changing global Geppetto engine semantics while still enforcing the registry-only contract for this app.
- The updated runtime composer tests passed after shifting expectations from “fallback succeeds” to “incomplete profile fails”.

### What didn't work

- The first attempt at fixing the regression by restoring API base URL defaults directly contradicted the intended design once the user clarified that the registry must own those values.
- OpenAI Responses currently still has its own engine-level base URL fallback, which means the engine alone cannot be trusted to enforce the registry-only contract. That is why validation had to move earlier into `cmd/web-chat`.

### What I learned

- There were actually two different default channels to worry about:
  - section-schema defaults in `settings.NewStepSettings()`,
  - provider-engine defaults inside the concrete engine implementations.
- Enforcing registry ownership requires stripping the former and validating before the latter can run.
- The “profile not available in registry” case was already handled by the request resolver; the missing piece was “profile exists but is incomplete”.

### What was tricky to build

- The tricky part was preserving enough initialized structure for the engine factory to work while removing only the fields that violate the registry-only contract. Replacing `NewStepSettings()` entirely would have been more invasive and riskier; clearing the fallback fields was the smaller, targeted change.
- Another subtlety was `openai-responses`: it can currently reuse `openai-api-key`, so the validation has to be strict about profile ownership without assuming a brand-new key namespace that existing profiles do not use.

### What warrants a second pair of eyes

- Whether `cmd/web-chat` should also reject an empty/default system prompt if the team wants *all* prompt/runtime semantics to be registry-owned rather than just engine/provider selection.
- Whether the Geppetto engine layer should eventually remove or gate its own provider URL fallbacks, now that higher-level apps are moving toward registry-owned runtime policy.

### What should be done in the future

- Add or update the affected profiles in `~/.config/pinocchio/profiles.yaml` so profiles like `gpt-5-mini` include the required URL/auth fields explicitly.
- Consider adding an integration test that exercises the full request path and asserts `/chat` fails immediately with a descriptive error when the selected profile is incomplete.

### Code review instructions

- Start with:
  - `cmd/web-chat/runtime_composer.go`
- Then inspect the regression contract coverage in:
  - `cmd/web-chat/runtime_composer_test.go`
- Confirm the diagnostic surface is still present in:
  - `pkg/webchat/http/api.go`
- Validation:
  - `go test ./pinocchio/cmd/web-chat ./pinocchio/pkg/webchat/... -count=1`

### Technical details

- `stripStepSettingsDefaultsForProfileRuntime(...)` clears the AI/provider fallback fields before `ApplyRuntimeStepSettingsPatch(...)` runs.
- `validateProfileOwnedRuntimeSettings(...)` enforces provider-specific required fields on the resolved runtime before `BuildEngineFromSettingsWithMiddlewares(...)`.
- The temporary `NewAPISettingsWithDefaults()` helper from Step 5 was removed, restoring `settings.NewStepSettings()` to its prior behavior.

## Step 7: Restore hidden section-default/config parsing under the profile overlay

After the registry-only validation pass, the user clarified the actual target design: `web-chat` should still get Geppetto section defaults and layered config/env values, but it should do so through an internal parsed-values pass rather than by exposing the full AI flag surface on the public CLI. The profile registry remains authoritative for profile selection and runtime patching, but it now patches over a hidden base `StepSettings` instead of an empty object.

This step keeps the public command surface small while restoring the lost defaults channel that GP-029 accidentally removed. The important distinction is that `web-chat` no longer depends on public AI flags, but it still deliberately reuses the same Glazed section schemas, config mapping, and env parsing rules as the rest of Pinocchio/Geppetto.

### Prompt Context

**User prompt (verbatim):** "yes, do that. also allow loading a config file like ~/.pinocchio/config.yaml as a second section of \"defaults\", exposing the config-file flag. Maybe we can still do that through parsed values but without exposing the command line flags. (except --config-file and the profile flags).

How would that work, mae me a diagram"

**Assistant interpretation:** Reintroduce schema/config/env-derived runtime defaults for `web-chat`, keep the public CLI limited to profile and config-file flags, and document the precedence model clearly.

**Inferred user intent:** Preserve the benefits of the unified Glazed configuration system without reopening `web-chat` as a direct AI-settings command surface.

**Commit (code):** pending

### What I did

- Kept `cmd/web-chat/runtime_composer.go` on the base-plus-profile-overlay model:
  - the composer now clones a caller-supplied base `StepSettings`,
  - then applies `ResolvedProfileRuntime.StepSettingsPatch`.
- Finished the hidden base-settings loader in `cmd/web-chat/main.go`:
  - builds hidden Geppetto sections via `geppetto/pkg/sections.CreateGeppettoSections()`,
  - parses `PINOCCHIO_*` env,
  - parses `~/.pinocchio/config.yaml` if present,
  - parses explicit `--config-file` as a higher-precedence config layer,
  - parses section defaults,
  - converts the result into `settings.StepSettings`.
- Left the public command surface narrow:
  - `--config-file` remains exposed,
  - `--profile` and `--profile-registries` remain exposed,
  - direct AI flags such as `--ai-engine` and `--ai-api-type` remain absent,
  - `--print-yaml`, `--print-parsed-fields`, and `--print-schema` are hidden.
- Added debug logging in `cmd/web-chat/main.go` for the hidden base-settings resolution so `/tmp/webchat.log` shows which config files were used and what non-secret step metadata was derived.
- Rewrote `cmd/web-chat/runtime_composer_test.go` to verify the current contract:
  - base settings are used when the resolved profile runtime is empty,
  - profile patches override the base settings,
  - middleware config resolution still behaves as before.
- Added `cmd/web-chat/main_profile_registries_test.go` coverage for:
  - `resolveWebChatConfigFiles(...)`,
  - `resolveWebChatBaseStepSettings(...)` precedence across defaults/config/env,
  - the minimal public flag surface.
- Ran:
  - `gofmt -w pinocchio/cmd/web-chat/main.go pinocchio/cmd/web-chat/runtime_composer.go pinocchio/cmd/web-chat/runtime_composer_test.go pinocchio/cmd/web-chat/main_profile_registries_test.go pinocchio/pkg/webchat/http/api.go geppetto/pkg/steps/ai/settings/settings-step.go`
  - `go test ./pinocchio/cmd/web-chat ./pinocchio/pkg/webchat/... -count=1`

### Why

- The regression showed that `web-chat` still needs the section-schema defaults path, especially for provider URL defaults and other non-profile runtime settings.
- The user explicitly wanted those defaults without re-exposing the AI CLI flags that GP-029 removed.
- A hidden parsed-values pass is the cleanest way to preserve one configuration system instead of inventing a special-case `web-chat` bootstrap format.

### What worked

- Reusing `CreateGeppettoSections()` means the hidden defaults/config/env layer uses the same schema source of truth as other Pinocchio/Geppetto commands.
- The resolved config precedence now has direct test coverage instead of being an emergent side effect of the old parsed-flag path.
- The debug log added in `main.go` provides the missing runtime trail for future config regressions.

### What didn't work

- The first test rewrite failed to compile because `Chat.ApiType` is `*types.ApiType`, not `*string`.

Exact failure:

- `pinocchio/cmd/web-chat/runtime_composer_test.go:87:20: cannot use ptr("openai") (value of type *string) as *"github.com/go-go-golems/geppetto/pkg/steps/ai/types".ApiType value in assignment`

### What I learned

- The right abstraction is not “bring back AI flags”; it is “run the same section parser internally and keep the resulting values private.”
- `web-chat` needs two separate layers now:
  - public command selection/config wiring,
  - hidden AI/runtime base settings resolution.
- The profile overlay story becomes much easier to reason about when the precedence is explicit and tested.

### What was tricky to build

- The tricky part was preserving the minimal public CLI while still letting `--config-file` influence the hidden base settings. That required disabling the normal builder-owned config loading path and then reusing only the `command-settings.config-file` field as an input into the hidden parse.
- Another subtlety is precedence: the config files need to be ordered low-to-high (`~/.pinocchio/config.yaml`, then explicit `--config-file`), while env must still override both. That is easy to get wrong if the middleware execution order is not checked carefully.

### What warrants a second pair of eyes

- Whether `~/.pinocchio/config.yaml` should stay the only implicit config location for `web-chat`, or whether it should eventually share the broader XDG resolution behavior used elsewhere.
- Whether the hidden base-settings debug log should include the selected config source labels in addition to the file list.

### What should be done in the future

- Restart the running `go run ./cmd/web-chat ...` process and verify `/chat` again with a layered config present in `~/.pinocchio/config.yaml` or via `--config-file`.
- Consider adding an end-to-end test that boots `web-chat` with a temp config file and asserts that request-time profile patching preserves the hidden base URL/auth defaults.

### Code review instructions

- Start with:
  - `cmd/web-chat/main.go`
  - `cmd/web-chat/runtime_composer.go`
- Then inspect:
  - `cmd/web-chat/main_profile_registries_test.go`
  - `cmd/web-chat/runtime_composer_test.go`
- Confirm the request-path diagnostics still exist in:
  - `pkg/webchat/http/api.go`
- Validation:
  - `go test ./pinocchio/cmd/web-chat ./pinocchio/pkg/webchat/... -count=1`

### Technical details

- Effective runtime precedence now works like this:

```text
Geppetto section defaults
        ->
~/.pinocchio/config.yaml
        ->
--config-file
        ->
PINOCCHIO_* env
        ->
base StepSettings
        ->
resolved profile runtime.step_settings_patch
        ->
effective StepSettings
        ->
BuildEngineFromSettingsWithMiddlewares(...)
```

- Public CLI surface:

```text
web-chat flags
  - server/runtime flags (addr, db paths, root, debug-api, ...)
  - --config-file
  - --profile
  - --profile-registries
  - hidden: --print-yaml, --print-parsed-fields, --print-schema
  - not exposed: direct AI/provider flags
```

- The hidden parse in `cmd/web-chat/main.go` is intentionally separate from `cli.BuildCobraCommand(...)` config loading:
  - `BuildCobraCommand` still exposes `command-settings.config-file`,
  - `ConfigFilesFunc` is disabled for the public parse,
  - `resolveWebChatBaseStepSettings(...)` performs the internal Geppetto parse and returns both the derived `StepSettings` and the config file list used.

## Usage Examples

- Use this diary to reconstruct the exact order of implementation steps and commits.
- Use the prompt context sections to understand why each step was taken.
- Use the code review instructions and technical details to repeat validation or continue the work.

## Related

- [Design Brief](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp/2026/03/07/GP-029--webchat-values-separation-brief/design-doc/01-webchat-values-separation-brief.md)
- [Ticket Index](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp/2026/03/07/GP-029--webchat-values-separation-brief/index.md)
