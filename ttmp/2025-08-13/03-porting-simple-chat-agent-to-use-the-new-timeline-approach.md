Title: Plan — Port simple-chat-agent to Engine/Turn and Bobatea Timeline UI
Slug: porting-simple-chat-agent-timeline-ui
Short: A step-by-step plan to migrate the simple chat agent to Engine/Turn with timeline UI and streaming
Topics:
- simple-chat-agent
- migration
- engine
- turns
- bobatea
- timeline
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic

## Goal

Adopt the Engine/Turn architecture and Bobatea’s timeline UI for `cmd/agents/simple-chat-agent/main.go`, aligning it with the recent Pinocchio migration (see also `ttmp/2025-08-13/02-report-about-porting-pinocchio-to-use-turns-and-the-new-bobatea.md`).

## Current snapshot (build status)

We installed missing deps and verified the agent builds:
- Added: `github.com/go-go-golems/go-sqlite-regexp`, `github.com/go-go-golems/uhoh`.
- `go build ./cmd/agents/simple-chat-agent` succeeded on latest code.

Note: The current agent uses a REPL + evaluator with a conversation manager, tools, and SQLite-backed snapshots.

## Target architecture

- Use `engine.Engine` + `turns.Turn` as the unit of inference.
- Stream engine events via Watermill sinks for UI (timeline) or log/structured handlers.
- Seed and maintain a Turn history for context; reduce history when running new prompts.
- Option A (REPL-first): Keep REPL UI, invoke engine per user input with Turns and keep the snapshot hooks.
- Option B (Timeline UI): Replace REPL with Bobatea chat timeline; emit entities, auto-submit, and integrate tools.

For this plan we detail both; implement Option A first (minimal), then add Option B as an alternative mode.

## Detailed steps

### 1) Turn-first evaluator

- Build seed `*turns.Turn` from system prompt (middleware already sets system), prior history, and the user input.
- Call `engine.RunInference(ctx, seed)`.
- Persist snapshots at phases using the existing `snapshotStore.SaveTurnSnapshot` hook (pre_middleware, pre_inference, post_inference, post_middleware, post_tools). The current code already wraps the engine with middleware to do this.
- Convert the updated Turn to conversation only when you need to print or log the assistant output (tail only).

Implementation notes:
- Maintain `sessionRunID` and ensure each Turn has `RunID` and `ID` (already done in code via middleware wrapper).
- Carry tool blocks across turns by reducing history before each new run.

### 2) Event streaming hooks (no timeline UI yet)

- Keep the existing event router. Register a tool event logger and info/log event handlers (already present).
- Optionally add a structured printer for the `chat` topic if needed (parity with Pinocchio CLI path).

### 3) Optional: integrate timeline UI (Alternate run mode)

- Add a Bobatea chat model and create an `ui.EngineBackend` from the engine.
- Attach the Bubble Tea program and register `ui.StepChatForwardFunc(p)` to the `ui` topic.
- Seed the backend with a reduced Turn so user/assistant history appears (system omitted in seeding to avoid duplication).
- Auto-submit on start if needed by sending `ReplaceInputTextMsg` + `SubmitMessageMsg`.

### 4) Prompt templating

- If the agent uses templated prompts for startup or commands, render templates using `glazed/pkg/helpers/templating` into Turn blocks before inference (same pattern as in `pkg/cmds/cmd.go`).

### 5) YAML/config alignment (if applicable)

- If the agent reads prompts/messages from YAML, align to Pinocchio’s simplified schema (`messages: []string`) and convert to user blocks.

### 6) Validate tools and snapshots

- Ensure `tool_call` / `tool_use` blocks appear in the updated Turn and are persisted in snapshots.
- Verify that the SQLite REGEXP tool remains functional and that DSN/tool config are available via middleware or `Turn.Data`.

## Checklist

- [ ] Ensure Turn-first evaluator path exists: reduce history → append user block → `RunInference` → snapshot phases → convert tail to print.
- [ ] Keep router handlers for tool/log/info events; add structured printer if necessary.
- [ ] Optional timeline UI mode:
  - [ ] Create `EngineBackend`, attach to Bubble Tea, register `StepChatForwardFunc`.
  - [ ] Seed backend with reduced Turn; emit user/assistant entities (dedup by block ID).
  - [ ] Add auto-submit for initial prompt (if desired).
- [ ] Add templating for any agent prompts; wire variables.
- [ ] Confirm tool flows and snapshot persistence across turns.

## Risks and mitigations

- Duplicate entities on seeding: mitigate by dedup on block ID (as in `EngineBackend`).
- Losing context across turns: always reduce history into a seed before appending the next user block.
- Tool visibility in UI: if adopting timeline UI, mirror tool events to dedicated timeline entities later.

## References

- Full Pinocchio migration report: `ttmp/2025-08-13/02-report-about-porting-pinocchio-to-use-turns-and-the-new-bobatea.md`
- Diff: `ttmp/2025-08-13/pkg-diff.txt`
- Timeline chat migration: `bobatea/ttmp/2025-08-12/03-porting-pinocchio-to-bobatea-timeline-chat.md`



