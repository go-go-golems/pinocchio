---
Title: Report: Migrating Pinocchio to Turns and the New Bobatea Timeline Chat
Slug: porting-pinocchio-to-turns-and-timeline-chat
Short: Detailed report of code changes, design choices, and a playbook to migrate remaining entry points (e.g., main.go)
Topics:
- pinocchio
- migration
- turns
- chat
- timeline
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Purpose and scope

This report documents the migration of Pinocchio from the legacy conversation/message architecture to Geppetto’s Turn-based engine model and the Bobatea timeline-first chat UI. It summarizes code changes (grounded in the current `git diff`), explains why they were made, and offers a practical playbook to apply the same approach to other entry points (notably `main.go`).

The goals are:
- Use `turns.Turn` as the provider-agnostic unit of inference
- Stream engine events to the timeline UI (`UIEntityCreated/Updated/Completed`)
- Remove runtime dependencies on `conversation.Manager`
- Render templates (system/messages/prompt) prior to inference
- Seed chat UI with prior assistant/user blocks (no duplicate system)

## What changed (by component)

### 1) Runtime run context (pinocchio/pkg/cmds/run/context.go)

- Removed `ConversationManager` from runtime; introduced `Variables map[string]interface{}` to render templates and `ResultConversation` for output-only conversions.
- Consolidated options:
  - `WithRouter(router *events.EventRouter)` sets the Watermill router
  - `WithVariables(vars map[string]interface{})` passes template variables
  - `NewRunContext()` no longer requires a manager

Why: the UI and engine now operate on Turns; conversations are reconstructed only at output boundaries (e.g., PrintPrompt or blocking output).

### 2) UI backend (pinocchio/pkg/ui/backend.go)

- Replaced `StepBackend` with `EngineBackend`:
  - `Start(ctx, prompt string) (tea.Cmd, error)` runs inference by reducing stored history into a seed Turn, appending a user text block, and calling `Engine.RunInference`.
  - Maintains a `history []*turns.Turn` and emits a `BackendFinishedMsg` when done.
  - `AttachProgram(*tea.Program)` allows the backend to emit initial timeline entities for prior blocks when seeding.
  - `SetSeedTurn` and `SetSeedFromConversation` seed the history; during seeding, the backend emits timeline entities:
    - Emits only user/assistant `llm_text` entities (system omitted to avoid duplication)
    - Deduplicates by block ID to prevent double emission
  - `StepChatForwardFunc` rewritten to translate engine events into timeline entities:
    - Partial start → `UIEntityCreated`
    - Partial deltas → `UIEntityUpdated` with full completion text
    - Final/Interrupt/Error → `UIEntityCompleted` + `BackendFinishedMsg`

Why: engines produce streaming events and final Turns; the UI expects timeline entities. Seeding ensures chat UI reflects prior context immediately upon entering chat.

### 3) Command pipeline and templating (pinocchio/pkg/cmds/cmd.go, loader.go)

- YAML schema: `messages:` now `[]string` instead of conversation messages. Loader converts them into `turns.NewUserTextBlock(text)` and passes them via `WithBlocks(blocks)`.
- `PinocchioCommand` now holds `Blocks []turns.Block` (no `conversation.Message`).
- Added templating helpers and `g.buildInitialTurn(vars)` to render:
  - `SystemPrompt`, each block payload’s `text`, and `Prompt`
  - Build a seed `Turn` from the rendered values
- Blocking path runs engine with the rendered seed Turn; result Turn is converted to `[]*conversation.Message` only for output (e.g., PrintPrompt), using `ResultConversation` in `RunContext`.
- Chat path constructs the Bobatea model, attaches the program to `EngineBackend`, registers the event forwarder, and seeds the backend Turn after `router.Running()`; if `autoStart`, the rendered prompt is pre-filled and submitted.

Why: Turn-first operation and decoupling from the conversation manager simplifies runtime logic, avoids schema round-trips, and aligns with provider-agnostic Turns for tool flows.

### 4) Documentation updates (pinocchio/pkg/doc/topics/01-chat-runner-events.md)

- Refocused terminology from Steps to Engine/Turn, describing event flow, engine creation with sinks, and handler registration for the timeline UI.

Why: bring docs in line with the migrated runtime.

