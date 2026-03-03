---
Title: 'Unified Pinocchio TUI: simple chat + agent tool-loop as reusable primitives'
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
      Note: |-
        Bobatea Backend contract: timeline-centric UIEntity* messages
        Backend contract: timeline-centric UIEntity* messages
    - Path: bobatea/pkg/chat/model.go
      Note: 'Bobatea chat model: timeline registry + WithTimelineRegister + WithExternalInput'
    - Path: bobatea/pkg/timeline/shell.go
      Note: 'Timeline Shell: embeddable timeline UI component'
    - Path: geppetto/pkg/events/event-router.go
      Note: 'EventRouter: Watermill handler wiring; per-handler subscriber option'
    - Path: geppetto/pkg/inference/middleware/sink_watermill.go
      Note: 'WatermillSink: publishes geppetto events onto a topic'
    - Path: geppetto/pkg/inference/session/session.go
      Note: |-
        Session: multi-turn state + StartInference lifecycle + AppendNewTurnFromUserPrompt
        Session lifecycle APIs (AppendNewTurnFromUserPrompt + StartInference) used by unified backend
    - Path: geppetto/pkg/inference/toolloop/enginebuilder/builder.go
      Note: |-
        Enginebuilder runner: registry nil => single-pass; registry non-nil => tool loop
        Core unification insight: registry nil => single-pass; registry != nil => tool-loop
    - Path: pinocchio/cmd/agents/simple-chat-agent/main.go
      Note: Agent cmd wiring (registry/middlewares/renderers/forwarder) to migrate to new builder
    - Path: pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go
      Note: |-
        Current tool-loop backend + rich event→timeline mapping; must move into pkg/
        Current tool-loop backend + rich event→timeline mapping to extract into pkg
    - Path: pinocchio/cmd/agents/simple-chat-agent/pkg/tools/tools.go
      Note: Generative UI tool request types + registration to be moved into pkg/
    - Path: pinocchio/cmd/agents/simple-chat-agent/pkg/ui/host.go
      Note: Sidebar host wrapper (Ctrl+T) to be moved into pkg/
    - Path: pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go
      Note: Generative UI overlay wrapper (Huh forms) to be moved into pkg/
    - Path: pinocchio/pkg/cmds/cmd.go
      Note: |-
        Pinocchio CLI wiring for chat mode; must be migrated to new builder
        Pinocchio CLI chat-mode wiring to migrate to new builder
    - Path: pinocchio/pkg/middlewares/agentmode/agent_mode_model.go
      Note: Example renderer factory Kind=agent_mode used by agent TUI
    - Path: pinocchio/pkg/ui/backend.go
      Note: |-
        Current EngineBackend + StepChatForwardFunc to be replaced by unified backend + projector
        Current EngineBackend + StepChatForwardFunc to be removed/replaced
    - Path: pinocchio/pkg/ui/runtime/builder.go
      Note: Current ChatBuilder (simple chat) to be replaced by unified builder
ExternalSources: []
Summary: Clean-break design to unify Pinocchio’s “simple chat” and “agent/tool-loop chat” TUIs into reusable `pinocchio/pkg/...` primitives (one backend, one event→timeline projector, composable UI wrappers), eliminating cmd-only APIs and duplicate forwarders.
LastUpdated: 2026-03-03T10:04:00-05:00
WhatFor: Provide an intern-ready, evidence-backed plan to refactor Pinocchio’s terminal UI stack into a single coherent reusable library surface (no compatibility shims), so third parties can build custom TUIs over the same core loop.
WhenToUse: 'Use when we want a “clean UI + clean primitives” refactor: unify simple inference and tool-loop under one backend and projector, extract reusable agent/tool-loop pieces out of `cmd/`, and update all call sites to the new API (keeping highly specialized agent-mode UI affordances in `cmd/`).'
---


# Unified Pinocchio TUI: simple chat + agent tool-loop as reusable primitives

## Executive Summary

Pinocchio currently has two terminal UI “tracks”:

1) a **simple chat** Bubble Tea UI built via `pinocchio/pkg/ui/runtime.ChatBuilder` + `pinocchio/pkg/ui.EngineBackend` + `pinocchio/pkg/ui.StepChatForwardFunc` (see `pinocchio/pkg/ui/runtime/builder.go:29`, `pinocchio/pkg/ui/backend.go:24`, `pinocchio/pkg/ui/backend.go:244`), and  
2) an **agent/tool-loop chat** UI whose core backend/forwarder lives under `pinocchio/cmd/agents/simple-chat-agent/pkg/...` (see `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:25` and `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:101`).

These two tracks duplicate:

- backend lifecycle (`Start/Interrupt/Kill/IsFinished`) and session management,
- “geppetto event → bobatea timeline entity” mapping,
- some UI composition primitives (overlay, sidebar host, per-event side effects).

The most important technical observation is that **simple chat is already a special case of the tool-loop runner**: Geppetto’s enginebuilder runner executes a *single-pass inference* when `Registry == nil` and executes the tool loop when `Registry != nil` (`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:31` and `geppetto/pkg/inference/toolloop/enginebuilder/builder.go:190`).

This document proposes a **clean-break refactor** (no compatibility wrappers) that:

- introduces one coherent reusable TUI library surface under `pinocchio/pkg/tui/...`,
- standardizes on a single backend based on `geppetto/pkg/inference/session.Session`,
- standardizes on a single “event → timeline entity” **projector** (instead of two different forwarders),
- turns “simple chat” into “tool-loop with no tools” by setting the tool registry to `nil`,
- moves the reusable “agent UI” primitives (tool-loop backend, projector mapping, overlay forms, tool-driven UI request types) out of `cmd/` into reusable `pkg/`,
- keeps the **agent-mode-specific** UI affordances (the specialized sidebar/host used by `simple-chat-agent`) in `cmd/` because they are not intended as general reusable primitives,
- updates **both** call sites (`pinocchio/pkg/cmds/cmd.go` and `pinocchio/cmd/agents/simple-chat-agent/main.go`) to use the unified primitives,
- provides a stable platform for third-party TUIs to build on without importing `cmd/`.

The result is a simpler mental model for new engineers:

> *There is one loop, one session, one event stream, one projector, one timeline UI.  
> “Simple chat” just means “no tools configured”.*

## Problem Statement

