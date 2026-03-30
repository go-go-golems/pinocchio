---
Title: Diary
Ticket: PI-04-AGENTMODE-DEFAULT-WEBCHAT
Status: active
Topics:
    - pinocchio
    - webchat
    - agentmode
    - profiles
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Detailed implementation diary for PI-04, covering the backend default-on runtime overlay, stack-aware app runtime merge, builtin agent_mode renderer, validation commands, and commit checkpoints.
LastUpdated: 2026-03-30T14:31:00-04:00
WhatFor: Capture the exact implementation steps, commands, findings, and review notes for the PI-04 rollout.
WhenToUse: Use when reviewing the implementation, reconstructing why the merge logic was built this way, or continuing follow-up work on web-chat runtime policy and agent mode UI.
---

# Diary

## Goal

Capture the exact step-by-step implementation of PI-04 so a reviewer or future maintainer can see what changed, why it changed, what commands validated it, and which parts of the code deserve a careful second pass.

## Context

This ticket followed the earlier agentmode structuredsink rollout. The remaining gaps were: a dead `--enable-agentmode` CLI flag, leaf-only app runtime resolution in `/chat`, and missing dedicated UI rendering for `agent_mode` entities in web-chat.

## Step 1: Make web-chat agentmode default-on in backend runtime resolution

This step removed the misleading CLI flag and changed web-chat runtime resolution so `agentmode` is part of the default app-owned runtime even when no profile runtime exists. The critical design change was making app runtime merging stack-aware instead of reading only the selected leaf profile extension.

The resulting backend behavior is consistent across the conversation plan, middleware composition, and agentmode structuredsink wrapper because they now all consume the same effective runtime. This step was committed as `9d30b0d` with message `Make web-chat agentmode default-on`.

### Prompt Context

**User prompt (verbatim):** "alright now implement task by task, committing at appropriate intervals, keeping a detailed diary"

**Assistant interpretation:** Implement PI-04 incrementally, commit each coherent slice, and keep the ticket docs synchronized with a detailed engineering diary.

**Inferred user intent:** Turn the ticket from a design artifact into finished code and leave a continuation-quality record behind for future reviewers.

**Commit (code):** `9d30b0d` — `Make web-chat agentmode default-on`

### What I did

- Removed the dead `--enable-agentmode` flag from `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/main.go`.
- Updated `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go` to return a default runtime even when no profile registry or resolved profile is present.
- Added `defaultWebChatProfileRuntime()` with a default `agentmode` middleware entry.
- Added stack-aware runtime merge helpers that replay `profile.stack.lineage` from the resolved Geppetto profile metadata and fetch each stacked profile’s `pinocchio.webchat_runtime@v1` extension.
- Implemented merge rules for system prompt, middleware list, and tools.
- Added resolver tests in `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy_test.go`.
- Ran `gofmt -w ...` on the touched Go files.
- Ran `go test ./cmd/web-chat -count=1`.
- Let the repository pre-commit hooks run on commit, which executed `go generate ./...`, frontend build, `go build ./...`, `golangci-lint`, `go vet`, and `go test ./...`.

### Why

- The CLI flag was misleading because activation had already moved to runtime policy.
- A default-on behavior implemented as runtime overlay is app policy and belongs in web-chat runtime resolution rather than in registry file mutation or middleware internals.
- Replaying the stack lineage was necessary because `ResolvedEngineProfile` exposes merged inference settings but not merged app-owned runtime extensions.

### What worked

- `profile.stack.lineage` from Geppetto resolution provided enough information to reconstruct the runtime stack with `GetEngineProfile(...)`.
- Middleware merge by normalized `(name, id)` gave a clean way for per-profile `enabled: false` or config overrides to replace the default `agentmode` entry.
- The no-profile path now resolves a stable runtime with key `default`, which made the overlay logic consistent instead of nil-fragile.

### What didn't work

- Initial assumption: overlaying only the leaf profile runtime would be sufficient. That was wrong once I re-read the user request about handling merge base + profiles from registry + app-specific profiles.
- There was no existing app-runtime merge helper in the codebase, so the merge logic had to be added explicitly in `profile_policy.go`.

### What I learned

- The Geppetto profile service already records stack lineage in resolved metadata, which is enough to make Pinocchio’s app-owned runtime policy stack-aware without changing the registry interface.
- Web-chat without profile registries effectively needed a default runtime anyway; the overlay work exposed and fixed that missing default behavior.

### What was tricky to build

- The tricky part was not adding a default middleware entry. The tricky part was making runtime resolution honor profile inheritance instead of reintroducing a new leaf-only policy path. The symptom was that a naive implementation would have made `agentmode` default-on but still ignored app-owned runtime extensions on base profiles. I addressed that by parsing `profile.stack.lineage`, replaying the stack from base to leaf, and merging each runtime extension in order.

### What warrants a second pair of eyes

- Review the chosen tool-merge semantics. Tools are now merged as an ordered unique union from base to leaf because the runtime model does not currently expose a “remove inherited tool” mechanism.
- Review whether middleware merge by `(name, id)` is the right long-term identity rule if multiple same-name middlewares with different IDs become common.

### What should be done in the future

