---
Title: 'Bug report: post-final thinking delta causes chat waiting-state stall'
Ticket: HC-53-THINKING-TAIL-STUCK
Status: active
Topics:
    - frontend
    - webchat
    - bugs
    - investigation
    - logs
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - ttmp/2026/02/18/HC-53-THINKING-TAIL-STUCK--investigate-llm-thinking-tail-ordering-and-chat-waiting-state-stall/scripts/10_import_logs.py
    - ttmp/2026/02/18/HC-53-THINKING-TAIL-STUCK--investigate-llm-thinking-tail-ordering-and-chat-waiting-state-stall/scripts/q03_post_final_thinking_delta.sql
    - ttmp/2026/02/18/HC-53-THINKING-TAIL-STUCK--investigate-llm-thinking-tail-ordering-and-chat-waiting-state-stall/scripts/q06_events_after_last_thinking_final.sql
    - pinocchio/pkg/webchat/sem_translator.go
    - geppetto/pkg/steps/ai/openai_responses/engine.go
    - 2026-02-12--hypercard-react/apps/inventory/src/features/chat/InventoryChatWindow.tsx
    - 2026-02-12--hypercard-react/packages/engine/src/hypercard-chat/sem/registry.ts
ExternalSources:
    - /tmp/gpt-5.log
    - ~/Downloads/event-log-6ac68635-37bc-4f4d-9657-dfc96df3c5c6-20260218-222305.yaml
Summary: "llm.thinking.delta is emitted after llm.thinking.final because the backend emits an info(reasoning-summary) after SSE loop end and the SEM translator maps it to llm.thinking.delta; the frontend then sets streaming=true again on the thinking message, so waiting state can stay active."
LastUpdated: 2026-02-18T17:36:00-05:00
WhatFor: "Root-cause analysis and fix plan for post-final thinking delta and stuck waiting state"
WhenToUse: "Use when chat remains in Waiting for response or thinking stream appears reopened after final"
---

# Bug report: post-final thinking delta causes chat waiting-state stall

## Executive summary

Observed behavior:
- Event stream includes `llm.thinking.delta` near the end **after** `llm.thinking.final`.
- Chat UI can remain in waiting/streaming state (`Waiting for response...`) even though the assistant final message arrived.

Confirmed root cause:
- Backend emits `EventInfo("thinking-ended")` during `response.output_item.done` for reasoning.
- Later, after SSE loop ends, backend emits `EventInfo("reasoning-summary")` with full summary text.
- SEM translator maps:
  - `thinking-ended` -> `llm.thinking.final`
  - `reasoning-summary` -> `llm.thinking.delta`
- Frontend treats `llm.thinking.delta` as streaming and upserts the thinking message with `streaming: true` again.
- UI streaming check in inventory chat is `any message with props.streaming===true`, so one reopened thinking message keeps waiting state true.

## Scope and inputs

Analyzed artifacts:
- `/tmp/gpt-5.log`
- `~/Downloads/event-log-6ac68635-37bc-4f4d-9657-dfc96df3c5c6-20260218-222305.yaml`

Reproducibility assets in ticket:
- Schema: `scripts/00_schema.sql`
- Importer: `scripts/10_import_logs.py`
- DB build runner: `scripts/11_build_db.sh`
- Query runner: `scripts/20_run_queries.sh`
- Queries: `scripts/q01_*.sql` ... `scripts/q06_*.sql`
- Outputs: `sources/query-results/*.txt`

## Evidence

### 1) Event ordering proves a post-final thinking delta

From `sources/query-results/q03_post_final_thinking_delta.txt`:
- `llm.thinking.final` at idx `214`
- matching thinking id receives `llm.thinking.delta` again at idx `237`

From `sources/query-results/q06_events_after_last_thinking_final.txt`:
- sequence tail is:
  - idx 214 `llm.thinking.final`
  - ...
  - idx 237 `llm.thinking.delta`
  - idx 239 `llm.final`

From raw YAML around the tail:
- `evt-214` -> `eventType: llm.thinking.final`
- `evt-237` -> `eventType: llm.thinking.delta`, same `eventId: ...:thinking`, and message streaming set true via projected upsert (`evt-238`).

### 2) Backend emits a late summary info event by design

In `geppetto/pkg/steps/ai/openai_responses/engine.go`:
- On reasoning output completion, emits `thinking-ended` (`engine.go:471`).
- After SSE loop ends, if `summaryBuf` has content, emits `reasoning-summary` (`engine.go:722`).

This creates a natural late info event after the earlier thinking-ended info event.

### 3) SEM translator maps late summary to thinking delta

In `pinocchio/pkg/webchat/sem_translator.go`:
- `thinking-ended` -> `llm.thinking.final` (`sem_translator.go:335-341`).
- `reasoning-summary` -> `llm.thinking.delta` (`sem_translator.go:342-349`).

Therefore a summary emitted after SSE close reopens thinking stream semantics at SEM level.

### 4) Frontend projection and stream flag behavior

