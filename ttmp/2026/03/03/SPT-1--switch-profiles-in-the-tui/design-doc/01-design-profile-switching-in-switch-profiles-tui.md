---
Title: 'Design: profile switching in switch-profiles-tui'
Ticket: SPT-1
Status: active
Topics:
    - tui
    - profiles
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: bobatea/pkg/chat/model.go
      Note: Where to intercept /profile before backend.Start; add submit interceptor + header for current profile
    - Path: geppetto/pkg/profiles/service.go
      Note: ResolveEffectiveProfile and ApplyRuntimeStepSettingsPatch are the core profile->effective runtime APIs
    - Path: geppetto/pkg/profiles/source_chain.go
      Note: Registry source stacking (yaml/sqlite/sqlite-dsn) used by --profile-registries
    - Path: pinocchio/pkg/persistence/chatstore/turn_store.go
      Note: Turn persistence contract; runtime_key/inference_id attribution expectations
    - Path: pinocchio/pkg/ui/backends/toolloop/backend.go
      Note: Tool-loop backend wiring; candidate to become profile-aware by swapping session.Builder
    - Path: pinocchio/pkg/ui/forwarders/agent/forwarder.go
      Note: Maps Geppetto events to timeline entities; must preserve/display profile attribution markers
    - Path: pinocchio/pkg/ui/timeline_persist.go
      Note: Timeline projection persister; needs to store runtime/profile attribution in entity props
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-03T16:36:39.615082919-05:00
WhatFor: ""
WhenToUse: ""
---


# Design: profile switching in switch-profiles-tui

## Executive Summary

We want to make **profiles** the primary runtime selection mechanism for the terminal chat UI (“the TUI”), and allow switching profiles mid-conversation via a local command (`/profile`) that opens a modal picker. We also want persistence to answer “which profile produced which output?” across both **turn persistence** (serialized `turns.Turn` snapshots) and **timeline persistence** (the timeline entity projection store).

This doc proposes:

1) Replace the current “Geppetto/Glazed config-layer middlewares” approach with explicit **profile + profile-registry source stack** selection (`--profile`, `--profile-registries` and/or an in-TUI modal).
2) Add a **profile manager** that loads registries, lists profiles, resolves effective runtime (system prompt + middleware uses + tools + step settings patch), and exposes a stable “current profile selection”.
3) Add a **profile-aware backend/runtime** that can rebuild the inference builder when the profile changes, without losing conversation history.
4) Extend persistence so each persisted turn/timeline entity includes a **runtime/profile attribution** (at minimum: `runtime_key` and `profile.slug`, optionally `profile.registry` + `profile.version` + `runtime_fingerprint`).

The design is intentionally intern-friendly: it explains the current architecture, defines glossary terms, shows flow diagrams, and provides concrete file-level implementation steps.

## Problem Statement

Today, the terminal chat UI can run an inference backend and render a timeline of entities, but it does not have a first-class concept of “the profile currently active in the conversation”. Runtime composition (which model/provider, which system prompt, which middlewares, which tools) is split between:

- CLI-layer configuration plumbing (Glazed “sources middlewares” such as `GetCobraCommandGeppettoMiddlewares`), and
- app-specific hardcoded inference middleware lists (e.g. `simple-chat-agent` builds a `[]middleware.Middleware` in code).

This makes it hard to:

- switch runtime configuration while the TUI is running,
- attribute persisted outputs to the runtime/profile that produced them,
- teach a new engineer/intern how the system works, because “middlewares” is overloaded and runtime selection is implicit.

We need a coherent, explicit model:

- A conversation has a **current profile selection**.
- A profile resolves to an **effective runtime** (system prompt, middleware uses/config, tools, step settings patch).
- Switching the profile changes runtime behavior for subsequent turns, and we record that change in persistence.

### Important terminology: two different “middlewares”

This ticket uses the word “middleware” in two separate senses:

1) **Glazed/CLI config middlewares**: `[]sources.Middleware` used to parse config/env/flags into `values.Values`.
   - Example: `geppetto/pkg/sections/sections.go:GetCobraCommandGeppettoMiddlewares`

