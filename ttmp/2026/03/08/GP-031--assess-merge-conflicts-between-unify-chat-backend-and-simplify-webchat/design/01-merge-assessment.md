---
Title: Merge assessment
Ticket: GP-031
Status: active
Topics:
    - backend
    - pinocchio
    - refactor
    - webchat
DocType: design
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/app_owned_chat_integration_test.go
      Note: Integration tests capture request contract changes
    - Path: cmd/web-chat/main.go
      Note: CLI/help text conflict for debug endpoints
    - Path: cmd/web-chat/profile_policy.go
      Note: Profile selector resolution and legacy-selector rejection
    - Path: cmd/web-chat/web/src/debug-ui/api/debugApi.ts
      Note: Frontend debug contract consumer
    - Path: pkg/webchat/chat_service.go
      Note: Runner lifecycle and ChatService simplification conflict
    - Path: pkg/webchat/http/api.go
      Note: Chat request contract changed from runtime_key to profile/registry
    - Path: pkg/webchat/router.go
      Note: Deps-first router construction versus simplify deletions
    - Path: pkg/webchat/router_debug_routes.go
      Note: Debug payload rename to resolved_runtime_key
    - Path: pkg/webchat/server.go
      Note: Server constructor and compatibility-surface conflict
    - Path: pkg/webchat/types.go
      Note: Shared Router field/type conflict
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T16:31:32.384867539-04:00
WhatFor: ""
WhenToUse: ""
---


# Merge Assessment

## Summary

`wesen/task/simplify-webchat` and the current `task/unify-chat-backend` work diverged from `main` at commit `c97af5b6642cd0bdc823d6ba1e84175f8004bed7`.

The simplify branch is not a simple cleanup branch anymore. It contains:

- safe API and naming cleanups that should still be carried forward
- removal of compatibility layers that now overlap with newer runner/dependency-injection work on `task/unify-chat-backend`

Recommendation: do not merge the branch wholesale. Replay the safe simplifications manually on top of the current architecture, then resolve the remaining structural debt as follow-up refactors on the current branch.

## Replay Status

The low-risk simplify-webchat improvements have now been replayed on the current branch in focused commits:

- `e847f19` `feat(webchat): adopt profile and registry selectors`
- `7458428` `feat(webchat): expose resolved runtime keys in debug api`
- `2755362` `refactor(webchat): drop unused alias api shims`

What remains from the simplify branch is no longer "safe replay" work. The unresolved differences are structural and should be treated as current-branch refactors with explicit review, not as merge conflict cleanup.

## Divergence Snapshot

- merge base: `c97af5b6642cd0bdc823d6ba1e84175f8004bed7`
- commits unique to `task/unify-chat-backend`: `38`
- commits unique to `task/simplify-webchat`: `8`

Current branch themes after the split:

- profile-driven runtime composition in `cmd/web-chat`
- `RouterDeps` / `NewRouterFromDeps` / `NewServerFromDeps`
- runner-oriented `ChatService` behavior
- queue/idempotency orchestration in `pkg/webchat/chat_service.go`
- LLM state and runtime-fingerprint correctness fixes

Simplify branch themes after the split:

- collapse `ChatService` into `ConversationService`
- remove alias subpackages under `pkg/webchat/*/api.go`
- remove router utility mux helpers and middleware registration surface
- rename request/debug contracts from legacy `runtime_key` terminology toward `profile` / `registry` and `resolved_runtime_key`

## Direct Conflict Areas

### 1. `pkg/webchat/chat_service.go`

This is the main semantic conflict.

On the current branch, `ChatService` is no longer a trivial wrapper. It now owns:

- `PrepareRunnerStart`
- `NewLLMLoopRunner`
- `StartPromptWithRunner`
- prompt idempotency persistence
- queue drain / completion handling

On `simplify-webchat`, `ChatService` is reduced to:

- `type ChatService = ConversationService`
- `NewChatService(...) -> NewConversationService(...)`
- `NewChatServiceFromConversation(...) -> svc`

Assessment: the simplify branch assumption is outdated. The wrapper has regained real behavior, so replacing it with a type alias would discard current runner orchestration.

### 2. `pkg/webchat/router.go`

This file has both textual and architectural conflicts.

Current branch adds:

- `NewRouterFromDeps`
- `RouterDeps`
- `RouterSettings` stored directly on the router
- dependency-based store/backend construction
- retained utility mux helpers: `Mount`, `Handle`, `HandleFunc`, `Handler`
- retained middleware factory registration

Simplify branch removes:

- router utility mux helpers
- middleware registration surface
- `ChatService` wrapper usage in favor of `*ConversationService`

Assessment: the current branch's dependency-injection work should win. The simplify branch's router deletions should be replayed selectively after checking which surfaces are still needed by callers and tests.

### 3. `pkg/webchat/server.go`

Current branch adds:

- `NewServerFromDeps`
- deps-first construction path
- compatibility surface for `RegisterMiddleware`
- compatibility constructor `NewFromRouter`

Simplify branch removes or simplifies:

- `RegisterMiddleware`
- `NewFromRouter`
- `ChatService()` return type becomes `*ConversationService`

Assessment: the deps-first server constructor should stay. The compatibility helpers can be retired later, but removing them inside the merge will tangle API cleanup with architecture reconciliation.

### 4. `pkg/webchat/types.go`

This conflict is downstream from router/service decisions:

- current branch keeps `MiddlewareBuilder`, `RouterSettings`, and `chatService *ChatService`
- simplify branch deletes middleware builder support and changes the router field to `*ConversationService`

Assessment: resolve this only after deciding whether the compatibility APIs are still exported intentionally or only accidentally.

### 5. `cmd/web-chat/main.go` and integration tests

There are direct conflicts in:

- `cmd/web-chat/main.go`
- `cmd/web-chat/app_owned_chat_integration_test.go`

The simplify branch carries low-risk but important contract adjustments:

- help text points at `/api/timeline` and `/api/debug/turns`
- tests use `profile` and `registry` instead of legacy selector fields

Assessment: these changes should be ported, but not by taking the simplify branch hunks blindly.

## Cleanups That Still Look Safe To Carry Forward

### Request contract cleanup

The simplify branch updates request handling to prefer:

- `profile`
- `registry`

and demotes/rejects:

- `runtime_key`
- `registry_slug`

Relevant files:

- `cmd/web-chat/profile_policy.go`
- `pkg/webchat/http/api.go`

This is still a good cleanup. The current branch still has many legacy-selector references, so this work is not already absorbed.

### Debug API naming cleanup

The simplify branch renames debug payload fields from:

- `current_runtime_key`

to:

- `resolved_runtime_key`

Relevant files:

- `pkg/webchat/router_debug_routes.go`
- `cmd/web-chat/web/src/debug-ui/api/debugApi.ts`
- `cmd/web-chat/web/src/debug-ui/mocks/msw/createDebugHandlers.ts`
- related debug route tests

This also still looks correct and should be replayed.

### Alias subpackage removal

The simplify branch deletes:

- `pkg/webchat/bootstrap/api.go`
- `pkg/webchat/chat/api.go`
- `pkg/webchat/stream/api.go`
- `pkg/webchat/timeline/api.go`

Repository grep against the current branch found no import sites for those alias packages. That suggests these deletions are probably safe, but they should still be removed in a focused cleanup commit rather than buried inside a conflict-heavy merge.

## Recommended Merge Strategy

### Preferred approach

Do not merge `wesen/task/simplify-webchat` directly into the current branch.

Instead:

1. keep the current branch architecture for `ChatService`, `Router`, `Server`, and deps-first construction
2. keep the replayed request/debug contract cleanups already landed from `simplify-webchat`
3. keep the alias-subpackage deletion already landed in a dedicated cleanup commit
4. separately evaluate whether router utility mux helpers and `RegisterMiddleware` are still externally needed before deleting them

### Why this is safer

The simplify branch was authored before the current branch rebuilt the runner boundary and moved more responsibility into `ChatService`. A direct merge would mix:

- API contract cleanup
- compatibility API deletion
- runner orchestration changes
- dependency injection changes

That makes it too easy to accidentally preserve the wrong side of a conflict.

### Suggested execution order

1. replay `profile` / `registry` request contract updates
2. replay `resolved_runtime_key` debug contract updates
3. delete unused alias subpackages
4. open a dedicated refactor ticket for router/server compatibility surface removal

Status:

- steps 1-3 are complete on the current branch
- step 4 remains open and should not be bundled into a future simplify-webchat merge

## Risk Notes

- The highest-risk mistake is accepting the simplify branch's `ChatService` aliasing, which would silently drop current queue/idempotency/runner behavior.
- The second highest-risk mistake is deleting router/server compatibility helpers without first checking external call sites outside `cmd/web-chat`.
- The safe wins are the request/debug contract cleanup and alias-package deletion.

## Bottom Line

The branch is still valuable, but not as a merge candidate. It should be treated as a source branch for manual cherry-picking of intent, not a source of truth for the current `pkg/webchat` structure.
