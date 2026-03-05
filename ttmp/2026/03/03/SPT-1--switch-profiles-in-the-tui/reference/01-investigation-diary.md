---
Title: Investigation diary
Ticket: SPT-1
Status: active
Topics:
    - tui
    - profiles
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: bobatea/pkg/chat/model.go
      Note: Removed backend.Kill() on completion to avoid canceling persistence (commit 34b05be)
    - Path: geppetto/pkg/sections/sections.go
      Note: CLI-layer config middlewares that are being replaced by profile selection
    - Path: go.work
      Note: Workspace structure (bobatea/geppetto/pinocchio)
    - Path: pinocchio/README.md
      Note: Pinocchio profile registry loading description and selection knobs
    - Path: pinocchio/cmd/switch-profiles-tui/main.go
      Note: TUI wiring for profile switching
    - Path: pinocchio/pkg/cmds/chat_persistence.go
      Note: How turns runtime_key/inference_id are persisted today
    - Path: pinocchio/pkg/ui/timeline_persist.go
      Note: Timeline projection persistence
    - Path: pinocchio/scripts/switch-profiles-tui-smoke-and-verify.sh
      Note: End-to-end orchestrator for tmux smoke + persistence assertions
    - Path: pinocchio/scripts/switch-profiles-tui-tmux-smoke.sh
      Note: tmux keystroke driver for /profile switching (real inference)
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-03T16:36:41.431138278-05:00
WhatFor: ""
WhenToUse: ""
---



# Diary

## Goal

Keep a chronological, continuation-friendly record of how we investigated “profile switching in the TUI”, which files/commands mattered, and what decisions the design doc is based on.

## Step 1: Create ticket + map the current architecture

This step creates the docmgr ticket workspace and maps the current code architecture relevant to profile switching. The point is to anchor the upcoming design to concrete files: where the TUI lives, how inference is wired, how profiles are represented, and where persistence happens.

The key outcome is a short “current state” map: Bobatea chat UI sends prompts to a backend; the backend runs Geppetto inference sessions; events flow through Watermill and forwarders into timeline entities; persistence stores turns and a timeline projection, but neither is currently “profile-switch aware” in the TUI.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket to analyze how to add switching profiles in the TUI. 

What I want to do: 

- remove all the geppetto middlewares and replace with profile+profile-registries selection
- allow a modal and slash command /profile to switch profiles in the conversation in the TUI. 
- store which profile was used at which point when storing timeline/turns persistence 

reate a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file
  references.
  It should be very clear and detailed. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create a docmgr ticket and write an intern-friendly, evidence-backed design+implementation guide for adding in-TUI profile switching (`/profile` + modal), replacing the current Geppetto/Glazed “middlewares” selection with explicit profile + registry selection, and persisting profile attribution across timeline/turn storage.

**Inferred user intent:** Make profile selection the primary UX/config knob for the TUI (not ad-hoc middleware wiring), and ensure historical persistence can explain which runtime/profile produced which outputs.

**Commit (code):** N/A

### What I did
- Created a new docmgr root (`ttmp/`) in this repo and seeded vocabulary.
- Created ticket `SPT-1` and added two docs: a primary design doc and this diary.
- Scanned the repo structure and identified the relevant modules: `bobatea/`, `geppetto/`, `pinocchio/`.
- Located the Bubble Tea chat UI (`bobatea/pkg/chat`) and its submit path (input → backend.Start).
- Located the TUI integration backend (`pinocchio/pkg/ui`) and the tool-loop backend (`pinocchio/pkg/ui/backends/toolloop`).
- Located profile domain APIs (`geppetto/pkg/profiles`) and profile registry source stacking (`geppetto/pkg/profiles/source_chain.go`).
- Located CLI-layer “Geppetto middlewares” (Glazed sources middlewares) entrypoint (`geppetto/pkg/sections/sections.go:GetCobraCommandGeppettoMiddlewares`) and Pinocchio wrapper (`pinocchio/pkg/cmds/cobra.go`).
- Located persistence points for turns and timeline projections:
  - Turn store interface/projection columns (`pinocchio/pkg/persistence/chatstore/turn_store.go`)
  - Timeline store interface (`pinocchio/pkg/persistence/chatstore/timeline_store.go`)
  - UI event → timeline projection persister (`pinocchio/pkg/ui/timeline_persist.go`)