### The user-visible problem

We want:

- one “Pinocchio TUI” that supports both:
  - **simple chat** (just stream assistant text), and
  - **agent/tool-loop chat** (stream assistant text + show tool calls/results/log events/web_search/agent_mode, and support tool-driven “generative UI” forms),
- and we want it to be **reusable from third-party packages** without importing `pinocchio/cmd/...`.

### The engineering problem (current state)

Today, the codebase has split ownership of the same UI responsibilities:

- `pinocchio/pkg/ui/backend.go` contains an `EngineBackend` backend and the default UI forwarder `StepChatForwardFunc` (see `pinocchio/pkg/ui/backend.go:25` and `pinocchio/pkg/ui/backend.go:246`).
- The agent/tool-loop equivalent backend and UI forwarder are in a cmd-only package (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:25` and `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:104`).
- The agent command also contains cmd-only UI wrappers (overlay + host/sidebar) and tool UI request types (see `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go:10`, `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/host.go:9`, `pinocchio/cmd/agents/simple-chat-agent/pkg/tools/tools.go:69`).

This split causes:

- duplicated behavior (event mapping differs subtly across implementations),
- fragmented extension points (third parties must copy/paste from cmd-only code),
- awkward migration paths (we can’t evolve the TUI as a unified product).

### Constraints (must keep in mind)

- Bobatea’s chat UI is explicitly “timeline-centric”: backends should send `timeline.UIEntityCreated/Updated/Completed/Deleted` into the program (`bobatea/pkg/chat/backend.go:11`–`bobatea/pkg/chat/backend.go:26`).
- Geppetto’s session is the correct place for long-lived multi-turn state and “only one active inference at a time” invariant (`geppetto/pkg/inference/session/session.go:21`–`geppetto/pkg/inference/session/session.go:35`).
- We must avoid introducing a public API that forces importing `cmd/` packages (Go convention: `cmd/` is not a reusable API surface).
- Per the user request: **no backwards-compatibility wrappers**. We should design a clean surface and refactor call sites directly.

### Goals

- One set of reusable primitives in `pinocchio/pkg/...` for:
  - session-backed backend execution (single-pass and tool-loop),
  - event routing and UI forwarding,
  - timeline renderer registration,
  - optional overlay (tool-driven forms).
- “Simple chat” is configured as “tool-loop runner with `Registry=nil`”.
- “Agent chat” is the same UI + backend, but with tool registry + agent middlewares + additional renderer factories.

### Non-goals

- Replace Bubble Tea or Bobatea.
- Replace Geppetto’s inference/session/toolloop architecture.
- Guarantee exact visual parity with the old agent cmd UI (we’ll accept UI cleanups).
- Preserve old import paths (`pinocchio/pkg/ui/...` and `pinocchio/cmd/...`) as stable APIs.

## Current-State Architecture (Observed)

This section is deliberately explicit and anchored to code so a new intern can *trace the runtime end-to-end*.

### Glossary (minimum vocabulary to read the code)

- **Bubble Tea program**: `*tea.Program` (created by `tea.NewProgram(model, ...)`, started by `p.Run()`).
- **Bobatea chat model**: `bobatea/pkg/chat` model created via `boba_chat.InitialModel(backend, ...)` (`bobatea/pkg/chat/model.go:144`).
- **Backend**: implements `boba_chat.Backend` (`bobatea/pkg/chat/backend.go:26`–`bobatea/pkg/chat/backend.go:43`).
- **Timeline entity**: items in Bobatea’s timeline created/updated/completed via `timeline.UIEntity*` messages sent into the program (`bobatea/pkg/chat/backend.go:11`–`bobatea/pkg/chat/backend.go:26`).
- **Geppetto event**: JSON-serialized inference event decoded via `events.NewEventFromJson` (used in both current forwarders).
- **Watermill**: message bus and router; handlers are registered per-topic (`geppetto/pkg/events/event-router.go:119`–`geppetto/pkg/events/event-router.go:150`).

### “Simple chat” TUI (Pinocchio CLI)

**Where it is wired today**

- CLI uses `runtime.NewChatBuilder()...BuildProgram()` and then registers the session’s handler on topic `"ui"` (`pinocchio/pkg/cmds/cmd.go:537`–`pinocchio/pkg/cmds/cmd.go:563`).
- `ChatBuilder.BuildProgram` creates a `WatermillSink` publishing to topic `"ui"` and an `EngineBackend` (`pinocchio/pkg/ui/runtime/builder.go:149`–`pinocchio/pkg/ui/runtime/builder.go:176`).
- Default event→timeline mapping is `StepChatForwardFunc` (`pinocchio/pkg/ui/runtime/builder.go:120`–`pinocchio/pkg/ui/runtime/builder.go:124` and `pinocchio/pkg/ui/backend.go:246`).

**Dataflow diagram (current)**

```
User prompt
  |
  v
bobatea chat model -> backend.Start(ctx, prompt)
  |
  v
EngineBackend.Start():
  AppendNewTurnFromUserPrompt()   (pinocchio/pkg/ui/backend.go:105-109)
  StartInference()                (pinocchio/pkg/ui/backend.go:111-114)
  |
  v
WatermillSink.PublishEvent(...) -> topic "ui" (geppetto events JSON)
  |
  v
StepChatForwardFunc:
  decode geppetto event -> send timeline.UIEntity* into tea.Program (pinocchio/pkg/ui/backend.go:294-420)
  |
  v
Bobatea timeline shell renders entities
```

### “Agent/tool-loop chat” TUI (simple-chat-agent command)

**Where it is wired today**

- Builds tool registry + middlewares, then constructs `ToolLoopBackend` (`pinocchio/cmd/agents/simple-chat-agent/main.go:200`–`pinocchio/cmd/agents/simple-chat-agent/main.go:218`).
- Builds a Bobatea chat model with additional renderer factories (`pinocchio/cmd/agents/simple-chat-agent/main.go:235`–`pinocchio/cmd/agents/simple-chat-agent/main.go:249`).
- Creates the program and registers `backend.MakeUIForwarder(p)` as a handler on topic `"chat"` (`pinocchio/cmd/agents/simple-chat-agent/main.go:281`–`pinocchio/cmd/agents/simple-chat-agent/main.go:292`).

**Dataflow diagram (current)**

```
User prompt
  |
  v
bobatea chat model -> backend.Start(ctx, prompt)
  |
  v
ToolLoopBackend.Start():
  AppendNewTurnFromUserPrompt()   (tool_loop_backend.go:57-60)
  StartInference()                (tool_loop_backend.go:62-65)
  enginebuilder runner executes toolloop (registry != nil)
  |
  v
WatermillSink.PublishEvent(...) -> topic "chat"
  |
  v
ToolLoopBackend.MakeUIForwarder:
  decode geppetto event -> send timeline.UIEntity* (tool_call/log/agent_mode/web_search too)
  (tool_loop_backend.go:116-266)
  |
  v
Bobatea timeline shell renders entities using extra factories
```

**Why the forwarders differ today**

- Tool-loop mode can produce multiple provider “final” events over the lifetime of the loop, so the forwarder explicitly avoids sending `BackendFinishedMsg` and relies on backend completion (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:101`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:104`).

### What Bobatea expects from backends/forwarders (contract)

Bobatea’s `Backend` interface is clear: communication is **timeline-centric**, and the UI consumes `timeline.UIEntityCreated/Updated/Completed/Deleted` messages (see `bobatea/pkg/chat/backend.go:11`–`bobatea/pkg/chat/backend.go:26`).

That implies our unification “center of gravity” should be:

- one backend that runs inference (simple or tool-loop),
- one projector that maps Geppetto events to timeline entities.

## Proposed Solution

### One-sentence proposal

Create a new `pinocchio/pkg/tui` library that provides:

1) a unified session-backed backend (`SessionBackend`),  
2) a unified Watermill handler that **projects** Geppetto events into Bobatea timeline entity events (`Projector`), and  
3) a reusable Bubble Tea overlay wrapper for tool-driven forms (`OverlayModel`),  
then refactor both the Pinocchio CLI chat and the simple-chat-agent command to use those primitives directly.

### Architectural “north star”

**Single mental model for the intern:**

- Geppetto emits “domain events” (partial tokens, final text, tool calls, tool results, logs, mode switches, web search).
- Our TUI library projects those events into **timeline entities** (llm_text, tool_call, tool_call_result, log_event, agent_mode, web_search).
- Bobatea chat UI renders the timeline and manages input.
- The backend is just: “append prompt to session turn → run inference (single pass or tool loop) → on completion, send BackendFinishedMsg”.

### Target package layout (new files)

Proposed new module structure under `pinocchio/pkg/tui/`:

```
pinocchio/pkg/tui/
  runtime/
    builder.go              # Unified TUI builder (BuildProgram/BuildComponents)
    wiring.go               # Watermill sink + handler wiring helpers (topic naming, handler names)
  backend/
    session_backend.go      # SessionBackend implementing bobatea/chat.Backend
    seed.go                 # Seed turn + project existing history into timeline
  projector/
    projector.go            # Event→timeline projector + handler registry
    handlers_llm.go         # Partial/final/interrupt/error + thinking events
    handlers_tools.go       # tool_call / tool_result / tool_exec events
    handlers_agent.go       # agent_mode events
    handlers_log.go         # log_event events
    handlers_web.go         # web_search events
    ids.go                  # entity ID conventions + stable id generation
  renderers/
    register.go             # convenience: RegisterDefaultFactories(reg) for pinocchio kinds
  widgets/
    overlay_form.go         # OverlayModel (Huh forms) extracted from cmd
  toolui/
    request.go              # ToolUIRequest/Reply types (no cmd imports)
    generative_ui_tool.go   # RegisterGenerativeUITool(registry, channel)
```

This makes “TUI reuse” a clean import:

- third-party TUIs import `pinocchio/pkg/tui/...`,
- no imports from `pinocchio/cmd/...`.

### The key unification: one backend

Both existing backends already follow the same session lifecycle:

- append prompt: `sess.AppendNewTurnFromUserPrompt(prompt)` (`geppetto/pkg/inference/session/session.go:44`–`geppetto/pkg/inference/session/session.go:93`)
- start inference: `sess.StartInference(ctx)` (same file, below `geppetto/pkg/inference/session/session.go:180`)
- wait and return `boba_chat.BackendFinishedMsg{}`.

We unify with a new `SessionBackend`:

- Always wraps `*session.Session`.
- Always uses `*enginebuilder.Builder` (Geppetto toolloop enginebuilder).
- Accepts an optional tool registry:
  - `nil` registry → single-pass inference (`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:190`–`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:208`).
  - non-nil registry → tool-loop inference (same code path at `geppetto/pkg/inference/toolloop/enginebuilder/builder.go:193`–`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:208`).

This eliminates the need for separate `EngineBackend` vs `ToolLoopBackend` types.

### The key unification: one projector (event → timeline)

Today, there are two different forwarders:

- `pinocchio/pkg/ui.StepChatForwardFunc` (simple chat) (`pinocchio/pkg/ui/backend.go:244`–`pinocchio/pkg/ui/backend.go:423`)
- `ToolLoopBackend.MakeUIForwarder` (agent) (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:101`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:268`)

The projector proposal:

- builds one handler that:
  - parses a watermill message payload into a Geppetto event (`events.NewEventFromJson`),
  - updates its own small internal state (dedupe; “assistant entity created yet?”),
  - sends `timeline.UIEntity*` messages into the `*tea.Program`,
  - **never** sends `boba_chat.BackendFinishedMsg` (backend does that, consistently).

This avoids the current “double-finish” hazard in simple chat:

- `EngineBackend.Start` already returns a cmd that sends `BackendFinishedMsg` on handle completion (`pinocchio/pkg/ui/backend.go:116`–`pinocchio/pkg/ui/backend.go:127`),
- while `StepChatForwardFunc` also sends `BackendFinishedMsg` on `EventFinal` / `EventInterrupt` / `EventError` (`pinocchio/pkg/ui/backend.go:333`, `pinocchio/pkg/ui/backend.go:349`, `pinocchio/pkg/ui/backend.go:371`, `pinocchio/pkg/ui/backend.go:387`).

### “Clean UI” strategy (Bobatea-first)

We standardize on Bobatea’s timeline-centric chat UI as the core widget:

- The backend’s job is to start inference and emit `BackendFinishedMsg` when done (`bobatea/pkg/chat/backend.go:21`–`bobatea/pkg/chat/backend.go:43`).
- The projector’s job is to translate Geppetto events to timeline entities.
- Bobatea’s timeline shell provides a reusable selection-and-viewport component (`bobatea/pkg/timeline/shell.go:11`–`bobatea/pkg/timeline/shell.go:27`).

We keep UI composition primitives optional and reusable:

- **Overlay forms** (tool-driven Huh forms) extracted from `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go:10`–`pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go:83`.

We explicitly keep the `simple-chat-agent` host/sidebar UX in `cmd/` (it is too specialized to be a generally reusable primitive).

The UI primitives should not depend on any particular agent command; they should depend only on:

- Bubble Tea (`tea`),
- Bobatea chat messages (`boba_chat.BlurInputMsg`, `boba_chat.UnblurInputMsg`),
- a small `toolui` request channel for form overlays.

### What changes at call sites (high-level)

- Pinocchio CLI chat mode currently uses `runtime.NewChatBuilder()...BuildProgram()` (`pinocchio/pkg/cmds/cmd.go:537`–`pinocchio/pkg/cmds/cmd.go:617`).
  - It will be updated to `tui/runtime.NewBuilder()...BuildProgram()` and register the unified projector.
- The agent command currently builds a tool-loop backend and a custom forwarder (`pinocchio/cmd/agents/simple-chat-agent/main.go:217`–`pinocchio/cmd/agents/simple-chat-agent/main.go:292`).
  - It will be updated to use the same unified builder/backend and the same projector.

## Proposed APIs (Intern-Friendly Sketches)

This section is intentionally concrete: it shows the shape of the primitives we expect to exist after the refactor, and how they map to existing code.

### `projector.Projector` (Geppetto event → timeline entity) API sketch

The projector is a small, reusable state machine:

- **input**: `events.Event` values decoded from Watermill messages,
- **output**: `timeline.UIEntity*` messages sent into a `*tea.Program`,
- **policy**: never emit `boba_chat.BackendFinishedMsg` (backend owns finish semantics; see DD4).

Pseudocode:

```go
// pinocchio/pkg/tui/projector/projector.go
type Projector struct {
  p *tea.Program

  assistantCreated map[string]bool
  assistantStarted map[string]time.Time
}

func NewProjector(p *tea.Program) *Projector

// HandleWatermillMessage is used as the handler in router.AddHandler(...).
func (pr *Projector) HandleWatermillMessage(msg *message.Message) error {
  msg.Ack()
  ev, err := events.NewEventFromJson(msg.Payload)
  if err != nil {
    return err
  }
  pr.HandleEvent(ev)
  return nil
}

func (pr *Projector) HandleEvent(ev events.Event) {
  md := ev.Metadata()
  entityID := md.ID.String()

  switch e := ev.(type) {
  case *events.EventPartialCompletionStart:
    pr.markAssistantStart(entityID)
  case *events.EventPartialCompletion:
    pr.ensureAssistantEntity(entityID, md, e.Completion) // create on first visible token
    pr.updateAssistant(entityID, md, e.Completion)
  case *events.EventFinal:
    pr.ensureAssistantEntity(entityID, md, e.Text)
    pr.completeAssistant(entityID, md, e.Text)
  case *events.EventToolCall:
    pr.createToolCall(e.ToolCall.ID, e.ToolCall.Name, e.ToolCall.Input)
  case *events.EventToolResult:
    pr.createToolResult(e.ToolResult.ID, e.ToolResult.Result)
  // ... + log_event, agent_mode, web_search
  }
}
```

This is a unified extraction of:

- `pinocchio/pkg/ui/backend.go:246`–`pinocchio/pkg/ui/backend.go:420` (simple chat forwarder), and
- `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:101`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:266` (agent forwarder).

### `backend.SessionBackend` API sketch (one backend for both modes)

The unified backend implements the Bobatea `Backend` interface (`bobatea/pkg/chat/backend.go:26`–`bobatea/pkg/chat/backend.go:43`) and is built around `*session.Session` (`geppetto/pkg/inference/session/session.go:21`–`geppetto/pkg/inference/session/session.go:35`).

Pseudocode:

```go
// pinocchio/pkg/tui/backend/session_backend.go
type SessionBackend struct {
  sess *session.Session
}

func NewSessionBackend(builder *enginebuilder.Builder) *SessionBackend {
  s := session.NewSession()
  s.Builder = builder
  return &SessionBackend{sess: s}
}

func (b *SessionBackend) SessionID() string { return b.sess.SessionID }
func (b *SessionBackend) CurrentTurn() *turns.Turn { return b.sess.Latest() }

func (b *SessionBackend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
  if b.sess.IsRunning() {
    return nil, errors.New("already running")
  }

  // Canonical "next prompt" builder (prevents mutating history in place):
  _, err := b.sess.AppendNewTurnFromUserPrompt(prompt) // geppetto/pkg/inference/session/session.go:50
  if err != nil {
    return nil, err
  }

  handle, err := b.sess.StartInference(ctx)
  if err != nil {
    return nil, err
  }

  return func() tea.Msg {
    _, _ = handle.Wait()
    return boba_chat.BackendFinishedMsg{}
  }, nil
}
```

This merges the overlapping logic of:

- `pinocchio/pkg/ui/backend.go:91`–`pinocchio/pkg/ui/backend.go:127` (EngineBackend.Start), and
- `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:49`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:74` (ToolLoopBackend.Start).

### Unified builder API sketch (replacement for `ChatBuilder`)

The unified builder replaces `pinocchio/pkg/ui/runtime.ChatBuilder` (`pinocchio/pkg/ui/runtime/builder.go:29`), and produces:

- a configured `SessionBackend`,
- a configured `*tea.Program`,
- a configured Watermill handler `func(*message.Message) error` that forwards Geppetto events into that program via the projector.

Pseudocode:

```go
// pinocchio/pkg/tui/runtime/builder.go
type Builder struct {
  ctx context.Context
  router *events.EventRouter

  engine engine.Engine
  engineFactory factory.EngineFactory
  settings *settings.StepSettings

  middlewares []middleware.Middleware
  toolRegistry tools.ToolRegistry // nil => single-pass

  loopCfg *toolloop.LoopConfig
  toolCfg *tools.ToolConfig
  snapshotHook toolloop.SnapshotHook
  persister enginebuilder.TurnPersister

  topic string // default "ui"

  programOpts []tea.ProgramOption
  modelOpts []boba_chat.ModelOption

  // Optional composables
  toolUIReq <-chan toolui.ToolUIRequest
  withSidebar bool
}

type Built struct {
  Backend *backend.SessionBackend
  Program *tea.Program
  Handler func(*message.Message) error
}

func (b *Builder) BuildProgram() (*Built, error)
```

Builder responsibilities (in order):

1) create the engine (or accept one),
2) create a Watermill sink: `middleware.NewWatermillSink(router.Publisher, topic)` (`geppetto/pkg/inference/middleware/sink_watermill.go:21`),
3) build a `enginebuilder.Builder` with `Registry=nil` (simple) or non-nil (agent) (`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:45`),
4) create `SessionBackend`,
5) build Bobatea chat model: `boba_chat.InitialModel(backend, ...)` (`bobatea/pkg/chat/model.go:144`),
6) register renderers via `boba_chat.WithTimelineRegister` (`bobatea/pkg/chat/model.go:127`–`bobatea/pkg/chat/model.go:132`),
7) wrap model via optional widgets (overlay forms; host/sidebar is app-owned),
8) create `tea.Program`,
9) create `projector.Projector` bound to the program and return `Handler`.

### Default renderer registration helper

We should ship a convenience function that registers the renderer factories needed for “agent chat” timeline kinds.

This aligns with what the agent cmd registers today (`pinocchio/cmd/agents/simple-chat-agent/main.go:239`–`pinocchio/cmd/agents/simple-chat-agent/main.go:248`).

Pseudocode:

```go
// pinocchio/pkg/tui/renderers/register.go
func RegisterDefaultFactories(r *timeline.Registry) {
  r.RegisterModelFactory(renderers.NewLLMTextFactory())
  r.RegisterModelFactory(renderers.NewToolCallFactory())
  r.RegisterModelFactory(renderers.ToolCallResultFactory{})
  r.RegisterModelFactory(renderers.LogEventFactory{})
  r.RegisterModelFactory(agentmode.AgentModeFactory{})
  // optional:
  // r.RegisterModelFactory(renderers.WebSearchFactory{})
}
```

## Proposed Unified Runtime Flow (New Architecture)

After unification, **both** “simple chat” and “agent/tool-loop chat” share the same runtime shape. The only difference is whether a tool registry is configured.

```
                       (configuration)
     +--------------------------------------------------+
     | toolRegistry == nil  => single-pass "simple chat"|
     | toolRegistry != nil  => tool-loop "agent chat"   |
     +--------------------------------------------------+

User prompt
  |
  v
bobatea chat model (timeline shell + input)
  |
  v
SessionBackend.Start():
  AppendNewTurnFromUserPrompt()
  StartInference() -> enginebuilder runner
  Wait() -> BackendFinishedMsg
  |
  v
WatermillSink publishes Geppetto events to topic (default "ui")
  |
  v
Projector handler:
  decode Geppetto events -> emit timeline.UIEntity* to program
  |
  v
Timeline shell renders entities (llm_text/tool_call/tool_call_result/log_event/agent_mode/web_search)
```

## Example Usage (Third-Party Friendly)

These are “wiring recipes” that should work both for internal commands and third-party TUIs.

### Example A: Build a “simple chat” TUI (no tools)

```go
router, _ := events.NewEventRouter()

b := tuiruntime.NewBuilder().
  WithRouter(router).
  WithEngineFactory(rc.EngineFactory).
  WithSettings(rc.StepSettings).
  WithMiddlewares(mws...).
  WithToolRegistry(nil). // <- simple chat
  WithTopic("ui").
  WithTimelineRegister(tuirenderers.RegisterDefaultFactories)

built, _ := b.BuildProgram()
router.AddHandler("ui-forward", "ui", built.Handler)
_ = router.RunHandlers(ctx)
_, _ = built.Program.Run()
```

### Example B: Build an “agent/tool-loop chat” TUI (tools enabled)

```go
reg := tools.NewInMemoryToolRegistry()
_ = mytools.RegisterCalculatorTool(reg)
_ = toolui.RegisterGenerativeUITool(reg, toolReqCh)

b := tuiruntime.NewBuilder().
  WithRouter(router).
  WithEngineFactory(rc.EngineFactory).
  WithSettings(rc.StepSettings).
  WithMiddlewares(agentmode.NewMiddleware(...), sqlitetool.NewMiddleware(...)).
  WithToolRegistry(reg).
  WithTopic("ui").
  WithTimelineRegister(tuirenderers.RegisterDefaultFactories).
  WithOverlayForms(toolReqCh).
  WithSidebar(true)

built, _ := b.BuildProgram()
router.AddHandler("ui-forward", "ui", built.Handler)
_ = router.RunHandlers(ctx)
_, _ = built.Program.Run()
```

## Projector Mapping (Recommended Timeline Entity Kinds)

To keep renderer wiring stable for third parties, we should standardize on a small set of timeline entity kinds:

- `llm_text`
  - roles: `assistant`, `thinking`
- `tool_call`
- `tool_call_result`
- `log_event`
- `agent_mode`
- `web_search` (optional)

These align with existing renderer factories:

- `llm_text`: `bobatea/pkg/timeline/renderers/llm_text_model.go:20`
- `tool_call`: `bobatea/pkg/timeline/renderers/tool_call_model.go:105`
- `tool_call_result`: `bobatea/pkg/timeline/renderers/tool_call_result_model.go:66`
- `log_event`: `bobatea/pkg/timeline/renderers/log_event_model.go:117`
- `agent_mode`: `pinocchio/pkg/middlewares/agentmode/agent_mode_model.go:107`

## Design Decisions

This section captures “why we chose this shape” and points to evidence in the codebase.

### DD1: Standardize on Geppetto `session.Session` for all TUIs

**Decision:** All terminal chat modes (simple and tool-loop) use a session-backed backend built on `*session.Session`.

**Why:**

- Session already owns correct invariants: stable SessionID, append-only turns, single active inference (`geppetto/pkg/inference/session/session.go:21`–`geppetto/pkg/inference/session/session.go:35`).
- Both existing backends already use it (simple chat backend: `pinocchio/pkg/ui/backend.go:47`–`pinocchio/pkg/ui/backend.go:54`; tool-loop backend: `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:36`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:47`).

