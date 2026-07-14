---
Title: Implementation diary
Ticket: PINOCCHIO-JS-OAUTH-RUNTIME-MANAGEMENT
Status: active
Topics:
    - oauth
    - javascript
    - credentials
    - inference
DocType: reference
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: repo://cmd/pinocchio/cmds/js.go
      Note: Forwards opaque source to both native modules (commit 6315889)
    - Path: repo://cmd/pinocchio/cmds/js_test.go
      Note: Both JavaScript builders source injection coverage (commit 6315889)
    - Path: repo://pkg/cmds/profilebootstrap/oauth.go
      Note: Shared selected-profile source helper (commit 6315889)
    - Path: repo://pkg/js/modules/pinocchio/module.go
      Note: Source-aware defaults builder (commit 6315889)
ExternalSources: []
Summary: Chronological evidence for JavaScript OAuth source injection and Pinocchio credential lifecycle operations.
LastUpdated: 2026-07-14T16:41:00-04:00
WhatFor: Implement, review, and validate host-only OAuth behavior in Pinocchio JavaScript runtimes.
WhenToUse: When continuing this ticket or reviewing OAuth source ownership.
---


# Diary

## Goal

Record the design, implementation, provider-contract investigation, validation, and delivery of OAuth bearer source support for all Pinocchio engine construction paths without exposing credential material.

## Step 1: Establish the JavaScript gap and archive provider-contract evidence

The first investigation confirms that Pinocchio’s regular Go engine path is already source-aware, but both JavaScript engine builders bypass it. The work therefore starts by preserving the same host-owned source through native-module registration rather than adding a JavaScript credential API.

The provider investigation also establishes that a real browser OAuth smoke cannot be responsibly guessed from marketing pages or MCP connector documentation. The OpenAI and Claude OAuth pages describe those products as clients authenticating users to third-party MCP servers, while their public inference API documentation describes API key or workload-identity authentication. Umans publicly presents API-key setup and has no discoverable public authorization-code/refresh contract. All material has been saved in this ticket before implementation conclusions were written.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket in pinocchio for this work, and build it step by step, commit at appropriate intervals, and keep a detailed diary. You can look at the umans.ai / openai / claude oauth and try it out with that"

**Assistant interpretation:** Create a ticket and evidence-backed plan, implement the missing Pinocchio JavaScript OAuth source path and credential lifecycle operations in phases, and attempt a real provider flow only where a safe compatible contract exists.

**Inferred user intent:** Make OAuth-backed profiles work consistently across Pinocchio’s Go and JavaScript entry points while giving operators safe local management commands and avoiding undocumented provider integrations.

### What I did

- Created ticket `PINOCCHIO-JS-OAUTH-RUNTIME-MANAGEMENT` with design and diary documents.
- Added tracked tasks for provider triage, JS injection, lifecycle verbs, and validation.
- Inspected the released Geppetto source-injection API, Pinocchio command JS runtime registration, Pinocchio native module factory, OAuth profile bootstrap, YAML store, and existing Glazed login verb.
- Collected and read six sources under `sources/` with Defuddle:
  - OpenAI Apps SDK OAuth authentication;
  - Claude connector authentication;
  - Umans AI public setup page;
  - local line-numbered source map;
  - Claude Platform API authentication;
  - OpenAI API quickstart.
- Wrote the primary design document, including source-role analysis, diagrams, API sketches, implementation phases, and a real-provider smoke gate.

### Why

The released Geppetto API is a Go-only dependency-injection capability. Pinocchio must use it from its trusted host runtime; allowing JavaScript to provide a bearer, callback, or source selection would violate the existing direct-YAML/profile ownership model.

### What worked

- Geppetto `v0.13.6` exposes `gp.Options.BearerTokenSource` as a Go-only field.
- `cmd/pinocchio/cmds/js.go` has one registration point for both `require("geppetto")` and `require("pinocchio")`.
- `Profile.Parse` treats credential tuple fields as optional, so a local logout can retain valid authorization/token endpoint policy for a later login.
- Public documents distinguish MCP connector OAuth from direct inference API authentication.

### What didn't work

A focused public search for a Umans OAuth authorization-code/PKCE/refresh contract returned no documentation. This is not a product failure; it means a real Umans OAuth smoke cannot be configured safely from public evidence. No local credential file or existing secret was inspected.

### What I learned

There are two JavaScript bypasses, not one. The Geppetto native builder lacks the source because Pinocchio omits it from `gp.Options`; Pinocchio’s own `engines.fromDefaults()` independently calls the no-options factory helper. Both need the same opaque Go source.

### What was tricky to build

Provider documentation uses the word OAuth for several different resource relationships. OpenAI Apps SDK and Claude connector OAuth describe ChatGPT/Claude authenticating a user to an external MCP server. Those flows do not authorize Pinocchio to call OpenAI or Claude inference APIs. The design documents this distinction before any browser attempt so an authorization flow cannot be directed at the wrong resource server.

### What warrants a second pair of eyes

