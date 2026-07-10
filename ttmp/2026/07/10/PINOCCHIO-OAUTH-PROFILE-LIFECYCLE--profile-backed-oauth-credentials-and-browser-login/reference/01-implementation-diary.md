---
Title: Implementation diary
Ticket: PINOCCHIO-OAUTH-PROFILE-LIFECYCLE
Status: active
Topics:
    - pinocchio
    - oauth
    - credentials
    - profiles
    - security
DocType: reference
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: ws://geppetto/pkg/steps/ai/credentials/oauth/oauth.go
      Note: Observed sibling OAuth primitive
    - Path: ws://pinocchio/pkg/cmds/profilebootstrap/engine_settings.go
      Note: Observed engine construction boundary
    - Path: ws://pinocchio/pkg/configdoc/types.go
      Note: Observed profile extension boundary
ExternalSources:
    - https://github.com/go-go-golems/geppetto/issues/387
Summary: Chronological investigation and implementation record for Pinocchio profile OAuth lifecycle work.
LastUpdated: 2026-07-10T23:35:00-04:00
WhatFor: Continue the profile-backed OAuth, persistence, browser-login, and runtime-source work safely.
WhenToUse: Use when resuming implementation or reviewing decisions and validation for this ticket.
---


# Implementation diary

## Goal

Record the evidence and implementation path for OAuth access/refresh/expiry credentials stored in an explicit Pinocchio profile, with secure persistence, a browser login flow, and Geppetto renewable-source injection.

## Step 1: Establish the Pinocchio OAuth lifecycle ticket and architecture baseline

This ticket is deliberately in the cloned Pinocchio worktree because Pinocchio owns local profile selection, command UX, browser lifecycle, and the requested owner-readable YAML state. The earlier Geppetto work introduced reusable token protocol and inference primitives, but it cannot decide where a user’s profile lives or which local callback/client policy should apply.

The design records the current profile machinery before implementation. It identifies the existing extension transport, standard factory construction, and inspection surfaces as both useful integration points and possible secret-leak paths. The result is a phased guide for an intern: first make profile state typed and safe, then persist it atomically, then wire runtime renewal, and only then add browser login.

### Prompt Context

**User prompt (verbatim):** "Create a detailed docmgr ticket in pinocchio and Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.


Use the --root flag for docmgr for all this."

**Assistant interpretation:** Create a comprehensive ticket in the provided Pinocchio worktree, document the OAuth profile lifecycle from current code through a detailed implementation plan, use docmgr with an explicit root, and publish the final documentation bundle to reMarkable.

**Inferred user intent:** Give a new engineer a complete, evidence-backed plan for implementing profile-backed OAuth credentials without rediscovering the Geppetto/Pinocchio boundary or accidentally leaking secrets.

### What I did

- Confirmed the clean Pinocchio worktree at `/home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio` and its existing `ttmp/` root.
- Created ticket `PINOCCHIO-OAUTH-PROFILE-LIFECYCLE` under the explicit absolute docmgr root.
- Added a Pinocchio-local `.ttmp.yaml` so the docs root resolves its own vocabulary instead of a parent workspace configuration.
- Mapped `pkg/configdoc/types.go`, `merge.go`, and `profiles.go` for inline profile extensions and Geppetto registry conversion.
- Mapped `pkg/cmds/profilebootstrap/engine_settings.go` for resolved settings and factory construction.
- Mapped `cmd/pinocchio/cmds/profiles/*` and `cmd/pinocchio/main.go` for inspection and command registration.
- Mapped Geppetto credential, OAuth, factory, Chat, and Responses APIs from the sibling updated worktree.
- Wrote the detailed design guide, task plan, and this diary without reading any real credential values.

### Why

- OAuth protocol mechanics now exist in Geppetto, but the profile YAML, local file permissions, loopback callback, and user-facing command belong to Pinocchio.
- Existing `Extensions` support is useful but untyped and may be rendered through inspection/provenance paths, so it is a security boundary requiring an explicit design.
- A durable credential store and runtime source must agree on identity and persistence ordering before browser login is safe to expose.

### What worked

- The Pinocchio config model has a concrete extension transport: `InlineProfile.Extensions` is cloned and copied into Geppetto `EngineProfile.Extensions`.
- Pinocchio’s resolved runtime retains both final inference settings and the resolved engine profile, providing a path to inspect typed OAuth metadata before factory construction.
- Geppetto exposes all required generic interfaces: profile host `Store`, `Refresher`, `RenewableBearerTokenSource`, source-aware factory option, and a pure OAuth client.

### What didn't work

The initial docmgr status/vocabulary commands displayed configuration from the parent workspace despite using an explicit `--root`; the Pinocchio root had no `.ttmp.yaml`. The ticket itself was created in the correct absolute root, but `docmgr vocab add --root` attempted to modify the parent vocabulary.