2) **Geppetto inference middlewares**: `middleware.Middleware` functions wrapping inference (`RunInference(ctx, *turns.Turn)`).
   - Example: `pinocchio/pkg/ui/backends/toolloop/backend.go` accepts `[]middleware.Middleware` for the tool loop session.

When you implement “remove all the geppetto middlewares”, you must first confirm which category is intended. This design assumes we are removing (1) from the TUI command wiring, and *deriving* (2) from the selected profile runtime instead of hardcoding them.

## Proposed Solution

### High-level idea

Treat “profile selection” as an explicit conversation state, and make all runtime composition flow from that selection:

1) Load a **profile registry source stack** (`--profile-registries`), producing a `profiles.Registry` that can list/resolve profiles across YAML/SQLite sources.
2) Choose an initial **profile slug** (`--profile` or default).
3) Resolve the profile to an **effective runtime** using `geppetto/pkg/profiles`:
   - system prompt, middleware uses/config, tools, step settings patch
4) Build a backend/runtime that uses that effective runtime to create:
   - engine settings (base settings + profile patch)
   - inference middleware chain (from profile runtime, via middleware definition registry)
   - tool availability settings (allowed tools)
5) Add `/profile`:
   - `/profile` opens a modal (picker/search)
   - choosing a profile:
     - swaps the runtime used for subsequent turns
     - creates a timeline marker (“profile switched: A → B”)
     - ensures persistence stores the new attribution

### System map (current, file-backed)

This repo is a `go.work` workspace containing:

- `bobatea/` — Bubble Tea UI components, including the chat model and timeline shell.
  - Chat UI submit path: `bobatea/pkg/chat/model.go` (`submit()` → `Backend.Start(ctx, userMessage)`).
  - Timeline infrastructure: `bobatea/pkg/timeline/*`.

- `geppetto/` — inference engines, events, profiles domain model, “sections” config parsing helpers.
  - Profile domain: `geppetto/pkg/profiles/*` (registry + resolution + stack + patching).
  - CLI config middlewares: `geppetto/pkg/sections/sections.go:GetCobraCommandGeppettoMiddlewares`.
  - Provider engines publish events using turn metadata: e.g. `geppetto/pkg/steps/ai/openai/engine_openai.go`.

- `pinocchio/` — higher-level app wiring, TUI backends/forwarders, and persistence stores.
  - Tool-loop backend: `pinocchio/pkg/ui/backends/toolloop/backend.go`.
  - Agent forwarder: `pinocchio/pkg/ui/forwarders/agent/forwarder.go`.
  - Timeline projection persister: `pinocchio/pkg/ui/timeline_persist.go`.
  - Turn/timeline stores: `pinocchio/pkg/persistence/chatstore/*`.

### End-to-end dataflow (TUI)

```
User types prompt in TUI
  ↓
Bobatea chat model: submit() trims input and calls backend.Start(ctx, prompt)
  ↓
Backend: appends new Turn to session, then starts inference
  ↓
Provider engine publishes Geppetto events (JSON) to a Watermill topic via WatermillSink
  ↓
Forwarder consumes Watermill messages, decodes events, and injects timeline.UIEntity* into Bubble Tea program
  ↓
Timeline shell updates + renders entities (assistant text, tool calls, logs, etc.)
  ↓
Persistence handlers (optional) store:
  - turns (serialized turn snapshots + runtime_key/inference_id columns)
  - timeline projection (entities stored by version)
```

Key anchors:

- Submit path: `bobatea/pkg/chat/model.go`
- Tool-loop backend: `pinocchio/pkg/ui/backends/toolloop/backend.go`
- Forwarder: `pinocchio/pkg/ui/forwarders/agent/forwarder.go`
- Timeline persist: `pinocchio/pkg/ui/timeline_persist.go`
- Turn store runtime columns: `pinocchio/pkg/persistence/chatstore/turn_store.go` + `.../turn_store_sqlite.go`

### Proposed architecture additions

We introduce two new “first-class” concepts for the TUI:

1) `ProfileManager`: loads registries and resolves profiles into an “effective runtime”.
2) `ProfileSwitchController` (UI command handler): parses `/profile`, opens modal, calls manager, and records the switch.

Optionally, we add:

