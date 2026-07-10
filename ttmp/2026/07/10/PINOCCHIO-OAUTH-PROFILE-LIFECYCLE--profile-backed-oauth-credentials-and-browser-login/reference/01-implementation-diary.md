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
    - Path: ws://pinocchio/lefthook.yml
      Note: Pre-push remains intentionally standalone-module isolated
    - Path: ws://pinocchio/pkg/cmds/profilebootstrap/engine_settings.go
      Note: Observed engine construction boundary
    - Path: ws://pinocchio/pkg/configdoc/types.go
      Note: Observed profile extension boundary
    - Path: ws://pinocchio/pkg/oauthprofiles/profile_test.go
      Note: Typed extension and redaction validation coverage
    - Path: ws://pinocchio/pkg/oauthprofiles/store_test.go
      Note: Credential persistence security and rotation coverage
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

- Review the implemented `extensions."pinocchio.oauth@v1"` schema and its profile-output redaction before broader runtime integration.
- Review whether plaintext owner-only YAML meets the intended local threat model.

### What should be done in the future

- Begin Phase 0 writable-registry and profile-output discovery.
- Re-upload an updated bundle after implementation phases materially change the guide or diary.

### Code review instructions

Review the uploaded PDF’s architecture and decision-record sections first, then validate the local ticket with the doctor command above.

### Technical details

Bundle destination: `/ai/2026/07/10/PINOCCHIO-OAUTH-PROFILE-LIFECYCLE/Pinocchio OAuth Profile Lifecycle Guide.pdf`.

## Step 3: Add typed OAuth profile state and secure direct-YAML persistence

The first code slice makes OAuth profile data concrete without placing any token in Geppetto inference settings. `pkg/oauthprofiles` parses the profile’s versioned extension, validates public PKCE-client policy, redacts secret fields for future inspection paths, and exposes a profile-bound Geppetto `credentials.Store` implementation.

The persistence target is an explicit direct Geppetto YAML registry, not an inline/composed/SQLite registry. The store is bound to one registry slug, profile slug, and expected provider/base URL. It locks the registry, rereads it, updates only the credential tuple, writes a `0600` temporary sibling, fsyncs, renames, and syncs the parent directory before a renewable source would cache the replacement.

### Prompt Context

**User prompt (verbatim):** "Ok, implement it all, task by task, keeping a detailed diary as you work, committing at appropriate intervals."

**Assistant interpretation:** Implement the planned Pinocchio OAuth lifecycle incrementally, test each slice, maintain ticket documentation, and create focused commits.

**Inferred user intent:** Produce a reviewable, secure implementation rather than only a design, while retaining an auditable step-by-step engineering record.

**Commit (code):** adea466b7ba14984bb16948b0390eba20461e5a6 — "feat: add secure OAuth profile YAML store"

### What I did

- Added `pkg/oauthprofiles/profile.go` with the typed `pinocchio.oauth@v1` extension parser, public-client validation, expiry parsing, explicit refresh-token policy, protocol-client configuration, and redaction helper.
- Added `pkg/oauthprofiles/store.go` plus Unix/non-Unix lock implementations. `YAMLStore` implements Geppetto `credentials.Store` and verifies the exact selected registry/profile/provider/base URL before load or save.
- Added focused parser, redaction, mode, request-identity, tuple-rotation, unrelated-profile-preservation, and race tests.
- Ran `go test ./pkg/oauthprofiles -count=1`, `go test -race ./pkg/oauthprofiles -count=1`, `make lintmax`, and `make test` after the workspace issue was resolved.

### Why

- Geppetto validates extension identifiers as `namespace.feature@vN`; a nested unversioned `extensions.pinocchio.oauth` payload cannot be persisted in a direct registry.
- Refresh state must have a single durable owner. Direct YAML provides a clear path and profile identity, unlike a composed runtime registry.
- Binding storage to the outbound request prevents a source configured for one OpenAI-compatible provider endpoint from being reused for another.

### What worked

- The direct YAML codec preserves the extension payload through decode/encode and validates the versioned extension key.
- The store enforces registry-file mode `0600`, rejects a group/world-writable directory, locks with `flock` on Unix, and atomically replaces the file after `fsync`.
- Focused normal and race tests passed. The complete workspace lint/test commands also passed after Step 4's build change.

### What didn't work

The original schema draft used an invalid nested extension key. Geppetto rejected the test registry with this exact error:

```text
engine profile YAML registry validation failed: validation error (profile.extensions[pinocchio]): extension key "pinocchio" is invalid (expected namespace.feature@vN)
```

The implementation was changed to a single versioned key: `extensions."pinocchio.oauth@v1"`.

### What I learned

