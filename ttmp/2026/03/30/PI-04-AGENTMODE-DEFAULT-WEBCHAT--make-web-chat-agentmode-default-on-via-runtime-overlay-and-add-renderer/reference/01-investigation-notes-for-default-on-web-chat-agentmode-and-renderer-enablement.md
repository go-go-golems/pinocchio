---
Title: Investigation notes for default-on web-chat agentmode and renderer enablement
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
RelatedFiles:
    - Path: pinocchio/cmd/web-chat/profile_policy.go
      Note: Exact /chat endpoint request-resolution flow
    - Path: pinocchio/cmd/web-chat/runtime_composer.go
      Note: Middleware composition consumes resolved runtime
    - Path: geppetto/pkg/sections/profile_sections.go
      Note: Default profile registry source fallback
    - Path: pinocchio/cmd/web-chat/web/src/sem/registry.ts
      Note: agent.mode SEM to agent_mode entity mapping
    - Path: pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts
      Note: No built-in agent_mode renderer registration
ExternalSources: []
Summary: Quick reference for where web-chat currently loads profiles from, how the chat endpoint merges inference settings but not app-owned runtime policy, where agentmode activation really happens, and where the frontend renderer gap lives.
LastUpdated: 2026-03-30T14:14:08.557884467-04:00
WhatFor: Give implementers a copy/paste-ready map of the exact files and merge points needed to make agentmode default-on in web-chat.
WhenToUse: Use when implementing PI-04 or reviewing whether web-chat already has the right runtime merge semantics and renderer registrations.
---

# Investigation notes for default-on web-chat agentmode and renderer enablement

## Goal

Capture the exact current-state answers to these questions:

- Where does web-chat actually enable `agentmode`?
- Where do profiles come from by default?
- How does the `/chat` endpoint merge base settings, resolved profile settings, and app-owned runtime policy?
- Where should per-profile `enabled: false` live?
- Does web-chat already have SEM entities and a renderer for agent mode switches?

## Context

This ticket is a follow-up to the earlier structuredsink and sanitize-backed agentmode work. The backend activation path is now runtime-policy based, but the surrounding activation story is still inconsistent. The CLI still exposes a dead flag, the chat endpoint does not currently apply a default runtime overlay, and the frontend renderer path is incomplete.

## Quick Reference

### Actual activation path today

- Declared but effectively dead CLI flag:
  - [`pinocchio/cmd/web-chat/main.go:95`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/main.go#L95)
- Real middleware definition:
  - [`pinocchio/cmd/web-chat/middleware_definitions.go:98`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/middleware_definitions.go#L98)
- Real sink-wrapper activation from resolved runtime:
  - [`pinocchio/cmd/web-chat/agentmode_sink_wrapper.go:24`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/agentmode_sink_wrapper.go#L24)

### Default profile registry source

If `--profile-registries` is absent, Geppetto falls back to:

```text
~/.config/pinocchio/profiles.yaml
```

Evidence:

- [`geppetto/pkg/sections/profile_sections.go:55`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/sections/profile_sections.go#L55)
- [`geppetto/pkg/sections/profile_sections.go:215`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/sections/profile_sections.go#L215)

### `/chat` endpoint merge behavior today

The current `/chat` request path is:

```text
resolveChat(req)
  -> resolveProfileSelection(...)
  -> resolveRegistrySelection(...)
  -> resolveEffectiveProfile(...)
  -> buildConversationPlan(...)
       -> resolveProfileRuntime(...)
       -> resolvedInferenceSettingsForRequest(...)
```

Evidence:

- [`profile_policy.go:235`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go#L235)
- [`profile_policy.go:309`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go#L309)
- [`profile_policy.go:356`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go#L356)
- [`profile_policy.go:398`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go#L398)

Current behavior split:

- Inference settings:
  - base + resolved profile inference settings are merged
- App-owned runtime policy:
  - only the selected profile’s `pinocchio.webchat_runtime@v1` extension is read
  - there is no default runtime overlay merge yet

### Where `enabled: false` belongs

It should live on the middleware entry inside the app-owned runtime extension:

```yaml
extensions:
  pinocchio.webchat_runtime@v1:
    middlewares:
      - name: agentmode
        enabled: false
```

Schema evidence:

- [`pinocchio/pkg/inference/runtime/profile_runtime.go:9`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/inference/runtime/profile_runtime.go#L9)

### Frontend entity vs renderer status

Backend and frontend semantic mapping already exist:

- backend maps `EventAgentModeSwitch` to `agent.mode`
  - [`pinocchio/pkg/webchat/sem_translator.go:489`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/webchat/sem_translator.go#L489)
- frontend maps `agent.mode` to `kind: 'agent_mode'`
  - [`pinocchio/cmd/web-chat/web/src/sem/registry.ts:230`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/sem/registry.ts#L230)

But no dedicated renderer appears to be registered:

- built-in renderers only:
  - [`pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts:10`](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts#L10)

### Recommended merge rule

Suggested policy:

```text
effective runtime =
  default web-chat runtime overlay
  + profile runtime extension

profile runtime wins
enabled:false is respected
missing agentmode entry inherits default-on
```

## Usage Examples

### Example profile opt-out

```yaml
extensions:
  pinocchio.webchat_runtime@v1:
    middlewares:
      - name: agentmode
        enabled: false
```

### Example profile override while staying enabled

```yaml
extensions:
  pinocchio.webchat_runtime@v1:
    middlewares:
      - name: agentmode
        config:
          default_mode: analyst
          sanitize_yaml: true
```

### Example implementation review checklist

- Verify the dead flag is removed from `cmd/web-chat/main.go`
- Verify default-on runtime overlay is applied before runtime fingerprinting
- Verify `enabled: false` disables both middleware composition and sink wrapping
- Verify `agent_mode` renders with a dedicated card instead of the generic fallback

## Related

- [`../design-doc/01-intern-guide-to-making-web-chat-agentmode-default-on-with-profile-opt-out-and-renderer-support.md`](../design-doc/01-intern-guide-to-making-web-chat-agentmode-default-on-with-profile-opt-out-and-renderer-support.md)