### DD2: Standardize on Geppetto toolloop enginebuilder everywhere

**Decision:** The unified backend always uses `geppetto/pkg/inference/toolloop/enginebuilder.Builder`.

**Why:**

- It explicitly supports both single-pass and tool-loop, by treating a nil registry as single-pass (`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:31`–`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:47`, and `geppetto/pkg/inference/toolloop/enginebuilder/builder.go:190`–`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:208`).
- It already supports sinks, snapshot hooks, persisters, and step controller (`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:58`–`geppetto/pkg/inference/toolloop/enginebuilder/builder.go:74`).

### DD3: Adopt a dedicated “projector” layer for event → timeline mapping

**Decision:** Implement one `projector.Projector` used by both simple and tool-loop modes.

**Why:**

- Bobatea explicitly expects timeline lifecycle messages; it does not expect “raw events” (`bobatea/pkg/chat/backend.go:11`–`bobatea/pkg/chat/backend.go:26`).
- The mapping logic is duplicated today (simple: `pinocchio/pkg/ui/backend.go:308`–`pinocchio/pkg/ui/backend.go:420`; agent: `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:116`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:266`).
- A projector also becomes a natural place to:
  - define consistent entity ID conventions,
  - define consistent “thinking entity” semantics,
  - centralize dedupe and ordering behaviors.

### DD4: Backend is the only component that emits `BackendFinishedMsg`

**Decision:** The projector must never send `boba_chat.BackendFinishedMsg`. Only `SessionBackend.Start` emits that on `handle.Wait()`.

**Why:**

- Tool-loop mode cannot finish on provider “final” events because there can be multiple inference phases; only the session-run completion is authoritative (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:101`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:104`).
- Simple chat mode already sends `BackendFinishedMsg` from the backend command (`pinocchio/pkg/ui/backend.go:116`–`pinocchio/pkg/ui/backend.go:127`), so sending it again in the forwarder is redundant and risks odd UI states.

### DD5: Clean-break refactor (no compatibility shims)

**Decision:** We do not preserve `pinocchio/pkg/ui.EngineBackend`, `pinocchio/pkg/ui.StepChatForwardFunc`, or `pinocchio/pkg/ui/runtime.ChatBuilder` as public entry points.

**Why:**

- Keeping them would either:
  - keep two conceptual stacks alive, or
  - force wrapper layers that are explicitly disallowed by the user request.
- A clean unified `pinocchio/pkg/tui` surface is clearer for third-party reuse.

### DD6: Keep Bobatea as the “UI protocol”

**Decision:** The shared API surface uses Bobatea timeline entity kinds as the canonical rendering contract.

**Why:**

- Existing renderer factories already exist for core kinds (`tool_call`, `tool_call_result`, `log_event`, `llm_text`) in Bobatea (`bobatea/pkg/timeline/renderers/tool_call_model.go:105`, `bobatea/pkg/timeline/renderers/tool_call_result_model.go:66`, `bobatea/pkg/timeline/renderers/log_event_model.go:117`, `bobatea/pkg/timeline/renderers/llm_text_model.go:20`).
- Pinocchio already defines additional factories like `agent_mode` (`pinocchio/pkg/middlewares/agentmode/agent_mode_model.go:107`).
- This minimizes “new protocol surface area”: third parties can plug in by registering renderers with `WithTimelineRegister` (`bobatea/pkg/chat/model.go:127`–`bobatea/pkg/chat/model.go:132`).

### DD7: Keep agent-mode-specific sidebar/host UI in `cmd/`

**Decision:** Keep the `simple-chat-agent` host/sidebar UI in `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/...` and do not extract it into `pinocchio/pkg/tui/...`.

**Why:**

- It is tightly coupled to that specific command’s UX and assumptions (e.g., special-casing tool names like `calc`, and parsing agent-mode-related log/info messages) (`pinocchio/cmd/agents/simple-chat-agent/pkg/ui/sidebar.go:72` and `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/sidebar.go:101`).
- Third-party TUIs should not inherit specialized UX by default. The reusable layer should focus on core primitives (backend + projector + entities + optional generic overlays).

## Alternatives Considered

### A1: Keep `ChatBuilder` stable and only extract agent pieces (minimal refactor)

This is essentially the approach proposed in `design-doc/01-reusable-pinocchio-tui-analysis-extraction-guide.md` (stable API; extract `cmd/...` toolloop backend and forwarder into `pkg/`).

**Why it’s not chosen here:** the user explicitly asked for a clean unified UI and no compatibility wrappers. Keeping the old API alive invites a “two stacks” situation.

### A2: Keep two backends but share only the projector

**Why it’s not chosen:** it still duplicates session + builder configuration and keeps the “simple vs tool-loop” split. Geppetto already provides a unified runner shape; we should use it.

### A3: Eliminate Watermill and push events directly from the engine into the UI

**Why it’s not chosen:** Watermill is a core composition tool in the repo (multiple handlers, optional Redis Streams transport, per-handler subscribers) (`geppetto/pkg/events/event-router.go:123`–`geppetto/pkg/events/event-router.go:150`). Removing it would be a much larger change than needed for “unify the TUI”.

### A4: Replace Bobatea chat UI with a custom Bubble Tea model

**Why it’s not chosen:** Bobatea already provides an interactive timeline shell with registry-driven renderers and embedding hooks (`bobatea/pkg/timeline/shell.go:11`–`bobatea/pkg/timeline/shell.go:27`, `bobatea/pkg/chat/model.go:127`–`bobatea/pkg/chat/model.go:140`). Replacing it would regress features and increase maintenance cost.

## Implementation Plan

This plan is designed so an intern can execute it in phases. Each phase should leave the tree in a buildable state (even if UI behavior isn’t perfect yet).

### Phase 0: Define the new package boundary (no code moves yet)

1) Create new directories under `pinocchio/pkg/tui/` with placeholder Go files and package docs:
   - `pinocchio/pkg/tui/runtime`, `pinocchio/pkg/tui/backend`, `pinocchio/pkg/tui/projector`, `pinocchio/pkg/tui/widgets`, `pinocchio/pkg/tui/toolui`, `pinocchio/pkg/tui/renderers`.
2) Write a `README`-style GoDoc comment in `pinocchio/pkg/tui/doc.go` describing the conceptual model and extension points.
3) Pick stable names for:
   - default Watermill topic name for UI events (recommend: `"ui"` to match existing CLI chat’s sink usage in `pinocchio/pkg/ui/runtime/builder.go:149`–`pinocchio/pkg/ui/runtime/builder.go:156`),
   - default handler name conventions.

Deliverable: directory skeleton compiles (even if unused).

### Phase 1: Implement the unified backend (`SessionBackend`)

**Goal:** one backend that supports both single-pass and tool-loop.

1) Implement `backend.SessionBackend`:
   - holds `*session.Session`,
   - configures `sess.Builder` to a `*enginebuilder.Builder`,
   - implements `boba_chat.Backend` (`bobatea/pkg/chat/backend.go:26`–`bobatea/pkg/chat/backend.go:43`).
2) Expose `SessionID()` and `CurrentTurn()` convenience methods (like today’s `EngineBackend.SessionID` and `ToolLoopBackend.CurrentTurn`):
   - `EngineBackend.SessionID` exists (`pinocchio/pkg/ui/backend.go:79`–`pinocchio/pkg/ui/backend.go:87`).
   - `ToolLoopBackend.CurrentTurn` exists (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:92`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:99`).
3) Ensure `Start(ctx, prompt)`:
   - calls `sess.AppendNewTurnFromUserPrompt(prompt)` (`geppetto/pkg/inference/session/session.go:50`),
   - calls `sess.StartInference(ctx)` (same file),
   - returns a `tea.Cmd` that waits and returns exactly one `boba_chat.BackendFinishedMsg{}`.

Deliverable: simple chat can run using `SessionBackend` with `Registry=nil`.

### Phase 2: Implement the unified projector + Watermill handler

**Goal:** one event forwarder for both chat types.

1) Implement `projector.Projector` as:
   - a small struct holding:
     - `program *tea.Program`,
     - state maps for “assistant entity created” and “assistant started at” (from `StepChatForwardFunc`, `pinocchio/pkg/ui/backend.go:247`–`pinocchio/pkg/ui/backend.go:293`),
     - optional config flags (e.g. “create assistant entity on first token vs on start event”).
2) Implement `projector.HandleWatermillMessage(msg *message.Message) error`:
   - `msg.Ack()`,
   - `events.NewEventFromJson(msg.Payload)` (same as both current forwarders),
   - type-switch dispatch to handlers.
3) Implement handlers to cover the superset:
   - LLM text events:
     - `EventPartialCompletionStart`, `EventPartialCompletion`, `EventFinal`, `EventInterrupt`, `EventError` (`pinocchio/pkg/ui/backend.go:308`–`pinocchio/pkg/ui/backend.go:388`, and `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:127`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:189`).
   - Thinking events:
     - `EventInfo` with `thinking-started`/`thinking-ended`, and `EventThinkingPartial` (`pinocchio/pkg/ui/backend.go:389`–`pinocchio/pkg/ui/backend.go:420`).
   - Tool events:
     - `EventToolCall`, `EventToolCallExecute`, `EventToolResult`, `EventToolCallExecutionResult` (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:190`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:214`).
   - Agent mode events:
     - `EventAgentModeSwitch` (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:215`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:229`).
   - Log events:
     - `EventLog` (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:117`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:127`).
   - Web search events (when enabled):
     - `EventWebSearchStarted/Searching/OpenPage/Done` and `EventToolSearchResults` (`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:233`–`pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:265`).
