---
Title: Diary
Ticket: GP-030
Status: active
Topics:
    - webchat
    - backend
    - pinocchio
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/webchat/conversation_service.go
      Note: Main extraction seam for moving the current LLM startup path behind a Runner
    - Path: pinocchio/pkg/webchat/conversation.go
      Note: Conversation remains the transport identity that runner startup must reuse
    - Path: pinocchio/pkg/webchat/http/api.go
      Note: HTTP helper layer will need to stay compatible while the runner seam is introduced
    - Path: pinocchio/pkg/inference/runtime/composer.go
      Note: Existing runtime composition contract informs the first Runner API shape
ExternalSources: []
Summary: Implementation diary for GP-030: introducing a Runner abstraction into Pinocchio webchat while preserving Conversation as the transport identity.
LastUpdated: 2026-03-07T16:40:00-05:00
WhatFor: Use this diary to capture the implementation steps, commits, validation, and design decisions for GP-030.
WhenToUse: Use when reviewing or continuing the runner extraction work in Pinocchio webchat.
---

# Diary

## Goal

Capture the GP-030 implementation step by step so the runner extraction can be reviewed as a sequence of small, validated changes rather than as one large refactor.

## Context

GP-030 starts from an architecture discussion and design guide. The job now is to convert that into code without breaking the existing generic transport layer or the current `cmd/web-chat` behavior.

## Step 1: Convert GP-030 into an implementation ticket

I started by turning GP-030 from a design-only ticket into an execution ticket. That means adding a diary, fixing the ticket index so it points at ongoing implementation notes, and collapsing the earlier open questions into explicit standing decisions before code changes begin.

This matters because the code work depends on a few architectural choices already made in the discussion: keep `Conversation`, keep runner instantiation app-owned, and use the current LLM startup path as the first extraction target. Without locking those in, the implementation would drift between alternative designs.

### Prompt Context

**User prompt (verbatim):** "ok, we merged and rebased and pinoicchio is all ready to tackle GP-030, close GP-029, then commit GP-030, and then work on it task by task, comitting at appropriate intervals, and keeping a detailed ddiary."

**Assistant interpretation:** Close out the finished GP-029 ticket, turn GP-030 into an active implementation ticket with a diary and executable tasks, commit that ticket state, and then begin implementing GP-030 incrementally with diary updates and commits.

**Inferred user intent:** Move from design to execution in a disciplined way, with ticket state, diary history, and code changes staying synchronized throughout the refactor.

**Commit (code):** pending

### What I did

- Marked GP-029 as closed in its ticket index.
- Updated the GP-030 index to point at this diary.
- Expanded the GP-030 task list into a real execution backlog with phase ordering.
- Converted previous open questions into standing implementation decisions.
- Created this diary document.

### Why

- The user explicitly asked to close GP-029 before starting GP-030.
- GP-030 previously described phases and alternatives but did not yet read like an implementation ticket.
- The first code slice needs the task order and design decisions captured before the refactor starts.

### What worked

- The design guide already contained enough detail to translate directly into code-oriented tasks.
- The earlier discussion provided clear answers for the most important architectural choices.

### What didn't work

- GP-030 did not yet have a diary or a single clear “phase 1 starts here” task sequence, so I had to create that structure first.

### What I learned

- The most important implementation boundary is still `ConversationService.startInferenceForPrompt(...)`.
- The most useful first increment is to introduce runner types and extract the current LLM path without changing transport ownership.

### What was tricky to build

- The subtle part is keeping runner instantiation app-owned while still exposing enough service-level help that embeddings do not have to reach through private `ConvManager` internals.

### What warrants a second pair of eyes

- Whether the first public helper after extraction should be a minimal `StartRequest` builder or a higher-level `StartWithRunner(...)` method.

### What should be done in the future

- Implement the core `Runner` API and extract the LLM startup path behind it in the next step.

### Code review instructions

- Start with the ticket docs under `pinocchio/ttmp/.../GP-030--webchat-process-oriented-core-brainstorm/`.
- Confirm the task ordering matches the design guide before reviewing the first code refactor.

### Technical details

- GP-029 closure is ticket-only state.
- GP-030 now has a dedicated diary and explicit phase tasks for runner extraction, service wiring, and HTTP adaptation.
