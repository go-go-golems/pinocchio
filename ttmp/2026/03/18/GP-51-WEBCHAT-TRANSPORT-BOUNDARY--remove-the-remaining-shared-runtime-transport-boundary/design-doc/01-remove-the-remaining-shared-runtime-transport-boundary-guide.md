---
Title: Remove the Remaining Shared Runtime Transport Boundary
Slug: remove-the-remaining-shared-runtime-transport-boundary-guide
Short: Detailed analysis and implementation guide for the final Pinocchio webchat cleanup after engine profiles and app-owned runtime planning are already in place.
Topics:
    - pinocchio
    - webchat
    - architecture
    - transport
    - migration
DocType: design
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/inference/runtime/profile_runtime.go
      Note: Shared runtime payload under consideration for removal.
    - Path: pkg/webchat/http/api.go
      Note: Shared HTTP transport request DTO currently used by resolvers and handlers.
    - Path: pkg/inference/runtime/composer.go
      Note: Shared runtime builder request and composed runtime interface.
    - Path: cmd/web-chat/profile_policy.go
      Note: Pinocchio’s current local-first resolver and explicit boundary conversion.
    - Path: ../2026-03-16--gec-rag/internal/webchat/resolver.go
      Note: CoinVault local-first resolver pattern to compare against.
    - Path: ../temporal-relationships/internal/extractor/httpapi/run_chat_transport.go
      Note: Temporal local-first resolver pattern to compare against.
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-18T23:59:00-04:00
WhatFor: Explain the final remaining shared transport seam in Pinocchio webchat, why it exists, why it is now safe to remove, and what the hard-cut implementation options look like.
WhenToUse: Use this document when planning the last webchat boundary cleanup after engine profiles and local-first app runtime plans are already complete.
---

## Why This Document Exists

Pinocchio webchat has already gone through the important architectural cleanup:

- Geppetto no longer owns mixed runtime profiles
- Geppetto engine profiles now resolve only final `InferenceSettings`
- Pinocchio, CoinVault, and Temporal each now build a local-first runtime plan before crossing into the shared webchat layer

That means the system is already correct and maintainable. This document is about the last layer of cleanup, not an urgent bug fix.

The remaining issue is that shared Pinocchio types still carry app-owned runtime policy at the final handoff boundary. Those types are smaller and safer than before, but they still carry more information than the shared layer should ideally know about.

This guide explains:

- what the remaining boundary is
- why it still exists
- why it is now reasonable to remove it
- what the final design options are
- how to implement the hard cut later

## The Current Boundary

The remaining shared transport boundary is spread across three related types.

### 1. `ProfileRuntime`

Defined in [`pkg/inference/runtime/profile_runtime.go`](../../../../../../pinocchio/pkg/inference/runtime/profile_runtime.go):

```go
type ProfileRuntime struct {
    SystemPrompt string
    Middlewares  []MiddlewareUse
    Tools        []string
}
```

This is already much narrower than the old mixed Geppetto runtime profile model. It only carries:

- prompt
- middleware selection
- tool names

Still, those are all app-owned concepts.

### 2. `ResolvedConversationRequest`

Defined in [`pkg/webchat/http/api.go`](../../../../../../pinocchio/pkg/webchat/http/api.go):

```go
type ResolvedConversationRequest struct {
    ConvID                    string
    RuntimeKey                string
    RuntimeFingerprint        string
    ProfileVersion            uint64
    ResolvedInferenceSettings *aisettings.InferenceSettings
    ResolvedRuntime           *infruntime.ProfileRuntime
    ProfileMetadata           map[string]any
    Prompt                    string
    IdempotencyKey            string
}
```

This is the shared HTTP-side output of request resolution. It is now used as a transport object, not as the app’s primary domain model. That is good, but it is still fairly fat.

### 3. `ConversationRuntimeRequest`

Defined in [`pkg/inference/runtime/composer.go`](../../../../../../pinocchio/pkg/inference/runtime/composer.go):

```go
type ConversationRuntimeRequest struct {
    ConvID                     string
    ProfileKey                 string
    ProfileVersion             uint64
    ResolvedInferenceSettings  *aisettings.InferenceSettings
    ResolvedProfileRuntime     *ProfileRuntime
    ResolvedProfileFingerprint string
}
```

This is the shared input to the runtime builder/composer seam.

## Why This Boundary Still Exists

It exists for a reasonable historical reason.

Earlier, Pinocchio webchat needed one shared object that could move from:

- request parsing
- to resolver output
- to conversation creation
- to runtime composition

That was practical when the app/runtime separation was not yet clean. A shared DTO reduced friction because every layer could pass the same object around.

The problem is that after the recent migration work, the app/runtime separation is now clean. The shared DTO remains mostly because it still works, not because it is the best shape anymore.