- If Pinocchio starts stacking more complex app-owned runtime policy, consider moving the merge helpers into a reusable runtime package instead of leaving them local to `profile_policy.go`.

### Code review instructions

- Start in `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go`.
- Review `defaultWebChatProfileRuntime`, `runtimeStackRefsFromResolvedProfile`, and `mergeWebChatProfileRuntime`.
- Check `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy_test.go` for the no-profile and stacked-profile coverage.
- Validate with `go test ./cmd/web-chat -count=1`.

### Technical details

- Default registry source reference: `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/sections/profile_sections.go`
- Resolved stack lineage source: `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/engineprofiles/service.go`
- Runtime extension model: `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/inference/runtime/profile_runtime.go`

## Step 2: Add a dedicated agent_mode renderer in web-chat

This step made the existing `agent.mode` SEM path visible as a first-class chat timeline card instead of a generic JSON fallback. Because `agent.mode` is now a core web-chat event rather than an optional extension feature, I implemented the renderer as a builtin chat renderer instead of adding another registration seam.

This step was committed as `0da28f9` with message `Add web-chat agent mode renderer`.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the UI side of the ticket with the same incremental, well-documented approach.

**Inferred user intent:** Ensure the backend behavior is visible and usable in the web-chat timeline instead of remaining a state-only projection.

**Commit (code):** `0da28f9` — `Add web-chat agent mode renderer`

### What I did

- Added `AgentModeCard` to `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`.
- Registered `agent_mode` as a builtin renderer in `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts`.
- Added a frontend assertion in `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/sem/registry.test.ts` proving `agent.mode` still projects to an `agent_mode` entity with the expected payload.
- Added `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.test.ts` to prove the builtin renderer exists.
- Updated `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.stories.tsx` so the `WidgetOnlyAgentMode` story uses a realistic `{from,to,analysis}` payload.
- Ran `npx vitest run src/sem/registry.test.ts src/webchat/rendererRegistry.test.ts`.
- Ran `npm run typecheck`.
- Ran `npm run build`.
- Let the staged-frontend pre-commit hook run `npm run typecheck && npm run lint`.

### Why

- The SEM backend path already existed and was sufficient.
- A builtin renderer is lower-friction than an extension registration path for an event kind that now belongs to the default web-chat surface.
- Updating the story made the new card easy to review visually.

### What worked

- The existing `agent.mode` protobuf payload already had the fields needed for a useful UI: title plus structured data.
- The current chat card system was simple to extend because all builtin cards already live in `cards.tsx` and are selected by `kind`.

### What didn't work

- N/A

### What I learned

- No frontend SEM registry changes were needed beyond preserving the existing projection; the missing functionality was purely renderer coverage.
- `ChatWidget.stories.tsx` already had the right fixture slot (`WidgetOnlyAgentMode`), so the story update was a refinement rather than a new test harness.

### What was tricky to build

- The main choice was architectural rather than technical: whether to add a new feature-module registration path or make `agent_mode` builtin. I chose builtin because the event is no longer optional in the default runtime, and reducing registration seams lowers the chance of state/render drift.

### What warrants a second pair of eyes

- Review whether the card should eventually show richer status language than the raw title plus `from`/`to` pills.
- Review whether any styling adjustments are needed once this appears alongside real conversation traffic instead of isolated Storybook data.

### What should be done in the future

- If agentmode starts emitting richer structured data, consider moving the extra-data JSON block to a more tailored presentation instead of a raw JSON fallback.

### Code review instructions

- Start in `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`.
- Then review `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts`.
- Confirm the projection remains unchanged in `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/sem/registry.ts`.
- Validate with:
  - `npx vitest run src/sem/registry.test.ts src/webchat/rendererRegistry.test.ts`
  - `npm run typecheck`
  - `npm run build`

### Technical details

- Renderer card file: `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`
- Builtin renderer registry: `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts`
- Story fixture: `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.stories.tsx`

## Quick Reference

### Per-profile opt-out shape

```yaml
extensions:
  pinocchio.webchat_runtime@v1:
    middlewares:
      - name: agentmode
        enabled: false
```

### Commands run

```bash
go test ./cmd/web-chat -count=1
npx vitest run src/sem/registry.test.ts src/webchat/rendererRegistry.test.ts
npm run typecheck
npm run build
```

## Usage Examples

- Review backend runtime merge:
  Start with `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go`
- Review frontend renderer:
  Start with `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`
- Reproduce the dedicated card visually:
  Open the `WidgetOnlyAgentMode` story in Storybook

## Related

- `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/ttmp/2026/03/30/PI-04-AGENTMODE-DEFAULT-WEBCHAT--make-web-chat-agentmode-default-on-via-runtime-overlay-and-add-renderer/design-doc/01-intern-guide-to-making-web-chat-agentmode-default-on-with-profile-opt-out-and-renderer-support.md`
- `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/ttmp/2026/03/30/PI-04-AGENTMODE-DEFAULT-WEBCHAT--make-web-chat-agentmode-default-on-via-runtime-overlay-and-add-renderer/reference/01-investigation-notes-for-default-on-web-chat-agentmode-and-renderer-enablement.md`