### Why
- The upcoming design needs to be “file-backed”: we must know where to intercept `/profile`, where to rebuild runtime/profile state, and where to persist attribution.
- Profile switching touches multiple subsystems (UI, backend/session, profile resolution, persistence); mapping early prevents a speculative design.

### What worked
- Found a clear UI seam: `bobatea/pkg/chat/model.go` `submit()` trims input and calls `m.backend.Start(ctx, userMessage)`.
- Found a clear profile resolution API: `geppetto/pkg/profiles` exposes `ResolveEffectiveProfile` and `ApplyRuntimeStepSettingsPatch`.
- Found existing persistence projection points and that turns already persist `runtime_key`/`inference_id` when present in turn metadata (`pinocchio/pkg/cmds/chat_persistence.go`, `pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go`).

### What didn't work
- Initial `docmgr status --summary-only` failed because `ttmp/` did not exist:
  - Error: `Error: root directory does not exist: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/ttmp`
  - Fix: created `ttmp/` then ran `docmgr init --seed-vocabulary`.

### What I learned
- “Middlewares” is overloaded in this codebase:
  - Glazed/CLI config middlewares (`[]sources.Middleware` in `geppetto/pkg/sections/sections.go`)
  - Geppetto inference middlewares (`middleware.Middleware` wrapping `RunInference`)
  The ticket needs to be explicit about which one we’re removing/replacing.

### What was tricky to build
- Establishing the correct “switch point” for profile selection: the UI submits a raw string, while the backend/runtime composition is driven by `StepSettings` + profile runtime spec. A clean design must decide whether `/profile` is handled in the UI layer (preferred) or in the backend (possible, but leaks UI concerns into inference).

### What warrants a second pair of eyes
- The interpretation of “remove all the geppetto middlewares”: confirm whether this refers to CLI config-layer middlewares (`GetCobraCommandGeppettoMiddlewares`) vs inference middlewares in the tool-loop backend.
- Persistence semantics: whether “timeline persistence” must include user messages and explicit profile-switch markers, or only assistant/tool entities.

### What should be done in the future
- Add a minimal “current profile” indicator to the UI header/status bar so the user sees what will be used before sending the next prompt.

### Code review instructions
- Start with `ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/design-doc/01-design-profile-switching-in-switch-profiles-tui.md` (the primary deliverable).
- Cross-check “current state” claims against:
  - `bobatea/pkg/chat/model.go` (submit path)
  - `pinocchio/pkg/ui/backend.go` and `pinocchio/pkg/ui/backends/toolloop/backend.go` (backend/session wiring)
  - `geppetto/pkg/profiles/service.go` and `geppetto/pkg/profiles/source_chain.go` (profile resolution + registry stacking)
  - `pinocchio/pkg/ui/timeline_persist.go` and `pinocchio/pkg/persistence/chatstore/*` (persistence)

### Technical details
- Ticket created with:
  - `docmgr init --seed-vocabulary`
  - `docmgr ticket create-ticket --ticket SPT-1 --title "Switch profiles in the TUI" --topics tui,profiles`
  - `docmgr doc add --ticket SPT-1 --doc-type design-doc --title "Design: profile switching in switch-profiles-tui"`
  - `docmgr doc add --ticket SPT-1 --doc-type reference --title "Investigation diary"`

## Step 2: Add submit interception + header hook to Bobatea chat UI

This step adds a small but crucial extension point to the Bobatea chat model: the embedding app can now intercept a submitted line *before* it is sent to inference. That’s the seam we need for `/profile` and other local slash commands (modal open, help text, etc.).

It also adds a header rendering hook so we can display the current profile/runtime key at the top of the TUI. This is a practical UX/debug improvement: interns and operators immediately see which profile will be used for the next prompt.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Add extensibility to the existing Bubble Tea chat UI so `/profile` can be handled locally and the currently-selected profile can be displayed.

**Inferred user intent:** Make profile switching a first-class UX in the TUI without leaking control-plane commands into real inference.

**Commit (code):** `f0ba314` — "chat: add submit interceptor and header hook"

### What I did
- Updated `bobatea/pkg/chat/model.go`:
  - added `WithSubmitInterceptor(...)` to intercept submitted input (after trimming).
  - ensured interceptors can run even while streaming (so the app can show an error like “cannot switch profile while streaming”).
  - added `WithHeaderView(...)` and wired `headerView()` to call it.
