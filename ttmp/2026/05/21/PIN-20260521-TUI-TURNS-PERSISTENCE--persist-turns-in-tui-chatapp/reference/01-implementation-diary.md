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
    - Path: cmd/pinocchio/doc/general/06-rpc-jsonl-output.md
      Note: User-facing TUI persistence help in commit 94c7b29
    - Path: pkg/cmds/chat_persistence.go
      Note: CLI turns and sessionstream hydration store helpers in commit 94c7b29
    - Path: pkg/cmds/cmd.go
      Note: |-
        Primary future implementation target for runChat persistence wiring
        runChat opens turn and sessionstream timeline stores in commit 94c7b29
    - Path: pkg/ui/chatapp_backend.go
      Note: |-
        Primary future implementation target for final-turn persistence
        TUI final-turn persistence seam implemented in commit 94c7b29
    - Path: ttmp/2026/05/21/PIN-20260521-TUI-TURNS-PERSISTENCE--persist-turns-in-tui-chatapp/design-doc/01-persisting-turns-in-the-tui-chatapp.md
      Note: Design produced in Step 1
ExternalSources: []
Summary: Diary for the TUI chatapp turns persistence design ticket.
LastUpdated: 2026-05-21T16:50:00-04:00
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

## Step 2: Persist command TUI turns and sessionstream timelines

This step implemented the first two phases of the TUI persistence design. Command TUI chat now has a durable final-turn persistence seam for `--turns-db` / `--turns-dsn`, and it can use a file-backed `sessionstream` SQLite hydration store for `--timeline-db` / `--timeline-dsn`.

The implementation preserves the important boundary from the design: the final Geppetto `turns.Turn` is persisted from `PromptRequest.OnFinalTurn`, while visible UI entities are persisted by `sessionstream.HydrationStore`. The TUI still uses `PromptRequest.InitialTurn` for inference context and does not reconstruct model context from timeline entities.

### Prompt Context

**User prompt (verbatim):** "go"

**Assistant interpretation:** Proceed with the next planned implementation work, which in context means implementing the TUI turns/timeline persistence ticket.

**Inferred user intent:** The user wants the documented persistence design converted into working code, with the diary maintained while implementation proceeds.

**Commit (code):** 94c7b29 — "chat: persist TUI turns and timelines"

### What I did

- Added `ui.TurnPersister` and `ui.WithTurnPersister` in `pkg/ui/chatapp_backend.go`.
- Updated `ChatAppBackend.Start` so successful final turns from `PromptRequest.OnFinalTurn` are persisted before `currentTurn` is advanced.
- Added backend tests in `pkg/ui/chatapp_backend_test.go`:
  - successful final-turn persistence;
  - persistence failure returns a Bubble Tea error message and leaves the backend not running.
- Added `openCLISessionstreamHydrationStore` in `pkg/cmds/chat_persistence.go` using:
  - `sessionstream.SchemaRegistry`;
  - `github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/sqlite`.
- Kept `openCLITurnStore` as the current turns-only helper after legacy timeline removal.
- Added helper tests in `pkg/cmds/chat_persistence_test.go` for:
  - no configured hydration store;
  - file-backed sessionstream hydration store path creation;
  - no configured turn store;
  - file-backed turn store path creation;
  - `cliTurnStorePersister` behavior.
- Extended command runner wiring in `pkg/cmds/cmd.go`:
  - added `commandRunnerOptionsWithPersistence`;
  - preserved reasoning and tool-call plugin registration;
  - opened timeline and turns stores in `runChat`;
  - passed `HydrationStore`, `TurnStore`, and schema registry to `chatapp.NewRunner`;
  - passed the CLI turn persister into `NewChatAppBackend`.
- Updated user-facing help in `cmd/pinocchio/doc/general/06-rpc-jsonl-output.md` with a `TUI Chat Persistence` section.
- Ran a real tmux smoke test with:
  - `PINOCCHIO_PROFILE=gpt-5-nano-low`
  - `--chat`
  - `--turns-db /tmp/pin-tui-persist-turns.db`
  - `--timeline-db /tmp/pin-tui-persist-timeline.db`
  - `--debug-events-jsonl /tmp/pin-tui-persist-debug.jsonl`

### Why

- `--turns-db` should store the final model-context turn that the command TUI already receives from inference.
- `--timeline-db` should store the projected sessionstream timeline, not the removed legacy `sem` timeline schema.
- The CLI flags already existed, and after legacy cleanup they needed to be wired to current persistence abstractions.
- Persisting the final turn from `OnFinalTurn` avoids reconstructing model context from UI projections.

### What worked

- Targeted tests passed:
  - `go test ./pkg/ui ./pkg/cmds -count=1`
  - `go test ./pkg/ui ./pkg/cmds ./pkg/chatapp -count=1`
- Schema vet passed:
  - `make schema-vet`
- Full Go tests passed before commit:
  - `go test ./... -count=1`
- Frontend checks passed before commit:
  - `cd cmd/web-chat/web && npm run typecheck`
  - `cd cmd/web-chat/web && npm run lint`
- Pre-commit hooks passed for the code commit, including `go generate`, frontend build, `go build ./...`, lint, vet, and `go test ./...`.
- Real tmux smoke produced visible TUI output with assistant text `tui_persist_ok`.
- The smoke-created turns DB had data:
  - `select count(*) from turns;` returned `1`.
  - `select count(*) from turn_block_membership;` returned `4`.