4) Remove any emission of `BackendFinishedMsg` from projector handlers (see DD4).

Deliverable: both simple chat and tool-loop chat render correctly using the same handler.

### Phase 3: Extract reusable UI wrappers from cmd into `pkg/tui/widgets`

**Goal:** third-party UI packages can reuse overlay forms without importing `cmd/`.

1) Move overlay form wrapper:
   - from `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go:10`–`pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go:83`
   - to `pinocchio/pkg/tui/widgets/overlay_form.go`
   - update imports to use the new `pinocchio/pkg/tui/toolui` types (next phase).
2) Keep the `simple-chat-agent` host/sidebar UX in `cmd/`:
   - `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/host.go:9`
   - `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/sidebar.go:34`

Deliverable: overlay forms become reusable; specialized agent-mode UI remains command-owned.

### Phase 4: Extract tool-driven UI request types and generative-ui tool registration

**Goal:** tools that request UI forms become reusable primitives, not cmd-specific.

1) Move `ToolUIRequest`, `ToolUIReply` definitions from:
   - `pinocchio/cmd/agents/simple-chat-agent/pkg/tools/tools.go:69`–`pinocchio/cmd/agents/simple-chat-agent/pkg/tools/tools.go:80`
   to:
   - `pinocchio/pkg/tui/toolui/request.go`
