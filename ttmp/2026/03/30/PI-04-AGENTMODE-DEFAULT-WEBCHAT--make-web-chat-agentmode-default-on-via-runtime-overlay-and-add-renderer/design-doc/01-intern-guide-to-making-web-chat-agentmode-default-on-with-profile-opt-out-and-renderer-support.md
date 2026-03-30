---
Title: Intern guide to making web-chat agentmode default-on with profile opt-out and renderer support
Ticket: PI-04-AGENTMODE-DEFAULT-WEBCHAT
Status: active
Topics:
    - pinocchio
    - webchat
    - agentmode
    - profiles
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/sections/profile_sections.go
      Note: Default profile registry source fallback
    - Path: pinocchio/cmd/web-chat/main.go
      Note: Declares the dead enable-agentmode flag and constructs the web-chat server
    - Path: pinocchio/cmd/web-chat/profile_policy.go
      Note: Chat endpoint runtime resolution and best merge point for default runtime overlay
    - Path: pinocchio/cmd/web-chat/runtime_composer.go
      Note: Consumes resolved runtime middlewares to build the engine middleware chain
    - Path: pinocchio/pkg/inference/runtime/profile_runtime.go
      Note: Defines MiddlewareUse and the pinocchio.webchat_runtime@v1 extension
    - Path: pinocchio/cmd/web-chat/web/src/sem/registry.ts
      Note: Maps agent.mode SEM frames to agent_mode entities
    - Path: pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts
      Note: Shows that no agent_mode renderer is currently registered
ExternalSources: []
Summary: Detailed intern guide for removing the dead web-chat enable-agentmode flag, making agentmode default-on via app-owned runtime overlay with per-profile enabled:false opt-out, and adding a dedicated web-chat renderer for agent_mode entities. Updated with the final implementation in commits 9d30b0d and 0da28f9.
LastUpdated: 2026-03-30T14:31:00-04:00
WhatFor: Understand how the web-chat chat endpoint currently resolves base settings, registry profiles, and app-owned runtime policy, then implement a safe default-on agentmode overlay and a frontend renderer.
WhenToUse: Use when implementing or reviewing the PI-04 follow-up work after the initial structuredsink-based agentmode rollout.
---

# Intern guide to making web-chat agentmode default-on with profile opt-out and renderer support

## Executive Summary

This ticket is a follow-up cleanup and productization task for Pinocchio web-chat agentmode. The current state is partly complete: `agentmode` exists, its YAML parsing has already been refactored to use a shared sanitize-backed parser, the web-chat event sink now supports agentmode structured extraction, and `EventAgentModeSwitch` already becomes an `agent.mode` SEM frame and then an `agent_mode` entity in frontend state. However, activation is still inconsistent and somewhat confusing. A CLI flag named `--enable-agentmode` still exists even though actual activation is profile-runtime driven, and web-chat does not yet make agentmode a default middleware across effective runtime resolution. In addition, the frontend does not register a dedicated `agent_mode` renderer, so these entities likely fall back to the generic timeline card.

The recommended solution is to remove the dead flag, introduce a default app-owned web-chat runtime overlay that injects `agentmode` into every resolved runtime, preserve per-profile `enabled: false` as the explicit opt-out mechanism, and add a dedicated `agent_mode` renderer in the web-chat frontend. The important architectural rule is that this should be implemented as runtime-policy merging, not by mutating registry data on disk and not by introducing special-case activation logic inside the middleware itself.

## Implementation Update

This design was implemented in two code commits:

- `9d30b0d` `Make web-chat agentmode default-on`
- `0da28f9` `Add web-chat agent mode renderer`

The final implementation differs from the early sketch in one important and useful way: the runtime merge is not only “default runtime plus leaf profile runtime.” It now replays the resolved profile stack using `profile.stack.lineage` metadata from Geppetto profile resolution, so Pinocchio’s app-owned runtime policy follows the same inheritance chain as the profile stack rather than collapsing to a leaf-only read.

The final merge rules are:

- Start from a default web-chat runtime containing one middleware entry: `agentmode`.
- Walk the resolved profile stack in base-to-leaf order using `profile.stack.lineage`.
- For each profile that carries `pinocchio.webchat_runtime@v1`, merge it into the effective runtime.
- System prompt: last non-empty value wins.
- Middlewares: merge by normalized `(name, id)` key; later layers replace earlier layers with the same key.
- Tools: ordered unique union from base to leaf.

That means a profile-level middleware entry such as:

```yaml
extensions:
  pinocchio.webchat_runtime@v1:
    middlewares:
      - name: agentmode
        enabled: false
```

still cleanly disables the default middleware, while profile-specific config such as `sanitize_yaml: false` overrides the default-on entry instead of creating a duplicate.

## Problem Statement

The user wants three concrete things:

1. Remove the dead `--enable-agentmode` flag.
2. Enable agentmode across web-chat profiles by default, while still allowing profile-level opt-out.
3. Enable the renderer so agent mode switch entities have an intentional UI presentation.

The current codebase falls short in three ways.

