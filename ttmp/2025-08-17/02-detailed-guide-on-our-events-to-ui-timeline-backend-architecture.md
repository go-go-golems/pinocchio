Title: End-to-end guide to the Events → Backend → Timeline UI architecture
Slug: events-to-ui-timeline-architecture
Short: How LLM completions and tool/mode events become timeline entities in the TUI
Topics:
- architecture
- events
- middleware
- timelines
- tool-calling
- renderers
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic

## Overview

This guide explains how textual completions, tool-calling, and agent-mode events flow from the inference engine and middlewares through the event bus to the Bubble Tea timeline UI. A new developer should be able to trace any event from its source (LLM, middleware, or tool execution), through the event router and backend forwarder, all the way to the renderer that draws it on screen.

We cover the involved packages, functions, and data structures, and show where to extend or debug the system.

## Architecture at a glance

- Engine and middlewares (Geppetto):
  - Package: `geppetto/pkg/inference/*`, `geppetto/pkg/events`, `geppetto/pkg/turns`
  - Emit events using `events.PublishEventToContext(ctx, events.NewXxxEvent(...))`
- Event transport:
  - Watermill-based router publishes/forwards events on the `chat` topic.
  - Package: `geppetto/pkg/events` (event types), `pinocchio/cmd/.../main.go` (router wiring)
- Backend forwarder:
  - Package: `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go`
  - Function: `(*ToolLoopBackend).MakeUIForwarder(p *tea.Program)`; transforms provider events into timeline UI entity messages
- Timeline and renderers:
  - Packages: `bobatea/pkg/timeline/*`, `bobatea/pkg/timeline/renderers/*`, `bobatea/pkg/chat`
  - Components:
    - `timeline.Controller` & `timeline.Shell` manage entities and viewport
    - Renderers are registered in a `timeline.Registry` and selected by `kind`

## Core event types (geppetto/pkg/events)

File: `geppetto/pkg/events/chat-events.go`
- Textual streaming:
  - `EventPartialCompletionStart` (type="start")
  - `EventPartialCompletion` (type="partial")
  - `EventFinal` (type="final")
  - `EventInterrupt` (type="interrupt")
  - `EventError` (type="error")