- Ran module-level pre-commit hooks (auto-invoked by `lefthook`) which included:
  - `go test ./...`
  - `golangci-lint run ...`
  - `govulncheck ./...`

### Why
- `/profile` must not be sent to the model/provider.
- We need a clean place to open a modal from the submitted input path.
- A header showing the active profile prevents “why did I get a different model?” confusion.

### What worked
- The interceptor hook is small and does not require the host app to understand Bobatea internal messages.

### What didn't work
- N/A

### What I learned
- Bobatea’s submit keybinding is `tab` (see `bobatea/pkg/chat/keymap.go`). This matters for tmux automation scripts later.

### What was tricky to build
- Ordering: the interceptor must run *before* the existing “already streaming” check; otherwise the UI cannot react to local commands while streaming.

### What warrants a second pair of eyes
- Interceptor semantics: whether handled input should create a local timeline entity automatically (today it does not; the host app must emit any UI feedback).

### What should be done in the future
- If we add more local commands, consider a standard “command output” timeline entity kind so these actions are visible and persisted consistently.

### Code review instructions
- Review `bobatea/pkg/chat/model.go`:
  - `WithSubmitInterceptor`, `WithHeaderView`
  - `submit()` ordering (interceptor before `backend.IsFinished` check)
  - `headerView()` hook

### Technical details
- Validation:
  - `go test ./bobatea/...` (from workspace root)
  - `git -C bobatea commit ...` ran `lefthook` checks automatically

## Step 3: Add a runnable “switch-profiles-tui” command (real inference + /profile)

This step builds a concrete, runnable TUI that uses **profile registries** as the runtime selection source and supports switching the active profile mid-conversation. The goal is to have something we can smoke-test in tmux with *real provider inference*, not just a fake backend.

The command is intentionally minimal and explicit: it does not rely on the older Glazed “Geppetto middlewares” config parsing; instead it requires `--profile-registries` and fails fast if no profiles are loaded.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Implement the first end-to-end slice: start the TUI with a profile registry stack, send a prompt through a real engine, and switch profiles via `/profile` (including a modal).

**Inferred user intent:** Provide an operationally testable feature slice early, so we can iterate on persistence and UX with confidence.

**Commit (code):** `a9500a3` — "tui: add switch-profiles-tui command and profile backend"

### What I did
- Added a new command: `pinocchio/cmd/switch-profiles-tui/main.go`
  - Flags:
    - `--profile-registries` (required; errors if empty)
    - `--profile` (optional; defaults to registry default profile)
    - `--conv-id` (optional; for persistence correlation)
    - `--timeline-db`, `--turns-db` (SQLite paths; default `/tmp/...`)
  - Starts a Bubble Tea program with the Bobatea chat UI.
  - Registers a Watermill router handler `ui.StepChatForwardFunc` to map provider events → timeline entities.
  - Registers `ui.StepTimelinePersistFunc` to persist assistant timeline entities (more attribution work comes later).
- Added a small profile “domain” package for the TUI:
  - `pinocchio/pkg/ui/profileswitch/manager.go` loads registry stacks and resolves effective runtime.
  - `pinocchio/pkg/ui/profileswitch/backend.go` owns a `session.Session` and swaps its `Builder` when switching profiles (idle-only).
  - `pinocchio/pkg/ui/profileswitch/manager_test.go` covers default-profile resolution.
- Implemented `/profile`:
  - `/profile` opens a `huh` modal picker.
  - `/profile <slug>` switches immediately.
  - both paths emit a local timeline marker and publish an `EventInfo("profile-switched")` (persistence hook will be extended later).
- Added a turn-store persister:
  - `pinocchio/cmd/switch-profiles-tui/persistence.go` stores final turns into `SQLiteTurnStore` and projects `runtime_key` when present.

### Why
- We need a real end-to-end harness to test:
  - profile registry loading,
  - switching logic,
  - event routing and UI updates,
  - persistence stores.

### What worked
- The new command builds and tests cleanly (`go test ./pinocchio/...`).
- Profile switching is gated on “not streaming” by checking `session.IsRunning()`.

### What didn't work
- N/A (no runtime execution performed yet; tmux smoke tests come next).

