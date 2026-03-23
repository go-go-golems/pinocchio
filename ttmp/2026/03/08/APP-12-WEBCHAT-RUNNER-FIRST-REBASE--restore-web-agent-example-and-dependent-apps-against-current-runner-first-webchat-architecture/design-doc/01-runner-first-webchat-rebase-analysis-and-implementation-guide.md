---
Title: Runner-first webchat rebase analysis and implementation guide
Ticket: APP-12-WEBCHAT-RUNNER-FIRST-REBASE
Status: active
Topics:
    - backend
    - webchat
    - runner
    - apps
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: go-go-os-chat/pkg/chatservice/component.go
      Note: Reusable chat embedding layer for downstream apps
    - Path: go-go-os-chat/pkg/profilechat/request_resolver.go
      Note: Strict profile and registry resolver aligned with the current contract
    - Path: go.work
      Note: Root workspace overlay restored multi-module local resolution
    - Path: pinocchio/pkg/webchat/chat_service.go
      Note: ChatService remains a real runner and queue boundary
    - Path: pinocchio/pkg/webchat/http/api.go
      Note: HTTP helper handler contracts used by downstream embedders
    - Path: pinocchio/pkg/webchat/http/profile_api.go
      Note: Profile API shape for the stricter app stack
    - Path: pinocchio/pkg/webchat/router.go
      Note: Deps-first router and ChatService wiring are preserved
    - Path: pinocchio/pkg/webchat/server.go
      Note: Server lifecycle surface proves chat and ws ownership stays in the app layer
    - Path: web-agent-example/cmd/web-agent-example/main.go
      Note: Minimal downstream embedder already targets the app-owned handler surface
    - Path: wesen-os/go.work
      Note: Nested workspace needed Go version alignment with pinocchio
    - Path: wesen-os/pkg/assistantbackendmodule/module.go
      Note: Assistant backend consumes go-go-os-chat rather than bypassing it
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T19:22:13.088156864-04:00
WhatFor: ""
WhenToUse: ""
---


# Runner-first webchat rebase analysis and implementation guide

## Executive Summary

The local breakage after updating `pinocchio` against `main` was primarily a workspace-composition regression, not an application-source regression. `web-agent-example`, `go-go-os-chat`, `wesen-os`, and the linked app modules were already written against the current app-owned handler model and the runner-first `ChatService` / `ConversationService` split described in the GP-031 playbook. What had drifted was the Go workspace layer that makes those sibling modules resolve against the local checkout rather than sparse, placeholder, or remote module versions.

The durable fix was therefore to restore a root `go.work`, align the nested `wesen-os/go.work` file to `go 1.26.1` to match the updated `pinocchio/go.mod`, and preserve the local `github.com/go-go-golems/go-go-os-chat v0.0.0` replacement semantics that the app stack already relied on. Once that was done, `web-agent-example`, `go-go-os-chat`, `wesen-os/pkg/assistantbackendmodule`, `go-go-app-inventory`, and `go-go-app-arc-agi-3` all compiled cleanly with no source changes in the apps themselves.

For a new intern, the most important lesson is this: the current `pinocchio` architecture is intentionally split. Do not “fix” downstream compile failures by collapsing `ChatService` back into `ConversationService`, by reviving older router-owned `/chat` or `/ws` behavior, or by removing the request/profile contract cleanup that has already been ported. The playbook explicitly says those simplify-webchat assumptions must stay dead.

## Problem Statement

After reverting parts of the earlier simplify-webchat changes and updating the local `pinocchio` checkout against the newer runner-first branch, the downstream app stack needed to compile again against the updated local code. The user specifically called out:

- `web-agent-example`
- `go-go-os-chat`
- “other apps which might need it,” which in practice meant at least the `wesen-os` assistant backend and linked app modules that embed `go-go-os-chat`

At first glance, the symptoms looked like ordinary application drift: missing packages, unresolved imports, and standalone build failures. The actual situation was more subtle:

1. `web-agent-example` is a sparse module that expects to live inside a multi-module workspace.
2. `wesen-os` already had its own nested workspace, but that workspace still claimed `go 1.25.7`.
3. The updated local `pinocchio/go.mod` now requires `go 1.26.1`.
4. `wesen-os` and `go-go-app-inventory` both depend on `github.com/go-go-golems/go-go-os-chat v0.0.0`, which only works locally when the workspace carries the correct local replacement semantics.

