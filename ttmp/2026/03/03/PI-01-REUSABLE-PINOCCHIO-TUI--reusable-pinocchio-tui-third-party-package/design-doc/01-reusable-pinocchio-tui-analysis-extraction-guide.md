---
Title: 'Reusable Pinocchio TUI: analysis + extraction guide'
Ticket: PI-01-REUSABLE-PINOCCHIO-TUI
Status: active
Topics:
    - tui
    - pinocchio
    - refactor
    - thirdparty
    - bobatea
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: bobatea/pkg/chat/backend.go
      Note: Bobatea chat.Backend contract (timeline-centric streaming)
    - Path: bobatea/pkg/chat/model.go
      Note: 'Bobatea chat model: WithTimelineRegister'
    - Path: bobatea/pkg/timeline/shell.go
      Note: 'Timeline Shell: embeddable timeline UI component'
    - Path: geppetto/pkg/events/event-router.go
      Note: 'EventRouter: Watermill router abstraction used to stream events'
    - Path: geppetto/pkg/inference/session/session.go
      Note: 'Session: long-lived multi-turn inference lifecycle'
    - Path: pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go
      Note: Agent/tool-loop backend + rich event→timeline mapping (currently trapped under cmd/)
    - Path: pinocchio/pkg/cmds/cmd.go
      Note: Pinocchio CLI wiring for chat mode using ChatBuilder
    - Path: pinocchio/pkg/doc/topics/01-chat-builder-guide.md
      Note: Existing guide describing ChatBuilder standalone + embedding usage
    - Path: pinocchio/pkg/middlewares/agentmode/agent_mode_model.go
      Note: Example reusable timeline renderer factory for agent-mode entities
    - Path: pinocchio/pkg/ui/backend.go
      Note: 'EngineBackend + StepChatForwardFunc: backend + default Watermill-to-timeline forwarder'
    - Path: pinocchio/pkg/ui/runtime/builder.go
      Note: 'ChatBuilder/ChatSession: reusable wiring for Bubble Tea chat programs'
    - Path: pinocchio/pkg/ui/timeline_persist.go
      Note: 'StepTimelinePersistFunc: best-effort persistence from UI event topic'
ExternalSources: []
Summary: Evidence-backed architecture map + refactor plan to make Pinocchio’s chat TUI reusable from third-party packages.
LastUpdated: 2026-03-03T08:02:37.717576379-05:00
WhatFor: ""
WhenToUse: ""
---


# Reusable Pinocchio TUI: analysis + extraction guide

## Executive Summary

You can already build a third-party Bubble Tea TUI on top of Pinocchio today without importing `pinocchio/cmd/...` by using the public runtime glue in:

- `pinocchio/pkg/ui/runtime` (ChatBuilder / ChatSession) — wiring helper for “engine → watermill events → UI”.
- `pinocchio/pkg/ui` (EngineBackend + default event→timeline forwarder) — the default “chat backend” and default Watermill handler.
- `bobatea/pkg/chat` + `bobatea/pkg/timeline` — the UI widgetry (timeline shell, renderers, input control messages).

However, the “agent-style” TUI (tool loop, tool call entities, log entities, web_search entities) is **not** cleanly reusable from a third-party package because key pieces live under `pinocchio/cmd/agents/simple-chat-agent/pkg/...` (a `cmd/` import path).

This doc maps the current architecture (Pinocchio + Geppetto + Bobatea), then proposes a minimal refactor that:

1) keeps the existing `ChatBuilder` API stable,  
2) extracts the tool-loop backend + UI forwarder into `pinocchio/pkg/...` packages, and  
3) standardizes “event → UI timeline entity” mapping in one place so third-party TUIs can reuse it without copy/paste.

Deliverables in this ticket:

- A detailed intern-ready explanation of the system and data flow (this document).
- Copy/paste recipes for building a third-party TUI (see `reference/02-third-party-pinocchio-tui-copy-paste-recipes.md`).
- A refactor plan with concrete file targets and suggested APIs.