2) Move `RegisterGenerativeUITool` from:
   - `pinocchio/cmd/agents/simple-chat-agent/pkg/tools/tools.go:82`–`pinocchio/cmd/agents/simple-chat-agent/pkg/tools/tools.go:127`
   to:
   - `pinocchio/pkg/tui/toolui/generative_ui_tool.go`
3) Ensure the overlay widget expects a channel of `toolui.ToolUIRequest` and replies via `toolui.ToolUIReply`.

Deliverable: any Pinocchio app can add “generative UI” tool support by importing `pinocchio/pkg/tui/toolui`.

### Phase 5: Implement the unified runtime builder and migrate call sites

**Goal:** there is exactly one “official” way to build a TUI program.

1) Implement `tui/runtime.Builder` (replacement for `pinocchio/pkg/ui/runtime.ChatBuilder`):
   - must build:
     - engine (via engineFactory),
     - watermill sink for UI events (`geppetto/pkg/inference/middleware/sink_watermill.go:21`),
     - unified session backend,
     - bobatea chat model (`bobatea/pkg/chat/model.go:144`),
     - optional overlay wrapper (host/sidebar remains app-owned),
     - tea.Program,
     - bound Watermill handler (projector).
2) Migrate Pinocchio CLI chat mode:
   - replace `runtime.NewChatBuilder()` usage in `pinocchio/pkg/cmds/cmd.go:537`–`pinocchio/pkg/cmds/cmd.go:617`
   - with the new builder and new handler registration.