The design problem was therefore: restore local compilation against the new `pinocchio` branch without regressing the runner-first architecture and without making unnecessary source changes in the downstream apps.

## Scope

In scope:

- local multi-module workspace composition under `/home/manuel/workspaces/2026-03-02/os-openai-app-server`
- `web-agent-example`
- `go-go-os-chat`
- `wesen-os/pkg/assistantbackendmodule`
- linked app modules that depend on the same local overlay model
- documentation, diary, validation, and reMarkable delivery

Out of scope:

- redesigning `pinocchio` webchat core
- reintroducing simplify-webchat structural deletions rejected by the GP-031 playbook
- shipping new runtime behavior or endpoint semantics
- changing frontend contracts beyond what already exists in the current branch

## Current-State Architecture

### 1. The upstream architectural contract from GP-031

The playbook at `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/ttmp/2026/03/08/GP-031--assess-merge-conflicts-between-unify-chat-backend-and-simplify-webchat/playbooks/01-how-simplify-webchat-should-adapt-to-the-current-runner-first-webchat-architecture.md` states three facts that constrain all downstream work:

- Keep `ChatService` as a real boundary for prompt submission, queueing, idempotency, and runner orchestration.
- Keep `ConversationService` as the lower-level runtime/conversation service.
- Keep deps-first construction through `NewRouterFromDeps` and `NewServerFromDeps`.

The playbook also explicitly rejects these simplify-webchat-era moves:

- `type ChatService = ConversationService`
- removing `ChatService.StartPromptWithRunner(...)`
- removing `ChatService.NewLLMLoopRunner(...)`
- replacing deps-first router/server construction with older values-only constructors

That means downstream compilation fixes must adapt to the current branch, not “simplify” it back to an older surface.

### 2. `pinocchio` runner-first webchat core

The current core is split across several layers:

1. `Server`
2. `Router`
3. `ChatService`
4. HTTP helper handlers in `pkg/webchat/http`

#### 2.1 `Server` is lifecycle and transport composition, not route ownership

`pinocchio/pkg/webchat/server.go` shows that `Server` builds a router and an `http.Server`, then exposes services and helper handlers to the application layer. The comments at lines 20-31 and the methods at lines 65-104 show the intended embedding model:

- `Server` drives event router plus HTTP server lifecycle
- applications own `/chat` and `/ws` route registration
- the reusable surface is `ChatService()`, `StreamHub()`, `APIHandler()`, `UIHandler()`, `TimelineService()`, and `HTTPServer()`

That surface exists at:

- `server.go:20-31` for the role description
- `server.go:65-104` for the service/handler accessors

#### 2.2 `Router` is deps-first and only auto-registers UI/core utility APIs

`pinocchio/pkg/webchat/router.go` shows the other half of the contract:

- `NewRouterFromDeps(...)` is the real constructor (`router.go:52-140`)
- it builds the `ConversationService`, then wraps it with `ChatService` (`router.go:123-137`)
- it registers only UI and core API utilities (`router.go:209-227`)
- `APIHandler()` explicitly excludes app-owned `/chat` and `/ws` routes (`router.go:215-220`)

This is the structural reason downstream apps must mount their own transport handlers.

#### 2.3 `ChatService` is still a real runner/orchestration boundary

`pinocchio/pkg/webchat/chat_service.go` confirms the playbook’s warning. `ChatService` is not dead weight:

- it wraps `ConversationService` (`chat_service.go:21-39`)
- it owns `NewLLMLoopRunner()` (`chat_service.go:84-96`)
- it owns `StartPromptWithRunner(...)` and queue/idempotency handling (`chat_service.go:98-209`)

If a downstream build breaks, do not delete this layer. Callers should depend on it.

#### 2.4 HTTP helper handlers are deliberately app-friendly

`pinocchio/pkg/webchat/http/api.go` defines the interfaces and helpers used by downstream embedders:

- `ConversationRequestResolver` at `api.go:62-65`
- `NewChatHandler(...)` at `api.go:121-198`
- `NewWSHandler(...)` at `api.go:200-254`
- canonical request payload fields `profile`, `registry`, and legacy-selector rejection support in `ChatRequestBody` at `api.go:21-32`

This is the stable “handler-first” surface that `web-agent-example` and `go-go-os-chat` are already using.

