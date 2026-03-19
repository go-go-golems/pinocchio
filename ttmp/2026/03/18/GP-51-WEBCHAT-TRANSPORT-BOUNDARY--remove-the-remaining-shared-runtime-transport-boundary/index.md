---
Title: remove the remaining shared runtime transport boundary
Ticket: GP-51-WEBCHAT-TRANSPORT-BOUNDARY
Status: active
Topics:
    - pinocchio
    - architecture
    - cleanup
    - backend
    - conversation
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/inference/runtime/profile_runtime.go
      Note: Current shared app-runtime payload that still crosses the shared boundary.
    - Path: pkg/inference/runtime/composer.go
      Note: Current shared runtime builder request that still carries resolved runtime payload fields.
    - Path: pkg/webchat/http/api.go
      Note: Current shared HTTP transport DTO for chat and websocket request resolution.
    - Path: cmd/web-chat/profile_policy.go
      Note: Current Pinocchio local-first resolver that still converts into the shared transport DTO.
ExternalSources: []
Summary: Plan for removing the remaining shared runtime transport boundary in Pinocchio webchat after engine profiles and local-first runtime planning are already in place.
LastUpdated: 2026-03-18T23:59:00-04:00
WhatFor: Use this ticket when planning the final cleanup that removes shared prompt/middleware/tool transport payloads from Pinocchio webchat and replaces them with a narrower app-owned composition boundary.
WhenToUse: Use when you want to shrink or remove `ProfileRuntime`, shrink `ResolvedConversationRequest`, or move the last shared runtime payload transport details back into app-owned code.
---

# remove the remaining shared runtime transport boundary

## Overview

Pinocchio webchat is now on stable legs:

- Geppetto owns engine profiles and final `InferenceSettings`
- apps own prompt, middleware, tools, runtime key, and runtime fingerprint
- each downstream app now builds a local-first resolved runtime plan before converting to the shared Pinocchio webchat transport

That means only one architectural seam still feels larger than it should:

- [`pkg/inference/runtime/ProfileRuntime`](../../../../../../pinocchio/pkg/inference/runtime/profile_runtime.go)
- [`pkg/webchat/http.ResolvedConversationRequest`](../../../../../../pinocchio/pkg/webchat/http/api.go)
- [`pkg/inference/runtime.ConversationRuntimeRequest`](../../../../../../pinocchio/pkg/inference/runtime/composer.go)

These types are no longer the app’s primary model, which is good. But they still carry app-owned prompt/middleware/tool payload across a shared boundary. This ticket captures the final cleanup: shrinking or removing that shared runtime transport layer so shared webchat only sees the narrow data it truly needs.

## Status

Current status: **active**

This ticket is design-only for now. We are not implementing it today.

## Key Questions

1. Should shared webchat continue to receive prompt/middleware/tool payload at all?
2. Should the remaining boundary become a compose-capable interface instead of a data DTO?
3. How much identity should remain in the shared transport:
   - `RuntimeKey`
   - `RuntimeFingerprint`
   - `ProfileVersion`
   - metadata
4. Which package should own the final boundary contract:
   - `pkg/webchat/http`
   - `pkg/inference/runtime`
   - app code only

## Tasks

See [tasks.md](./tasks.md).

## Changelog

See [changelog.md](./changelog.md).

## Structure

- design-doc/ - Architecture and migration documents
- reference/ - Investigation diary and notes
