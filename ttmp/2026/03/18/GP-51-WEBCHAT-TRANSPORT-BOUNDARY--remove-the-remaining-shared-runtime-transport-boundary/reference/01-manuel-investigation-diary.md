---
Title: Manuel investigation diary
Ticket: GP-51-WEBCHAT-TRANSPORT-BOUNDARY
Status: active
Topics:
    - pinocchio
    - webchat
    - transport
    - cleanup
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/inference/runtime/profile_runtime.go
      Note: Current shared runtime payload being evaluated for removal.
    - Path: pkg/webchat/http/api.go
      Note: Current shared HTTP transport DTO under review.
    - Path: pkg/inference/runtime/composer.go
      Note: Current runtime builder boundary under review.
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-18T23:59:00-04:00
WhatFor: ""
WhenToUse: ""
---

# Manuel investigation diary

## Goal

Document the final cleanup ticket for removing the remaining shared runtime transport boundary in Pinocchio webchat after the engine-profile migration and local-first app runtime plan work are already complete.

## Step 1: Define the remaining problem precisely

The user asked what still felt shrinkable after the engine-profile migration was finished. The answer was that the system is now healthy, but one shared boundary still carries too much app-owned payload:

- `pkg/inference/runtime.ProfileRuntime`
- `pkg/webchat/http.ResolvedConversationRequest`
- `pkg/inference/runtime.ConversationRuntimeRequest`

The important nuance is that this is no longer a correctness problem. It is a boundary-quality problem. Each app now keeps its own local runtime model first, which was the big win. But at the final handoff, shared Pinocchio types still transport prompt, middleware, and tool policy even though those concepts are fundamentally app-owned.

### Prompt Context

**User prompt (verbatim):** "make a ticket on how and why to kill this remaining boundary and Create a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and detailed. Store in the ticket. We'll do that another day. Close the other tickets."

**Assistant interpretation:** Create a new follow-up ticket that explains why the current shared webchat runtime transport is still larger than ideal, document the options for removing it, and defer implementation.

**Inferred user intent:** Finish the current migration line cleanly, then leave behind a precise next-step ticket for the last remaining transport simplification.

### What I did

- Inspected the current shared transport and builder contracts:
  - [`pkg/inference/runtime/profile_runtime.go`](../../../../../../pinocchio/pkg/inference/runtime/profile_runtime.go)
  - [`pkg/inference/runtime/composer.go`](../../../../../../pinocchio/pkg/inference/runtime/composer.go)
  - [`pkg/webchat/http/api.go`](../../../../../../pinocchio/pkg/webchat/http/api.go)
- Confirmed that each downstream app now has a local-first runtime plan before conversion.
- Created the follow-up ticket and scoped it around two main design options:
  - narrower shared DTO
  - compose-capable interface/closure boundary

### Why

- The migration line is mostly done, so this is the right time to isolate the final cleanup as a separate decision rather than mixing it into the completed engine-profile work.
- The ticket needs to explain both **how** to remove the boundary and **why** the boundary is still larger than the final design should allow.

### What worked

- The problem scope is now narrow and well-defined. It is no longer “fix profiles.” It is “shrink the last shared runtime transport seam.”
- The existing code already provides strong examples of the desired local-first shape in Pinocchio, CoinVault, and Temporal.

### What should be done later

- Decide whether the final boundary is:
  - a smaller data DTO, or
  - a compose-capable app-owned interface
- Then hard-cut the shared transport in one narrow migration sequence.