First, activation semantics are misleading. `cmd/web-chat/main.go` still defines `--enable-agentmode`, but the actual middleware composition path does not use that flag. Instead, web-chat activation comes from the app-owned runtime extension `pinocchio.webchat_runtime@v1`, specifically the `middlewares` list. That means the CLI suggests a behavior that the server does not actually implement.

Second, web-chat does not currently merge a default app-owned runtime policy with the selected profile runtime policy. The `/chat` endpoint resolves inference settings through the registry stack and merges them with base settings, but app-owned runtime policy is only read from the selected engine profile’s runtime extension. In practice, that means a profile must explicitly contain an `agentmode` middleware entry or agentmode will not be active. There is no default-on web-chat runtime overlay yet.

Third, the frontend already stores `agent_mode` entities but does not appear to register a dedicated renderer for them. As a result, the agent mode switch path exists semantically, but it does not have a polished user-facing widget in the chat timeline.

## Proposed Solution

The proposed solution has three coordinated parts.

### Part 1: Remove the dead CLI flag

Delete the `--enable-agentmode` flag from [`pinocchio/cmd/web-chat/main.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/main.go). No runtime behavior should depend on it, because it is already superseded by profile runtime policy.

### Part 2: Add a default web-chat runtime overlay

Introduce a small app-owned default runtime policy for web-chat that always includes:

```yaml
middlewares:
  - name: agentmode
```

This default should be merged with the resolved profile runtime before the conversation plan is finalized. The best current insertion point is inside [`buildConversationPlan(...)`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go#L356) or immediately after [`resolveProfileRuntime(...)`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go#L398) returns.

The merge semantics should be:

- If the profile has no `agentmode` entry, inject the default one.
- If the profile has an `agentmode` entry with `enabled: false`, do not re-add it.
- If the profile has an `agentmode` entry with config, keep that config and treat it as overriding the default entry.
- If future default config is added, merge default config first and profile config second.

The resulting effective runtime should then feed both:

- the middleware chain via [`runtime_composer.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/runtime_composer.go)
- the structuredsink wrapper via [`agentmode_sink_wrapper.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/agentmode_sink_wrapper.go)

### Part 3: Add a dedicated frontend renderer

The backend already emits `agent.mode`, and the frontend already converts that into `kind: 'agent_mode'` entities. What is missing is a renderer registration in [`rendererRegistry.ts`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts). Add a dedicated `agent_mode` card and register it so the chat timeline renders agent mode switches intentionally rather than using the generic fallback.

### High-level data flow

```text
CLI/config base settings
  + registry profile resolution
  -> resolved inference settings

registry leaf profile
  -> pinocchio.webchat_runtime@v1
  + default web-chat runtime overlay
  -> effective web-chat runtime

effective web-chat runtime
  -> middleware composition
  -> agentmode structuredsink wrapper config

agentmode switch event
  -> SEM frame: agent.mode
  -> frontend entity: agent_mode
  -> dedicated renderer
```

## Design Decisions

### Decision 1: Default-on should be implemented as runtime overlay, not as registry mutation

This change should happen in code, not by editing every registry profile in place. The reason is that “default-on for web-chat” is application policy, not source-of-truth registry data. Profiles should still be able to opt out, but the default behavior belongs to the app-owned web-chat runtime merge layer.

### Decision 2: `enabled: false` lives on the middleware entry in `pinocchio.webchat_runtime@v1`

This is already the right schema. `MiddlewareUse` includes `Enabled *bool` in [`profile_runtime.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/inference/runtime/profile_runtime.go), runtime composition already respects that field, and the agentmode sink wrapper already skips disabled entries. No new opt-out field is needed.

### Decision 3: Use the same effective runtime for middleware composition and sink wrapping

The current agentmode sink wrapper inspects `ResolvedProfileRuntime.Middlewares`. If we add a default-on overlay for middleware composition but forget to apply the same effective runtime to the sink wrapper input, we will create drift. The effective runtime must be built once and reused consistently.

### Decision 4: No backend SEM redesign is needed for this ticket

`EventAgentModeSwitch` already maps to `agent.mode`, and `agent.mode` already becomes `agent_mode` entities. The gap is the renderer, not the SEM contract.

## Alternatives Considered

### Alternative A: Keep the CLI flag and wire it up for real

Rejected. That would create two activation mechanisms:

- CLI flag
- profile runtime middleware list

That split would be confusing and would not solve the “default-on across profiles” requirement cleanly.

### Alternative B: Edit every profile in the registry to add agentmode

Rejected as the main strategy. It is brittle, source-specific, and pushes app policy into profile data. It also does not help when a user switches profile registries or relies on the default `~/.config/pinocchio/profiles.yaml` fallback.

### Alternative C: Make the middleware self-enabling

Rejected. Middleware activation belongs to runtime-policy composition, not to middleware internals.

### Alternative D: Leave renderer fallback as-is

Possible, but not recommended. The entity already exists and deserves an intentional card, especially if agentmode becomes default-on and therefore more visible.

## Implementation Plan

### Phase 1: Clean up activation semantics

1. Remove `--enable-agentmode` from `cmd/web-chat/main.go`.
2. Update any README or help text that still implies a CLI flag controls activation.
3. Add tests proving runtime behavior does not depend on that flag.