## Problem Statement

### What the user wants

“I want to be able to provide my own TUI version of `pinocchio/` in a third-party package.”

Concretely, the desired outcome is:

- Build a separate Go module/package (outside `pinocchio/`) that can:
  - run Pinocchio/Geppetto inference,
  - render a Bubble Tea TUI of the conversation (and optionally tool calls, logs, etc),
  - avoid importing `pinocchio/cmd/...` packages (because those are binary-only internals),
  - reuse as much as possible from the existing codebase (Pinocchio, Geppetto, Bobatea),
  - be understandable and implementable by a new intern.

### What is currently blocking “elegant reuse”

Pinocchio’s “basic chat” runtime wiring is already in `pinocchio/pkg/ui/...` and is reusable.

But the more complete “agent-style chat” (tool loop + tool renderers + UI forwarder emitting tool entities) is implemented inside:

- `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go` (backend + UI event→timeline mapping)
- plus additional app composition models under the same `cmd/...` tree.

That means a third-party package currently must either:

- copy/paste code out of `cmd/` (fragile), or
- import `pinocchio/cmd/...` (undesirable public API), or
- reimplement the same backend + forwarder logic (duplication).

### Non-goals (explicit)

- Replace Bobatea or rewrite the whole TUI framework.
- Replace Geppetto’s engine/session/toolloop architecture.
- Design a new “web UI”; this ticket is about terminal TUIs.

## Current-State Architecture (How it works today)

This section is intentionally detailed and “intern-first”. The goal is that a new engineer can:

1) find the code,  
2) understand the dataflow, and  
3) confidently build a third-party Bubble Tea app that drives Pinocchio/Geppetto inference.

### Glossary (terms you’ll see in code)

- **Bubble Tea program**: `*tea.Program` (the runtime event loop; you call `p.Run()` to start it).
- **Bubble Tea model**: a `tea.Model` (has `Init()`, `Update(msg)`, `View()`).
- **Bobatea chat model**: `bobatea/pkg/chat`’s `model` (the “chat widget” used by Pinocchio).
- **Timeline entity**: a unit in the Bobatea timeline (`timeline.UIEntityCreated/Updated/Completed/Deleted`).
- **Renderer / entity model factory**: a registered factory used to render/handle a specific entity kind (`timeline.Registry`).
- **Geppetto engine**: an `engine.Engine` implementation (OpenAI/Claude/Gemini/etc).
- **Geppetto session**: `geppetto/pkg/inference/session.Session` (a long-lived multi-turn interaction).
- **Turn**: `geppetto/pkg/turns.Turn` (conversation snapshot containing a list of Blocks; “append-only history”).
- **Watermill**: the pub/sub + router library used by `geppetto/pkg/events.EventRouter` for streaming events.
- **Event sink**: a sink the engine writes events to (e.g., `middleware.NewWatermillSink(...)`).
- **UI forwarder**: a Watermill handler that parses Geppetto events and sends Bubble Tea messages.

### “Basic chat” architecture (Pinocchio runtime + Bobatea chat)

#### High-level flow (sequence)

```
User types prompt in TUI
  ↓ (Bubble Tea message: SubmitMessageMsg)
Bobatea chat model calls Backend.Start(ctx, prompt)
  ↓
Pinocchio EngineBackend starts Geppetto session inference
  ↓
Geppetto emits streaming events (partial tokens, final, errors, thinking)
  ↓ (Watermill topic: "ui")
Pinocchio StepChatForwardFunc converts events → timeline.UIEntity* messages
  ↓ (tea.Program.Send)
Bobatea timeline consumes UIEntity* and re-renders
```

#### Concrete objects and where they live (with file anchors)

