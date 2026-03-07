---
Title: Webchat Values Separation Brief
Ticket: GP-029
Status: active
Topics:
    - webchat
    - backend
    - pinocchio
    - refactor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/inference/runtime/composer.go
      Note: Reference for the explicit runtime builder interface Router should continue to consume
    - Path: pinocchio/pkg/webchat/http/api.go
      Note: Reference for app-owned chat and websocket handler boundary
    - Path: pinocchio/pkg/webchat/router.go
      Note: Current constructor decodes router settings from Glazed values
    - Path: pinocchio/pkg/webchat/server.go
      Note: Companion constructor that should follow the same separation pattern
    - Path: pinocchio/pkg/webchat/stream_backend.go
      Note: Current stream backend creation decodes Redis settings from Glazed values
ExternalSources: []
Summary: Brief for separating Glazed values parsing from Pinocchio webchat router construction while preserving app-owned /chat semantics and generic SEM/websocket projection.
LastUpdated: 2026-03-07T14:13:31.868873174-05:00
WhatFor: Use this brief to refactor Pinocchio webchat so router/service construction depends on explicit infrastructure inputs instead of Glazed parsed values, while keeping application-owned chat-start semantics intact.
WhenToUse: Use when extracting configuration parsing out of webchat core, designing a cleaner embedding API, or aligning Pinocchio with applications that want app-specific /chat behavior and generic SEM/websocket plumbing.
---


# Webchat Values Separation Brief

## Executive Summary

`pinocchio/pkg/webchat.Router` currently depends directly on `*values.Values`. That mixes two separate concerns:

- generic webchat core composition: stream backend, stores, conversation lifecycle, websocket attach, hydration;
- application/config parsing: Glazed flags and sections such as default router settings and Redis transport settings.

The goal of this refactor is to make webchat core depend on explicit infrastructure inputs rather than on Glazed directly. Applications should remain responsible for deciding what `POST /chat` means and how a conversation/runtime is resolved, while Pinocchio continues to own the generic SEM stream, timeline projection, websocket, and hydration machinery.

## Problem Statement

The current API forces embedders to pass `*values.Values` into `webchat.NewRouter(...)` even when the embedding application already has a stronger application boundary.

That creates three problems:

1. `pkg/webchat` depends on Glazed parsing instead of depending on already-resolved infrastructure.
2. Applications cannot cleanly construct webchat from explicit dependencies such as a `StreamBackend`, `TimelineStore`, `TurnStore`, and timeout settings.
3. It muddies the architecture. The app should own request resolution and chat-start semantics; core should own generic conversation/SEM transport behavior.

Concrete current usage:

- [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go) decodes default router settings from `parsed`.
- [stream_backend.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/stream_backend.go) decodes Redis settings from `parsed`.
- applications that want to embed Router must therefore preserve and thread Glazed values even if they only want the generic runtime/transport layer.

## Proposed Solution

Split the API into two layers:

1. Explicit dependency-injected construction in core.
2. Optional Glazed-backed convenience helpers layered on top.

Recommended shape:

```go
type RouterDeps struct {
    StreamBackend    StreamBackend
    TimelineStore    chatstore.TimelineStore
    TurnStore        chatstore.TurnStore
    RuntimeBuilder   infruntime.RuntimeBuilder
    StepController   *toolloop.StepController
    StaticFS         fs.FS
    Settings         RouterSettings
}

func NewRouterFromDeps(ctx context.Context, deps RouterDeps, opts ...RouterOption) (*Router, error)
```

Then keep a convenience path for current Glazed callers:

```go
type ParsedConfigInputs struct {
    Parsed *values.Values
    StaticFS fs.FS
}

func NewRouterFromParsed(ctx context.Context, in ParsedConfigInputs, opts ...RouterOption) (*Router, error)
```

or retain `NewRouter(...)` as a compatibility wrapper that internally calls the parsed adapter.