- Tool-calling (provider and local):
  - `EventToolCall` (type="tool-call") carries `ToolCall{ID, Name, Input}`
  - `EventToolCallExecute` (type="tool-call-execute") carries `ToolCall{...}`
  - `EventToolResult` (type="tool-result") carries `ToolResult{ID, Result}`
  - `EventToolCallExecutionResult` (type="tool-call-execution-result`) carries `ToolResult{...}`
- Informational/logging:
  - `EventLog` (type="log"), `EventInfo` (type="info")
- Agent-mode:
  - `EventAgentModeSwitch` (type="agent-mode-switch"), exported; `Message` + `Data{"from","to","analysis"}`

Every event embeds `EventMetadata` with `RunID`, `TurnID`, and `ID` (`message_id`). Middlewares and tools should set `RunID` and `TurnID`. For UI correlation, having a non-zero `ID` is strongly recommended.

## Emitting events in middlewares

File: `geppetto/pkg/inference/middleware/agentmode/middleware.go`
- The agent-mode middleware inspects assistant text and emits:
  - Analysis-only: `NewAgentModeSwitchEvent(meta, from=current, to=current, analysis)` when no switch
  - Mode switch: `NewAgentModeSwitchEvent(meta, from=current, to=newMode, analysis)` when a switch is detected
- Important: pass a non-zero `EventMetadata.ID` (e.g., `uuid.New()`) and set `RunID`, `TurnID`.

Example (conceptual):
```go
meta := events.EventMetadata{ID: uuid.New(), RunID: turn.RunID, TurnID: turn.ID}
events.PublishEventToContext(ctx, events.NewAgentModeSwitchEvent(meta, from, to, analysis))
```

## Tool-calling loop and events

Files: `geppetto/pkg/inference/toolhelpers/helpers.go`, `geppetto/pkg/inference/tools/executor.go`
- The loop emits provider tool events (advertised tools) and local execution events when tools are executed in-process. These events reference a `ToolCall.ID` that is used by the UI to correlate updates.
- The loop also emits completion text events before, during, and after tool use.

## Event router and sinks

File: `pinocchio/cmd/agents/simple-chat-agent/main.go`
- A `events.EventRouter` is created and a `WatermillSink` is attached for publishing on topic `chat`.
- The agent registers two types of handlers:
  - `ui-forwarder`: forwards provider events to the UI program (see next section)
  - `event-sql-logger`: persists events into a SQLite store for audit/debug

## Backend forwarder: provider events → timeline messages

File: `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go`
- Function: `MakeUIForwarder(p *tea.Program) func(*message.Message) error`
- Parses incoming JSON into typed `events.Event` (via `events.NewEventFromJson`).
- For each provider event, emits timeline entity lifecycle messages to the running Bubble Tea program:
  - Text streaming:
    - `EventPartialCompletionStart` → `UIEntityCreated{kind:"llm_text"}` with `streaming=true`
    - `EventPartialCompletion` → `UIEntityUpdated{kind:"llm_text"}` with accumulated text
    - `EventFinal`, `EventInterrupt`, `EventError` → `UIEntityCompleted{kind:"llm_text"}` and mark `streaming=false`
  - Tool-calling:
    - `EventToolCall` → `UIEntityCreated{kind:"tool_call"}` with props: `name`, `input`
    - `EventToolCallExecute` → `UIEntityUpdated{kind:"tool_call"}` (e.g., `exec:true`)
    - `EventToolResult` / `EventToolCallExecutionResult` → create `tool_call_result` entity and complete it
  - Agent-mode:
    - `EventAgentModeSwitch` → `UIEntityCreated{kind:"agent_mode"}` with `title`, `from`, `to`, `analysis` then complete
  - Logs:
    - `EventLog` → `UIEntityCreated{kind:"plain"}` with title `[LEVEL] message` (+ fields), then complete

The forwarder adds detailed `log.Debug()` records (event type, run_id, turn_id, message_id/tool_id) so you can trace issues like zero UUIDs or misrouted kinds.

## Timeline entity lifecycle and registry

Packages: `bobatea/pkg/timeline/*`
- `timeline.Registry` contains model factories keyed by a unique key and grouped by `kind`.
- `timeline.Controller` and `timeline.Shell` handle entity creation/update/completion and viewport state.
- UI messages used:
  - `UIEntityCreated{ID: EntityID{LocalID, Kind}, Renderer: RendererDescriptor{Kind}, Props, StartedAt}`
  - `UIEntityUpdated{ID, Patch, Version, UpdatedAt}`
  - `UIEntityCompleted{ID, Result}`
- The `kind` must match a registered model factory; the forwarder is responsible for setting the correct kind.

## Renderers

Files in `bobatea/pkg/timeline/renderers`:
- `llm_text_model.go`: renders LLM text with optional metadata/status; registered via `NewLLMTextFactory()`
- `tool_call_model.go`: renders tool input as YAML (glamour highlighting); factory via `NewToolCallFactory()`
- `tool_call_result_model.go`: renders result in light pink; factory `ToolCallResultFactory{}`
- `agent_mode_model.go`: renders analysis and switch info; factory `AgentModeFactory{}`
- `plain_model.go`: generic debugging renderer; factory `PlainFactory{}`

Renderers are registered either:
- In the chat model initialization (`bobatea/pkg/chat/model.go`) for core kinds (`llm_text`, `plain`, `tool_calls_panel`), or
- By the agent/backend at startup via a timeline hook (see `WithTimelineRegister` option used in `main.go`).

## UI composition

File: `bobatea/pkg/chat/model.go`
- `InitialModel(backend, options...)` builds the reusable chat UI model, attaching a `timeline.Shell` and a text input. It processes timeline lifecycle messages and input submission.
- The backend must implement `boba_chat.Backend` (`Start`, `Interrupt`, `Kill`, `IsFinished`), and is called when the user submits input.

## Putting it together: request flow

1. User submits a prompt in the TUI.
2. Chat model calls `backend.Start(ctx, prompt)`.
3. Backend’s tool loop (`toolhelpers.RunToolCallingLoop`) runs using an `events.WithEventSinks(ctx, sink)` context.
4. Engine and middlewares publish events (text/tool/mode/log/info) via `events.PublishEventToContext`.
5. Router receives events and the forwarder translates them into timeline UI messages.
6. Timeline registry selects renderers by kind and instantiates renderer models.
7. Shell updates the viewport; chat model renders.

## Debugging tips

- To confirm kinds are correctly mapped:
  - Check forwarder logs (kind, props) and renderer factory logs (`NewEntityModel` with `kind=...`).
- Zero UUIDs for `message_id`:
  - Ensure middlewares/tools set `EventMetadata.ID` (e.g., `uuid.New()`).
- Duplicate agent-mode entries:
  - Emit either analysis-only or switch, not both in the same turn; the middleware can select with `if newMode != current { switch } else if analysis != "" { analysis-only }`.
- Renderer missing:
  - Ensure the renderer factory is registered before the forwarder begins dispatching events; register via `WithTimelineRegister` in `main.go`.

## Key files and APIs

- Event types and utilities: `geppetto/pkg/events/chat-events.go` (`NewEventFromJson`, `NewXxxEvent`, `EventMetadata`)
- Agent-mode middleware: `geppetto/pkg/inference/middleware/agentmode/middleware.go` (`DetectYamlModeSwitch`, `NewMiddleware`)
- Tool loop: `geppetto/pkg/inference/toolhelpers/helpers.go` (`RunToolCallingLoop`)
- Backend forwarder: `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go` (`MakeUIForwarder`)
- Router setup: `pinocchio/cmd/agents/simple-chat-agent/main.go`
- Renderers: `bobatea/pkg/timeline/renderers/*.go`
- Chat UI model: `bobatea/pkg/chat/model.go`

## Extending the system

- Add a new event type in `geppetto/pkg/events` and publish from the appropriate middleware or tool.
- Map the event to a new `kind` in the forwarder and provide a renderer implementing `timeline.EntityModel`.
- Register the renderer in the `timeline.Registry` (preferably within the agent/backend via a timeline hook).

## Conclusion

The system cleanly separates concerns: engines and middlewares publish provider-agnostic events; the backend forwarder translates them into UI lifecycle messages; and the timeline registry renders them via dedicated models. With the patterns above, you can add new event categories, customize renderers, and debug the full pipeline end to end.