### What I learned
- `geppetto/pkg/profiles.ResolvedProfile.EffectiveRuntime` is a value (not a pointer), so nil checks are invalid; runtime fields must be read directly.

### What was tricky to build
- Keeping the backend “profile-aware” without losing conversation state: the correct seam is `session.Session.Builder`, which is read at `StartInference`.
- UI modal integration: easiest is a wrapper model that shows `huh.Form` when it receives an `openProfilePickerMsg`, while blurring the inner chat input.

### What warrants a second pair of eyes
- Whether we want the forwarder (UI handler) or backend (Start Cmd) to emit `BackendFinishedMsg`. Right now the backend relies on `ui.StepChatForwardFunc` to emit it on provider final/error.

### What should be done in the future
- Add runtime/profile attribution to provider event metadata and ensure timeline persistence stores it (next planned step).

### Code review instructions
- Start at `pinocchio/cmd/switch-profiles-tui/main.go` and follow:
  - profile registry load → manager resolve → backend init
  - `/profile` interceptor → modal picker → backend switch
  - router handlers: UI forwarder + timeline persistence
- Review `pinocchio/pkg/ui/profileswitch/backend.go` for correct “idle-only switch” semantics.

### Technical details
- Tests:
  - `go test ./pinocchio/...`
- Commit hook note:
  - Pinocchio pre-commit runs `go generate ./...` and may attempt frontend installs; this is expected in this repo.

## Step 4: Propagate runtime/profile attribution into events and persist it

This step makes profile switching *auditable* after the fact. We already set `turns.KeyTurnMetaRuntime` on each new turn before inference, but that wasn’t enough for timeline persistence: the timeline persister only sees Geppetto events, not the full Turn object. The fix is to propagate runtime/profile attribution into `events.EventMetadata.Extra` at the provider-engine layer, then teach the timeline persister to store those fields.

We also persisted explicit “profile switched” markers by handling a dedicated `EventInfo("profile-switched")` event and upserting it as a dedicated timeline entity kind.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Ensure timeline/turn persistence can answer “which profile produced which output” by carrying runtime info through events and storing it in the timeline projection.

**Inferred user intent:** Make post-hoc debugging and auditing possible (and teachable to an intern) without guesswork.

**Commit (code, geppetto):** `ae962af` — "events: propagate runtime/profile attribution from turns"

**Commit (code, pinocchio):** `322d2c0` — "persistence: store runtime attribution and profile-switch markers"

### What I did
- Added `geppetto/pkg/steps/ai/runtimeattrib/runtimeattrib.go`:
  - reads `turns.KeyTurnMetaRuntime` (string or map) and writes normalized keys into `EventMetadata.Extra`:
    - `runtime_key`
    - `runtime_fingerprint`
    - `profile.slug`
    - `profile.registry`
    - `profile.version`
- Wired this into provider engines so every run’s events carry attribution:
  - `geppetto/pkg/steps/ai/openai/engine_openai.go`
  - `geppetto/pkg/steps/ai/claude/engine_claude.go`
  - `geppetto/pkg/steps/ai/gemini/engine_gemini.go`
  - `geppetto/pkg/steps/ai/openai_responses/engine.go`
- Updated `pinocchio/pkg/ui/timeline_persist.go`:
  - when events include attribution keys in `md.Extra`, they are stored in `TimelineEntityV2.Props` for assistant/thinking messages.
  - added special handling for `EventInfo` with `Message == "profile-switched"`:
    - upserts a `TimelineEntityV2` with `Kind == "profile_switch"` containing `from`/`to` and the same attribution fields.
- Added tests in `pinocchio/pkg/ui/timeline_persist_test.go` to verify:
  - runtime attribution keys persist into props,
  - `profile-switched` persists as `Kind == "profile_switch"`.

### Why
- Turns store the canonical runtime choice, but timeline persistence is driven by events; without attribution in event metadata, you can’t reliably annotate persisted timeline messages with profile/runtime data.

### What worked
- The attribution path is now deterministic:
  - backend sets `turns.KeyTurnMetaRuntime` on the Turn,
  - provider engine copies it into event metadata extra,
  - timeline persister stores it on the timeline entity props.

### What didn't work
- N/A

### What I learned
- `structpb.Struct` stores numeric values as float-like scalars; for `profile.version` this is acceptable (small integers).

