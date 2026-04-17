---
Title: Pinocchio TUI Integration Playbook (Debugging + Ops)
Slug: tui-integration-playbook
Short: Operational checklist and debugging playbook for Pinocchio terminal TUI integrations using Bobatea timeline entities and Watermill event routing.
Topics:
- pinocchio
- tui
- bobatea
- bubbletea
- debugging
- playbook
Commands:
- simple-chat-agent
- pinocchio
Flags: []
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

This playbook is for when you already understand the architecture and you need to **operate/debug** a terminal TUI integration quickly (or teach a new intern how to do so safely). It focuses on checklists, failure modes, and concrete commands.

If you want the “intern-first” explanation of the whole system, start with:

- `pinocchio help tui-integration-guide`

## Quick Checklist: “Does my integration look correct?”

This section is designed so you can scan it during review or during a live debugging session.

### Wiring checklist (must all be true)

- You create exactly one Bubble Tea `*tea.Program`.
- You create a Bobatea chat model via `boba_chat.InitialModel(backend, ...)`.
- Your backend implements `boba_chat.Backend` (`Start/Interrupt/Kill/IsFinished`).
  - Contract reference: `bobatea/pkg/chat/backend.go`
- Your engine publishes events to Watermill using `middleware.NewWatermillSink(router.Publisher, topic)`.
  - Sink reference: `geppetto/pkg/inference/middleware/sink_watermill.go`
- You register a Watermill handler that:
  - `Ack()`s the message,
  - decodes with `events.NewEventFromJson`,
  - calls `program.Send(timeline.UIEntity*)`.
  - Agent forwarder reference: `pinocchio/pkg/ui/forwarders/agent/forwarder.go`
- The sink topic and the handler topic match exactly.

### Behavior checklist (what you should see)

- On first prompt:
  - The UI should create an assistant entity (`llm_text`) and then update/complete it.
- If tool loop is enabled:
  - Tool call entities should appear (`tool_call`, `tool_call_result`) and you should still get assistant text entities.
- When the backend finishes:
  - The UI should receive `boba_chat.BackendFinishedMsg{}` to re-enable input.

## How to instrument and debug (intern-safe)

This section describes the safe debugging steps that don’t require deep code surgery.

### 1) Confirm you’re on the correct topic

Most “nothing shows up” failures are topic mismatches.

What to check:

- Your sink topic, e.g.:

```go
// Pseudocode imports:
//
//   "github.com/go-go-golems/geppetto/pkg/inference/middleware"
//
sink := middleware.NewWatermillSink(router.Publisher, "chat")
```

- Your handler registration topic, e.g.:

```go
// Pseudocode imports:
//
//   agentforwarder "github.com/go-go-golems/pinocchio/pkg/ui/forwarders/agent"
//   "github.com/go-go-golems/geppetto/pkg/events"
//   tea "github.com/charmbracelet/bubbletea"
//
router.AddHandler("ui-forward", "chat", agentforwarder.MakeUIForwarder(p))
```

If these differ (`"ui"` vs `"chat"`), your UI will never update.

### 2) Confirm messages are being ack’d

If Watermill messages are not ack’d, event flow can stall (especially with transports that block until ack).

What to look for:

- In your forwarder, early in the handler:

```go
// Pseudocode imports:
//
//   "github.com/ThreeDotsLabs/watermill/message"
//
msg.Ack()
```

Anchor:

- `pinocchio/pkg/ui/forwarders/agent/forwarder.go`

### 3) Confirm renderers are registered for the entity kinds you emit

If your forwarder emits `Kind: "tool_call"` but you never registered a model factory for `"tool_call"`, the UI may render it poorly (or not at all, depending on fallback).

Reference patterns:

- Renderer registration in `simple-chat-agent`: `pinocchio/cmd/agents/simple-chat-agent/main.go`
- Registry hook: `bobatea/pkg/chat/model.go` (`WithTimelineRegister`)

### 4) Confirm backend completion behavior matches your forwarder

The “simple chat” forwarder (`StepChatForwardFunc`) sends `BackendFinishedMsg` on provider final/error/interrupt.