- Confirm source-backed JavaScript endpoint overrides are rejected rather than silently changing source authority.
- Review status output fields so local readiness is useful without exposing expiry timestamps or filesystem path details by default.
- Review logout idempotence and concurrent save/delete behavior under the existing sibling lock.
- Revisit real-provider smoke only after one provider publishes or supplies an approved public-PKCE, refresh-token, OpenAI-compatible bearer contract.

### What should be done in the future

- Implement bootstrap-to-JavaScript source forwarding and both source-aware builders.
- Implement secret-free status and atomic local logout.
- Run the defined validation matrix; do not run a provider browser flow until the documented smoke gate passes.

### Code review instructions

- Read the design document first, then compare the current registration gap at `cmd/pinocchio/cmds/js.go:203-345` with Geppetto’s `Options.BearerTokenSource` API.
- Read `pkg/js/modules/pinocchio/module.go:17-67` for the second no-options factory path.
- Inspect the saved primary sources before accepting the provider-smoke decision.

### Technical details

The proposed public command surface is deliberately narrow:

```text
pinocchio auth login   # existing PKCE login/re-login
pinocchio auth status  # proposed local, redacted state
pinocchio auth logout  # proposed local atomic tuple removal
pinocchio auth revoke  # deferred pending provider-specific contract
```

There is no proposed `auth refresh` verb. Runtime inference refresh remains the sole path that obtains a bearer.

## Step 2: Inject the resolved source into both JavaScript engine paths

The JavaScript command runtime now resolves the selected OAuth profile once in Go and passes its opaque `BearerTokenSource` to both native modules before any script executes. Geppetto’s fluent builder and Pinocchio’s `engines.fromDefaults()` builder can therefore construct source-backed OpenAI-compatible engines without adding a static API key or exposing credentials to JavaScript.

The construction paths retain their old behavior for static profiles. A nil source still uses the no-options factory helper, while a non-nil source uses the standard factory with `WithBearerTokenSource`. This keeps provider routing and source authority centralized in Geppetto.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Implement the host-only source forwarding as the first code phase and protect it with behavioral JavaScript tests.

**Inferred user intent:** Make OAuth-backed profile behavior consistent for ordinary Go and JavaScript-built engines without granting scripts credential capabilities.

**Commit (code):** `6315889` — "feat: inject OAuth sources into Pinocchio JS engines"

### What I did

- Added `profilebootstrap.NewBearerTokenSourceForResolvedSettings` and made the existing source-aware factory use it.
- Resolved the source during Pinocchio JS runtime bootstrap and passed it through `pinocchioJSRuntimeOptions`.
- Set the released `gp.Options.BearerTokenSource` option when registering Geppetto’s native module.
- Added the same opaque source field to `pkg/js/modules/pinocchio.Options` and used `factory.WithBearerTokenSource` in `engines.fromDefaults()`.
- Added a JS runtime regression test that builds engines through both modules with an empty static-key map and asserts neither module exports a source property.
- Ran focused tests, then the complete repository pre-commit lint/test hook.

### Why

Both JavaScript builders previously used a no-options factory helper even though the trusted host had already resolved an OAuth profile. Forwarding the interface at registration is the smallest change that preserves the source boundary and lets Geppetto perform its existing request-time refresh behavior.

### What worked

- `go test ./pkg/cmds/profilebootstrap ./pkg/js/modules/pinocchio ./cmd/pinocchio/cmds -count=1` passed.
- The complete pre-commit hook passed `go generate ./...`, the frontend build, `go build ./...`, golangci-lint, custom lint, Glazed lint, and `go test ./...`.
- The test double returns no credential material because engine construction must not invoke the source. It proves source-aware factory validation without adding a bearer-shaped fixture value.

### What didn't work

No implementation or validation failure occurred in this step.

### What I learned

The Pinocchio command runtime is the correct place to resolve the source because it already owns selected profile resolution and module registration. Native modules need only receive an interface; they do not need access to registry file paths, OAuth extensions, or provider protocol configuration.

### What was tricky to build

There were two independent factory bypasses. Fixing only `gp.Options` would leave `require("pinocchio").engines.fromDefaults()` static-key-only. The regression test constructs one engine through each module under the same host source so future refactors cannot restore either bypass unnoticed.

### What warrants a second pair of eyes

- The source’s profile-bound YAML store rejects an unexpected provider/base URL before releasing a credential. Consider a future earlier rejection for JavaScript API/base-URL overrides if product requirements make that error clearer.
- Review future source-sharing across multiple profiles as a host authorization problem; do not add a JavaScript source selector.

### What should be done in the future

- Implement local status and logout using the selected profile/store path.
- Add source-compatible override validation if scripts gain richer endpoint override features.

### Code review instructions

- Start with `pkg/cmds/profilebootstrap/oauth.go`, then follow the source through `cmd/pinocchio/cmds/js.go` into both native module registrations.
- Run the focused command above and inspect `TestPinocchioJSRuntimeForwardsHostBearerSourceToBothEngineBuilders`.

### Technical details

The JavaScript surface has not changed. The new fields exist only in Go option structs and are never assigned to Goja values, module exports, settings, or engine metadata.
