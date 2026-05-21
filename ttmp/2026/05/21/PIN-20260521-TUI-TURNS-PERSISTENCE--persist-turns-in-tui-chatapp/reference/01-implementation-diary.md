---
Title: Implementation Diary
Ticket: PIN-20260521-TUI-TURNS-PERSISTENCE
Status: active
Topics:
    - pinocchio
    - chatapp
    - tui
    - persistence
    - sessionstream
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/cmds/cmd.go
      Note: Primary future implementation target for runChat persistence wiring
    - Path: pkg/ui/chatapp_backend.go
      Note: Primary future implementation target for final-turn persistence
    - Path: ttmp/2026/05/21/PIN-20260521-TUI-TURNS-PERSISTENCE--persist-turns-in-tui-chatapp/design-doc/01-persisting-turns-in-the-tui-chatapp.md
      Note: Design produced in Step 1
ExternalSources: []
Summary: Diary for the TUI chatapp turns persistence design ticket.
LastUpdated: 2026-05-21T15:00:00-04:00
WhatFor: Track analysis, design decisions, validation, and future implementation notes for persisting command TUI chat turns.
WhenToUse: Read before resuming implementation of PIN-20260521-TUI-TURNS-PERSISTENCE.
---


# Diary

## Goal

This diary captures the design work for persisting final `turns.Turn` values in Pinocchio command TUI chatapp mode. It records why the ticket exists, what system seams were inspected, what design was produced, and what the next implementer should validate.

## Step 1: Create the ticket and write the persistence design

The goal of this step was to turn the user's question about using turns and timeline databases in command chat mode into a durable implementation ticket. The existing code already exposes CLI persistence flags and already has web-chat persistence patterns, but command TUI chat currently keeps the final turn accumulator in memory only.

I created a new docmgr ticket and wrote a long-form design guide for a new intern. The guide explains the system model, the current implementation, the distinction between turns DB and timeline DB, and a phased implementation plan.

### Prompt Context

**User prompt (verbatim):** "Create a new ticket for persisting turns in the TUI chatapp, Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create a docmgr ticket for TUI chat turn persistence, write a detailed technical design/implementation guide suitable for an intern, store it under the ticket, and upload the resulting bundle to reMarkable.

**Inferred user intent:** The user wants future implementation work to start from a clear shared design that explains current Pinocchio chatapp/sessionstream/web-chat/TUI persistence boundaries and avoids repeating prior confusion between model-context turns and visible timelines.

**Commit (code):** N/A — this step created design documentation only.

### What I did

- Created ticket `PIN-20260521-TUI-TURNS-PERSISTENCE` with title `Persist turns in TUI chatapp`.
- Added design doc `design-doc/01-persisting-turns-in-the-tui-chatapp.md`.
- Added this implementation diary `reference/01-implementation-diary.md`.
- Inspected existing implementation files:
  - `pkg/cmds/cmd.go`
  - `pkg/cmds/chat_persistence.go`
  - `pkg/chatapp/runner.go`
  - `pkg/chatapp/service.go`
  - `pkg/ui/chatapp_backend.go`
  - `pkg/cmds/cmdlayers/helpers.go`
  - `cmd/web-chat/runtime_composer.go`
  - `cmd/web-chat/turn_persistence.go`
- Wrote the design around two separate persistence tracks:
  - final `turns.Turn` values in `chatstore.TurnStore`;
  - visible sessionstream timeline snapshots in `sessionstream.HydrationStore`.

### Why

- Command TUI chat currently stores `ChatAppBackend.currentTurn` only in memory.
- web-chat already persists final turns and timeline state, so it provides a good reference architecture.
- The CLI already exposes `--turns-db` and `--timeline-db`, but command chat needs explicit wiring to make those flags meaningful.
- The design needs to be clear enough for a new intern to implement safely without reconstructing model context from UI timeline entities.

### What worked

- `chatapp.RunnerOptions` already has the main seams needed for this design:
  - `HydrationStore sessionstream.HydrationStore`
  - `TurnStore chatstore.TurnStore`
  - `UIFanout sessionstream.UIFanout`
  - `Plugins []ChatPlugin`