### What was tricky to build
- Picking a stable schema for attribution keys. The dotted keys (`profile.slug`, `profile.registry`) match existing conventions in the codebase and make downstream JSON/proto consumption easier.

### What warrants a second pair of eyes
- Provider coverage: verify we touched all provider engines that can be used by the TUI in this repo (OpenAI, Claude, Gemini, Responses).

### What should be done in the future
- Consider persisting user messages and other event kinds (tool calls, logs) with attribution too, if timeline replay wants full fidelity.

### Code review instructions
- Review new helper: `geppetto/pkg/steps/ai/runtimeattrib/runtimeattrib.go`.
- Spot-check each provider engine now calls `runtimeattrib.AttachToExtra(...)` near metadata initialization.
- Review `pinocchio/pkg/ui/timeline_persist.go`:
  - attribution mapping from `md.Extra`
  - `profile-switched` handling
- Run tests:
  - `go test ./geppetto/...`
  - `go test ./pinocchio/pkg/ui -run TestStepTimelinePersistFunc`

## Step 5: Make tmux smoke + persistence verification reliable (Watermill + SQLite + shutdown ordering)

This step turns the feature into something we can reliably validate end-to-end using tmux automation and real inference (using `/tmp/profile-registry.yaml`). The core work here was not “add more features”, but “remove flakiness”: we chased down a hang, then chased down intermittent missing persistence, and then codified the validation steps into repeatable scripts.

The key outcome is a deterministic CI-ish local run: a scripted TUI session submits a prompt, switches profiles, submits another prompt, and the scripts assert both the turns DB and timeline DB show correct runtime boundaries and profile attribution.

### Prompt Context

**User prompt (verbatim):** "ok, add tasks to the ticket and then implement them one by one as you go. YOu can test the TUI in tmux. Use the profile registry /tmp/profile-registry.yaml for testing (switch between haiku and sonnet). 

Commit at appropriate intervals. 
Make sure real inference is done. 
Fail if no profiles are loaded on startup. 

Verify the proper persistence. 

Keep a detailed diary as you work. 

Store all scripts you might write for testing and running in the scripts/ folder of the repo. 

After you are done, write a detailed and exhaustive postmortem for our intern. and upload to remarkable."

**Assistant interpretation:** Make the implementation testable with real inference via tmux, enforce startup failure when registries load zero profiles, verify persistence correctness, and keep scripts + diary updated while committing changes in small chunks.

**Inferred user intent:** Treat this like “production-ish UX”: switching profiles should be robust and auditable, and an intern should be able to re-run a single script and see whether the feature still works.

**Commit (code, bobatea):** `34b05be` — "chat: don't kill backend on completion"

**Commit (code, pinocchio):** `ba0623d` — "tui: harden profile switching persistence"

**Commit (code, pinocchio):** `4f719cd` — "scripts: add tmux smoke + persistence checks"

### What I did
- Added a tmux smoke harness that drives the TUI with real inference and `/profile` switching:
  - `pinocchio/scripts/switch-profiles-tui-tmux-smoke.sh`
  - `pinocchio/scripts/switch-profiles-tui-smoke-and-verify.sh` (orchestrator)
  - `pinocchio/scripts/switch-profiles-tui-verify-persistence.sh`
  - `pinocchio/scripts/switch-profiles-tui-startup-fail.sh` (asserts “no profiles loaded” is fatal)
- Diagnosed and fixed a Watermill pubsub hang/flake by switching the TUI event router to a buffered `gochannel` pubsub and disabling publish→ack blocking:
  - `pinocchio/cmd/switch-profiles-tui/main.go`
- Reduced SQLite lock contention for timeline writes by serializing `TimelineStore.Upsert` calls in-process:
  - `pinocchio/cmd/switch-profiles-tui/main.go` (wrapper store)
  - `pinocchio/pkg/ui/timeline_persist.go` (serialize in handler)
- Ensured profile switch markers are persisted even when the EventInfo path is unreliable under automation:
  - Directly upsert `Kind="profile_switch"` entities into the timeline store using a shared monotonic version counter (`atomic.Uint64`).
- Fixed missing turn persistence during tmux runs by removing a shutdown-ordering cancellation:
  - `bobatea/pkg/chat/model.go` no longer calls `backend.Kill()` when it receives `BackendFinishedMsg`.