## A quick look at the diff

Key highlights from `git diff origin/main -- pkg` (summarized):
- run/context.go: Removed manager option, added router/variables, new constructor without manager.
- ui/backend.go: Complete rewrite from `StepBackend` to `EngineBackend` with history, seeding, event forwarding, and timeline entity emission.
- docs: Updated to Engine/Turn concepts and examples.
- Deleted legacy `pkg/codegen/codegen.go` tied to old conversation/step path.

## New runtime model

- Build a seed Turn from rendered inputs (system + pre-seeded user blocks + optional prompt)
- Run engine: `RunInference(ctx, seed)` → `updatedTurn`
- Stream events → router → `StepChatForwardFunc` → timeline UI
- Convert final Turn to conversation only at output boundaries (if needed)

Seed building (simplified):

```go
// on PinocchioCommand
func (g *PinocchioCommand) buildInitialTurn(vars map[string]interface{}) (*turns.Turn, error) {
    // 1) render system
    // 2) render each block text
    // 3) render prompt
    // 4) assemble Turn with system + blocks + prompt
}
```

Timeline mapping (high level):
- Partial start → create assistant `llm_text` entity with empty text
- Partial delta → patch `text` with accumulated completion
- Final/Interrupt → complete entity and emit `BackendFinishedMsg`
- Error → complete with error text and emit `BackendFinishedMsg`

## Migration playbook (apply to main.go and other entry points)

1) Replace conversation-centric runtime with Turn-first
- Gather inputs: system prompt, any pre-seeded messages (as user blocks), and an optional prompt
- Render them to build a Turn (`g.buildInitialTurn(vars)`)

2) Create engine and event router
- Create `events.EventRouter()`
- Pass `engine.WithSink(middleware.NewWatermillSink(router.Publisher, "ui"))` when creating the engine via factory

3) Wire the UI
- Use `bobatea.InitialModel(backend, ...)` with `EngineBackend`
- Attach the program via `backend.AttachProgram(p)`
- `router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))`
- Run router handlers and program

4) Seeding and auto-start
- After `<-router.Running()`, seed the backend with the rendered Turn (`SetSeedTurn`)
- If `auto-start`, render the prompt and submit it via `ReplaceInputTextMsg` + `SubmitMessageMsg`

5) Output
- For blocking/print-only, convert the final Turn to conversation for printing (use `ResultConversation` from the run context)

6) Conversation manager removal
- Remove `conversation.Manager` from runtime paths
- Keep conversion helpers at output boundaries only

7) Messages schema change in YAML
- Switch `messages: []string` and convert them to user blocks in the loader

## Practical guidance for main.go

1) Build `PinocchioCommand` (or equivalent) with `SystemPrompt`, `messages` (strings), `Prompt`.
2) On run:
- Construct `RunContext` with `WithVariables(...)`, `WithRouter(router)`, `WithStepSettings(stepSettings)`, `WithRunMode(...)`
- In chat mode, build engine with UI sink, create `EngineBackend`, attach program; add UI forwarder; seed backend Turn after `router.Running()`; optionally auto-submit rendered prompt
- In blocking mode, run the engine with `g.buildInitialTurn(vars)` and write the final conversation messages to output (converted from the final Turn for display only)

## Notes and pitfalls

- Do not emit system blocks as timeline entities during seeding; it clutters the chat history and duplicates other previews.
- Deduplicate seeded entities by block ID to prevent double emission when re-entering chat.
- Always wait for `<-router.Running()>` before seeding or auto-submitting to avoid lost messages.
- Use Turn history (`reduceHistory`) to maintain conversational context across runs.

## Next steps

- Apply this playbook to `cmd/pinocchio/main.go` to migrate the entry point.
- Update any agents or examples to supply message strings (converted to blocks) instead of conversation messages.
- Expand tests around seeding, auto-start, and template rendering.

## Appendix: Reference pointers

- Turn builders and rendering: `pinocchio/pkg/cmds/cmd.go`
- Runtime context and options: `pinocchio/pkg/cmds/run/context.go`
- UI backend and event forwarding: `pinocchio/pkg/ui/backend.go`
- Loader message parsing to blocks: `pinocchio/pkg/cmds/loader.go`