3) Migrate agent command:
   - remove `backendpkg.NewToolLoopBackend` usage (`pinocchio/cmd/agents/simple-chat-agent/main.go:217`–`pinocchio/cmd/agents/simple-chat-agent/main.go:218`)
   - and remove `backend.MakeUIForwarder(p)` wiring (`pinocchio/cmd/agents/simple-chat-agent/main.go:281`–`pinocchio/cmd/agents/simple-chat-agent/main.go:292`)
   - use the shared builder/backend/projector instead.

Deliverable: both commands build and run using the same primitives.

### Phase 6: Delete old code paths (clean break)

Once both call sites are migrated, remove:

- `pinocchio/pkg/ui/backend.go`’s `EngineBackend` and `StepChatForwardFunc` (or move remaining pieces into `pkg/tui/backend` and delete the old package entirely),
- `pinocchio/pkg/ui/runtime/builder.go` (superseded),
- cmd-only tool-loop backend and cmd-only UI wrappers (moved into `pkg/`).

Deliverable: there is only one TUI stack in-tree.

### Phase 7: Optional improvements (post-migration)

- Extend persistence to store tool/log entities, not only message-ish entities:
  - current persistence only handles assistant + thinking text (`pinocchio/pkg/ui/timeline_persist.go:19`–`pinocchio/pkg/ui/timeline_persist.go:176`).
