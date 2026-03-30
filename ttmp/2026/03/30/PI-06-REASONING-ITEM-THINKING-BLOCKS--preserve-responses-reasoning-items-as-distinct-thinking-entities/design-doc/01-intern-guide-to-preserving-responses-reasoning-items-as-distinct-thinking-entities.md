---
Title: Intern guide to preserving Responses reasoning items as distinct thinking entities
Ticket: PI-06-REASONING-ITEM-THINKING-BLOCKS
Status: active
Topics:
    - pinocchio
    - geppetto
    - webchat
    - open-responses
DocType: design-doc
Intent: implementation
Summary: Detailed analysis, design, and implementation guide for changing the thinking-stream model from one flattened inference-wide lane to per-reasoning-item entities.
LastUpdated: 2026-03-30T16:54:10-04:00
---

# Intern guide to preserving Responses reasoning items as distinct thinking entities

## Goal

This guide explains a long-term fix for a subtle but important streaming bug in the current OpenAI Responses plus Pinocchio web-chat stack. The visible symptom is malformed markdown inside the "Thinking" card. The underlying problem is bigger: the provider emits multiple reasoning items, but the system flattens them into one cumulative thinking stream per inference. The result is lost boundaries, muddled semantics, and awkward rendering artifacts such as bold heading fragments being glued onto previous paragraphs.

Your job in this ticket is not to invent new provider structure. The provider already exposes structure. Your job is to preserve it.

## What the system is doing today

At a high level, the current system has four stages:

1. Geppetto receives streaming provider events from OpenAI.
2. The Responses engine converts those provider events into Geppetto chat events.
3. Pinocchio translates Geppetto events into SEM frames for the web UI.
4. The timeline projector and frontend render those SEM frames as cards or timeline entities.

The bug exists because stage 2 has partial knowledge of provider reasoning items, while stage 3 throws that item identity away and stage 4 never sees it.

## The mental model you need

Think of the provider stream as a tree:

- One inference
- Many output items
- Some output items are `message`
- Some output items are `reasoning`
- A reasoning item itself may stream over many deltas

The current implementation instead models it like this:

- One inference
- One assistant output stream
- One thinking stream

That flattening is the core architectural mistake.

## Current architecture map

### Provider and engine

The Responses engine handles raw provider stream events in:

- `geppetto/pkg/steps/ai/openai_responses/engine.go`

This file already watches normalized provider events such as:

- `response.output_item.added`
- `response.reasoning_text.delta`
- `response.reasoning_text.done`
- `response.output_item.done`

When a new reasoning item starts, the engine currently does this:

- emits `thinking-started`
- resets `currentReasoningText`
- resets `currentReasoningSummary`
- captures `currentReasoningItemID`

That is an important clue. The engine already knows the provider item id. It simply does not preserve that identity in the live SEM-facing event stream.

### Event model

Relevant event definitions live in:

- `geppetto/pkg/events/chat-events.go`

Important current event types include:

- `EventThinkingPartial`
- `EventReasoningTextDelta`
- `EventReasoningTextDone`
- `EventInfo` with `thinking-started` and `thinking-ended`

The current problem is that `EventThinkingPartial` does not carry item identity. It only knows:

- delta
- completion

That is enough for one thinking stream per inference, but not enough for multiple reasoning items within one inference.

### SEM translation

The web-chat translator lives in:

- `pinocchio/pkg/webchat/sem_translator.go`

It currently maps thinking events using one fixed id:

```text
baseID + ":thinking"
```

That means every thinking delta for the entire inference lands on the same SEM entity id.

### Timeline projection

The projector lives in:

- `pinocchio/pkg/webchat/timeline_projector.go`

This code is less broken than it first appears. It keys message content by SEM event id. That means it can already support multiple thinking entities if the ids are different. The projector is not the main obstacle. The translator and engine event model are.

### Frontend

The frontend registry lives in:

- `pinocchio/cmd/web-chat/web/src/sem/registry.ts`

The frontend also is not the fundamental blocker. It already handles streamed `llm.thinking.start`, `llm.thinking.delta`, `llm.thinking.final`, and `llm.thinking.summary` events. If those events arrive with multiple ids, the frontend can render multiple thinking cards.

## Diagram of the current broken flow