### Phase 2: Add the default runtime overlay

1. Introduce a helper that produces the default web-chat runtime policy.
2. Add a merge helper for app-owned runtime policy:
   - default runtime
   - resolved profile stack runtime extensions
   - later stack layers win
   - explicit `enabled: false` remains respected
3. Use the merged effective runtime in `buildConversationPlan(...)`.
4. Ensure the effective runtime fingerprint includes the post-merge middleware list.

Suggested pseudocode:

```go
func defaultWebChatRuntime() *infruntime.ProfileRuntime {
    return &infruntime.ProfileRuntime{
        Middlewares: []infruntime.MiddlewareUse{
            {Name: "agentmode"},
        },
    }
}

func resolveProfileRuntime(ctx context.Context, resolved *ResolvedEngineProfile) (*infruntime.ProfileRuntime, error) {
    runtime := defaultWebChatRuntime()
    for _, ref := range runtimeStackRefsFromResolvedProfile(resolved) {
        profile := registry.GetEngineProfile(ctx, ref.registrySlug, ref.profileSlug)
        profileRuntime := ProfileRuntimeFromEngineProfile(profile)
        runtime = mergeWebChatRuntime(runtime, profileRuntime)
    }
    return runtime, nil
}
```

### Phase 3: Keep sink wrapper behavior aligned

1. Ensure `ResolvedProfileRuntime` exposed to the conversation runtime uses the merged runtime, not the raw leaf extension.
2. Re-run or add tests for `agentmode_sink_wrapper.go` to confirm default-on and opt-out behavior.

### Phase 4: Add the frontend renderer

1. Add an `AgentModeCard` component to [`cards.tsx`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/cards.tsx).
2. Register it as a builtin `agent_mode` renderer in [`rendererRegistry.ts`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts).
3. Add a Storybook story or fixture to make the UI testable in isolation.

The final renderer shows:

- the switch title
- `from` and `to` pills when present
- Markdown-rendered `analysis`
- any remaining data payload as JSON

### Phase 5: Update docs

1. Update web-chat profile docs to show:
   - default-on behavior
   - explicit opt-out with `enabled: false`
   - where default profile registries load from
2. Update the ticket diary/tasks/changelog as implementation progresses.

## Open Questions

Open but bounded questions:

- Should the default runtime overlay live in `profile_policy.go` or in a reusable runtime helper package?
- Should the merge helper deduplicate middleware purely by normalized `Name`, or should `ID` also matter if multiple entries with the same name are ever allowed?
- Should the frontend renderer be minimal and informational, or should it expose richer switch metadata like analysis text by default?

## References

Primary references:

- [`pinocchio/cmd/web-chat/profile_policy.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go)
- [`pinocchio/cmd/web-chat/runtime_composer.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/runtime_composer.go)
- [`pinocchio/pkg/inference/runtime/profile_runtime.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/inference/runtime/profile_runtime.go)
- [`pinocchio/cmd/web-chat/main.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/main.go)
- [`pinocchio/cmd/web-chat/web/src/sem/registry.ts`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/sem/registry.ts)
- [`pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts)
- [`geppetto/pkg/sections/profile_sections.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/sections/profile_sections.go)

Documentation and code references that materially informed the implementation:

- [`geppetto/pkg/sections/profile_sections.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/sections/profile_sections.go)
  This established the default profile-registry source fallback: `~/.config/pinocchio/profiles.yaml`.
- [`geppetto/pkg/engineprofiles/service.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/engineprofiles/service.go)
  This showed that `ResolveEngineProfile(...)` emits `profile.stack.lineage`, and that the resolved profile itself is the last stack layer.
- [`geppetto/pkg/engineprofiles/registry.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/engineprofiles/registry.go)
  This clarified what `ResolvedEngineProfile` does and does not contain, which is why runtime extension merging had to be reconstructed from lineage metadata plus `GetEngineProfile(...)`.
- [`pinocchio/pkg/inference/runtime/profile_runtime.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/inference/runtime/profile_runtime.go)
  This defined the app-owned runtime extension model and confirmed that `Enabled *bool` already existed for per-profile opt-out.
- [`pinocchio/cmd/web-chat/runtime_composer.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/runtime_composer.go)
  This confirmed that middleware composition already honors `Enabled`, so no special-case middleware logic was necessary.
- [`pinocchio/cmd/web-chat/agentmode_sink_wrapper.go`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/agentmode_sink_wrapper.go)
  This confirmed that the structuredsink wrapper reads the same resolved runtime and already respects disabled middleware entries.
- [`pinocchio/cmd/web-chat/web/src/sem/registry.ts`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/sem/registry.ts)
  This showed that `agent.mode` to `agent_mode` projection already existed and that no backend SEM contract change was needed.
- [`pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/cards.tsx)
  This set the visual and structural pattern for adding a builtin card instead of inventing a separate rendering pipeline.
- [`pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.stories.tsx`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.stories.tsx)
  This already contained a minimal `WidgetOnlyAgentMode` scenario and served as the right fixture to upgrade once the renderer existed.