- Add a test harness that feeds sample Geppetto events into the projector and asserts emitted timeline events.

## Open Questions

1) **Topic naming**: should UI events always publish to `"ui"` (like the CLI builder uses today at `pinocchio/pkg/ui/runtime/builder.go:149`–`pinocchio/pkg/ui/runtime/builder.go:156`), or should we standardize on `"chat"` (like the agent cmd uses at `pinocchio/cmd/agents/simple-chat-agent/main.go:131` and `pinocchio/cmd/agents/simple-chat-agent/main.go:286`)?
2) **Sidebar/inspector**: should we ever define a *generic* reusable inspector sidebar in `pkg/tui/widgets`, separate from the specialized `simple-chat-agent` sidebar?
3) **Entity ID semantics**: for logs and agent_mode, do we want:
   - stable deterministic IDs, or
   - intentionally unique per event (current code uses `time.Now().UnixNano()`; see `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:120` and `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:223`)?
4) **Thinking display**: should thinking always be a timeline entity of kind `llm_text` with role `thinking` (current implementations do that: `pinocchio/pkg/ui/backend.go:391` and `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:156`), or should it be a dedicated kind like `llm_thinking` with a dedicated renderer?
5) **Renderer registration**: should `pinocchio/pkg/tui/renderers` provide a “batteries included” default registry hook, or should each app pick which kinds to register (more flexible, but more boilerplate)?