```text
OpenAI Responses stream
  -> response.output_item.added (reasoning item A)
  -> response.reasoning_text.delta
  -> response.reasoning_text.delta
  -> response.output_item.done (reasoning item A)
  -> response.output_item.added (reasoning item B)
  -> response.reasoning_text.delta
  -> response.output_item.done (reasoning item B)

Geppetto engine
  -> thinking-started
  -> EventThinkingPartial(delta, cumulative)
  -> thinking-ended
  -> thinking-started
  -> EventThinkingPartial(delta, cumulative)
  -> thinking-ended

Pinocchio SEM translator
  -> llm.thinking.start id=llm-inf-123:thinking
  -> llm.thinking.delta id=llm-inf-123:thinking
  -> llm.thinking.final id=llm-inf-123:thinking
  -> llm.thinking.start id=llm-inf-123:thinking
  -> llm.thinking.delta id=llm-inf-123:thinking
  -> llm.thinking.final id=llm-inf-123:thinking

Timeline/UI
  -> one thinking card repeatedly reopened and overwritten
```

## Diagram of the target flow

```text
OpenAI Responses stream
  -> reasoning item A
  -> reasoning item B

Geppetto engine
  -> ThinkingStarted(item_id=A)
  -> ThinkingPartial(item_id=A, delta, cumulative)
  -> ThinkingDone(item_id=A)
  -> ThinkingStarted(item_id=B)
  -> ThinkingPartial(item_id=B, delta, cumulative)
  -> ThinkingDone(item_id=B)

Pinocchio SEM translator
  -> llm.thinking.start id=llm-inf-123:thinking:A
  -> llm.thinking.delta id=llm-inf-123:thinking:A
  -> llm.thinking.final id=llm-inf-123:thinking:A
  -> llm.thinking.start id=llm-inf-123:thinking:B
  -> llm.thinking.delta id=llm-inf-123:thinking:B
  -> llm.thinking.final id=llm-inf-123:thinking:B

Timeline/UI
  -> one thinking card for item A
  -> one thinking card for item B
```

## Why the long-term fix is better than the short-term fix

The short-term fix can add missing blank lines where markdown looks obviously broken. That is useful, but it is only a formatting repair. It does not preserve the provider's original structure.

The long-term fix is better because:

- it preserves provider reasoning-item boundaries
- it stops reopening or mutating one giant thinking entity
- it makes debug output and timeline history more truthful
- it gives future UI work a much cleaner model
- it avoids heuristic formatting patches as the main correctness mechanism

## Design choices you need to make

### Choice 1: extend existing events or add new typed events

You have two realistic options.

Option A: extend the current thinking events.

- Add `ItemID string` to `EventThinkingPartial`
- Add `ItemID` in the data payload of `thinking-started` and `thinking-ended`

Pros:

- Smallest surface-area change
- Easier migration for existing handlers

Cons:

- `EventInfo` remains a weakly typed carrier for important lifecycle state
- The contract stays slightly muddy

Option B: introduce dedicated typed lifecycle events.

- `EventThinkingStarted`
- `EventThinkingPartial`
- `EventThinkingDone`
- optionally `EventThinkingSummary`

Pros:

- Clearer contracts
- Better long-term type safety
- Easier to reason about than overloading `EventInfo`

Cons:

- More invasive change
- More code churn

Recommended choice for this ticket:

- Use Option A if you want the smallest correct change
- Use Option B if you want a cleaner event model and can afford a broader refactor

## Recommended implementation strategy

Do the work in these layers, in this order.

### Phase 1: make the Geppetto event model item-aware

Update `geppetto/pkg/events/chat-events.go`.

Recommended minimum contract:

```go
type EventThinkingPartial struct {
    EventImpl
    ItemID     string `json:"item_id,omitempty"`
    Delta      string `json:"delta"`
    Completion string `json:"completion"`
}
```

If you stay with `EventInfo` for start and end events, make sure the `Data` map includes:

- `item_id`

Example:

```go
events.NewInfoEvent(metadata, "thinking-started", map[string]any{
    "item_id": currentReasoningItemID,
})
```

### Phase 2: update the Responses engine to emit per-item lifecycle

Update:

- `geppetto/pkg/steps/ai/openai_responses/engine.go`

When `response.output_item.added` announces a `reasoning` item:

- if another reasoning item is already active, finish it first
- set `currentReasoningItemID`
- reset per-item accumulators
- emit item-aware `thinking-started`

When `response.reasoning_text.delta` arrives:

- append to the per-item cumulative buffer
- emit item-aware `EventThinkingPartial`

When `response.output_item.done` for a `reasoning` item arrives:

- emit item-aware `thinking-ended`
- persist the completed reasoning block
- clear current item state

Pseudocode:

```go
if event == output_item_added && item.type == "reasoning" {
    if activeReasoningItemID != "" {
        emitThinkingEnded(activeReasoningItemID)
    }
    activeReasoningItemID = item.id
    currentReasoningText.Reset()
    currentReasoningSummary.Reset()
    emitThinkingStarted(activeReasoningItemID)
}

if event == reasoning_text_delta {
    currentReasoningText.WriteString(delta)
    emitThinkingPartial(activeReasoningItemID, delta, currentReasoningText.String())
}

if event == output_item_done && item.type == "reasoning" {
    emitThinkingEnded(activeReasoningItemID)
    persistReasoningBlock(activeReasoningItemID, currentReasoningText.String(), ...)
    activeReasoningItemID = ""
}
```

### Phase 3: define the SEM id scheme

Update:

- `pinocchio/pkg/webchat/sem_translator.go`

Do not keep using only:

```go
baseID + ":thinking"
```

Use item-aware ids such as:

```go
thinkingID := baseID + ":thinking:" + itemID
```

If the provider item id is absent, fall back safely:

```go
thinkingID := baseID + ":thinking"
```

This fallback matters for engines that do not expose item ids.

### Phase 4: update SEM lifecycle translation

The translator must ensure that:

- start
- delta
- final
- summary

all use the same thinking id for the same provider reasoning item.

Be careful here. If start uses one id and summary uses another, the projector will produce separate cards and the UI will look broken again.

### Phase 5: update projector and frontend tests

Files to update:

- `pinocchio/pkg/webchat/sem_translator_test.go`
- `pinocchio/pkg/webchat/timeline_projector_test.go`
- `pinocchio/cmd/web-chat/web/src/sem/registry.test.ts`

Add tests that simulate:

- reasoning item A streaming
- reasoning item A finishing
- reasoning item B starting
- reasoning item B finishing

Expected result:

- two thinking message entities
- each one has its own content
- neither one reopens the other

## Summary semantics

This is one of the few tricky design decisions.

Today `reasoning-summary` maps to one fixed thinking id. With multiple reasoning items, you need to decide where summary text belongs.

Recommended rule:

- attach summary to the most recently completed reasoning item

Why:

- it keeps summary near the reasoning segment that produced it
- it avoids reopening a global inference-wide thinking card

Do not attach the same summary to all prior reasoning items.

## Backward compatibility

Do not force every engine to emit multiple thinking items.

Instead:

- Responses engine becomes item-aware
- older engines can continue to emit one itemless thinking stream
- SEM translator uses item-aware ids when available and falls back otherwise

That keeps the system compatible while improving the richer path.

## Review checklist

- Does the Geppetto event model preserve `item_id` for live thinking events?
- Does the Responses engine close the old item before starting a new one?
- Does the translator derive thinking ids from item identity?
- Do timeline tests prove multiple thinking ids can coexist in one inference?
- Does the UI show multiple thinking cards rather than one repeatedly reopened card?
- Does summary text land on the correct item?
- Do non-Responses engines still work?

## Risks and sharp edges

- Provider event ordering might not always be as clean as expected.
- Summary events may arrive after a final event, so id reuse must be deliberate.
- If per-item ids are unstable or missing, fallback behavior must remain safe.
- Tests that assume one thinking entity per inference will fail until updated.

## Practical implementation notes

- Start by making the event model expressive enough.
- Do not try to patch everything only in the frontend.
- Do not try to fake multiple thinking items by splitting on markdown headings. That would be a UI heuristic, not a provider-accurate model.
- Use the provider item id whenever possible. The provider already did the hard work of segmentation.

## File-by-file reading order for a new intern

Read these in order:

1. `geppetto/pkg/steps/ai/openai_responses/engine.go`
   - Understand provider event handling and current reasoning state.
2. `geppetto/pkg/events/chat-events.go`
   - Understand what event contracts currently exist.
3. `pinocchio/pkg/webchat/sem_translator.go`
   - Understand where item identity is currently lost.
4. `pinocchio/pkg/webchat/timeline_projector.go`
   - Understand how SEM ids become timeline entities.
5. `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
   - Understand frontend event projection.
6. `geppetto/pkg/steps/ai/openai_responses/engine_test.go`
   - Understand current reasoning streaming expectations.

## Final recommendation

Treat this ticket as an entity-modeling change, not a markdown-formatting change.

The short-term patch can insert missing blank lines. The long-term fix should preserve provider reasoning items as first-class streamed thinking entities. That is the correct abstraction and the correct place to spend engineering effort if the product expects richer reasoning UIs in the future.
