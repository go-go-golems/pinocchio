---
Title: os-openai-app-server workspace impact assessment for adopting task/unify-chat-backend
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
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T17:48:09.859484026-04:00
WhatFor: ""
WhenToUse: ""
---

# os-openai-app-server Workspace Impact Assessment For Adopting `task/unify-chat-backend`

## Executive Summary

Pushing the current `pinocchio` branch does not, by itself, change anything inside `/home/manuel/workspaces/2026-03-02/os-openai-app-server`. The real impact begins when that workspace starts consuming the pushed branch, either by switching its nested `pinocchio` checkout away from `task/simplify-webchat` or by rebuilding `wesen-os`, whose `go.work` file points directly at the local `../pinocchio` checkout.

The good news is that the downstream code in `go-go-os-chat`, `wesen-os`, and `web-agent-example` is already aligned with the contract changes that we actually kept:

- request selection uses `profile` / `registry`
- legacy `runtime_key` / `registry_slug` selectors are rejected
- debug payloads use `resolved_runtime_key`
- chat mounting depends on `webchat.Server` plus `pkg/webchat/http`, not on the simplify-only structural deletions

The main thing that would need to be undone is not downstream app code. It is the local `os-openai-app-server/pinocchio` checkout itself, which is still on `task/simplify-webchat` and therefore still carries four simplify-only structural changes that the pushed branch intentionally does not keep:

- collapsing `ChatService` into `ConversationService`
- removing `Server.NewFromRouter`
- removing router utility mux helpers
- removing middleware registration APIs

In other words: the workspace is mostly contract-compatible with the pushed branch, but the nested `pinocchio` worktree is not branch-compatible with it and would need to be realigned.

## Scope

This assessment answers:

1. which repositories and worktrees inside the `os-openai-app-server` workspace are relevant to `pinocchio` webchat changes,
2. which consumers are likely to break or remain stable if the workspace adopts `task/unify-chat-backend`,
3. which simplify-only edits would need to be backed out from the local `pinocchio` checkout,
4. which docs and tickets inside the workspace would become stale after that switch.

This assessment does not attempt to perform the switch or rewrite any consumer code.

## Workspace Topology

`/home/manuel/workspaces/2026-03-02/os-openai-app-server` is not itself a Git repository. It is a workspace folder containing multiple nested repositories. The relevant ones for this change are:

- `pinocchio`
- `go-go-os-chat`
- `web-agent-example`
- `wesen-os`
- `openai-app-server`
- `geppetto`

Important current branch state from repository inspection:

- `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio` is on `task/simplify-webchat` at `e93c449`
- `/home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example` is on `task/simplify-webchat` at `bcec871`
- `/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat` is on `main`
- `/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os` is on `task/os-openai-app-server`
- the pushed branch lives in a different worktree: `/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio` on `task/unify-chat-backend` at `8a594b8`

That means the same underlying `pinocchio` repository currently has two active worktrees on two diverged feature branches.

## Why This Workspace Is Sensitive To The Local `pinocchio` Checkout

`wesen-os` is not using a released `pinocchio` module in isolation. Its `go.work` includes the local sibling checkout directly, so changing `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio` changes what `wesen-os` and the local `go-go-os-chat` workspace build against immediately.

Evidence:

- `wesen-os/go.work` includes `../pinocchio` and `../go-go-os-chat` directly at [go.work](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/go.work#L3)
- `wesen-os/go.mod` still declares `github.com/go-go-golems/pinocchio v0.10.2`, but the workspace overlay wins locally at [go.mod](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/go.mod#L14)
- `go-go-os-chat/go.mod` declares `github.com/go-go-golems/pinocchio v0.10.1`, but inside the `wesen-os` workspace that also resolves to the local sibling checkout at [go.mod](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/go.mod#L8)

Consequence:

- a plain `git push` is harmless to this workspace
- switching the nested `pinocchio` checkout to `task/unify-chat-backend` is the real compatibility event

## Consumer Impact Assessment

## 1. `go-go-os-chat`

### What it uses

`go-go-os-chat` imports `pinocchio/pkg/webchat` and `pinocchio/pkg/webchat/http` directly in its shared chat component and request resolver:

- [request_resolver.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/profilechat/request_resolver.go#L11)
- [component.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go#L10)

The mounting pattern is the modern handler-first API:

- `Server.ChatService()` at [component.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go#L71)
- `Server.StreamHub()` at [component.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go#L72)
- `Server.TimelineService()` at [component.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go#L77)
- `Server.APIHandler()` at [component.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go#L87)
- `webhttp.RegisterProfileAPIHandlers(...)` at [component.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go#L93)

### Contract alignment

Its request resolver is already aligned with the contract changes we kept:

- request body accepts `profile` and `registry` at [request_resolver.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/profilechat/request_resolver.go#L25)
- it still parses legacy fields only to reject them at [request_resolver.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/profilechat/request_resolver.go#L27)
- it rejects `runtime_key` and `registry_slug` explicitly at [request_resolver.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/profilechat/request_resolver.go#L253)

### Impact

Expected runtime impact from adopting `task/unify-chat-backend`: low.

Reason:

- it depends on the request/debug contract changes we replayed, and those are present on the pushed branch
- it does not depend on simplify-only deletions such as `NewFromRouter`, `RegisterMiddleware`, or the router utility mux API

No `go-go-os-chat` source rollback is indicated by the current evidence.

## 2. `wesen-os`

### What it uses

`wesen-os` consumes the shared chat component rather than the simplify-only internal `pinocchio` surfaces:

- [module.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/pkg/assistantbackendmodule/module.go#L12) imports `go-go-os-chat/pkg/chatservice`
- it passes `*webchat.Server` and `webhttp.ConversationRequestResolver` through to that component at [module.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/pkg/assistantbackendmodule/module.go#L20)
- it mounts the shared chat service routes at [module.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/pkg/assistantbackendmodule/module.go#L87)

### Contract alignment

`wesen-os` tests are already written against the kept contract:

- profile routes are `/api/chat/profiles` and `/api/chat/profile` in the launcher frontend test harness at [setup.ts](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/apps/os-launcher/src/__tests__/setup.ts#L74)
- integration tests reject the legacy `registry_slug` selector at [main_integration_test.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/cmd/wesen-os-launcher/main_integration_test.go#L1216)
- integration tests expect `resolved_runtime_key` in debug payloads at [main_integration_test.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/cmd/wesen-os-launcher/main_integration_test.go#L1465) and [main_integration_test.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/cmd/wesen-os-launcher/main_integration_test.go#L1859)

One compatibility nuance remains in the tests:

- the SEM-frame helper still accepts both camelCase and snake_case runtime-key payloads at [main_integration_test.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/cmd/wesen-os-launcher/main_integration_test.go#L1687)

That is not a blocker for the pushed branch. It is just a tolerant test helper.

### Impact

Expected runtime impact from adopting `task/unify-chat-backend`: low to medium.

Reason:

- the code and tests are already aligned with the replayed request/debug contract changes
- `wesen-os` is the workspace most sensitive to the branch switch because its `go.work` resolves the local `pinocchio` checkout directly
- any breakage would come from branch switching at the module/workspace level, not from a selector-contract mismatch

Recommended caution:

- treat `wesen-os` as the main verification target after any `pinocchio` branch switch

## 3. `web-agent-example`

### What it uses

`web-agent-example` is on a `task/simplify-webchat` branch, but the code itself is already using the stable handler-first surface rather than the simplify-only deletions:

- `webchat.NewServer(...)` at [main.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example/cmd/web-agent-example/main.go#L66)
- `Server.ChatService()` at [main.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example/cmd/web-agent-example/main.go#L75)
- `Server.StreamHub()` at [main.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example/cmd/web-agent-example/main.go#L76)
- `Server.APIHandler()` and `Server.UIHandler()` at [main.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example/cmd/web-agent-example/main.go#L100)

Its local request resolver also builds on the shared `webhttp.ChatRequestBody` contract at [request_resolver.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example/cmd/web-agent-example/request_resolver.go#L56).

### Impact

Expected runtime impact from adopting `task/unify-chat-backend`: low.

Reason:

- the repo was already ported to the modern handler-first API
- it does not rely on simplify-only structural deletions
- the fact that the branch name matches `task/simplify-webchat` is misleading here; the implementation is not tightly coupled to the simplify-only `pinocchio` internals

No immediate `web-agent-example` code rollback is indicated by the current evidence.

## 4. `openai-app-server`

### What it uses

Repository-wide search did not find live source-code references to `pinocchio` webchat APIs under `openai-app-server` itself. The matches are in ticket docs under `ttmp/`, not application code.

That fits its own architecture docs, which describe `openai-app-server` as self-contained and not dependent on `pinocchio`.

### Impact

Expected runtime impact: none.

Expected documentation impact: medium.

Reason:

- no live code appears to consume `pinocchio`
- there are multiple workspace tickets describing the selector-contract cleanup, so ticket text may become stale or need reconciliation after the branch switch

## What Must Be Undone In The Local `pinocchio` Worktree

The nested workspace checkout at `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio` is still on `task/simplify-webchat` (`e93c449`), whose unique commit history is:

- `10caa7e` `refactor: collapse webchat chat service wrapper`
- `8221fec` `refactor: remove webchat newfromrouter constructor`
- `51053f0` `refactor: remove webchat alias subpackages`
- `7ab4beb` `refactor: remove webchat router utility mux api`
- `a091f2d` `docs: update webchat route help text`
- `e1ae805` `refactor: remove dead webchat middleware registration api`
- `5452ac8` `Remove legacy profile selector aliases from web chat`
- `e93c449` `Align debug UI with resolved runtime key payloads`

Relative to that list, the pushed branch already keeps the last three logical outcomes:

- alias packages are still deleted on the pushed branch
- legacy selector aliases are still removed on the pushed branch
- debug payloads still use `resolved_runtime_key` on the pushed branch

The simplify-only changes that would need to be backed out in the workspace checkout are:

1. `10caa7e` wrapper collapse
2. `8221fec` `NewFromRouter` removal
3. `7ab4beb` router utility mux removal
4. `e1ae805` middleware registration API removal

Why:

The pushed branch still contains these surfaces:

- `Server.RegisterMiddleware(...)` at [server.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/server.go#L58)
- `NewFromRouter(...)` at [server.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/server.go#L115)
- `Router.RegisterMiddleware(...)` at [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go#L145)
- `Router.Mount(...)` at [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go#L159)
- `Router.Handle(...)`, `HandleFunc(...)`, and `Handler()` at [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go#L173)
- `ChatService` remains a real wrapper with runner/idempotency behavior at [chat_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/chat_service.go#L23) and [chat_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/chat_service.go#L77)

This is the core compatibility conclusion:

- downstream workspace apps mostly align with the pushed branch
- the nested `pinocchio` simplify branch does not

## Documentation And Ticket Drift Inside The Workspace

Two documentation clusters inside the workspace would become historically misleading after switching the local `pinocchio` checkout:

### 1. `geppetto` ticket `PI-021`

This ticket records simplify-only structural removals as completed facts:

- `ChatService` wrapper collapse at [changelog.md](/home/manuel/workspaces/2026-03-02/os-openai-app-server/geppetto/ttmp/2026/03/06/PI-021-WEBCHAT-SERVICE-EXTRACTION--extract-a-tight-chat-service-and-remove-legacy-webchat-surface-area/changelog.md#L7)
- `NewFromRouter` removal at [changelog.md](/home/manuel/workspaces/2026-03-02/os-openai-app-server/geppetto/ttmp/2026/03/06/PI-021-WEBCHAT-SERVICE-EXTRACTION--extract-a-tight-chat-service-and-remove-legacy-webchat-surface-area/changelog.md#L8)
- router utility mux removal at [changelog.md](/home/manuel/workspaces/2026-03-02/os-openai-app-server/geppetto/ttmp/2026/03/06/PI-021-WEBCHAT-SERVICE-EXTRACTION--extract-a-tight-chat-service-and-remove-legacy-webchat-surface-area/changelog.md#L10)
- middleware registration removal at [changelog.md](/home/manuel/workspaces/2026-03-02/os-openai-app-server/geppetto/ttmp/2026/03/06/PI-021-WEBCHAT-SERVICE-EXTRACTION--extract-a-tight-chat-service-and-remove-legacy-webchat-surface-area/changelog.md#L13)

The task list also marks these as done at [tasks.md](/home/manuel/workspaces/2026-03-02/os-openai-app-server/geppetto/ttmp/2026/03/06/PI-021-WEBCHAT-SERVICE-EXTRACTION--extract-a-tight-chat-service-and-remove-legacy-webchat-surface-area/tasks.md#L6).

If the workspace moves to `task/unify-chat-backend`, those docs stop matching the actual local `pinocchio` checkout and should be annotated or superseded.

### 2. `openai-app-server` APP-08 / APP-09 docs

These are mostly in good shape. They already describe the selector/debug contract cutover that the pushed branch preserves:

- APP-08 summary says request selection uses `profile` / `registry`, legacy aliases are rejected, and debug payloads use `resolved_runtime_key` at [index.md](/home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/06/APP-08-PROFILE-RUNTIME-CONTRACT-ALIGNMENT--align-frontend-profile-selection-with-backend-runtime-contract/index.md#L15)
- APP-08 current state repeats that same conclusion at [index.md](/home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/06/APP-08-PROFILE-RUNTIME-CONTRACT-ALIGNMENT--align-frontend-profile-selection-with-backend-runtime-contract/index.md#L53)

So the contract-alignment docs are not the rollback risk. The structural-cleanup docs are.

## Impact Matrix

| Area | Current state | Impact if workspace adopts `task/unify-chat-backend` | Action |
| --- | --- | --- | --- |
| `os-openai-app-server/pinocchio` worktree | On `task/simplify-webchat` | High branch drift | Switch branch and back out simplify-only structural deletions |
| `go-go-os-chat` code | Uses stable `webchat.Server` + `webhttp` contracts | Low | No code undo expected |
| `wesen-os` code | Uses shared component and aligned profile/debug contract | Low to medium | Rebuild and re-run launcher/integration tests after branch switch |
| `web-agent-example` code | Uses stable handler-first API | Low | No code undo expected |
| `openai-app-server` code | No live `pinocchio` dependency found | None | No code action |
| `geppetto` PI-021 docs | Describe simplify-only removals as completed | Medium doc drift | Annotate or supersede after branch switch |
| `openai-app-server` APP-08 docs | Already aligned with kept contract changes | Low | Keep, no rollback needed |

## Recommended Switching Plan

1. Do not treat `git push` as the change event; treat the `os-openai-app-server/pinocchio` branch switch as the change event.
2. Switch `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio` from `task/simplify-webchat` to the pushed branch only when ready to validate the whole workspace.
3. Expect no consumer-code rollback in `go-go-os-chat`, `wesen-os`, or `web-agent-example` for the kept selector/debug changes.
4. Expect the local `pinocchio` simplify-only structural deletions to disappear after the switch.
5. Re-run the main verification surface in this order:
   - `go-go-os-chat`
   - `wesen-os`
   - `web-agent-example`
6. Follow with doc cleanup:
   - annotate or replace `geppetto` PI-021 conclusions
   - keep APP-08 / APP-09 as the authoritative contract history

## Bottom Line

If the workspace adopts the pushed `task/unify-chat-backend` branch, the biggest change is not downstream code breakage. The biggest change is that the workspace-local `pinocchio` checkout stops matching its current `task/simplify-webchat` narrative.

The selector-contract cleanup and debug payload rename are already compatible across the workspace. The rollback work is mostly:

- undo simplify-only structural deletions in the local `pinocchio` checkout,
- then clean up the workspace docs that claimed those deletions were final.

## References

- [go-go-os-chat request resolver](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/profilechat/request_resolver.go)
- [go-go-os-chat shared chat component](/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go)
- [wesen-os workspace overlay](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/go.work)
- [wesen-os assistant backend module](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/pkg/assistantbackendmodule/module.go)
- [wesen-os launcher integration tests](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/cmd/wesen-os-launcher/main_integration_test.go)
- [web-agent-example main](/home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example/cmd/web-agent-example/main.go)
- [current pushed branch server surface](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/server.go)
- [current pushed branch router surface](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go)
- [current pushed branch chat service surface](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/chat_service.go)