- Added a one-off inference helper (safe output; no credential dumps) to isolate provider/profile resolution problems:
  - `pinocchio/scripts/profile-infer-once.go`

### Why
- The tmux smoke scripts needed to be robust enough to act as a regression harness.
- The persistence requirements (“store which profile was used at which point”) are only meaningful if the system is deterministic under realistic execution timing (streaming, background goroutines, UI teardown, etc.).

### What worked
- Running `pinocchio/scripts/switch-profiles-tui-smoke-and-verify.sh` now produces:
  - 2 turns in `/tmp/spt-1-smoke.turns.db` with distinct `runtime_key` values (`mento-haiku-4.5` then `mento-sonnet-4.6`) and non-empty `inference_id`.
  - at least 2 assistant messages in `/tmp/spt-1-smoke.timeline.db` with:
    - `props.runtime_key`
    - `props["profile.slug"]`
    - `props["profile.registry"]`
    - `props.streaming=false` (final)
  - a `profile_switch` entity containing `from`/`to` and runtime attribution.
- Startup-fail behavior is enforced and exercised by script:
  - `pinocchio/scripts/switch-profiles-tui-startup-fail.sh`

### What didn't work
- Initial tmux smoke runs hung waiting for the first persisted turn:
  - Symptom: `turns` table stayed empty while the UI visibly showed an assistant response.
  - Root cause: Bobatea called `backend.Kill()` on `BackendFinishedMsg`, which canceled the inference context before the enginebuilder persister reliably finished writing the final turn snapshot.
- Timeline persistence intermittently failed with `context canceled` during streaming:
  - Root cause: Watermill message contexts can be canceled/invalidated unexpectedly based on ack/teardown ordering; coupling SQLite writes to `msg.Context()` was too fragile.

### What I learned
- In this architecture, “streaming is done” (UI sees EventFinal) and “backend pipeline is completely done” (persister flushed, turn snapshot committed) are not always the same instant; cancellation during that gap breaks persistence.
- Watermill’s in-memory `gochannel` defaults (especially ack-related blocking with low buffering) can produce deadlocks or starvation patterns in UI+streaming situations if used as a critical-path queue.
- SQLite lock errors are almost always “more than one goroutine writing at once” unless explicitly serialized.

### What was tricky to build
- Making the router “fast enough” to not stall inference while also making persistence “reliable enough” to always land:
  - We needed publish to be non-blocking (to avoid inference stalls), but still wanted persistence to happen.
  - The final shape is: event bus is best-effort + buffered; persistence is forced into a single-writer path for the timeline store; turn snapshots are allowed to complete without being canceled by UI cleanup.

### What warrants a second pair of eyes
- The “direct timeline upsert for profile_switch” path: confirm the chosen props schema (`schemaVersion: 1`, `from`, `to`, `runtime_key`, `runtime_fingerprint`, `profile.slug`) matches downstream expectations (webchat projector, future replay tooling).
- The concurrency assumptions around the shared `timelineVersion` counter: confirm no other process is writing to the same timeline DB concurrently in expected deployments.

### What should be done in the future
- N/A for this step (next work is the ticket postmortem + upload bundle).

### Code review instructions
- Reliability fixes:
  - `pinocchio/cmd/switch-profiles-tui/main.go` (gochannel config, `persistSwitch`, `lockedTimelineStore`)
  - `pinocchio/pkg/ui/timeline_persist.go` (detached context for persistence, serialization)
  - `bobatea/pkg/chat/model.go` (removed `backend.Kill()` in `finishCompletion`)
- Validation:
  - `cd pinocchio && ./scripts/switch-profiles-tui-smoke-and-verify.sh`
  - Confirm it uses `/tmp/profile-registry.yaml` but does not print its contents/credentials.

### Technical details
- Watermill configuration used for TUI runs:
  - `gochannel.Config{OutputChannelBuffer: 256, BlockPublishUntilSubscriberAck: false}`
- Persistence artifacts:
  - Turns: `/tmp/spt-1-smoke.turns.db` table `turns` (`runtime_key`, `inference_id`)
  - Timeline: `/tmp/spt-1-smoke.timeline.db` table `timeline_entities` (`kind='message'` + `kind='profile_switch'`)