3) `ProfileAwareBackend`: wraps a `session.Session` and applies the current effective runtime (engine + inference middlewares + tool exposure) when starting inference.

#### Conceptual interfaces (pseudocode)

```go
// ProfileManager is *not* a UI component. It is the runtime policy/selection component.
type ProfileManager interface {
  // Registry sources configured for this chat session.
  RegistrySources() string // comma-separated (yaml/sqlite/sqlite-dsn)

  // Current selection (what will be used for the next turn).
  Current() ProfileSelection

  // Discovery
  ListProfiles(ctx context.Context) ([]ProfileListItem, error)

  // Switch selection; returns the resolved effective runtime.
  SwitchProfile(ctx context.Context, profileSlug string) (ResolvedRuntime, error)

  // Resolve without switching (used at startup).
  Resolve(ctx context.Context, profileSlug string) (ResolvedRuntime, error)
}

type ProfileSelection struct {
  ProfileSlug      string
  RegistrySlugHint string // optional (usually empty for stack lookup)
  RuntimeKey       string
  RuntimeFingerprint string
}

type ResolvedRuntime struct {
  Selection ProfileSelection

  // Effective runtime inputs (from Geppetto profile domain)
  SystemPrompt      string
  MiddlewareUses    []profiles.MiddlewareUse
  AllowedTools      []string
  StepSettingsPatch map[string]any

  // “Effective step settings” after applying patch to base settings
  EffectiveStepSettings *settings.StepSettings

  // Extra metadata to persist (profile version, stack trace, etc.)
  ProfileMetadata map[string]any
}
```

### `/profile` UX and command semantics

#### Slash command parsing

We add a local command parser before inference starts:

- `/profile` → open modal picker
- `/profile <slug>` → switch immediately (no modal)
- `/profile help` → print usage

Optional (future):

- `/profile registries` → show current registry stack string
- `/profile registries set <raw>` → replace registry stack and re-load profiles (more advanced)

#### Modal picker behavior

The picker should:

- display the current profile,
- allow searching by slug/display name,
- show (optional) small metadata: registry slug, last updated version, short description,
- confirm switch and show a timeline marker.

Implementation suggestion (consistent with existing code):

- Use `huh` in an overlay (see `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go` for patterns).
- For large profile lists, consider `bubbles/list` or `bobatea/pkg/listbox` if a typeahead is needed.

### Backend/runtime switching behavior

We want profile switching to affect subsequent turns without losing conversation history.

The cleanest mechanism is:

- Keep a single `session.Session` (it owns turn history and the “only one active inference” invariant).
- When profile selection changes, update `session.Builder` to a builder that uses the newly composed engine/middlewares.

This is supported by the session design:

- `geppetto/pkg/inference/session/session.go` stores `Builder EngineBuilder` and calls `s.Builder.Build(...)` at `StartInference`.

#### Switching safety rule

Switch profile only when the session is idle:

- If `session.IsRunning()` is true, either:
  - reject the switch (`"cannot switch profile while streaming"`) and ask the user to interrupt first, or
  - queue the switch and apply after completion (more complex; not required for v1).

### Persistence changes (profile attribution)

We need to persist *attribution* in two places:

1) **Turn persistence** (serialized `turns.Turn` snapshots):
   - The turn store already projects `runtime_key` and `inference_id` columns when the turn metadata contains them.
   - The persister reads `turns.KeyTurnMetaRuntime` and `turns.KeyTurnMetaInferenceID` from turn metadata:
     - `pinocchio/pkg/cmds/chat_persistence.go` (`cliTurnStorePersister.PersistTurn`)
   - SQLite backfill logic already knows how to extract `runtime_key` from either a string value or a nested map:
     - `pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go` (`stringFromMaybeRuntimeValue`)

   Proposed: ensure each new turn has `turns.KeyTurnMetaRuntime` set to the *current runtime key*, which should match the selected profile key.