- `PromptRequest.OnFinalTurn` is already available and is the right source for final-turn persistence.
- `pkg/cmds/chat_persistence.go` already contains a reusable `cliTurnStorePersister` that serializes turns through Geppetto serde and saves them to `chatstore.TurnStore`.
- The existing TUI backend already has the right accumulator shape: clone current turn, append user prompt, run inference with `InitialTurn`, replace current turn with final turn.

### What didn't work

- No code implementation was attempted in this step.
- The existing `openChatPersistenceStores` helper opens a `chatstore.TimelineStore`, but command chat parity with web-chat needs a live `sessionstream.HydrationStore` for `chatapp.NewRunner`. The design therefore recommends a new helper for sessionstream SQLite hydration instead of blindly reusing the older timeline store helper.

### What I learned

- Command chat and web-chat should not persist/resume turns in exactly the same way because they use different request paths.
- web-chat submits no `InitialTurn`, so `chatapp` can load the latest final turn from `TurnStore` and append the new prompt.
- command TUI submits explicit `InitialTurn`, so the backend must persist the final turn observed from `OnFinalTurn`; passing `TurnStore` to the runner alone is not enough to persist TUI turns.
- Timeline DB and turns DB solve different problems and should be explained separately in user-facing help.

### What was tricky to build

The tricky design issue was deciding where persistence belongs when the TUI uses `InitialTurn`. If `InitialTurn` is present, `chatapp` intentionally skips loading history from `TurnStore` because the caller has supplied complete model context. That means a naive implementation that only passes `TurnStore` into `chatapp.NewRunner` would not persist command TUI final turns by itself.

The design resolves this by adding a small `ui.TurnPersister` interface to `ChatAppBackend`. The backend already receives `OnFinalTurn`, so it can persist the final Geppetto turn immediately after a successful run and then update `currentTurn`. This keeps `pkg/ui` independent of `chatstore` and keeps model context based on final turns rather than timeline reconstruction.

### What warrants a second pair of eyes

- Whether persistence failures should fail the TUI message or be best-effort warnings.
- Whether `convID=sessionID` is sufficient for the first persistence implementation, or whether `--conversation-id` / `--session-id` should be added immediately.
- Whether timeline DB wiring should be in the same PR as turns DB wiring or split into a follow-up.
- Whether existing web-chat export/list tooling can be reused for command chat persisted turns without schema/semantic mismatch.

### What should be done in the future

- Implement Phase 1: persist command TUI final turns to `--turns-db` / `--turns-dsn`.
- Implement Phase 2: wire `--timeline-db` / `--timeline-dsn` to a `sessionstream` SQLite hydration store.
- Design and implement resume UX with stable session/conversation ids.
- Add user-facing help that clearly distinguishes turns DB from timeline DB.

### Code review instructions

- Start with the design doc:
  - `ttmp/2026/05/21/PIN-20260521-TUI-TURNS-PERSISTENCE--persist-turns-in-tui-chatapp/design-doc/01-persisting-turns-in-the-tui-chatapp.md`
- Then inspect the current accumulator code:
  - `pkg/ui/chatapp_backend.go`, especially `Start` and `turnWithUserPrompt`.
- Inspect command wiring:
  - `pkg/cmds/cmd.go`, especially `runChat`.
- Inspect persistence helpers:
  - `pkg/cmds/chat_persistence.go`.
- Validate future implementation with:
  - `go test ./pkg/ui ./pkg/cmds ./pkg/chatapp -count=1`
  - a tmux smoke test using `--chat --turns-db /tmp/pin-turns.db`.

### Technical details

The core design invariant is:

```text
final turns.Turn -> next model context
sessionstream timeline -> visible UI/debug/RPC state
```

The intended TUI persistence loop is:

```text
currentTurn.Clone() + user prompt
  -> PromptRequest.InitialTurn
  -> Geppetto inference
  -> PromptRequest.OnFinalTurn(finalTurn)
  -> persist finalTurn to turns DB
  -> currentTurn = finalTurn.Clone()
```

The intended timeline persistence loop is:

```text
chatapp projections
  -> sessionstream Hub
  -> sessionstream SQLite HydrationStore if --timeline-db configured
  -> TUI fanout / debug JSONL live views
```
