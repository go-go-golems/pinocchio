Title: Chat UI Cleanups and Next Steps

Scope: Follow-ups after unifying around timeline events, the new ChatBuilder/ChatSession, and removing legacy Stream* messages.

---

## Summary of State

- Chat model (`bobatea/pkg/chat/model.go`) now only handles:
  - Timeline lifecycle: `timeline.UIEntity{Created,Updated,Completed,Deleted}`
  - Control messages: input focus/blur, submit, cancel, copy, save, navigation, backend finished
- Fake backend (`bobatea/cmd/chat/fake-backend.go`) emits timeline events exclusively.
- Engine backend (`pinocchio/pkg/ui/backend.go`) bridges Geppetto engines to UI via Watermill and forwards to timeline using `StepChatForwardFunc`.
- Unified builder/session (`pinocchio/pkg/ui/runtime/builder.go`) matches the documented API in `01-chat-builder-guide.md`.

---

## Proposed Cleanups

1) Chat model surface tightening
- Remove internal viewport.SetContent calls where redundant; prefer regenerating once per update path.
- Audit TRACE/DEBUG logs; keep high-signal but reduce chatty traces in hot paths.
- Revisit `StateMovingAround` vs. a generic timeline selection mode; potentially encapsulate selection logic into a small `TimelineShell` helper and keep the model thinner.

2) Backend Start ergonomics
- In `EngineBackend.Start`, consider returning a `tea.Batch(boba_chat.BlurInputMsg{}, runCmd)` style command from the model side (already blurring on start); ensure double-blur/focus isn’t happening across layers.
- Validate cancellation: make sure `Interrupt` reliably yields an `EventInterrupt` → `BackendFinishedMsg` via the forwarder even when engines short-circuit.

3) Handler binding and factories
- In `BuildComponents`, improve the lazy-binding proxy’s error text to include a short hint ("call sess.BindHandlerWithProgram(p) before RunHandlers").
- Consider exposing `ChatSession.EventHandlerWith(p *tea.Program)` helper that binds and returns the handler in one step for embedding ergonomics.

4) Docs alignment and examples
- Add a minimal end-to-end example that seeds a `turns.Turn` with both user and assistant blocks to demonstrate `emitInitialEntities` and timeline continuity.
- Update any old references in docs or comments that still mention `Stream*` messages in bobatea chat.

5) Consistency and naming
- Ensure package-level comments in `bobatea/pkg/chat` and `pinocchio/pkg/ui` clearly state the timeline-first architecture.
- Standardize log component names to `chat_model`, `engine_backend`, `step_forward`, `runtime_builder`.

6) Tests
- Add tests for:
  - Timeline-only model paths: Created → Updated → Completed flows update selection, viewport, and copy behaviors.
  - `BackendFinishedMsg` transitions focus/state correctly (from streaming to input).
  - `emitInitialEntities` deduplicates by block ID and renders expected items.
  - `FakeBackend` streaming produces monotonically increasing versions.

---

## Next Steps

- Implement selection shell extraction from the model and migrate keybindings accordingly.
- Add `runtime.ChatSession.EventHandlerWith(p)` and improve `BuildComponents` error hinting.
- Write an integration test using a small fake engine that emits Watermill events to validate the full pipeline (engine → sink → handler → timeline).
- Create example snippets referenced by `01-chat-builder-guide.md` showcasing seeding + autosubmit and embedding flows.

---

## References

- `bobatea/pkg/chat/model.go`
- `bobatea/cmd/chat/fake-backend.go`
- `pinocchio/pkg/ui/backend.go`
- `pinocchio/pkg/ui/runtime/builder.go`
- `pinocchio/pkg/doc/topics/01-chat-builder-guide.md`