2) **Timeline persistence** (timeline entity projection store):
   - Current persister `pinocchio/pkg/ui/timeline_persist.go` stores assistant/thinking “message” entities with props:
     - `schemaVersion`, `role`, `content`, `streaming`
   - It currently does not include runtime/profile attribution.

   Proposed: add fields to persisted entity props, e.g.:

   - `runtime_key`: string
   - `profile.slug`: string
   - `profile.registry`: string (optional)
   - `profile.version`: number (optional)
   - `runtime_fingerprint`: string (optional)

   Where does this data come from?

   - Provider engines already copy `session_id`, `inference_id`, `turn_id` into `events.EventMetadata` by reading turn metadata:
     - example: `geppetto/pkg/steps/ai/openai/engine_openai.go` sets `metadata.SessionID`, `metadata.InferenceID`, `metadata.TurnID` from `t.Metadata`.
   - Extend provider engines to also copy runtime/profile attribution into `metadata.Extra`, then teach `StepTimelinePersistFunc` to persist it.

   This keeps attribution close to the source of truth (the turn being inferred) and avoids brittle joins later.

### Profile switch marker in persisted timeline

We should persist explicit “profile switched” markers so replay/debug UIs can render a stable boundary.

Two options:

**Option A (recommended):** persist a dedicated timeline entity kind.

- Entity kind: `profile_switch`
- Props:
  - `from_profile`, `to_profile`, `at_ms`, `reason` (e.g. “user command”)

**Option B:** store as an assistant/system “message” with role `info`.

Option A is cleaner long-term: it avoids conflating human/assistant messages with control-plane actions.

## Design Decisions

### Decision 1: Handle `/profile` in the UI layer, not inside inference

Rationale:

- `/profile` is a local control-plane action; it should not be sent to the LLM engine.
- Keeping it in the UI layer makes it easier to add a modal and to show errors/help text without touching inference.

Implementation consequence:

- The chat UI needs a hook to intercept submit input *before* calling `backend.Start(...)`.
  - If this hook does not exist, we add one (or wrap the chat model with a host model that inspects and rewrites submissions).

### Decision 2: Switch runtime by swapping `session.Builder`

Rationale:

- `session.Session` already owns turn history and “single active inference” semantics.
- Replacing the builder is a natural way to change runtime composition without forking session history.

### Decision 3: Persist attribution at the source (turn metadata + event metadata)

Rationale:

- Turns are the canonical “what happened” artifact; `runtime_key` belongs on the turn.
- Timeline persistence is a projection derived from events; runtime/profile data should be carried along in the event metadata so the projection can store it without requiring joins.

## Alternatives Considered

### Alternative: implement `/profile` as a tool call (LLM-visible)

Rejected because:

- switching runtime is a control-plane action; it should not depend on model behavior,
- it introduces security and correctness concerns (“LLM can change its own profile”),
- it complicates persistence and auditing.

### Alternative: rebuild the entire session on profile switch

Rejected because:

- it loses or complicates conversation continuity,
- it forces expensive “re-seeding” and UI replay.

### Alternative: keep `GetCobraCommandGeppettoMiddlewares` and add profile switching on top

Rejected for v1 because:

- it keeps runtime selection implicit and spreads logic across config parsing + hardcoded inference wiring,
- it makes intern onboarding much harder (too many moving parts to understand before making progress).

## Implementation Plan

This plan is written so an intern can execute it in order. Each phase ends with something testable.

### Phase 0: confirm target TUI(s)

There are (at least) two relevant “TUI-ish” entrypoints in this workspace:

- A simple standalone chat demo: `bobatea/cmd/chat` (fake backend).
- Pinocchio’s agent TUI example: `pinocchio/cmd/agents/simple-chat-agent` (tool loop + Watermill + overlay).

Decide which of these is “the TUI” in scope. This design assumes we are targeting the Pinocchio-style integration (tool loop), because it already has:

- a chat timeline,
- overlay forms (`huh`),
- Watermill event routing and forwarders,
- persistence stores in `pinocchio/pkg/persistence/chatstore`.

### Phase 1: build profile loading + resolution (non-UI)

1) Parse registry sources:
   - input: comma-separated string
   - helper: `geppetto/pkg/profiles.ParseProfileRegistrySourceEntries`
2) Open the chained registry:
   - `profiles.ParseRegistrySourceSpecs`
   - `profiles.NewChainedRegistryFromSourceSpecs`
3) Resolve a profile:
   - use `Registry.ResolveEffectiveProfile(ctx, profiles.ResolveInput{...})`