I reverted those unintended vocabulary changes and created `/home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/.ttmp.yaml` pointing at `ttmp/vocabulary.yaml`. The Pinocchio-local vocabulary was then updated directly with the needed OAuth, credential, and security topics. Every remaining docmgr command uses the required explicit `--root` path.

### What I learned

- Pinocchio distinguishes layered inline config profiles from imported Geppetto registries; a resolved profile is not automatically a safely writable local YAML source.
- The existing standard factory call has a natural source injection point, but it currently has no OAuth-awareness.
- llm-proxy’s encrypted vault is a distinct host store; this ticket must not conflate it with Pinocchio’s local profile YAML.

### What was tricky to build

The tricky part is separating metadata transport from secret persistence. `Extensions` already travel through profile merging, but treating the whole extension map as safe would make refresh tokens visible to profile tools. The proposed solution uses a typed reserved namespace, resolves it before engine construction, strips it from generic display paths, and gives the credential store a direct writable registry identity rather than trying to rewrite an arbitrary composed profile.

### What warrants a second pair of eyes

- Verify the exact direct-registry YAML codec and whether it preserves formatting/comments during a safe targeted rewrite.
- Review every full/detailed/JSON profile reporting path for extension leakage.
- Confirm the first provider’s loopback redirect, public-client, scopes, client-auth, and refresh-token rotation requirements.
- Review the plaintext `0600` YAML threat model before implementation.

### What should be done in the future

- Complete Phase 0 direct-registry/writable-source discovery before selecting a data model implementation.
- Implement the typed model, redaction, durable store, runtime source, and browser command in the sequence listed in `tasks.md`.
- Upload a refreshed bundle after implementation work updates the guide and diary.

### Code review instructions

Start with the design document sections 3–7. Review the cited Pinocchio profile/config/bootstrap files alongside Geppetto’s credential APIs. Confirm that every proposal keeps refresh data out of inference settings and generic inspection output.

### Technical details

No access token, refresh token, authorization code, client secret, profile file content, or provider endpoint specific to a real account was read, printed, or stored in ticket documentation.

## Step 2: Validate and publish the intern guide

The ticket design, diary, tasks, index, and changelog were validated against the Pinocchio docs root and bundled as a single offline-readable PDF. The bundle gives an intern the proposed architecture, file evidence, API sketches, lifecycle pseudocode, decision records, security constraints, and phased implementation plan without embedding any credential material.

The current ticket is intentionally still active: only architecture/discovery documentation is complete. Implementation tasks for typed profile state, secure persistence, runtime injection, and browser login remain open and are clearly separated in the task list.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Finish the documentation deliverable by validating the ticket and publishing the review bundle to reMarkable.

**Inferred user intent:** Make the implementation plan accessible and reviewable away from the local worktree while preserving security boundaries.

### What I did

- Ran `docmgr doctor --root /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/ttmp --ticket PINOCCHIO-OAUTH-PROFILE-LIFECYCLE --stale-after 30` successfully.
- Ran a dry-run bundle upload containing the ticket index, design guide, diary, tasks, and changelog.
- Uploaded the same bundle with ToC depth 2 to the ticket-specific reMarkable directory.

### Why

- The bundle supplies a stable review artifact before implementation begins.
- Keeping the current documentation separate from live profile credentials ensures a review artifact cannot leak local OAuth material.

### What worked

The dry run selected all five intended Markdown documents. The actual upload returned:

```text
OK: uploaded Pinocchio OAuth Profile Lifecycle Guide.pdf -> /ai/2026/07/10/PINOCCHIO-OAUTH-PROFILE-LIFECYCLE
```

### What didn't work

N/A.

### What I learned

The repository did not initially have a local `.ttmp.yaml`; adding one made the intended Pinocchio docs root explicit for ticket creation and validation. The `docmgr vocab add --root` behavior still resolved vocabulary through a parent workspace, so the local vocabulary was updated directly and the unintended external changes were reverted before delivery.

### What was tricky to build

The delivery artifact needs to be complete enough for a review but cannot include live configuration examples with actual values. The guide uses only placeholder endpoints and token markers, while retaining enough detail for implementation.

### What warrants a second pair of eyes

- Review the proposed `extensions.pinocchio.oauth` schema against the actual direct registry YAML codec before code begins.
- Review whether plaintext owner-only YAML meets the intended local threat model.

### What should be done in the future

- Begin Phase 0 writable-registry and profile-output discovery.
- Re-upload an updated bundle after implementation phases materially change the guide or diary.

### Code review instructions

Review the uploaded PDF’s architecture and decision-record sections first, then validate the local ticket with the doctor command above.

### Technical details

Bundle destination: `/ai/2026/07/10/PINOCCHIO-OAUTH-PROFILE-LIFECYCLE/Pinocchio OAuth Profile Lifecycle Guide.pdf`.