The “agent/tool-loop” forwarder must **not** do that because the tool loop may continue past a provider final.

Rules of thumb:

- If you use `pinocchio/pkg/ui/forwarders/agent`, your backend must emit `BackendFinishedMsg` only when the whole loop completes.
- If you use `pinocchio/pkg/ui.StepChatForwardFunc`, your backend is typically “single pass” and provider final ends the UI step.

## tmux playbook: smoke checks and capture-pane

Use tmux for any check that starts a long-running UI, because it makes “start → observe → kill” reproducible.

### Help-mode sanity check (no DB / credentials side effects)

This checks that the command still starts and the Cobra/Glazed wiring works, without creating runtime artifacts.

```bash
tmux new-session -d -s tui-smoke -c pinocchio \
  "sh -lc 'go run ./cmd/agents/simple-chat-agent --help; echo DONE; sleep 10'"

tmux capture-pane -t tui-smoke:0.0 -p | tail -n 80
tmux kill-session -t tui-smoke
```

### Full TUI smoke check (interactive)

If you have credentials/profiles configured and you accept local artifact creation, run:

```bash
tmux new-session -d -s tui-smoke -c pinocchio \
  "sh -lc 'go run ./cmd/agents/simple-chat-agent simple-chat-agent'"

tmux attach -t tui-smoke
```

Then:

- Type a short prompt (“hello”) and watch entities appear.
- Exit with the app’s quit keybinding (often `q` or `Ctrl+C` depending on model).

## Pseudocode: “golden wiring” (copy this shape, not literal code)

```go
// Pseudocode imports:
//
//   "context"
//   tea "github.com/charmbracelet/bubbletea"
//   boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
//   "github.com/go-go-golems/geppetto/pkg/events"
//   "github.com/go-go-golems/geppetto/pkg/inference/middleware"
//   toolloopbackend "github.com/go-go-golems/pinocchio/pkg/ui/backends/toolloop"
//   agentforwarder "github.com/go-go-golems/pinocchio/pkg/ui/forwarders/agent"
//   "golang.org/x/sync/errgroup"
//
router := events.NewEventRouter(...)
sink := middleware.NewWatermillSink(router.Publisher, topic)

backend := toolloopbackend.NewToolLoopBackend(engine, middlewares, registry, sink, hook)
model := boba_chat.InitialModel(backend, WithTimelineRegister(...))
program := tea.NewProgram(model, tea.WithAltScreen())

router.AddHandler("ui-forward", topic, agentforwarder.MakeUIForwarder(program))

run router + program concurrently (errgroup)
cancel router when UI exits
_ = context.Canceled // (placeholder so context import stays visible in pseudocode)
```

## Troubleshooting table

| Symptom | Cause | Fix |
|---|---|---|
| UI is responsive but never shows assistant text | Forwarder never sees events | Topic mismatch; router not started; sink not attached to engine. |
| Assistant entity appears but never completes | Final event not handled or never emitted | Check provider events; inspect forwarder `EventFinal` handling. |
| Tool calls don’t show up | No tools or no renderer | Register tools; register `tool_call` / `tool_call_result` renderers. |
| UI “freezes” during inference | Watermill handler blocks, or messages not ack’d | Ensure `msg.Ack()` is called; avoid long blocking work in handler. |
| Streaming stalls/hangs under load | In-memory pub/sub backpressure (publish blocks until handler ACK) | Configure `gochannel` buffering and disable publish→ACK blocking, or switch to Redis Streams transport. |
| Timeline/turn persistence flakes | Context cancellation races | Avoid canceling inference during UI completion cleanup; for DB writes in handlers, use detached bounded contexts (not `msg.Context()`). |
| You see duplicate entities for logs/modes | ID collisions | Ensure unique local IDs for non-message events (agent forwarder uses a timestamp suffix). |

## See Also

- `pinocchio help chatbuilder-guide` (simple chat wiring via `ChatBuilder`)
- `pinocchio help tui-integration-guide` (full intern-first integration tutorial)
- `pinocchio help webchat-debugging-and-ops` (observability and debugging patterns)
