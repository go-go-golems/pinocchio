---
Title: Web Chat Follow-up Plan
Ticket: GP-50-PINOCCHIO-ENGINE-PROFILE-MIGRATION
Status: active
Topics:
    - pinocchio
    - webchat
    - migration
    - engineprofiles
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/profile_policy.go
      Note: Current resolver still assumes Geppetto-owned profile runtime metadata
    - Path: cmd/web-chat/runtime_composer.go
      Note: Current runtime composer still consumes system prompt, middleware, and tools from removed Geppetto runtime spec
    - Path: pkg/inference/runtime/composer.go
      Note: Shared runtime request still carries removed RuntimeSpec fields
    - Path: pkg/webchat/conversation.go
      Note: Conversation state persists runtime key/fingerprint and resolved profile metadata
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-18T18:10:00-04:00
WhatFor: ""
WhenToUse: ""
---

# Web Chat Follow-up Plan

## Why web chat is separate

The CLI and `pinocchio js` migration is mostly an engine-configuration problem:

- choose base `InferenceSettings`
- optionally resolve an engine profile
- build an engine

Web chat is different. It still has a real app-owned runtime model:

- system prompt selection
- middleware selection/config
- tool subset selection
- runtime identity used for conversation reuse and rebuild decisions
- profile switching UX and persistence

Those concerns no longer belong in Geppetto `engineprofiles`, but they do still belong somewhere. That means web chat needs its own local profile/runtime layer rather than trying to overload engine profiles again.

## Current mixed concerns still present

### 1. Shared runtime request still embeds removed Geppetto runtime concepts

[`pkg/inference/runtime/composer.go`](../../../../../../pinocchio/pkg/inference/runtime/composer.go) still carries:

- `ResolvedProfileRuntime *gepprofiles.RuntimeSpec`
- `ResolvedProfileFingerprint string`

That is the direct leftover from the removed mixed-runtime model.

### 2. The web-chat runtime composer still expects Geppetto-owned prompt/middleware/tool data

[`cmd/web-chat/runtime_composer.go`](../../../../../../pinocchio/cmd/web-chat/runtime_composer.go) still reads:

- `ResolvedProfileRuntime.SystemPrompt`
- `ResolvedProfileRuntime.Middlewares`
- `ResolvedProfileRuntime.Tools`

and then computes a web-chat-specific runtime fingerprint from those fields plus resolved inference settings.

This is exactly the app-owned logic that must move out of the engine-profile layer.

### 3. The request resolver still pretends engine profile resolution returns executable runtime behavior

[`cmd/web-chat/profile_policy.go`](../../../../../../pinocchio/cmd/web-chat/profile_policy.go) still relies on:

- `resolvedProfile.EffectiveRuntime`
- `resolvedProfile.RuntimeFingerprint`

Those no longer exist in Geppetto core and should not come back there.

### 4. Conversation lifecycle code depends on app-owned runtime identity

[`pkg/webchat/conversation.go`](../../../../../../pinocchio/pkg/webchat/conversation.go) and [`pkg/webchat/conversation_service.go`](../../../../../../pinocchio/pkg/webchat/conversation_service.go) persist and compare:

- `RuntimeKey`
- `RuntimeFingerprint`
- `ResolvedProfileMetadata`

This is valid app behavior, but it needs a Pinocchio-owned source of truth.

## Recommended target model

Web chat should have two layers:

```text
Engine profile layer (Geppetto)
  engine profile slug
  final InferenceSettings

Web-chat app profile layer (Pinocchio)
  runtime key
  system prompt
  middleware uses
  tool names
  profile-switch metadata
  fingerprint inputs
```

## Recommended local format

Do not reuse Geppetto engine-profile YAML for this.

Instead, add a narrow Pinocchio-local app profile format, for example:

```yaml
profiles:
  default:
    engine_profile: default
    system_prompt: You are a helpful analyst.
    middlewares:
      - name: agentmode
    tools:
      - search
      - open_url

  agent:
    engine_profile: gpt-5-mini
    system_prompt: You are an action-oriented research agent.
    middlewares:
      - name: agentmode
      - name: sqlitetool
    tools:
      - search
      - open_url
      - sqlite_query
```

That keeps the responsibility split clear:

- `engine_profile` points to Geppetto engine config
- the rest is web-chat-specific runtime behavior

## Migration steps for the future web-chat ticket

1. Replace `ResolvedProfileRuntime` in [`pkg/inference/runtime/composer.go`](../../../../../../pinocchio/pkg/inference/runtime/composer.go) with explicit app-owned fields:
   - `SystemPrompt`
   - `MiddlewareUses`
   - `ToolNames`
   - `RuntimeFingerprint`
2. Replace `profile_policy.go` engine-profile resolution with:
   - resolve web-chat app profile
   - resolve engine profile referenced by that app profile
   - merge into one conversation runtime request
3. Update `runtime_composer.go` to consume app-owned fields directly, not Geppetto runtime specs.
4. Keep `RuntimeKey` and `RuntimeFingerprint` in conversation state, but make them Pinocchio-owned derived values.
5. Rewrite tests and fixtures under `cmd/web-chat` to use the new local app-profile format.

## Recommendation

Do not block GP-50 CLI/JS completion on web chat.

The clean handoff is:

- GP-50 covers CLI and JS migration to engine-only profiles
- a follow-up ticket handles web chat as a separate app-runtime migration
