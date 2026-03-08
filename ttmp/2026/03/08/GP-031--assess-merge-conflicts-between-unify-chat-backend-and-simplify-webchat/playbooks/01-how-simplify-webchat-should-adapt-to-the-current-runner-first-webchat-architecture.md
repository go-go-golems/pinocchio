---
Title: How simplify-webchat should adapt to the current runner-first webchat architecture
Ticket: GP-031
Status: active
Topics:
    - backend
    - pinocchio
    - refactor
    - webchat
DocType: playbooks
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/profile_policy.go
      Note: Resolver rejects legacy selectors and parses registry/profile cookies
    - Path: pkg/webchat/chat_service.go
      Note: Architectural boundary upstream should not collapse
    - Path: pkg/webchat/conversation_service.go
      Note: Lower-level conversation/runtime service retained
    - Path: pkg/webchat/http/api.go
      Note: Canonical chat request payload now uses profile and registry
    - Path: pkg/webchat/http/profile_api.go
      Note: Current-profile cookie route now speaks profile and registry
    - Path: pkg/webchat/router.go
      Note: Deps-first router surface upstream must preserve
    - Path: pkg/webchat/server.go
      Note: Deps-first server surface upstream must preserve
ExternalSources: []
Summary: Guide for upstreaming simplify-webchat cleanups onto the newer ChatService/ConversationService split and deps-first router/server design.
LastUpdated: 2026-03-08T17:08:38.898647411-04:00
WhatFor: ""
WhenToUse: ""
---


# How simplify-webchat should adapt to the current runner-first webchat architecture

## Purpose

This playbook explains how to carry forward the useful cleanup intent from `wesen/task/simplify-webchat` without regressing the newer `task/unify-chat-backend` architecture.

Use it when:

- replaying upstream cleanups onto the current branch
- reviewing future simplify-webchat follow-up PRs
- explaining to upstream why certain structural deletions should not be merged as-is

Current status on `task/unify-chat-backend`:

- request/profile contract cleanup has been replayed
- debug payload cleanup has been replayed
- alias API shims have been removed
- router/server compatibility-surface cleanup has been completed
- migration help now lives in `pkg/doc/topics/webchat-compatibility-surface-migration-guide.md`

## Keep These Current-Branch Architectural Decisions

Do not replace these with the simplify branch versions:

- `pkg/webchat/chat_service.go`
- `pkg/webchat/conversation_service.go`
- `pkg/webchat/router.go`
- `pkg/webchat/server.go`
- `pkg/webchat/types.go`

Specifically keep:

- `ChatService` as a distinct wrapper boundary for prompt submission, queueing, idempotency, and runner orchestration
- `ConversationService` as the lower-level conversation/runtime service
- deps-first construction via `NewRouterFromDeps` and `NewServerFromDeps`
- `Runner` / `LLMLoopRunner` / `PrepareRunnerStart` split

## Safe Improvements To Replay

### 1. Request contract cleanup

Carry over:

- `profile` instead of `runtime_key`
- `registry` instead of `registry_slug`
- explicit rejection of legacy selector inputs
- cookie parsing that supports `registry/profile`

Files:

- `pkg/webchat/http/api.go`
- `pkg/webchat/http/profile_api.go`
- `cmd/web-chat/profile_policy.go`
- `cmd/web-chat/profile_policy_test.go`
- `cmd/web-chat/app_owned_chat_integration_test.go`
- `cmd/web-chat/web/src/store/profileApi.ts`
- `cmd/web-chat/web/src/webchat/ChatWidget.tsx`

### 2. Debug contract cleanup

Carry over:

- `resolved_runtime_key` instead of `current_runtime_key`
- turn/debug payload naming aligned with the resolved runtime identity

Files:

- `pkg/webchat/router_debug_routes.go`
- `pkg/webchat/router_debug_api_test.go`
- `cmd/web-chat/web/src/debug-ui/api/debugApi.ts`
- `cmd/web-chat/web/src/debug-ui/api/debugApi.test.ts`
- `cmd/web-chat/web/src/debug-ui/mocks/msw/createDebugHandlers.ts`

### 3. Alias-package cleanup

Delete the alias shims once verification is complete:

- `pkg/webchat/bootstrap/api.go`
- `pkg/webchat/chat/api.go`
- `pkg/webchat/stream/api.go`
- `pkg/webchat/timeline/api.go`

## Changes Upstream Should Not Reapply

Do not reintroduce these simplify-branch assumptions:

- `type ChatService = ConversationService`
- removing `ChatService.StartPromptWithRunner(...)`
- removing `ChatService.NewLLMLoopRunner(...)`
- removing queue/idempotency handling from `ChatService`
- replacing deps-first router/server construction with older values-only constructors

Reason:

Those changes were reasonable when `ChatService` was a thin wrapper, but that is no longer true. The current branch moved real behavior into `ChatService`.

## Recommended Upstream Rebase Strategy

1. Start from the current `task/unify-chat-backend` descendants, not from old `main`.
2. Port request/profile contract cleanups first.
3. Port debug payload renames second.
4. Delete alias packages third.
5. If future cleanup ideas touch public helpers again, update the migration guide instead of resurrecting the deleted APIs.

## Review Checklist

- `ChatService` still exists as a real type, not a type alias.
- `ConversationService` still owns lower-level conversation/runtime preparation.
- `/chat` and `/ws` request resolution uses `profile` and `registry`.
- legacy `runtime_key` and `registry_slug` selectors fail with `400`.
- `/api/chat/profile` reads and writes `profile` and `registry`.
- debug payloads expose `resolved_runtime_key`.
- alias `pkg/webchat/*/api.go` shims remain deleted and are not reintroduced.
- router/server compatibility helpers remain deleted and embedders use explicit mux registration plus app-owned middleware-definition registries.

## Suggested Commit Slicing

Use separate commits for:

1. request/profile contract cleanup
2. debug contract cleanup
3. alias-package deletion
4. router/server compatibility-surface removal
5. documentation/playbook/diary/help updates

That keeps future cherry-picking and blame much cleaner than one combined merge-resolution commit.
