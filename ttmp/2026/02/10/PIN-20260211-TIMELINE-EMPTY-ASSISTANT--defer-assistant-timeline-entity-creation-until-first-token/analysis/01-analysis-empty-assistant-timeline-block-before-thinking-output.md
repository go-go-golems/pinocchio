---
Title: 'Analysis: Empty assistant timeline block before thinking output'
Ticket: PIN-20260211-TIMELINE-EMPTY-ASSISTANT
Status: active
Topics:
    - pinocchio
    - bug
    - chat
    - backend
    - analysis
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/ui/backend.go
      Note: Primary cmd/pinocchio timeline forwarder causing empty assistant block
    - Path: pkg/webchat/sem_translator.go
      Note: Emits llm.start before assistant deltas
    - Path: pkg/webchat/timeline_projector.go
      Note: Persists empty message entity on llm.start
ExternalSources: []
Summary: ""
LastUpdated: 2026-02-10T20:09:57.782197817-05:00
WhatFor: ""
WhenToUse: ""
---


# Analysis

## Problem Statement

When using a thinking-capable model through `cmd/pinocchio`, the timeline shows:

1. an empty `(assistant)` block,
2. then a `(thinking)` block streaming content,
3. then assistant text starts streaming into the previously empty assistant block.

Desired behavior: do not create the assistant timeline widget at stream start; create it only when the assistant receives its first text token.

## Where This Comes From

### Primary root cause (cmd/pinocchio chat UI path)

File: `pkg/ui/backend.go`

- `StepChatForwardFunc` creates an assistant timeline entity immediately on `EventPartialCompletionStart`:
  - `pkg/ui/backend.go:234`
  - `pkg/ui/backend.go:237`
  - props include `"text": ""` and `"streaming": true`.
- Thinking starts via `EventInfo` (`"thinking-started"`) and creates a separate thinking entity:
  - `pkg/ui/backend.go:288`
  - `pkg/ui/backend.go:292`
- Assistant text arrives later in `EventPartialCompletion`, which only updates the already-created assistant entity:
  - `pkg/ui/backend.go:243`
  - `pkg/ui/backend.go:246`

Result: a visible empty assistant block is rendered while thinking is active.

### Why this ordering happens

`EventPartialCompletionStart` means "generation started", not "assistant has text".
Thinking-capable engines can emit thinking events before any assistant text delta.
So creating an assistant UI entity on start is too early for this model behavior.

## Secondary Affected Path

The same eager-create pattern also exists in webchat timeline projection:

File: `pkg/webchat/timeline_projector.go`

- On `llm.start`, projector upserts a `message` snapshot with empty `Content` and `Streaming=true`:
  - `pkg/webchat/timeline_projector.go:117`
  - `pkg/webchat/timeline_projector.go:133`
  - `pkg/webchat/timeline_projector.go:140`
- On first `llm.delta`, content is updated:
  - `pkg/webchat/timeline_projector.go:147`
  - `pkg/webchat/timeline_projector.go:171`

This means webchat timeline consumers can show the same empty assistant placeholder behavior.

## Additional Similarity

`cmd/agents/simple-chat-agent` has the same start-time assistant creation:

- `cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:127`
- `cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go:129`

This is not the immediate user-reported surface, but any fix should consider keeping these forwarders aligned.

## Recommended Behavior

For assistant role entities:

1. On start event: record stream metadata/identity only; do not create assistant entity yet.
2. On first assistant delta:
   - create assistant entity if missing,
   - set text to cumulative content,
   - mark streaming true.
3. On final/interrupt/error:
   - if assistant entity does not exist and final text is non-empty, create+complete it.
   - if no assistant text was ever produced, do not create an empty assistant widget.

Thinking entities can continue to be created at thinking start (or similarly deferred to first thinking delta, depending on UX preference).

## Implementation Sketch (no code changes in this ticket)

### cmd/pinocchio UI forwarder

File: `pkg/ui/backend.go`

- Add a small in-memory set/map for "assistant entity created for stream ID".
- In `EventPartialCompletionStart`, store metadata only (no `UIEntityCreated`).
- In `EventPartialCompletion`, if not created:
  - emit `UIEntityCreated` with current completion text,
  - then emit `UIEntityUpdated` as usual.
- In `EventFinal`, `EventInterrupt`, `EventError`, guard for missing entity:
  - create on demand when there is displayable text.

### webchat projector parity

File: `pkg/webchat/timeline_projector.go`

- Keep role cache updates on `llm.start` but skip upsert of empty message snapshot.
- Upsert message entity on first `llm.delta` (or final with non-empty text).

## Test Gaps and Suggested Coverage

Current repo has no focused tests for `StepChatForwardFunc` behavior around start/delta ordering.

Suggested tests:

1. `start -> thinking-started -> thinking-delta -> assistant-delta`:
   - assert no assistant entity created before assistant delta.
2. `start -> final(with text) with no deltas`:
   - assert assistant entity still appears with final text.
3. `start -> error with no assistant text`:
   - assert no empty assistant placeholder is emitted.
4. Webchat projector:
   - assert `llm.start` does not persist empty `message` snapshot if deferred strategy is adopted.

## "What's Left" Snapshot In Pinocchio

From `docmgr list tickets` (pinocchio docs root), there are 6 active tickets as of 2026-02-10:

- `001-ADD-OCR-VERB` (49 open / 9 done)
- `FIX-GLAZED-FLAGS` (1 open)
- `001-FIX-KEY-TAG-LINTING` (11 open / 1 done)
- `001-FIX-GLAZED-LINTING` (1 open)
- `IMPROVE-PROFILES-001` (1 open)
- `PIN-20251118` (9 open)

This new ticket is focused strictly on the empty assistant timeline placeholder bug and analysis.