## Why It Is Worth Killing Now

This cleanup is now safe precisely because the larger migration is already done.

### Before

If we had tried to remove the shared boundary earlier, we would have been changing too many things at once:

- Geppetto profile model
- engine settings naming
- JS runtime logic
- downstream request resolution
- webchat runtime contracts

That would have been reckless.

### Now

Now the system already has the correct conceptual split:

- engine selection is resolved through Geppetto engine profiles
- app runtime policy is resolved in each app
- local-first runtime plan types already exist in all three downstream apps

That means the remaining shared transport is no longer a safety mechanism. It is mostly an artifact.

## Current End-to-End Flow

The current Pinocchio webchat flow is:

```text
HTTP request
  -> cmd/web-chat/profile_policy.go
  -> local resolved plan
  -> toResolvedConversationRequest(...)
  -> webhttp.NewChatHandler / NewWSHandler
  -> SubmitPromptInput / ConversationRuntimeRequest
  -> RuntimeBuilder.Compose(...)
  -> ComposedRuntime
```

The important observation is this:

The app already has the local resolved plan before it builds the shared transport object.

That means the shared transport object is no longer the source of truth. It is only a handoff envelope.

## The Architectural Question

What should replace the current shared envelope?

There are two serious options.

## Option A: Keep a Shared DTO, But Make It Smaller

This is the lower-risk option.

### Idea

Keep a shared request type, but remove app-owned runtime policy from it. Let the app resolve prompt, middleware, and tool behavior before the shared handoff.

### What would remain

- `ConvID`
- `Prompt`
- `IdempotencyKey`
- `RuntimeKey`
- `RuntimeFingerprint`
- maybe `ProfileVersion`
- maybe `ResolvedInferenceSettings`

### What would leave

- `ProfileRuntime`
- prompt/tool/middleware payload
- maybe metadata

### What it might look like

```go
type ConversationStartTransport struct {
    ConvID             string
    Prompt             string
    IdempotencyKey     string
    RuntimeKey         string
    RuntimeFingerprint string
    InferenceSettings  *aisettings.InferenceSettings
}
```

The app would be responsible for making sure anything needed for runtime composition is already encoded into the way the composer is constructed or selected.

### Advantages

- Smaller change
- Easier migration
- Keeps shared handlers simple

### Disadvantages

- Still DTO-oriented
- Shared layer may still know more than it should
- Could still leave awkward duplication between app plan and transport DTO

## Option B: Replace the DTO with a Compose-Capable Boundary

This is the cleaner long-term option.

### Idea

Stop transporting app runtime policy as data. Instead, let the app hand shared webchat a compose-capable object or function.

The app would resolve everything locally, then expose only:

- identity
- prompt/request metadata
- a way to build or compose the runtime when needed

### What it might look like

```go
type RuntimeHandle interface {
    RuntimeKey() string
    RuntimeFingerprint() string
    Compose(ctx context.Context) (runtime.ComposedRuntime, error)
}

type StartPlan struct {
    ConvID         string
    Prompt         string
    IdempotencyKey string
    Runtime        RuntimeHandle
}
```

Or as a function-based adapter:

```go
type StartPlan struct {
    ConvID         string
    Prompt         string
    IdempotencyKey string
    RuntimeKey     string
    Fingerprint    string
    ComposeRuntime func(ctx context.Context) (runtime.ComposedRuntime, error)
}
```

### Why this is attractive

The shared layer no longer knows:

- prompt policy
- middleware names
- tool names
- local runtime metadata structure

It only knows how to ask for the composed runtime when it needs it.

### Advantages

- Cleanest separation of concerns
- Removes the shared app-runtime payload entirely
- Matches the current local-first architecture better

### Disadvantages

- Bigger API change
- More invasive testing changes
- Requires careful treatment of rebuild and reuse semantics

## Recommended Direction

My recommendation is Option B, but only if the implementation stays disciplined.

The reason is that Option B matches the architecture we have been moving toward:

- app-owned resolution
- shared lifecycle
- no transport of app-owned policy as data

If the team wants the smallest possible follow-up, Option A is acceptable. But if we are already doing a hard cut anyway, Option B is the better final shape.

## What the Final Code Should Feel Like

This is the intended end-state feel, not necessarily the exact final API.

### App-side resolver

```go
type resolvedWebchatPlan struct {
    ConvID         string
    Prompt         string
    IdempotencyKey string
    Runtime        *resolvedRuntime
}

type resolvedRuntime struct {
    RuntimeKey         string
    RuntimeFingerprint string
    InferenceSettings  *aisettings.InferenceSettings
    SystemPrompt       string
    Middlewares        []runtime.MiddlewareUse
    ToolNames          []string
}

func (r *resolver) buildPlan(ctx context.Context, req *http.Request) (*resolvedWebchatPlan, error) {
    // app-owned selection and resolution
}
```