## References

### Primary observed code paths (with anchor lines)

- Simple chat “builder + backend + forwarder”
  - `pinocchio/pkg/ui/runtime/builder.go:29` (ChatBuilder)
  - `pinocchio/pkg/ui/runtime/builder.go:136` (BuildProgram constructs backend/model/program)
  - `pinocchio/pkg/ui/backend.go:25` (EngineBackend)
  - `pinocchio/pkg/ui/backend.go:91` (EngineBackend.Start)
  - `pinocchio/pkg/ui/backend.go:246` (StepChatForwardFunc)
- Pinocchio CLI chat wiring (entrypoint usage)
  - `pinocchio/pkg/cmds/cmd.go:537` (BuildProgram usage)
  - `pinocchio/pkg/cmds/cmd.go:561` (router handler registration)
- Agent/tool-loop backend + forwarder (cmd-only today)
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:25` (ToolLoopBackend)
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:33` (NewToolLoopBackend configures enginebuilder.New + loop/tool config)
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:101` (MakeUIForwarder)
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:190` (ToolCall/Result mapping)
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:215` (AgentMode + web_search mapping)
- Agent command wiring (how the cmd composes UI + backend)
  - `pinocchio/cmd/agents/simple-chat-agent/main.go:217` (NewToolLoopBackend)
  - `pinocchio/cmd/agents/simple-chat-agent/main.go:235` (bobatea chat model with WithTimelineRegister)
  - `pinocchio/cmd/agents/simple-chat-agent/main.go:251` (Overlay + Host composition)
  - `pinocchio/cmd/agents/simple-chat-agent/main.go:281` (tea.NewProgram + forwarder handler wiring)
- Reusable UI primitives to extract
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/overlay.go:10` (OverlayModel)
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/tools/tools.go:69` (ToolUIRequest/Reply + RegisterGenerativeUITool)
- Cmd-specific (do not extract)
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/host.go:9` (HostModel)
- Geppetto primitives this design builds on
  - `geppetto/pkg/inference/session/session.go:44` (AppendNewTurnFromUserPrompt semantics)
  - `geppetto/pkg/inference/toolloop/enginebuilder/builder.go:31` (runner contract and `Registry` meaning)
  - `geppetto/pkg/inference/toolloop/enginebuilder/builder.go:190` (nil registry → single-pass)
  - `geppetto/pkg/inference/middleware/sink_watermill.go:21` (WatermillSink publishing)
  - `geppetto/pkg/events/event-router.go:136` (AddHandlerWithOptions)
- Bobatea primitives this design standardizes on
  - `bobatea/pkg/chat/backend.go:8` (timeline-centric contract)
  - `bobatea/pkg/chat/model.go:127` (WithTimelineRegister)
  - `bobatea/pkg/chat/model.go:134` (WithExternalInput)
  - `bobatea/pkg/timeline/shell.go:11` (Shell embeddability)
  - `bobatea/pkg/timeline/renderers/tool_call_model.go:105` (tool_call factory Kind)
  - `bobatea/pkg/timeline/renderers/tool_call_result_model.go:66` (tool_call_result factory Kind)
  - `bobatea/pkg/timeline/renderers/log_event_model.go:117` (log_event factory Kind)
  - `pinocchio/pkg/middlewares/agentmode/agent_mode_model.go:107` (agent_mode factory Kind)

### Related ticket docs

- `design-doc/01-reusable-pinocchio-tui-analysis-extraction-guide.md` (minimal/stable refactor path; contrasts with this clean-break path)
- `reference/02-third-party-pinocchio-tui-copy-paste-recipes.md` (third-party usage recipes; should be updated after refactor)