- The smoke-created timeline DB had sessionstream tables and
 persisted entities:
  - `.tables` included `sessionstream_entities`, `sessionstream_events`, `sessionstream_entity_versions`, `sessionstream_projection_cursors`, `sessionstream_errors`, and `sessionstream_sessions`.
  - `select count(*) from sessionstream_entities;` returned `2`.
- The debug JSONL smoke file had `16` lines.

### What didn't work

- The first diary append command was interrupted before its intended `EOF` terminator, leaving Step 2 truncated after the first half of `What worked`. I inspected the diary tail and appended the missing sections explicitly.
- No implementation blocker was hit in code. The main risk was choosing the correct seam: persisting from `OnFinalTurn` instead of relying only on `TurnStore` injection into `chatapp.RunnerOptions`.

### What I learned

- The existing `cliTurnStorePersister` was already the right turns persistence primitive; the missing piece was exposing it to the TUI backend without importing `chatstore` into `pkg/ui`.
- Passing a `sessionstream.HydrationStore` into `chatapp.NewRunner` is the correct replacement for the removed legacy `chatstore.TimelineStore` path.
- The runner can share one explicit schema registry with the SQLite hydration store, which keeps sessionstream schema handling in the command layer.
- The tmux smoke verified both persistence paths at once: final-turn rows in `turns.db` and visible timeline rows in `timeline.db`.

### What was tricky to build

The key tricky point was persistence ordering. `ChatAppBackend.Start` receives the final turn asynchronously through `PromptRequest.OnFinalTurn`, while `WaitIdle` is the point where the Bubble Tea command can decide success or failure. The implementation now waits for successful idle completion, reads the captured final turn, persists a clone when a persister is configured, and only then advances `currentTurn` and marks the backend idle.

This ordering means a persistence error is surfaced as a backend error instead of silently advancing in-memory state while durable state is stale. That is a deliberately strict choice for now. If operators later prefer best-effort persistence, this behavior can be relaxed with explicit logging/status events.

The other sharp edge was avoiding import cycles and layering violations. `pkg/ui` now depends only on a tiny `TurnPersister` interface. `pkg/cmds` owns the concrete `chatstore.TurnStore` and `sessionstream` SQLite hydration setup, which matches its role as command composition code.

### What warrants a second pair of eyes

- Whether TUI persistence failure should abort the current Bubble Tea command or become best-effort with a visible warning.
- Whether `convID=sessionID=commandSessionID(seed)` is enough until resume UX lands.
- Whether storing both `TurnStore` in `RunnerOptions` and an explicit TUI `TurnPersister` is surprising; it is intentional because explicit `InitialTurn` skips runner-side history loading.
- Whether `--timeline-db` should receive a more discoverable inspection/export command against `sessionstream.HydrationStore`.

### What should be done in the future

- Implement Phase 3: stable `--session-id` / `--conversation-id` and explicit resume UX.
- Add sessionstream timeline inspection tooling if operators need to inspect `--timeline-db` without custom SQLite queries.
- Consider converting the `OnFinalTurn` callback into a typed run-result API so command callers can receive final turn, status, and persistence metadata through one structured return value.

### Code review instructions

- Start with `pkg/cmds/cmd.go`:
  - inspect `runChat` store opening/closing order;
  - inspect `commandRunnerOptionsWithPersistence` and confirm plugin registration is still preserved.
- Then inspect `pkg/ui/chatapp_backend.go`:
  - `TurnPersister`;
  - `WithTurnPersister`;
  - final-turn persistence inside `Start`.
- Then inspect `pkg/cmds/chat_persistence.go`:
  - `openCLISessionstreamHydrationStore`;
  - `openCLITurnStore`;
  - `cliTurnStorePersister`.
- Validate with:
  - `go test ./pkg/ui ./pkg/cmds ./pkg/chatapp -count=1`
  - `make schema-vet`
  - `go test ./... -count=1`
- Optional real TUI smoke:
  - run `PINOCCHIO_PROFILE=gpt-5-nano-low go run ./cmd/pinocchio code professional 'Reply with exactly tui_persist_ok' --chat --turns-db /tmp/pin-tui-persist-turns.db --timeline-db /tmp/pin-tui-persist-timeline.db --debug-events-jsonl /tmp/pin-tui-persist-debug.jsonl` inside tmux;
  - verify `select count(*) from turns;` is greater than zero;
  - verify `sessionstream_entities` exists in the timeline DB and has rows.

### Technical details

The new TUI turn persistence seam is intentionally small:

```go
type TurnPersister interface {
    PersistTurn(ctx context.Context, t *turns.Turn) error
}
```

The command layer adapts the existing CLI turn-store persister:

```go
if turnStore != nil {
    turnPersister = newCLITurnStorePersister(turnStore, string(sid), string(sid), "final")
}
backend, err := pinui.NewChatAppBackend(
    runner.Service,
    sid,
    &infruntime.ComposedRuntime{Engine: eng},
    seed,
    pinui.WithTurnPersister(turnPersister),
)
```

The timeline persistence path uses current `sessionstream` storage, not the removed `sem` timeline stack:

```go
reg := sessionstream.NewSchemaRegistry()
hydrationStore, closeHydrationStore, err := openCLISessionstreamHydrationStore(rc.Persistence, reg)
runner, err := chatapp.NewRunner(commandRunnerOptionsWithPersistence(
    fanoutProxy,
    reg,
    hydrationStore,
    turnStore,
))
```