4) Convert `ResolvedProfile` into `ResolvedRuntime`:
   - take `EffectiveRuntime.SystemPrompt`, `EffectiveRuntime.Middlewares`, `EffectiveRuntime.Tools`, `EffectiveRuntime.StepSettingsPatch`
   - keep `ResolvedProfile.RuntimeFingerprint` and `ResolvedProfile.Metadata`

File anchors to reuse:

- Registry stack loader: `geppetto/pkg/profiles/source_chain.go`
- Effective resolution + patching: `geppetto/pkg/profiles/service.go` (`ResolveEffectiveProfile`)

Deliverable:

- A unit test that loads a trivial in-memory registry and resolves a profile, asserting `runtime_key` and `system_prompt` are set.

### Phase 2: runtime composer for TUI (engine + middlewares + tools)

We need to turn `ResolvedRuntime` into a concrete runtime:

- an engine configured from step settings
- a `[]middleware.Middleware` chain derived from profile middleware uses
- a tool registry / allowed tool list for tool loop

We already have a working pattern in webchat:

- `pinocchio/cmd/web-chat/runtime_composer.go` resolves middleware uses via `middlewarecfg` definitions and applies `step_settings_patch`.

Recommended approach:

- Create a reusable “TUI runtime composer” (similar structure to `ProfileRuntimeComposer`):
  - input: base parsed values (for provider keys), middleware definitions registry, build deps
  - output: base engine + inference middleware chain + allowed tools + runtime key/fingerprint

Key building blocks:

- Middleware definitions registry: `geppetto/pkg/inference/middlewarecfg` + app-owned registries (see `pinocchio/cmd/web-chat/middleware_definitions.go`)
- Engine building helper: `pinocchio/pkg/inference/runtime/engine.go:BuildEngineFromSettingsWithMiddlewares`

Deliverable:

- A function that takes `ResolvedRuntime` and returns `(engine.Engine, []middleware.Middleware, []string allowedTools, runtimeKey string, fingerprint string)`.

### Phase 3: profile-aware backend that can switch runtime

We implement a backend that:

- holds a single `*session.Session`,
- holds a `ProfileManager` (current selection + composer),
- on `Start(ctx, prompt)`:
  1) appends a new turn (`AppendNewTurnFromUserPrompt`)
  2) sets turn metadata for runtime attribution (`turns.KeyTurnMetaRuntime`)
  3) starts inference (`StartInference`) using the current `session.Builder`
- on profile switch:
  1) compose runtime from the selected profile
  2) update `session.Builder` for subsequent runs

Anchor:

- Session API: `geppetto/pkg/inference/session/session.go`

Deliverable:

- A unit test that:
  - creates backend with two profiles,
  - switches profile while idle,
  - asserts `session.Builder` changes (e.g. fingerprint/runtime key differs).

### Phase 4: `/profile` command + modal

We need to intercept input before `backend.Start` is called.

Implementation options:

**Option A (preferred):** add a “submit interceptor” option to Bobatea chat model.

- Add a `ModelOption` in `bobatea/pkg/chat/model.go`:

```go
type SubmitInterceptor func(input string) (handled bool, cmd tea.Cmd)
func WithSubmitInterceptor(fn SubmitInterceptor) ModelOption { ... }
```

- In `submit()`:
  - after `userMessage := strings.TrimSpace(rawInput)` and before adding user timeline entity / calling backend:
    - call interceptor
    - if handled: do not call backend, do not add user message to timeline (or add as local “command” entity)

**Option B:** wrap the chat model in a host model that detects submission messages and replaces them with UI actions.

Given we already have an overlay/host model in `simple-chat-agent`, Option B can be implemented without modifying Bobatea, but it is more brittle (it must understand Bobatea internal messages).

Modal wiring:

- Use `huh` overlay pattern from `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go`.
- Add a new “UI request channel” (parallel to tool UI request channel) for profile switch UI, or reuse the same overlay but with a new request type.

Deliverable:

- Typing `/profile` opens the picker.
- Selecting a profile changes the “current profile” indicator (see Phase 5).

### Phase 5: persistence updates

**Turns**