### App-side adapter into shared webchat

```go
func (p *resolvedWebchatPlan) toStartPlan() webchat.StartPlan {
    return webchat.StartPlan{
        ConvID:         p.ConvID,
        Prompt:         p.Prompt,
        IdempotencyKey: p.IdempotencyKey,
        RuntimeKey:     p.Runtime.RuntimeKey,
        Fingerprint:    p.Runtime.RuntimeFingerprint,
        ComposeRuntime: func(ctx context.Context) (runtime.ComposedRuntime, error) {
            return composeFromResolvedRuntime(ctx, p.Runtime)
        },
    }
}
```

### Shared webchat usage

```go
plan, err := resolver.Resolve(req)
if err != nil { ... }

resp, err := svc.SubmitPrompt(ctx, plan.toStartPlan())
```

The key idea is that the shared layer receives a plan that can compose the runtime, not a bundle of prompt/tool/middleware settings it has to understand.

## Implementation Plan

When this ticket is actually implemented, use this order.

### Step 1: Measure the true shared needs

Read every use of:

- `ProfileRuntime`
- `ResolvedConversationRequest`
- `ConversationRuntimeRequest`

Classify each field:

- required for shared lifecycle
- required only for app runtime composition
- debug-only
- metadata-only

Do not redesign before this inventory is written down.

### Step 2: Choose the replacement boundary

Make an explicit decision between:

- smaller DTO
- compose-capable interface

Do not try to half-implement both.

### Step 3: Change the shared HTTP handler seam

Likely files:

- [`pkg/webchat/http/api.go`](../../../../../../pinocchio/pkg/webchat/http/api.go)
- [`cmd/web-chat/profile_policy.go`](../../../../../../pinocchio/cmd/web-chat/profile_policy.go)

This is where `ResolvedConversationRequest` currently anchors the transport.

### Step 4: Change runtime composition seam

Likely files:

- [`pkg/inference/runtime/composer.go`](../../../../../../pinocchio/pkg/inference/runtime/composer.go)
- [`cmd/web-chat/runtime_composer.go`](../../../../../../pinocchio/cmd/web-chat/runtime_composer.go)

The runtime builder boundary should stop depending on a shared app-runtime payload if Option B is chosen.

### Step 5: Migrate apps one at a time

Order:

1. Pinocchio `cmd/web-chat`
2. CoinVault
3. Temporal Relationships

Reason:

- Pinocchio is the reference app and owns the shared transport
- CoinVault and Temporal already have local-first plans, so their migrations should then be adapter-only

### Step 6: Shrink or delete `ProfileRuntime`

If Option B is chosen, `ProfileRuntime` may disappear entirely from the shared path.

If Option A is chosen, rename it to something clearly transport-oriented, for example:

```go
type RuntimeTransport struct { ... }
```

That at least stops pretending it is a primary domain type.

## Validation Plan

When implementation starts, validate in layers.

### Pinocchio

```bash
go test ./cmd/web-chat ./pkg/webchat/... -count=1
go test ./cmd/pinocchio/... -count=1
```

### CoinVault

```bash
go test ./internal/... -count=1
```

### Temporal Relationships

```bash
go test ./internal/extractor/... -count=1
```

### Behavioral checks

- profile change rebuild still works
- runtime key remains stable and visible where needed
- runtime fingerprint still invalidates rebuild correctly
- shared HTTP handlers still accept the same external request contract

## Risks

This is cleanup, but it is not trivial. The risks are:

- accidentally moving too much logic into shared webchat again
- breaking rebuild invalidation semantics
- introducing a clever boundary that is harder to test than the current DTO

The antidote is discipline:

- local-first plan remains the app truth
- shared boundary becomes smaller, not more abstract for its own sake

## Troubleshooting Questions for Future Implementation

When you implement this later, ask these questions during review:

- Does the shared layer still know prompt/middleware/tool details?
- Does the app still build its own local plan first?
- Can a new webchat-style app follow the same pattern without copying Pinocchio-specific DTOs?
- Is rebuild invalidation still driven by runtime fingerprint, not by hidden side effects?

## See Also

- [`pkg/inference/runtime/profile_runtime.go`](../../../../../../pinocchio/pkg/inference/runtime/profile_runtime.go) — current shared runtime payload
- [`pkg/webchat/http/api.go`](../../../../../../pinocchio/pkg/webchat/http/api.go) — current shared request transport
- [`cmd/web-chat/profile_policy.go`](../../../../../../pinocchio/cmd/web-chat/profile_policy.go) — current local-first resolver plus explicit conversion
- [`pkg/doc/topics/webchat-engine-profile-migration-playbook.md`](../../pkg/doc/topics/webchat-engine-profile-migration-playbook.md) — the migration that got us to the current state