1) **Runtime wiring helper**: `pinocchio/pkg/ui/runtime.ChatBuilder`  
   - Creates the engine, backend, model, program (`pinocchio/pkg/ui/runtime/builder.go:29`).
   - Returns a `ChatSession` that holds the backend and the bound event handler (`pinocchio/pkg/ui/runtime/builder.go:101`).

2) **Backend (chat.Backend)**: `pinocchio/pkg/ui.EngineBackend`  
   - Implements `bobatea/pkg/chat.Backend` (`pinocchio/pkg/ui/backend.go:24`, `bobatea/pkg/chat/backend.go:26`).
   - Owns a `geppetto/pkg/inference/session.Session` and uses it to run inference (`pinocchio/pkg/ui/backend.go:89`).

3) **UI forwarder (Watermill handler)**: `pinocchio/pkg/ui.StepChatForwardFunc`  
   - Parses event JSON (`events.NewEventFromJson`) and translates to `timeline.UIEntity*` (`pinocchio/pkg/ui/backend.go:244`).

4) **UI widget**: Bobatea chat model  
   - Initialized via `bobatea/pkg/chat.InitialModel(...)` (`bobatea/pkg/chat/model.go:144`).
   - Uses a `timeline.Shell` internally to render and manage selection (`bobatea/pkg/chat/model.go:173` and `bobatea/pkg/timeline/shell.go:11`).
   - Allows renderers to be extended via `WithTimelineRegister` (`bobatea/pkg/chat/model.go:127`).

#### What `ChatBuilder.BuildProgram()` actually does (step-by-step)

Read in `pinocchio/pkg/ui/runtime/builder.go:136`:

1) Validates inputs:
   - requires `EngineFactory`, `StepSettings`, and `EventRouter`.
2) Creates a Watermill sink for topic `"ui"`:
   - `uiSink := middleware.NewWatermillSink(b.router.Publisher, "ui")` (`pinocchio/pkg/ui/runtime/builder.go:150`).
3) Creates a Geppetto engine:
   - `eng, err := b.engineFactory.CreateEngine(b.settings)` (`pinocchio/pkg/ui/runtime/builder.go:151`).
4) Builds the Pinocchio backend (EngineBackend):
   - `backend := ui.NewEngineBackend(eng, uiSink)` (`pinocchio/pkg/ui/runtime/builder.go:156`).
5) Builds the Bobatea chat model and Bubble Tea program:
   - `model := boba_chat.InitialModel(backend, ...)` (`pinocchio/pkg/ui/runtime/builder.go:158`)
   - `program := tea.NewProgram(model, ...)` (`pinocchio/pkg/ui/runtime/builder.go:159`)
6) Attaches the program to the backend (so the backend can seed entities):
   - `backend.AttachProgram(program)` (`pinocchio/pkg/ui/runtime/builder.go:162`)
7) Constructs the session + binds the Watermill handler:
   - handler is either custom (`WithHandlerFactory`) or default (`ui.StepChatForwardFunc`) (`pinocchio/pkg/ui/runtime/builder.go:169` + `pinocchio/pkg/ui/runtime/builder.go:175`).

#### How the CLI registers and runs the handler

In the Pinocchio CLI runtime (not the `cmd/pinocchio` main, but the shared command logic), you can see the intended wiring:

- Build chat program and session with `ChatBuilder` (`pinocchio/pkg/cmds/cmd.go:537`).
- Register the handler on topic `"ui"`:
  - `rc.Router.AddHandler("ui", "ui", sess.EventHandler())` (`pinocchio/pkg/cmds/cmd.go:562`).
- Start the router handlers (`pinocchio/pkg/cmds/cmd.go:563`), then run `p.Run()` (`pinocchio/pkg/cmds/cmd.go:612`).

This is the same wiring your third-party package should do.

### “Agent-style chat” architecture (tool loop + multiple entity kinds)

The “simple chat agent” example is the best reference for a richer TUI:

- Entry point: `pinocchio/cmd/agents/simple-chat-agent/main.go:89`
- Backend: `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:24`

Key differences vs EngineBackend:

1) It uses Geppetto’s **tool loop**:
   - `enginebuilder.New(... WithToolRegistry(...), WithLoopConfig(...))` (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:37`).
2) It forwards **many** Geppetto event types to timeline entities:
   - tool calls, tool results, log events, web search aggregation, etc. (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:104`).
3) It explicitly does **not** “finish” the UI on each provider event; it finishes when the tool loop completes:
   - `BackendFinishedMsg{}` is only sent from `Start()`’s wait command (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:67`).

This agent backend + forwarder is exactly what a third-party “Pinocchio TUI” package likely wants to reuse — hence the proposed extraction out of `cmd/`.

### Bobatea internals you need to understand (minimum required)

If you are building a custom TUI on top of Bobatea, focus on these concepts:

1) **Timeline messages are your rendering protocol**  
   Entities are created/updated/completed by sending:
   - `timeline.UIEntityCreated` (`bobatea/pkg/timeline/types.go:20`)
   - `timeline.UIEntityUpdated` (`bobatea/pkg/timeline/types.go:29`)
   - `timeline.UIEntityCompleted` (`bobatea/pkg/timeline/types.go:37`)

2) **Renderers are pluggable**  
   The chat model creates a registry and registers model factories. You can add your own with `WithTimelineRegister` (`bobatea/pkg/chat/model.go:127`).

3) **Embedding is supported**  
   If you want your own input widget or layout, you can:
   - embed the Bobatea chat model inside a parent model (`runtime.ChatBuilder.BuildComponents()` is designed for this),
   - hide the built-in input via `WithExternalInput(true)` (`bobatea/pkg/chat/model.go:134`),
   - control input with messages like `ReplaceInputTextMsg` and `SubmitMessageMsg` (`bobatea/pkg/chat/user_messages.go:40`).

### Dependency map (mentally useful when refactoring)

```
pinocchio/pkg/ui/runtime  ─┬─> geppetto (engine/session/events)
                           ├─> pinocchio/pkg/ui (EngineBackend + StepChatForwardFunc)
                           └─> bobatea/pkg/chat (UI model)

pinocchio/cmd/.../simple-chat-agent/pkg/backend  ─┬─> geppetto (toolloop)
                                                  ├─> bobatea/pkg/chat + timeline
                                                  └─> pinocchio/pkg/middlewares/... (renderers)
```

The refactor goal is to move the second branch (agent backend/forwarder) from `cmd/` into `pkg/` so third-party packages can depend on it.

## Proposed Solution

### Summary

Support third-party TUIs via two layers:

1) **Today (no refactor required):** document how to build a third-party TUI using `pinocchio/pkg/ui/runtime` + `pinocchio/pkg/ui` + Bobatea, with optional custom handler factories.  
2) **Refactor for elegance:** move “agent-style” backend and UI forwarder out of `cmd/` into `pinocchio/pkg/...` and expose a small, stable API.

### Layer 1: “Use what exists today” (works now)

Use the public runtime API:

- Build a `geppetto/pkg/events.EventRouter`.
- Create a Geppetto engine via `geppetto/pkg/inference/engine/factory.EngineFactory`.
- Use `pinocchio/pkg/ui/runtime.NewChatBuilder()` to:
  - create the backend (`pinocchio/pkg/ui.EngineBackend`),
  - create the Bobatea chat model,
  - return either a ready-to-run `tea.Program` (`BuildProgram`) or components (`BuildComponents`) for embedding into your own Bubble Tea app.

Key evidence:

- `pinocchio/pkg/ui/runtime/builder.go:29` defines `ChatBuilder` and supports both standalone and embedded flows.
- `pinocchio/pkg/ui/backend.go:24` defines `EngineBackend` implementing `bobatea/pkg/chat.Backend` and handles session state + inference lifecycle.
- `pinocchio/pkg/ui/backend.go:244` defines `StepChatForwardFunc`, the default Watermill handler that parses Geppetto events and emits Bobatea timeline messages.
- `bobatea/pkg/chat/backend.go:26` documents the “contract”: backends should inject `timeline.UIEntity*` messages into the Bubble Tea program.

### Layer 2: Extract “agent-style TUI” out of cmd/

The agent example (`pinocchio/cmd/agents/simple-chat-agent`) demonstrates features most TUIs will want:

- Tool call entities (`tool_call`, `tool_call_result`).
- Log entities (`log_event`).
- Web search entities (`web_search`).
- Agent mode entities (`agent_mode`) (renderer already lives in `pinocchio/pkg/middlewares/agentmode`).

But its backend+forwarder are not reusable because they live under `cmd/`.

Proposed refactor:

1) Move the tool-loop backend into `pinocchio/pkg/ui/backends/toolloop` (or similar).
2) Move the forwarder `MakeUIForwarder` into `pinocchio/pkg/ui/forwarders/agent` (or similar).
3) Provide a builder mirroring `ChatBuilder` for tool-loop runs (optional but recommended):
   - `runtime.NewToolLoopChatBuilder()` or `runtime.NewAgentChatBuilder()`.
4) Standardize “Geppetto event → Bobatea timeline entity” mappings into one dedicated package with composable rules:
   - avoid duplicating mapping logic in multiple backends.

This keeps third-party packages from ever importing `pinocchio/cmd/...`.

### Recommended “public surface” (small and stable)

Public API candidates (in `pinocchio/pkg/ui/runtime` and adjacent packages):

```go
// package runtime
type ChatBuilder struct { /* existing */ }
type ChatSession struct { /* existing */ }

// package forwarders (new)
type ForwarderOption func(*Forwarder)
type Forwarder struct { /* stateless mapping rules */ }
func NewForwarder(opts ...ForwarderOption) *Forwarder
func (f *Forwarder) WatermillHandler(p *tea.Program) func(*message.Message) error

// package backends/toolloop (new)
type ToolLoopBackend struct { /* extracted */ }
func NewToolLoopBackend(/* ... */) *ToolLoopBackend
```

Third-party TUIs then choose:

- “simple chat”: `runtime.ChatBuilder` + default `ui.StepChatForwardFunc`
- “agent chat”: `toolloop.ToolLoopBackend` + shared forwarder
- “custom UI”: supply their own `HandlerFactory` (already supported by `ChatBuilder`)

## Design Decisions

### 1) Use Bobatea timeline entities as the “UI wire protocol”

Decision: the reusable API should keep using `bobatea/pkg/timeline` messages (`timeline.UIEntityCreated`, `timeline.UIEntityUpdated`, …) as the primary UI update protocol.

Rationale:

- This is already the contract described by `bobatea/pkg/chat/backend.go:8`.
- It allows small “renderer plugins” (timeline entity model factories) without redesigning the UI core.
- It supports third-party TUIs that want a different layout but still want the timeline data model.

### 2) Keep Watermill router as the boundary between “engine emits events” and “UI consumes events”

Decision: continue using `geppetto/pkg/events.EventRouter` (Watermill) for streaming events.

Rationale:

- Engines already publish event JSON into Watermill sinks (e.g., `middleware.NewWatermillSink(...)`).
- The router abstraction supports both in-memory pub/sub and Redis Streams (see `geppetto/pkg/events/event-router.go:73`).

### 3) Extract, don’t rewrite: move cmd/ logic into pkg/ with minimal changes

Decision: extract existing logic (tool-loop backend + forwarder) into `pinocchio/pkg/...` with minimal API polishing.

Rationale:

- De-risks the refactor by preserving existing behavior.
- Gives third-party packages a clean import path immediately.

### 4) Fix or remove misleading builder knobs (context/seed turn) as part of making the API “intern-safe”

Observation: `pinocchio/pkg/ui/runtime/builder.go` exposes:

- `WithContext(...)` (stored in `b.ctx`), but `b.ctx` is not used in `BuildProgram` or `BuildComponents`.
- `WithSeedTurn(...)` (stored in `b.seedTurn`), but `b.seedTurn` is not used either.

Decision: in the refactor phase, either:

- implement them (recommended if semantics are clear), or
- remove them to avoid interns assuming they do something.

Rationale:

- “No-op options” are a common source of confusion and subtle bugs in reuse scenarios.

## Alternatives Considered

### A) “Just import `pinocchio/cmd/...` from third-party code”

Rejected because:

- `cmd/` is conventionally binary-internal; it is not a stable library API.
- It is likely to churn, breaking external consumers.

### B) “Copy/paste the `simple-chat-agent` code into the third-party package”

Rejected because:

- It duplicates logic (backend, forwarder, renderers) and will diverge over time.
- It defeats the purpose of having a reusable Pinocchio core.

### C) “Expose the entire `pinocchio/pkg/cmds` CLI layer as a public embedding API”

Rejected because:

- The CLI layer mixes concerns: prompt templating, one-shot runs, interactive chat, persistence, flag parsing, etc.
- Third-party TUIs need a small runtime API, not full CLI orchestration.

### D) “Build a new TUI framework API independent of Bubble Tea/Bobatea”

Rejected because:

- The codebase already uses Bubble Tea and Bobatea heavily.
- A new framework would be large and risky, and not necessary to meet the stated goal.

## Implementation Plan

This plan is written as “intern-ready phases”: each phase should leave the repo in a usable state.

### Phase 0 — Documentation + proof of reuse (no refactor)

Goal: make it possible for a third-party package to implement a custom TUI using current public APIs.

Tasks:

1. Document the “basic chat” integration via `runtime.ChatBuilder` (standalone and embedded).
2. Document how to customize:
   - timeline renderers (`bobatea/pkg/chat/model.go:127` via `WithTimelineRegister`)
   - layout using a host model wrapper (your own `tea.Model`)
   - external input (`bobatea/pkg/chat/model.go:134` via `WithExternalInput`)
3. Provide copy/paste recipes (done in this ticket).

Validation:

- Build a minimal third-party prototype (outside `pinocchio/cmd/...`) using `runtime.ChatBuilder`.

### Phase 1 — Extract tool-loop backend (eliminate cmd import)

Goal: enable agent-style TUIs without importing `pinocchio/cmd/...`.

Move (or re-home with minimal changes):

- From: `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go`
- To: `pinocchio/pkg/ui/backends/toolloop/backend.go` (suggested path)

Key API:

- `NewToolLoopBackend(...)` should remain, but accept stable types from Geppetto and Pinocchio packages.

Validation:

- Update `pinocchio/cmd/agents/simple-chat-agent` to import the new `pkg` backend.
- `go test ./...` (at least within the `pinocchio` module) still passes.

### Phase 2 — Extract and standardize UI forwarders (event → timeline mapping)

Goal: avoid duplicated “event → timeline entity” mapping across backends.

Refactor:

- Extract `MakeUIForwarder` mapping logic (currently embedded in `ToolLoopBackend`) into a dedicated package.
- Optionally also move `ui.StepChatForwardFunc` into the same package, so there is one “forwarder family”.

Suggested package:

- `pinocchio/pkg/ui/forwarders`:
  - `forwarders.NewChatForwarder(...)`
  - `forwarders.NewAgentForwarder(...)`
  - Or a single forwarder with options.

Validation:

- Both `pinocchio/pkg/ui/backend.go` and the extracted tool-loop backend use forwarders from the new package.
- The simple agent UI behavior remains unchanged.

### Phase 3 — “Intern-proof” builder API cleanup

Goal: make the public runtime builders obvious and hard to misuse.

Fixes:

- Implement or remove unused knobs:
  - `ChatBuilder.WithContext` (currently stored but unused in `pinocchio/pkg/ui/runtime/builder.go:31`)
  - `ChatBuilder.WithSeedTurn` (currently stored but unused in `pinocchio/pkg/ui/runtime/builder.go:37`)
- Consider adding:
  - `BuildProgramAndRegister(router, ctx)` helpers (optional) to reduce wiring mistakes.

Validation:

- Add compilation tests for the public API (simple `go test` packages that only import and wire).

### Phase 4 — Optional: publish a separate “pinocchio-tui” module

Goal: offer a curated third-party TUI library (your version) as a standalone module.

This module would:

- depend on `pinocchio` + `geppetto` + `bobatea`,
- expose your own `tea.Model` (layout, keymaps, theming),
- use the extracted backends + forwarders.

## Open Questions

1) What level of “own TUI” is desired?

- A) Same core chat widget, different layout/keybindings/theme (easy).
- B) Same timeline entity protocol, but completely different “renderers” (still easy).
- C) Not using Bobatea at all (possible, but then the API surface should expose a UI-agnostic stream).

2) Should Pinocchio officially support tool-call visualization in the “basic chat” forwarder?

- `ui.StepChatForwardFunc` currently contains a note: “Tool-related events can be mapped…” (`pinocchio/pkg/ui/backend.go:388`) but does not do it.
- If we standardize forwarders, this becomes a first-class choice.

3) Should “topic names” be standardized (`ui` vs `chat`)?

- `ChatBuilder` publishes to topic `"ui"` (`pinocchio/pkg/ui/runtime/builder.go:150`).
- Many other handlers use topic `"chat"` (see `pinocchio/cmd/agents/simple-chat-agent/main.go:131` and the CLI `pinocchio/pkg/cmds/cmd.go:489` snippet).

4) How stable should the third-party API be?

- If it’s “internal-ish”, minimal extraction is enough.
- If it’s a stable external contract, add compilation tests and versioning policy.

## References

### Core Pinocchio runtime (reusable today)

- `pinocchio/pkg/ui/runtime/builder.go:29` — `ChatBuilder`, `ChatSession`, `BuildProgram`, `BuildComponents`
- `pinocchio/pkg/ui/backend.go:24` — `EngineBackend` (`bobatea/pkg/chat.Backend` implementation)
- `pinocchio/pkg/ui/backend.go:244` — `StepChatForwardFunc` (default Watermill handler → timeline entities)
- `pinocchio/pkg/ui/timeline_persist.go:19` — `StepTimelinePersistFunc` (best-effort persistence from Geppetto events)

### How the Pinocchio CLI uses the runtime API

- `pinocchio/pkg/cmds/cmd.go:537` — uses `runtime.NewChatBuilder().BuildProgram()` then `router.AddHandler("ui", "ui", sess.EventHandler())`

### Agent-style TUI example (currently stuck under cmd/)

- `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:24` — `ToolLoopBackend` + `MakeUIForwarder`
- `pinocchio/cmd/agents/simple-chat-agent/main.go:235` — Bobatea chat model configured with additional renderers

### Geppetto (engine + event bus)

- `geppetto/pkg/events/event-router.go:28` — `EventRouter` (Watermill router + in-memory default)
- `geppetto/pkg/inference/engine/factory/factory.go:21` — `EngineFactory` interface
- `geppetto/pkg/inference/session/session.go:21` — `Session` model (turn history + inference lifecycle)

### Bobatea (UI framework components)

- `bobatea/pkg/chat/backend.go:26` — `chat.Backend` contract (inject timeline messages into program)
- `bobatea/pkg/chat/model.go:127` — `WithTimelineRegister` for custom renderers
- `bobatea/pkg/chat/model.go:134` — `WithExternalInput` for embedding with custom input UX
- `bobatea/pkg/timeline/types.go:20` — `UIEntityCreated/Updated/Completed/Deleted` types
- `bobatea/pkg/timeline/shell.go:11` — `timeline.Shell` (embeddable timeline UI)