- Geppetto engine-profile YAML enforces a versioned extension-key grammar even when a host owns the payload schema.
- Existing Geppetto `YAMLFileEngineProfileStore` writes `0644` and lacks the required cross-process locking/durability semantics, so it cannot be reused for secret-bearing profile state.
- Direct registry persistence can be implemented without modifying Geppetto or storing refresh material in `InferenceSettings.APIKeys`.

### What was tricky to build

The hard edge was preventing a refresh write from racing with a profile edit or from silently updating a similarly shaped but different profile. The store treats a credential as a tuple and takes an advisory sibling lock before rereading the file. It then verifies the registry slug, profile slug, typed OAuth extension, and normalized request identity before encoding and atomically replacing the target. This avoids using stale in-memory YAML and prevents cross-provider credential release.

### What warrants a second pair of eyes

- Review the POSIX-only `flock` support policy and whether Windows needs a native lock implementation instead of the intentional unsupported error.
- Review the plaintext-at-rest threat model despite the required `0600` file mode.
- Review direct-registry source resolution when it is wired into runtime construction; the store currently receives an explicit path and must not be pointed at a composed source.

### What should be done in the future

- Wire typed profile resolution to profile bootstrap and Geppetto’s renewable bearer source.
- Add profile-output redaction integration tests and direct-registry source selection.
- Add browser login only after the runtime/store source has an explicit selected profile identity.

### Code review instructions

Start with `pkg/oauthprofiles/profile.go`, then review `YAMLStore.Save` and its request/file-security guards in `pkg/oauthprofiles/store.go`. Run the focused normal/race tests, followed by `make lintmax` and `make test` from the Pinocchio worktree.

### Technical details

The store uses fake credentials only in tests. No real token, authorization code, client secret, or local profile content was read or written during implementation.

## Step 4: Keep the pre-commit validation in the active workspace

Pinocchio’s pre-commit hook calls `make lintmax`, and that target forced one Glazed lint invocation outside the workspace. The new OAuth package depends on already-implemented Geppetto packages in the sibling workspace, so that forced module mode incorrectly made the staged code appear to have missing imports.

The lint targets now inherit the caller’s workspace. This fixes local pre-commit validation without weakening the pre-push policy: `lefthook.yml` still explicitly runs the release/lint/test pre-push commands with `GOWORK=off`.

### Prompt Context

**User prompt (verbatim):** "remove the GOWORK=off from the pre-commit hook"

**Assistant interpretation:** Make the pre-commit lint path use the workspace rather than forcing standalone module resolution.

**Inferred user intent:** Allow the Pinocchio worktree to validate integrations with sibling workspace modules during normal local commits.

**Commit (code):** de6517cd4dd0700a8dbc0889ad234c47cc3269c3 — "build: keep workspace active for lint"

### What I did

- Removed `GOWORK=off` from the Glazed-lint commands used by `lint`, `lintmax`, and `glazed-lint` in `Makefile`.
- Kept the `GOWORK=off` pre-push/release commands in `lefthook.yml` unchanged.
- Ran `make lintmax` and `make test`, then committed the build change before committing the OAuth-store code.

### Why

The pre-commit hook should validate the checked-out workspace composition. It must not claim a local sibling dependency is absent when that sibling is intentionally included in the repository’s checked-in `go.work`.

### What worked

The rerun passed all lint and test stages. The OAuth code commit then passed the actual pre-commit hook, including code generation, workspace build, golangci-lint, Geppetto lint, Glazed lint, and the full test suite.

### What didn't work

Before the change, the hook failed during Glazed lint with:

```text
pkg/oauthprofiles/profile.go:14:2: no required module provides package github.com/go-go-golems/geppetto/pkg/steps/ai/credentials; to add it:
	go get github.com/go-go-golems/geppetto/pkg/steps/ai/credentials
```

This was a workspace-mode mismatch, not an absent local implementation.

### What I learned

`lefthook.yml` did not itself force workspace-off pre-commit execution; the failing command was inside the `lintmax` Makefile target. Removing that target-local override keeps pre-push’s stronger isolated-module check available.

### What was tricky to build

The sharp edge was scope: blindly removing every `GOWORK=off` occurrence would change release and pre-push guarantees. The implemented change is limited to the lint targets reached by pre-commit, leaving the explicit pre-push isolation unchanged.

### What warrants a second pair of eyes

- Confirm the intended policy is indeed workspace-based pre-commit and standalone-module pre-push.
- Resolve/publish the Geppetto OAuth dependency before treating the pre-push check as expected to pass for this branch.

### What should be done in the future

- Continue runtime source injection and browser login implementation.
- Before publishing Pinocchio independently, update the Geppetto module requirement to a released revision containing the credential/OAuth packages.

### Code review instructions

Inspect the three modified Makefile lint invocations and `lefthook.yml` pre-push section. Validate locally with `make lintmax && make test`.

### Technical details

No credential data was involved in the build change.