- Ensure runtime key is set on each new turn before inference:
  - `turns.KeyTurnMetaRuntime.Set(&t.Metadata, runtimeKeyOrProfileSlug)`
- Confirm `TurnStore` row gets `runtime_key` set (already supported):
  - `pinocchio/pkg/cmds/chat_persistence.go`

**Timeline**

- Extend provider engines to copy runtime/profile fields from turn metadata into event metadata extra.
  - Example insertion point: `geppetto/pkg/steps/ai/openai/engine_openai.go` (right after setting `metadata.SessionID`/`metadata.InferenceID`/`metadata.TurnID`).
  - Suggested extra keys:
    - `runtime_key`
    - `profile.slug`
    - `profile.registry`
    - `profile.version`
    - `runtime_fingerprint`
- Extend `pinocchio/pkg/ui/timeline_persist.go` to read those extra keys (when present) and add them into `TimelineEntityV2.Props`.

**Profile switch markers**

- When the user switches profiles, persist an explicit marker into the timeline store:
  - either by writing directly to the store (if you have it in scope), or
  - by publishing an `events.EventInfo` with a well-known message + data that `StepTimelinePersistFunc` handles.

Deliverable:

- A stored timeline snapshot where assistant entries include `runtime_key` (and ideally profile slug).

### Phase 6: remove the old CLI-layer “Geppetto middlewares” wiring

Where it exists today:

- `pinocchio/pkg/cmds/cobra.go:BuildCobraCommandWithGeppettoMiddlewares` uses `sections.GetCobraCommandGeppettoMiddlewares`.

We introduce a new builder that only parses:

- profile selection (`--profile`)
- profile registries (`--profile-registries`)
- persistence DSNs/paths (if needed)
- any app-specific knobs

Then runtime composition uses the profile registry to produce effective settings, rather than relying on `GetCobraCommandGeppettoMiddlewares`.

Deliverable:

- The TUI command still runs, but the “config parsing story” is now: select registries + profile, resolve effective runtime, build runtime.

## File-by-file implementation guide (intern checklist)

This section turns the architecture into concrete edits. Use it as your “day-to-day” guide.

### 1) Intercept submit input (`/profile`) before inference

**File:** `bobatea/pkg/chat/model.go`

Goal: add a hook so the embedding app can intercept the user’s submitted string.

Where:

- `submit()` currently:
  - trims input,
  - adds a user timeline entity,
  - calls `m.backend.Start(ctx, userMessage)`.

What to add:

- a new `ModelOption` (recommended shape):

```go
type SubmitInterceptor func(input string) (handled bool, cmd tea.Cmd)
func WithSubmitInterceptor(fn SubmitInterceptor) ModelOption { ... }
```

- in `submit()` call the interceptor right after `userMessage := strings.TrimSpace(rawInput)` and before:
  - adding the user message entity, and
  - starting the backend.

Edge cases to handle:

- If handled, you may still want to clear the input box.
- If handled, you probably want to create a local timeline entity:
  - Kind: `plain` or `log_event`
  - Props: “Switched profile to …” or “Opening profile picker…”

### 2) Display current profile in the UI (very helpful)

**File:** `bobatea/pkg/chat/model.go`

Goal: show something like `profile: analyst` in the header.

Notes:

- `headerView()` currently returns `""` (empty).
- Add a `WithHeaderRenderer(func() string)` or a simple `WithHeaderText(string)` option.
- Keep it short and stable; the header should not cause layout flicker.

### 3) Implement a profile manager (loading + resolve + list)

**New file (suggested):** `pinocchio/pkg/ui/profiles/manager.go`

Responsibilities:

- Parse `--profile-registries` into specs and open a chained registry:
  - `profiles.ParseProfileRegistrySourceEntries`
  - `profiles.ParseRegistrySourceSpecs`
  - `profiles.NewChainedRegistryFromSourceSpecs`
- List available profiles (for modal):
  - `registry.ListProfiles(ctx, ...)` for each registry slug (or use stack lookup rules)
- Resolve a profile into:
  - system prompt
  - middleware uses
  - allowed tools
  - effective step settings (apply patch)
  - runtime key + fingerprint + profile metadata payload for persistence

Recommended data model:

```go
type Manager struct {
  registry profiles.Registry       // chained registry over sources
  baseStepSettings *settings.StepSettings
  current ResolvedRuntime
}
```

### 4) Implement a runtime composer (profile runtime → concrete middlewares)

**Prefer reusing:** `pinocchio/cmd/web-chat/runtime_composer.go`

For the TUI, you need a similar composition step:

- take `RuntimeSpec.Middlewares` (list of `{name,id,enabled,config}`)
- resolve against a `middlewarecfg.DefinitionRegistry`
- build a `[]middleware.Middleware` chain with app-owned dependencies (`middlewarecfg.BuildDeps`)

Suggestion:

- Extract the reusable parts from `pinocchio/cmd/web-chat/runtime_composer.go` into a package usable by TUI code (if it isn’t already).
- Or copy the minimal subset into `pinocchio/pkg/ui/profiles/composer.go` initially, then refactor later.

Also decide where middleware definitions live for the TUI:

- webchat example: `pinocchio/cmd/web-chat/middleware_definitions.go`
- for TUI: create a similar registry in `pinocchio/pkg/ui/profiles/middleware_definitions.go`

### 5) Make the backend profile-aware (switchable builder)

**File(s):**

- `pinocchio/pkg/ui/backends/toolloop/backend.go` (current tool-loop backend)
- or introduce `pinocchio/pkg/ui/backends/profileaware/backend.go` that wraps a `*session.Session`

Goal: on profile switch, update the session’s builder so the next `StartInference` uses the new runtime.

Minimal behavior:

- deny switching while `sess.IsRunning()`
- on switch:
  - compose runtime from new profile
  - set `sess.Builder = enginebuilder.New(... WithBase(engine), WithMiddlewares(mws...) ...)`

Where to set runtime attribution for turn persistence:

- Right after appending the new turn:
  - `turns.KeyTurnMetaRuntime.Set(&t.Metadata, runtimeKey)`
- Consider also recording:
  - profile slug, registry slug, profile version, fingerprint
  - (either in typed turn metadata keys, or in a safe “extra” metadata map)

### 6) Implement `/profile` modal and switch action

**Likely file:** the host/overlay model in the embedding app.

If you follow the `simple-chat-agent` pattern:

- Overlay patterns:
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go` blurs input while a `huh.Form` is active.

Modal pseudocode:

```go
// /profile -> open form with a select list of profiles
// on completion: manager.SwitchProfile(...)
// then: emit a timeline marker and update header text
```

Sequence diagram (profile switch):

```
User types "/profile"
  ↓
SubmitInterceptor sees "/profile" and returns handled=true + cmd(openModal)
  ↓
OverlayModel shows huh.Form (select profile)
  ↓
User selects "analyst" and hits Enter
  ↓
UI calls manager.SwitchProfile("analyst")
  ↓
Backend updates session.Builder (idle-only)
  ↓
UI appends local timeline marker: "profile switched: ... -> analyst"
  ↓
