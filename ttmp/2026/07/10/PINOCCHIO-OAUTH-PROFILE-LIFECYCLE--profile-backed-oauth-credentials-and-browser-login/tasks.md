# Tasks

## Phase 0 — Architecture discovery and design

- [x] Confirm direct Geppetto registry YAML versus inline Pinocchio config YAML formats and choose the writable OAuth profile source <!-- t:p0a1 -->
- [x] Map profile extension transport, merge behavior, bootstrap engine construction, profile CLI inspection, and Geppetto OAuth/source APIs <!-- t:p0a2 -->
- [x] Write the intern-oriented analysis/design/implementation guide and implementation diary <!-- t:p0a3 -->
- [x] Identify all list/show/provenance/debug output paths that require OAuth secret redaction <!-- t:p0a4 -->
- [ ] Select the initial provider and document authorization/token endpoint, loopback redirect, scopes, client auth, and refresh-token rotation contract <!-- t:p0a5 -->

## Phase 1 — Typed OAuth profile model and redaction

- [x] Add typed `extensions."pinocchio.oauth@v1"` parser/validator and profile identity model <!-- t:p1a1 -->
- [x] Define static-key/OAuth conflict and migration behavior <!-- t:p1a2 -->
- [x] Add redacted profile display/provenance behavior and tests <!-- t:p1a3 -->
- [x] Test layered extension merge, clone behavior, invalid configuration, and secret-free errors <!-- t:p1a4 -->

## Phase 2 — Secret-safe YAML credential store

- [x] Implement explicit writable local registry selection and owner-only permission validation <!-- t:p2a1 -->
- [x] Implement locked read/patch/write with temporary `0600` file, fsync, rename, and cleanup <!-- t:p2a2 -->
- [x] Implement Geppetto `credentials.Store` with provider/base-URL identity checks <!-- t:p2a3 -->
- [ ] Test tuple rotation, concurrent saves, failed writes, unsafe permissions, and unrelated-profile preservation <!-- t:p2a4 -->

## Phase 3 — Runtime credential source integration

- [x] Bind typed profile config to Geppetto `credentials/oauth.Client` and a `credentials.Refresher` adapter <!-- t:p3a1 -->
- [x] Build/inject `RenewableBearerTokenSource` through the Pinocchio source-aware engine factory <!-- t:p3a2 -->
- [x] Ensure OAuth profiles never pass refresh material through `InferenceSettings.APIKeys` <!-- t:p3a3 -->
- [ ] Test proactive refresh, source precedence, first-401 replay, second-401 stop, and redaction <!-- t:p3a4 -->

## Phase 4 — Browser login command

- [x] Add and register `pinocchio auth` command group <!-- t:p4a1 -->
- [x] Implement `auth login --profile` PKCE/state/loopback callback flow <!-- t:p4a2 -->
- [x] Exchange authorization code, persist tuple, and show sanitized success/recovery output <!-- t:p4a3 -->
- [x] Test timeout, cancellation, wrong path, duplicate callback, provider error, state mismatch, and success <!-- t:p4a4 -->

## Phase 5 — Operations, validation, and delivery

- [x] Document permissions, backup/recovery, revoke/re-login, migration, and plaintext-at-rest tradeoffs <!-- t:p5a1 -->
- [ ] Run focused/full/race/lint/logcopter/security validation and a secret-safe real-provider smoke <!-- t:p5a2 -->
- [ ] Relate files, update diary/changelog, run `docmgr doctor`, and deliver an updated review bundle <!-- t:p5a3 -->
