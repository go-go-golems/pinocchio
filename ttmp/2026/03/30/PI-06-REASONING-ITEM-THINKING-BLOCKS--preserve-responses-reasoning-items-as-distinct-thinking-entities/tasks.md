---
Title: Tasks
Ticket: PI-06-REASONING-ITEM-THINKING-BLOCKS
Status: active
Topics:
    - pinocchio
    - geppetto
    - webchat
    - open-responses
DocType: reference
Intent: implementation
Summary: Detailed implementation task list for preserving Responses reasoning items as distinct thinking entities.
LastUpdated: 2026-03-30T16:54:10-04:00
---

# Tasks

- [ ] Add item-aware thinking lifecycle events in Geppetto.
- [ ] Decide whether to extend `EventThinkingPartial` and `EventInfo` or introduce dedicated typed start and done events for thinking items.
- [ ] Ensure the chosen event model can carry provider `item_id` without breaking non-Responses engines.
- [ ] Update `geppetto/pkg/steps/ai/openai_responses/engine.go` so each `response.output_item.added` for `reasoning` starts a distinct thinking item.
- [ ] Update `geppetto/pkg/steps/ai/openai_responses/engine.go` so each `response.output_item.done` for `reasoning` ends the matching thinking item instead of ending an inference-global stream.
- [ ] Preserve per-item cumulative text and per-item summaries separately from inference-global aggregates.
- [ ] Decide how `reasoning-summary` should map once there are multiple thinking items.
- [ ] Keep the old OpenAI chat engine behavior compatible, even if it still only exposes one thinking lane.
- [ ] Update `pinocchio/pkg/webchat/sem_translator.go` so thinking SEM ids incorporate provider reasoning item identity rather than always using `baseID + ":thinking"`.
- [ ] Verify that `llm.thinking.start`, `llm.thinking.delta`, `llm.thinking.final`, and `llm.thinking.summary` all refer to the same per-item SEM id.
- [ ] Update `pinocchio/pkg/webchat/timeline_projector.go` tests to cover multiple thinking ids in one inference.
- [ ] Update `pinocchio/pkg/webchat/sem_translator_test.go` to cover per-item reasoning lifecycle translation.
- [ ] Update frontend registry tests in `pinocchio/cmd/web-chat/web/src/sem/registry.test.ts` to assert that multiple thinking ids project into multiple cards or entities.
- [ ] Manually validate the change against a local web-chat session with a model that emits multiple Responses reasoning items.
- [ ] Decide whether to expose reasoning item identity in persisted timeline metadata for debugging and future UI features.
- [ ] Document the new reasoning-item contract in the relevant web-chat and SEM docs after implementation.