(Optional) UI persists marker into timeline store
```

### 7) Update timeline persistence to store runtime/profile attribution

**File:** `pinocchio/pkg/ui/timeline_persist.go`

Add fields in `Props` for assistant/thinking messages:

```json
{
  "schemaVersion": 2,
  "role": "assistant",
  "content": "…",
  "streaming": false,
  "runtime_key": "analyst",
  "profile.slug": "analyst",
  "profile.registry": "default",
  "profile.version": 7,
  "runtime_fingerprint": "…"
}
```

How to obtain those fields:

- Prefer: read them from `ev.Metadata().Extra` keys set by provider engines.
- Fallback: if absent, store nothing (do not guess).

### 8) Update provider engines to emit runtime/profile fields in `EventMetadata.Extra`

**Files (examples):**

- `geppetto/pkg/steps/ai/openai/engine_openai.go`
- `geppetto/pkg/steps/ai/claude/engine_claude.go`
- `geppetto/pkg/steps/ai/gemini/engine_gemini.go`
- `geppetto/pkg/steps/ai/openai_responses/engine.go`

Insertion point:

- after reading `SessionID`/`InferenceID`/`TurnID` from `t.Metadata`
- before publishing start event

Suggested keys:

- `runtime_key` (string)
- `profile.slug` (string)
- `profile.registry` (string)
- `profile.version` (number)
- `runtime_fingerprint` (string)

The source of truth should be the current turn metadata and/or the backend’s resolved runtime.

### 9) Remove CLI-layer config middlewares and keep only profile selection (if in scope)

**Files:**

- `pinocchio/pkg/cmds/cobra.go` (current helper)
- call sites like `pinocchio/cmd/agents/simple-chat-agent/main.go` (uses `cli.WithCobraMiddlewaresFunc(geppettosections.GetCobraCommandGeppettoMiddlewares)`)

Target:

- new cobra builder that doesn’t rely on `GetCobraCommandGeppettoMiddlewares`
- instead:
  - parse minimal flags
  - load registries
  - resolve profile
  - compose runtime

## Testing and Validation Strategy

### Unit tests (fast)

- Profile registry parsing:
  - `profiles.ParseProfileRegistrySourceEntries("a.yaml, sqlite-dsn:file:x.db")` parses cleanly.
- Profile resolution:
  - resolve “default” profile from a tiny in-memory registry and assert the system prompt / patch fields.
- Runtime composer:
  - given a resolved runtime with 1 known middleware, ensure the middleware chain builds.
- Backend switching:
  - switching while idle updates runtime key/fingerprint; switching while running is rejected.

### Integration tests (slower)

- Persistence:
  - run a minimal inference and assert the stored turn row has `runtime_key` populated.
  - run UI timeline persister against events containing runtime fields and assert entity props include runtime/profile fields.

### Manual smoke test (tmux)

Follow the shape of `glaze help pinocchio-tui-integration-playbook` (anchors: `pinocchio/pkg/doc/topics/pinocchio-tui-integration-playbook.md`).

Expected behaviors:

- `/profile` opens picker
- switching profile:
  - prints a timeline marker
  - subsequent assistant messages show different runtime metadata (if surfaced)
  - persisted artifacts show runtime_key changes at the correct boundary

## Open Questions

1) Which “TUI” is the target for this ticket?
   - `pinocchio/cmd/agents/simple-chat-agent` (agent/tool loop) seems most relevant, but confirm.

2) What exactly does “remove all the geppetto middlewares” mean?
   - remove Glazed config middlewares (`GetCobraCommandGeppettoMiddlewares`) from the TUI command wiring?
   - also remove hardcoded inference middlewares in the agent example, replacing them with profile runtime `middlewares:`?

3) Should timeline persistence include user messages and explicit profile switch markers, or is turn persistence sufficient for historical attribution?

4) What is the “runtime key” contract in this TUI?
   - should `runtime_key == profile.slug` always, or do we allow custom runtime keys?

5) Should registry source stack switching be supported inside the TUI, or only at startup?
   - supporting it dynamically implies reloading registry sources (YAML/SQLite) and handling write capabilities.

## References

### Primary code anchors (read these first)

- `bobatea/pkg/chat/model.go` — submit path and where `/profile` must be intercepted
- `pinocchio/pkg/ui/backends/toolloop/backend.go` — tool-loop backend/session builder wiring
- `pinocchio/pkg/ui/forwarders/agent/forwarder.go` — event → timeline mapping
- `geppetto/pkg/profiles/service.go` — `ResolveEffectiveProfile`, `ApplyRuntimeStepSettingsPatch`
- `geppetto/pkg/profiles/source_chain.go` — profile registry source stacking (`NewChainedRegistryFromSourceSpecs`)
- `pinocchio/pkg/ui/timeline_persist.go` — event → timeline store projection (needs attribution fields)
- `pinocchio/pkg/persistence/chatstore/turn_store.go` — turn persistence contract (`runtime_key` projection)

### Useful existing documentation in-repo

- `pinocchio/pkg/doc/tutorials/06-tui-integration-guide.md` — intern-friendly explanation of the TUI wiring
- `pinocchio/pkg/doc/topics/pinocchio-tui-integration-playbook.md` — ops/debugging playbook
- `geppetto/pkg/doc/topics/01-profiles.md` — profile registry model and selection precedence
- `pinocchio/pkg/doc/topics/webchat-profile-registry.md` — important: “current profile vs conversation runtime” semantics (same concept applies to TUI)