### 3. `web-agent-example` is a minimal app-owned embedder

`web-agent-example/cmd/web-agent-example/main.go` is a good reference implementation for the smallest supported downstream embedder:

- It still calls `webchat.NewServer(...)` with a runtime composer (`main.go:65-69`).
- It creates a resolver and explicitly builds `chat`, `ws`, and timeline handlers (`main.go:74-84`).
- It mounts those handlers itself (`main.go:94-101`).
- It uses `srv.APIHandler()` and `srv.UIHandler()` rather than expecting router-owned chat routes (`main.go:100-101`).

This is important because it means `web-agent-example` was not broken by app code drift. It was broken because the module graph stopped resolving local sibling repos.

### 4. `go-go-os-chat` is the reusable app embedding layer

`go-go-os-chat/pkg/chatservice/component.go` is the reusable wrapper used by the broader app stack:

- it accepts a prebuilt `*webchat.Server` plus a `ConversationRequestResolver` (`component.go:28-40`)
- it mounts `/chat`, `/ws`, `/api/timeline`, and `/api/*` using the same helper surfaces as `web-agent-example` (`component.go:60-107`)
- it optionally adds profile APIs through `RegisterProfileAPIHandlers(...)` (`component.go:89-99`)

So the downstream stack is already converged on the current `pinocchio` embedding model.

### 5. `go-go-os-chat` also carries the stricter profile/registry resolver

`go-go-os-chat/pkg/profilechat/request_resolver.go` is the concrete evidence that the current stack already speaks the new request contract:

- chat bodies use `profile` and `registry` rather than treating `runtime_key` / `registry_slug` as normal inputs (`request_resolver.go:21-30`)
- legacy selectors are explicitly rejected (`request_resolver.go:131-132`, `request_resolver.go:253-260`)
- profile resolution goes through `ResolveEffectiveProfile(...)` on the registry (`request_resolver.go:230-250`)

That aligns with the GP-031 playbook’s “safe improvements to replay” and confirms that the downstream stack should stay on this contract.

### 6. `wesen-os` assistant backend is a consumer of `go-go-os-chat`

`wesen-os/pkg/assistantbackendmodule/module.go` uses `chatservice.New(...)` directly (`module.go:37-55`) and delegates route mounting to the reusable component (`module.go:87-95`). This means:

- if `go-go-os-chat` fails to compile against the local `pinocchio`, the assistant backend fails too
- once the workspace is repaired, the assistant backend benefits automatically

### 7. Workspace topology is part of the runtime architecture

There are two important workspace files:

- root workspace: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/go.work`
- nested `wesen-os` workspace: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/go.work`

The nested file already expressed the intended local overlay model:

```text
go 1.26.1
use (
    .
    ../geppetto
    ../go-go-os-chat
    ../pinocchio
    ./workspace-links/go-go-app-arc-agi-3
    ./workspace-links/go-go-app-inventory
    ./workspace-links/go-go-app-sqlite
    ./workspace-links/go-go-gepa
    ./workspace-links/go-go-os-backend
)
replace github.com/go-go-golems/go-go-os-chat v0.0.0 => ../go-go-os-chat
```

The repaired root workspace now mirrors those semantics for standalone sibling modules such as `web-agent-example`.

## Failure Analysis

### Symptom 1: `web-agent-example` could not resolve imports

Without a root workspace, `web-agent-example` behaved like an isolated sparse module. It immediately reported missing direct imports such as:

- `github.com/go-go-golems/clay/pkg`
- `github.com/go-go-golems/geppetto/pkg/sections`
- `github.com/go-go-golems/pinocchio/pkg/webchat`
- `github.com/go-go-golems/pinocchio/pkg/webchat/http`

This looked like an application problem, but it was actually a visibility problem: the module no longer saw the sibling repos that its development workflow expects.

### Symptom 2: `wesen-os` workspace version lag

Before the fix, `wesen-os/go.work` still declared `go 1.25.7` even though `pinocchio/go.mod` now requires `go 1.26.1`. That made the workspace internally invalid. The failure was not a compile error inside `assistantbackendmodule`; it was a workspace declaration mismatch.

### Symptom 3: partial root workspace still tried to fetch `go-go-os-chat v0.0.0`