In `packages/engine/src/hypercard-chat/sem/registry.ts`:
- `llmDeltaHandler` upserts with `streaming: true` (`registry.ts:105-119`).
- This handler is used for both `llm.delta` and thinking delta registrations in this registry setup.

In `apps/inventory/src/features/chat/InventoryChatWindow.tsx`:
- `isStreaming` is `timelineEntities.some(entity.kind === 'message' && entity.props.streaming===true)` (`InventoryChatWindow.tsx:171-176`).
- If thinking message gets updated to streaming true late, global waiting state remains true.

## Why this happens specifically at the end

Chronology for the failing turn (UTC in exported YAML):
1. `2026-02-18 22:23:00` `llm.thinking.final` emitted (`evt-214`).
2. Assistant content and suggestions continue.
3. Late `reasoning-summary` is transformed into `llm.thinking.delta` (`evt-237`).
4. Projected thinking entity is upserted with `streaming: true` (`evt-238`).
5. `llm.final` for assistant exists (`evt-239`), but chat-level `isStreaming` still sees at least one message streaming.

## Additional data quality finding (affects tooling only)

The exported EventViewer YAML is malformed indentation-wise (mapping continuation under `entries`).
- Standard `yaml.safe_load` fails.
- Importer includes a robust fallback parser `parse_malformed_event_yaml(...)` in `scripts/10_import_logs.py`.

This does not cause the product bug; it only impacted analysis tooling.

## Impact

User-visible:
- Chat window can stay in waiting/streaming state after a completed assistant response.
- Thinking stream appears to “reopen” at tail of turn.

Engineering:
- Creates confusing timeline semantics: final for thinking no longer terminal.
- Can break UX assumptions and metrics around stream completion.

## Recommended fix options

### Option A (recommended): make thinking-final terminal in translator

Change `sem_translator.go` behavior:
- Keep `thinking-ended` -> `llm.thinking.final`.
- For `reasoning-summary`, emit a dedicated non-streaming SEM kind (`llm.thinking.summary`) instead of `llm.thinking.delta`.
- Projectors update content while forcing `streaming: false`.

Why this is best:
- Preserves stream contract: final means no more deltas for that stream id.
- Minimal surface area and aligned semantics.

### Option B: frontend hardening only

Frontend projector/UI guard:
- Ignore `llm.thinking.delta` after final for same id, or
- Never include role `thinking` in global send-lock `isStreaming` predicate.

Why not first choice:
- Masks semantic inconsistency instead of fixing source.
- Different consumers may still be impacted.

### Option C: backend emission order change

In `engine.go`:
- Emit `reasoning-summary` before `thinking-ended`, or stop emitting summary as info.

Tradeoff:
- Works but is less explicit than translator-side terminal-state enforcement.

## Verification plan

1. Trigger one turn that includes reasoning summary.
2. Run `scripts/11_build_db.sh` then `scripts/20_run_queries.sh`.
3. Assert `q03_post_final_thinking_delta.txt` has zero rows.
4. Assert event tail has `llm.thinking.final` and no later `llm.thinking.delta` for same thinking id.
5. In UI, after assistant final arrives, waiting indicator clears and composer re-enables.

## Conclusion

The post-final thinking delta is not random network disordering. It is a deterministic event mapping pipeline issue:
- `engine.go` emits late `reasoning-summary` info,
- translator maps it to `llm.thinking.delta`,
- projector marks stream as active again,
- chat waiting state remains stuck.

The clean fix is to enforce terminal semantics for thinking streams in translator/projector flow.

## Implementation decision (2026-02-18)

Selected approach: **non-streaming summary event kind**.

Implemented changes:
- `pinocchio/pkg/webchat/sem_translator.go`
  - `EventInfo(\"reasoning-summary\")` now maps to `llm.thinking.summary` with `{ id, text }`.
- `pinocchio/pkg/webchat/timeline_projector.go`
  - Added `llm.thinking.summary` handling as a message upsert with `streaming: false`.
- `2026-02-12--hypercard-react/packages/engine/src/hypercard-chat/sem/registry.ts`
  - Added `llm.thinking.summary` handler that upserts role `thinking`, content from summary text, and `streaming: false`.
- `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
  - Added `llm.thinking.summary` non-streaming handler for the web-chat client.

Rationale:
- Keeps stream semantics clean: `llm.thinking.final` remains terminal.
- Preserves reasoning summary text for UIs without re-opening stream state.

## Anthropic equivalence check

Reviewed Claude Messages API streaming docs:
- Streaming emits thinking as deltas (`thinking_delta`) and verification chunks (`signature_delta`) inside content block deltas.
- Docs do not define an OpenAI-style trailing `reasoning-summary` event emitted after stream completion.

Implication for this fix:
- No parallel change is required for Claude in this specific bug path.
- Existing/new `llm.thinking.summary` handling is still safe cross-provider: if a provider emits a finalized summary event in the future, it remains non-streaming.

References:
- https://docs.claude.com/en/docs/build-with-claude/streaming
- https://docs.claude.com/en/docs/build-with-claude/extended-thinking