The important architectural rule:

- `NewRouterFromDeps(...)` is the real core API.
- parsed-values decoding becomes adapter code, not the core constructor.

This preserves the intended layering:

- app-owned:
  - `/chat` semantics
  - request resolver
  - profile/runtime policy
  - feature-specific conversation bootstrap
- Pinocchio-owned:
  - stream hub
  - conversation manager
  - websocket attach
  - timeline projection
  - timeline hydration
  - turn persistence
  - helper API/UI mounting

## Design Decisions

### 1. Keep `/chat` App-Owned

Do not move application-specific chat-start semantics into webchat core.

The application should decide:

- how `conv_id` is resolved;
- which runtime/profile applies;
- which tools are allowed for this conversation;
- what domain-specific session bootstrap data is required.

Pinocchio should continue to expose generic services and helper handlers, not impose a single app-level meaning for `POST /chat`.

### 2. Make Glazed Parsing An Adapter, Not A Core Dependency

`*values.Values` is still useful, but it belongs in a thin adapter layer.

That adapter can:

- decode `RouterSettings`;
- decode Redis transport settings;
- create stores/backends from parsed config;
- call `NewRouterFromDeps(...)`.

### 3. Preserve Existing Convenience For Current Callers

Avoid a flag day if possible.

Recommended migration plan:

- add `NewRouterFromDeps(...)`;
- keep `NewRouter(...)` as a compatibility wrapper;
- gradually move docs and embedders to the explicit API.

### 4. Keep Generic SEM/Websocket Machinery In Core

The SEM projection loop, websocket attach, and timeline hydration are already the generic pieces. That should remain the stable Pinocchio layer.

This refactor is about cleaning the construction boundary, not re-architecting transport ownership.

## Alternatives Considered

### Alternative 1: Leave `Values` Inside `NewRouter`

Rejected.

This keeps the API convenient for Glazed-first applications, but it preserves the layering problem and makes third-party embedding less explicit.

### Alternative 2: Remove Convenience Parsing Entirely

Rejected for now.

This would improve purity but would create unnecessary migration churn for current callers such as `cmd/web-chat`.

### Alternative 3: Push More Chat-Start Logic Into Core

Rejected.

The goal is the opposite: keep app-specific `/chat` semantics outside core and keep only generic conversation/SEM transport machinery inside core.

## Implementation Plan

1. Introduce an explicit dependency struct or constructor:
   - `RouterDeps`
   - `NewRouterFromDeps(...)`
2. Extract parsed-values decoding into a thin adapter:
   - `BuildRouterDepsFromValues(...)` or similar
3. Update existing `NewRouter(...)` to delegate to the adapter + explicit constructor.
4. Keep `NewServer(...)` working by forwarding through the same split.
5. Update docs to describe the dependency-injected constructor as the preferred embedding API.
6. Add or update tests proving:
   - router can be built without `*values.Values`;
   - Glazed-backed construction still works;
   - Redis/timeline/turn store wiring remains unchanged.

## Acceptance Criteria

- A caller can build a webchat router without passing `*values.Values`.
- `pkg/webchat` core constructor no longer decodes Glazed sections directly.
- Existing convenience construction still works for current command-line apps.
- The documented architecture remains:
  - app-owned `/chat` semantics;
  - generic Pinocchio SEM/timeline/ws/hydration infrastructure.

## Open Questions

1. Should the compatibility wrapper remain named `NewRouter(...)`, or should the new explicit constructor take that name and the parsed version be renamed?
2. Should `NewServer(...)` also get a dependency-injected variant in the same change, or can that follow immediately after?
3. Should stream backend construction be fully externalized, or is it acceptable for the adapter layer inside `pkg/webchat` to still own Redis-specific setup?

## References

Primary files:

- [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go)
- [stream_backend.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/stream_backend.go)
- [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go)
- [composer.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/inference/runtime/composer.go)
- [server.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/server.go)