After adding a first-pass root `go.work`, full module-graph walks still attempted to resolve `github.com/go-go-golems/go-go-os-chat@v0.0.0` remotely. The key clue was that `wesen-os/go.work` already had:

```text
replace github.com/go-go-golems/go-go-os-chat v0.0.0 => ../go-go-os-chat
```

The root workspace needed the same explicit override, because `wesen-os` and `go-go-app-inventory` depend on the placeholder version `v0.0.0`.

## Gap Analysis

### Gap 1: Missing umbrella workspace for standalone sibling modules

Observed state:

- standalone module `web-agent-example` had no parent `go.work`
- only nested `wesen-os/go.work` existed

Consequence:

- `web-agent-example` could not resolve the local sibling copies of `pinocchio`, `geppetto`, `go-go-os-chat`, and related repos as intended

### Gap 2: Nested workspace version drift

Observed state:

- `wesen-os/go.work` still declared `go 1.25.7`
- local `pinocchio/go.mod` required `go 1.26.1`

Consequence:

- the app workspace was invalid before any compile step even reached package type-checking

### Gap 3: Root workspace initially omitted linked app modules and local replace semantics

Observed state:

- the first root workspace fixed some resolution but not the full graph
- `go-go-os-chat v0.0.0` still needed explicit local replacement
- linked app modules were part of the actual local app topology already represented in `wesen-os/go.work`

Consequence:

- graph traversal still hit remote resolution for placeholder versions

### Non-gap: downstream application source

Important negative finding:

- `web-agent-example`
- `go-go-os-chat`
- `wesen-os/pkg/assistantbackendmodule`

did **not** require source changes to align with the current `pinocchio` architecture. They were already on the intended surface.

## Proposed Solution

### High-level proposal

Fix the workspace layer, not the app layer.

Concretely:

1. Create a root `go.work`.
2. Set its Go version to `1.26.1`.
3. Include the sibling modules used by the local app stack.
4. Include the linked `wesen-os` app modules.
5. Add `replace github.com/go-go-golems/go-go-os-chat v0.0.0 => ./go-go-os-chat`.
6. Update `wesen-os/go.work` to `go 1.26.1`.
7. Re-run compile validations across the app stack.

### Why this is the right fix

- It matches the already-working design encoded in `wesen-os/go.work`.
- It preserves the runner-first core exactly as the playbook requires.
- It fixes both standalone modules and nested app modules with one coherent local-overlay model.
- It avoids risky, unnecessary source churn in downstream apps.

## Diagrams

### 1. Build-graph picture before the fix

```text
web-agent-example
  |
  | no parent go.work
  v
isolated go.mod world
  |
  +--> cannot see local ../pinocchio
  +--> cannot see local ../geppetto
  +--> cannot see local ../go-go-os-chat
  |
  v
missing import / remote placeholder / invalid workspace errors
```

### 2. Build-graph picture after the fix

```text
root go.work
  |
  +--> geppetto
  +--> go-go-goja
  +--> go-go-os-chat
  +--> openai-app-server
  +--> pinocchio
  +--> web-agent-example
  +--> wesen-os
  +--> linked app modules
  |
  +--> replace github.com/go-go-golems/go-go-os-chat v0.0.0 => ./go-go-os-chat
  |
  v
standalone modules and app modules resolve against the same local checkout set
```

### 3. Request/route ownership picture

```text
Application code
  |
  +--> creates *webchat.Server
  +--> creates request resolver
  +--> mounts /chat, /ws, /api/timeline
  +--> optionally mounts profile APIs
  |
  v
webhttp.NewChatHandler / NewWSHandler / NewTimelineHandler
  |
  v
ChatService / StreamHub / TimelineService
  |
  v
ConversationService + Runner orchestration
```

## API Reference for Interns

### `pinocchio/pkg/webchat/server.go`

Use this when you need lifecycle plus access to reusable services.

Important methods:

- `NewServer(...)`
- `NewServerFromDeps(...)`
- `ChatService()`
- `StreamHub()`
- `APIHandler()`
- `UIHandler()`
- `TimelineService()`
- `HTTPServer()`
- `Run(ctx)`

### `pinocchio/pkg/webchat/router.go`

Use this when you are integrating with explicit infrastructure dependencies or embedding the event router into a host that already owns the HTTP server.

Important methods:

- `NewRouterFromDeps(...)`
- `BuildHTTPServer()`
- `APIHandler()`
- `UIHandler()`
- `RunEventRouter(ctx)`

### `pinocchio/pkg/webchat/chat_service.go`

Use this when you need prompt submission and runner orchestration.

Important methods:

- `ResolveAndEnsureConversation(...)`
- `PrepareRunnerStart(...)`
- `NewLLMLoopRunner()`
- `StartPromptWithRunner(...)`
- `SubmitPrompt(...)`

### `pinocchio/pkg/webchat/http/api.go`

Use this when building app-owned handlers.

Important types and functions:

- `ConversationRequestResolver`
- `ResolvedConversationRequest`
- `NewChatHandler(...)`
- `NewWSHandler(...)`
- `NewTimelineHandler(...)`
- `RequestResolutionError`

### `go-go-os-chat/pkg/chatservice/component.go`

Use this as the reusable embedding layer for host applications that do not want to rewire the handlers manually.

Important responsibilities:

- mount chat routes
- mount timeline route
- mount core API handler
- optionally mount profile API handler
- optionally mount plz-confirm backend

### `go-go-os-chat/pkg/profilechat/request_resolver.go`

Use this when the application needs the strict profile/registry contract and explicit rejection of legacy selector fields.

## Pseudocode

### Pseudocode: correct embedding pattern for a simple app

```go
deps := resolveInfra()
srv := webchat.NewServerFromDeps(ctx, deps,
    webchat.WithRuntimeComposer(appRuntimeComposer),
)

resolver := appResolver()

mux := http.NewServeMux()
mux.HandleFunc("/chat", webhttp.NewChatHandler(srv.ChatService(), resolver))
mux.HandleFunc("/ws", webhttp.NewWSHandler(srv.StreamHub(), resolver, upgrader))
mux.HandleFunc("/api/timeline", webhttp.NewTimelineHandler(srv.TimelineService(), logger))
mux.Handle("/api/", srv.APIHandler())
mux.Handle("/", srv.UIHandler())

srv.HTTPServer().Handler = mux
return srv.Run(ctx)
```

### Pseudocode: correct workspace recovery pattern

```text
if local app stack spans sibling modules:
    create or update root go.work
    include all sibling modules that participate in local development
    include linked app modules if they are part of the active workspace
    mirror any required local replace directives for placeholder versions
    align every nested go.work Go version with the highest required local module
    validate representative downstream builds
```

## Design Decisions

### Decision 1: Fix workspace composition before touching source

Rationale:

- source code already matched the intended app-owned handler pattern
- workspace regressions explained every observed failure more directly
- this was the smallest safe fix

### Decision 2: Mirror `wesen-os` local replacement semantics in the root workspace

Rationale:

- the nested `wesen-os` workspace already represented working local expectations
- placeholder `v0.0.0` requirements are not safe unless the workspace provides a local replacement

### Decision 3: Keep the fix architectural, not tactical

Rationale:

- changing downstream code would have obscured the real cause
- preserving the runner-first split matches the playbook and reduces future merge pain

## Alternatives Considered

### Alternative A: Add explicit `replace` directives to each downstream `go.mod`

Rejected because:

- it duplicates workspace concerns across multiple modules
- it is harder to maintain than a single root overlay
- it fights the existing local development model already present in `wesen-os/go.work`

### Alternative B: Rework `web-agent-example` to have a fully explicit standalone `go.mod`

Rejected because:

- it would be valid only for that single module
- it would not fix `go-go-os-chat`, `wesen-os`, or linked app modules
- it would hide the underlying workspace regression

### Alternative C: Reintroduce older simplify-webchat compatibility surfaces

Rejected because:

- GP-031 explicitly says not to do this
- the current branch intentionally keeps the runner-first `ChatService` boundary
- the apps already compile without those reversions once the workspace is correct

## Implementation Plan

### Phase 1: Capture the intended architecture

1. Read GP-031 playbook.
2. Confirm that `ChatService` remains a real orchestration boundary.
3. Confirm that apps own `/chat` and `/ws`.
4. Confirm that profile/registry request resolution now lives in app-layer resolvers.

### Phase 2: Reproduce the failures

1. Run `go build ./cmd/web-agent-example` in `web-agent-example`.
2. Run `go list ./pkg/assistantbackendmodule` in `wesen-os`.
3. Capture missing-module and Go-version mismatch evidence.

### Phase 3: Repair workspace composition

1. Add root `go.work`.
2. Include sibling repos and linked app modules.
3. Add the `go-go-os-chat v0.0.0` local replace.
4. Update `wesen-os/go.work` to `go 1.26.1`.

### Phase 4: Validate the actual app surface

1. Build `web-agent-example`.
2. Build `go-go-os-chat/pkg/...`.
3. Build `wesen-os/pkg/assistantbackendmodule`.
4. Build representative linked apps.

### Phase 5: Document for future interns

1. Record the exact failure modes in the diary.
2. Explain why the fix was at the workspace layer.
3. Provide a reproducible validation script.
4. Relate the relevant files in docmgr.

## Validation Strategy

Run this validation set:

```bash
cd /home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example
go build ./cmd/web-agent-example

cd /home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat
go build ./pkg/...

cd /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os
go build ./pkg/assistantbackendmodule

cd /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-app-inventory
go build ./...

cd /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-app-arc-agi-3
go build ./...
```

Or rerun the ticket helper:

```bash
/home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/08/APP-12-WEBCHAT-RUNNER-FIRST-REBASE--restore-web-agent-example-and-dependent-apps-against-current-runner-first-webchat-architecture/scripts/validate-workspace-builds.sh
```

## Risks, Tradeoffs, and Open Questions

### Risks

- Future local modules may be added to the workspace without updating the root `go.work`.
- The root workspace may grow broad enough that some developers prefer a narrower local overlay.

### Tradeoffs

- A broad root workspace is convenient and matches the current local app stack, but it is another piece of configuration that must stay synchronized.
- A narrow workspace would reduce maintenance burden but would make standalone downstream builds less representative of the local multi-repo development environment.

### Open questions

- Should the root `go.work` become the canonical developer entrypoint for the whole repository, or should it remain a lightweight convenience layer over more specialized nested workspaces?
- Are there any other linked modules not currently in scope that should be pulled into the root `use` list?

## Evidence Summary

### Workspace evidence

- Root workspace now composes all relevant sibling modules and linked app modules: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/go.work:1-18`
- Nested `wesen-os` workspace now matches `pinocchio`’s Go version and still carries the local `go-go-os-chat` replacement: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/go.work:1-15`

### Core architectural evidence

- `Server` is an app-composed lifecycle wrapper and does not own `/chat` or `/ws`: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/server.go:20-31`
- `Router` builds `ConversationService`, wraps it in `ChatService`, and keeps app-owned transport routes out of `APIHandler()`: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/router.go:52-140`, `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/router.go:215-220`
- `ChatService` still owns runner orchestration and queue/idempotency behavior: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/chat_service.go:21-39`, `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/chat_service.go:84-166`
- HTTP helper handlers expect app-provided request resolution and service surfaces: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/http/api.go:62-65`, `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/http/api.go:121-254`

### Downstream consumer evidence

- `web-agent-example` already uses the stable handler-first server surface: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example/cmd/web-agent-example/main.go:65-123`
- `go-go-os-chat` already wraps the current `pinocchio` surface correctly: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go:60-107`
- `go-go-os-chat` strict resolver already matches the newer `profile` / `registry` contract: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/profilechat/request_resolver.go:21-30`, `/home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/profilechat/request_resolver.go:131-170`
- `wesen-os` assistant backend consumes `go-go-os-chat` rather than bypassing it: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/pkg/assistantbackendmodule/module.go:37-55`

## References

- GP-031 playbook: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/ttmp/2026/03/08/GP-031--assess-merge-conflicts-between-unify-chat-backend-and-simplify-webchat/playbooks/01-how-simplify-webchat-should-adapt-to-the-current-runner-first-webchat-architecture.md:56-71`
- Root workspace: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/go.work`
- Nested workspace: `/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/go.work`
- Validation script: `../scripts/validate-workspace-builds.sh`

## Proposed Solution

<!-- Describe the proposed solution in detail -->

## Design Decisions

<!-- Document key design decisions and rationale -->

## Alternatives Considered

<!-- List alternative approaches that were considered and why they were rejected -->

## Implementation Plan

<!-- Outline the steps to implement this design -->

## Open Questions

<!-- List any unresolved questions or concerns -->

## References

<!-- Link to related documents, RFCs, or external resources -->
