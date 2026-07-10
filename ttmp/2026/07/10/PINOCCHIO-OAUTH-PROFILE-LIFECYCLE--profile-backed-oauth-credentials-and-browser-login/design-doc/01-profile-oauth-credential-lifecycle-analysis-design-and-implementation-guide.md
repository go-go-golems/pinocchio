---
Title: Profile OAuth credential lifecycle analysis design and implementation guide
Ticket: PINOCCHIO-OAUTH-PROFILE-LIFECYCLE
Status: active
Topics:
    - pinocchio
    - oauth
    - credentials
    - profiles
    - security
DocType: design-doc
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: repo://pkg/cmds/profilebootstrap/oauth.go
      Note: Direct source resolution and renewable bearer-source construction
    - Path: repo://pkg/oauthprofiles/refresher.go
      Note: OAuth protocol refresher adapter
    - Path: ws://geppetto/pkg/steps/ai/credentials/bearer.go
      Note: Reusable renewable source/store/refresher contracts
    - Path: ws://geppetto/pkg/steps/ai/credentials/oauth/oauth.go
      Note: Reusable pure OAuth protocol operations
    - Path: ws://pinocchio/Makefile
      Note: Pre-commit lint now retains workspace dependency resolution
    - Path: ws://pinocchio/cmd/pinocchio/main.go
      Note: CLI root command registration point for planned auth group
    - Path: ws://pinocchio/pkg/cmds/profilebootstrap/engine_settings.go
      Note: |-
        Resolved profile and source-aware engine factory integration point
        Default OAuth-aware engine factory integration
    - Path: ws://pinocchio/pkg/configdoc/profiles.go
      Note: Copies extensions into Geppetto engine profiles
    - Path: ws://pinocchio/pkg/configdoc/types.go
      Note: Inline profile extension model proposed as typed OAuth metadata transport
    - Path: ws://pinocchio/pkg/oauthprofiles/profile.go
      Note: Implemented versioned OAuth extension parser and redaction policy
    - Path: ws://pinocchio/pkg/oauthprofiles/store.go
      Note: Implemented locked atomic owner-only YAML credential persistence
ExternalSources:
    - https://github.com/go-go-golems/geppetto/issues/387
Summary: Intern-oriented design for Pinocchio profile-backed OAuth tokens, secure persistence, browser login, and Geppetto source injection.
LastUpdated: 2026-07-10T23:35:00-04:00
WhatFor: Implement secure profile OAuth lifecycle support without placing secrets in Geppetto inference settings.
WhenToUse: Use before changing Pinocchio profile schemas, CLI bootstrap, profile display, OAuth login, or credential refresh behavior.
---




# Profile OAuth credential lifecycle: analysis, design, and implementation guide

## 1. Executive summary

Pinocchio currently resolves engine profiles and turns their `inference_settings` into Geppetto engines. That is appropriate for static provider API keys, but it cannot safely maintain an OAuth access token that expires, refreshes, or rotates its refresh token. The requested feature makes one Pinocchio profile capable of holding OAuth configuration plus an owner-readable access/refresh/expiry credential tuple, obtaining an initial grant through a browser, refreshing it durably, and injecting a renewable source into Geppetto at inference time.

The architecture is deliberately split across three layers. Geppetto now owns generic OAuth protocol mechanics and request-time bearer renewal. Pinocchio owns profile format, filesystem permissions, browser callback lifecycle, and source construction. llm-proxy is a separate server host that will need a vault-backed adapter; it must not read Pinocchio’s local profile file at request time. This ticket implements neither a provider-specific OAuth configuration nor llm-proxy vault support; it is the evidence-backed Pinocchio implementation plan.

A new engineer should retain one central invariant: **refresh/access material is secret state, not ordinary inference configuration**. It must never be copied into `InferenceSettings.APIKeys`, profile list/show output, debug reports, event metadata, logs, generated ticket content, or a reMarkable bundle. The profile file may store the secret tuple because that is an explicit user requirement, but it must be an opt-in `oauth_bearer` profile with strict mode, atomic write, redaction, and recovery behavior.

## 2. Scope, goals, and terms

### 2.1 Goals

1. Add an explicit OAuth-backed profile representation to Pinocchio.
2. Persist `access_token`, `refresh_token`, and `expires_at` together in a local owner-readable profile YAML file.
3. Provide `pinocchio auth login --profile <slug>` using Authorization Code with PKCE S256 and an exact loopback callback.
4. Build a Geppetto `credentials.RenewableBearerTokenSource` for an OAuth profile so Chat and Responses use renewable tokens at request time.
5. Persist a rotated refresh token before returning a replacement access token to Geppetto.
6. Preserve ordinary static API-key profiles and existing profile resolution behavior.

### 2.2 Explicit non-goals

- Do not put OAuth protocol, profile paths, browser handling, or YAML persistence into Geppetto.
- Do not automatically support every provider, Claude, Gemini, embeddings, or transcription in the first Pinocchio release.
- Do not make llm-proxy read `~/.config/pinocchio/profiles.yaml` while serving requests.
- Do not emit token values through stdout, YAML inspection, telemetry, errors, support bundles, tests, docs, or reMarkable uploads.
- Do not provide a remote/server-hosted callback flow in this ticket; the initial flow is a local CLI loopback callback.

### 2.3 Terms

- **Profile:** A named engine configuration resolved through Pinocchio’s inline or imported Geppetto registry machinery.
- **OAuth profile:** A profile opting into `kind: oauth_bearer`, with non-secret OAuth endpoint/client policy and secret token state.
- **Credential tuple:** `access_token`, `refresh_token`, and `expires_at`; it is updated atomically as one logical record.
- **Protocol client:** Geppetto’s `pkg/steps/ai/credentials/oauth` package. It performs standard PKCE/code exchange/refresh HTTP mechanics but knows no profile or browser.
- **Store/refresher/source:** The Geppetto contracts `credentials.Store`, `credentials.Refresher`, and `credentials.BearerTokenSource` used to load, refresh, persist, and inject a bearer token at request time.

## 3. Current-state architecture and evidence

### 3.1 Pinocchio has two profile shapes

`pkg/configdoc/types.go` defines Pinocchio’s layered config `Document` and `InlineProfile`. An inline profile contains display metadata, a stack, `InferenceSettings`, and an untyped `Extensions map[string]any`. `pkg/configdoc/profiles.go` copies that data into a Geppetto `EngineProfile`, including a deep copy of extensions. `pkg/configdoc/merge.go` recursively merges extensions across config layers.

This gives a natural metadata transport path, but it is not a secret store. Untyped extensions can appear in merged documents and provenance/debug code. An OAuth secret extension therefore needs a typed parser plus deliberate redaction before any profile inspection path renders it.

```text
Pinocchio config document
  profiles.<slug>.extensions
          |
          v
Geppetto EngineProfile.Extensions
          |
          v
resolved profile / bootstrap runtime
          |
          +--> typed Pinocchio OAuth resolver (new)
          +--> InferenceSettings merge (must NOT receive refresh token)
```

### 3.2 Existing engine creation has no credential source seam at Pinocchio level

`pkg/cmds/profilebootstrap/engine_settings.go` resolves a profile registry chain, resolves an engine profile, merges its inference settings, then calls `factory.NewStandardEngineFactory()` in `NewEngineFromResolvedCLIEngineSettingsWithFactory`. The function accepts a factory override, which is the useful extension point: a caller that has an OAuth source can construct `factory.NewStandardEngineFactory(factory.WithBearerTokenSource(source))`.

The existing resolved settings object retains both the merged `FinalInferenceSettings` and the `ResolvedEngineProfile`. A new OAuth resolver should inspect profile metadata from the resolved profile before engine construction. It should not serialize token state back into the final inference settings or use `APIKeys` as a cache.

### 3.3 Existing profile commands are inspection commands

`cmd/pinocchio/cmds/profiles/root.go` currently registers `list` and `show`. `cmd/pinocchio/main.go` adds the profiles command to the root command during normal command initialization. `cmd/pinocchio/cmds/profiles/list.go` and `show.go` obtain report data from the profile bootstrap/runtime machinery.

This means an OAuth implementation must audit and test both commands before putting secrets into profile YAML. Their current purpose is to inspect profile structure; their output must become secret-aware rather than assuming all extensions are safe to render.

### 3.4 Geppetto now supplies the reusable lower layer

The sibling Geppetto worktree provides these APIs:

```go
// pkg/steps/ai/credentials/bearer.go
type Store interface {
    Load(context.Context, credentials.Request) (credentials.Credential, error)
    Save(context.Context, credentials.Request, credentials.Credential) error
}

type Refresher interface {
    Refresh(context.Context, credentials.Request, credentials.Credential) (credentials.Credential, error)
}

source, err := credentials.NewRenewableBearerTokenSource(store, refresher)
factory := factory.NewStandardEngineFactory(
    factory.WithBearerTokenSource(source),
)
```

The same package implements an optional `UnauthorizedBearerTokenSource`: OpenAI Chat and Responses use it to force one refresh and replay only the first provider HTTP 401 before successful response/SSE output exists. Geppetto’s `credentials/oauth` package provides PKCE generation, authorization URL construction, code exchange, forced refresh grants, expiry normalization, rotation policy, and redacted errors.

## 4. Gap analysis

| Required capability | Current observed behavior | Required Pinocchio change |
| --- | --- | --- |
| Typed OAuth profile | Only untyped extensions and static `InferenceSettings` exist | Define/validate an `oauth_bearer` extension model |
| Durable token tuple | No profile credential write path | Add locked, atomic owner-only YAML credential store |
| Browser grant | No `auth` command group exists | Add CLI command, PKCE/state, loopback listener, exchange, save |
| Runtime renewal | Bootstrap creates a factory with no source | Resolve OAuth metadata and inject a source into a source-aware factory |
| Secret-safe inspection | Profile commands report profile data | Redact OAuth secrets in list/show/provenance/debug output |
| Refresh rotation | Static configuration cannot replace a token | Save complete replacement tuple before source cache update |
| Writable source selection | Registry chain may compose inline/imported sources | Require a direct, local writable registry path for secret-bearing OAuth profiles |

## 5. Proposed architecture

### 5.1 Ownership diagram

```text
                         non-secret profile policy
+--------------------+  endpoints/client/scopes  +----------------------+
| Pinocchio YAML     | ------------------------> | Geppetto oauth.Client|
| credential store   |                            | PKCE/exchange/refresh|
| access/refresh/exp | <------------------------  +----------+-----------+
+----------+---------+      replacement tuple                |
           |                                             Credential
           | Store.Load / Save                                |
           v                                                   v
+---------------------------+                      +---------------------+
| RenewableBearerTokenSource| -------------------> | Standard engine     |
| cache/skew/singleflight   | request-time bearer  | factory + source    |
+---------------------------+                      +----------+----------+
                                                              |
                                                              v
                                                     OpenAI Chat/Responses

Browser login:
User browser <--> provider authorization endpoint <--> Pinocchio auth login
                                                    loopback callback only
```

### 5.2 Profile schema

Use one reserved typed extension namespace rather than adding secret fields to `InferenceSettings`:

```yaml
profiles:
  umans-base:
    display_name: Umans
    inference_settings:
      chat:
        api_type: openai
      api:
        base_urls:
          openai-base-url: https://provider.example.invalid/v1
    extensions:
      "pinocchio.oauth@v1":
        kind: oauth_bearer
        authorization_url: https://issuer.example.invalid/authorize
        token_url: https://issuer.example.invalid/token
        client_id: public-cli-client
        scopes: [inference]
        refresh_token_policy: preserve_previous
        access_token: <secret>
        refresh_token: <secret>
        expires_at: "2026-07-10T23:30:00Z"
```

The direct Geppetto YAML codec validates extension keys as `namespace.feature@vN`; implementation therefore uses `pinocchio.oauth@v1`, not the earlier nested `extensions.pinocchio.oauth` sketch. `pkg/oauthprofiles` now validates the public-client policy and token tuple. Runtime static-key conflict rejection remains to be wired before source injection.

### 5.3 Typed Pinocchio API sketch

```go
// pkg/oauthprofiles/types.go
type ProfileOAuthConfig struct {
    Provider                string
    AuthorizationURL        string
    TokenURL                string
    ClientID                string
    Scopes                  []string
    RefreshTokenPolicy      oauth.RefreshTokenPolicy
    AccessToken             string
    RefreshToken            string
    ExpiresAt               time.Time
}

type ProfileCredentialStore struct {
    RegistryPath string
    RegistrySlug string
    ProfileSlug  string
}

func ResolveOAuthProfile(ctx context.Context, runtime *profilebootstrap.ResolvedCLIEngineSettings) (*ProfileOAuthConfig, error)
func (s *ProfileCredentialStore) Load(ctx context.Context, request credentials.Request) (credentials.Credential, error)
func (s *ProfileCredentialStore) Save(ctx context.Context, request credentials.Request, replacement credentials.Credential) error
```

The store binds one source to exactly one registry/profile identity. `Load` and `Save` must verify that Geppetto’s requested provider/base URL is the expected profile endpoint before returning a token. This prevents an accidentally reused source from releasing a credential for a different provider route.

### 5.4 Runtime source construction

```go
resolved := profilebootstrap.ResolveCLIEngineSettings(ctx, values)
oauthProfile := oauthprofiles.ResolveOAuthProfile(ctx, resolved)

if oauthProfile == nil {
    return factory.NewStandardEngineFactory().CreateEngine(resolved.FinalInferenceSettings)
}

store := oauthprofiles.NewProfileCredentialStore(writableRegistry, oauthProfile.Identity())
client := oauth.NewClient(oauthProfile.ProtocolConfig())
refresher := oauthprofiles.NewRefresher(client)
source := credentials.NewRenewableBearerTokenSource(store, refresher)

f := factory.NewStandardEngineFactory(
    factory.WithBearerTokenSource(source),
)
return f.CreateEngine(withoutStaticProviderKey(resolved.FinalInferenceSettings))
```

`withoutStaticProviderKey` must remove only the OAuth profile’s applicable static key slot. It must not mutate the stored profile or unrelated provider keys. For an OAuth profile, the dynamic source is authoritative; an expired-source failure must not quietly fall back to stale static data.

## 6. Security and persistence design

### 6.1 File security invariants

1. A secret-bearing OAuth registry YAML must be mode `0600` on POSIX systems.
2. Its parent directory must not permit unsafe replacement by another user.
3. Temporary files and lock files must be owner-only.
4. Token tuples are written atomically: parse → modify only targeted fields → write temporary sibling → fsync → rename → parent fsync where supported.
5. No successful refresh is cached until the matching tuple is durably saved.
6. No token appears in errors; return category/operation only.

### 6.2 Atomic update pseudocode

```text
Save(profile identity, replacement):
  lock <registry>.oauth.lock
  verify registry mode == 0600
  document = parse registry YAML fresh from disk
  profile = locate exact registry/profile
  validate profile OAuth configuration still matches expected endpoint/client
  profile.extensions["pinocchio.oauth@v1"].access_token = replacement.access
  profile.extensions["pinocchio.oauth@v1"].refresh_token = replacement.refresh
  profile.extensions["pinocchio.oauth@v1"].expires_at = RFC3339 UTC(replacement.expiry)
  write YAML to same-directory temporary file mode 0600
  fsync temporary file
  rename temporary over registry
  fsync parent directory when available
  unlock
```

The reread under lock is required. Writing a cached in-memory document could discard unrelated profile edits made by another process. If Pinocchio’s existing YAML registry store already supplies compatible locking/durability, prefer adapting it after verification; do not assume it does.

### 6.3 Redaction policy

Secrets must be replaced with a stable marker such as `<redacted>` before:

- `pinocchio profiles list` detailed/full/JSON output;
- `pinocchio profiles show` output;
- config provenance/explain reports;
- debug logs and error wrapping;
- browser completion pages;
- test failure messages;
- ticket artifacts and reMarkable bundle inputs.

Redaction is defense in depth. The stronger control is to avoid passing the OAuth extension map to generic rendering surfaces at all.

## 7. Browser Authorization Code + PKCE flow

```text
pinocchio auth login --profile umans-base --profile-registries /abs/profiles.yaml
  |
  +-- resolve profile and require writable local registry
  +-- validate OAuth config and no unsafe file permissions
  +-- bind 127.0.0.1:0 before opening browser
  +-- generate state, PKCE verifier/challenge
  +-- call geppetto oauth.Client.AuthorizationURL(state, pkce)
  +-- open browser or print sanitized URL
  |
provider redirects exactly to http://127.0.0.1:<port>/oauth/callback
  |
  +-- require GET, exact path, one callback, non-empty code
  +-- constant-time compare callback state with pending state
  +-- reject provider error/state mismatch/timeout/duplicate
  +-- oauth.Client.ExchangeAuthorizationCode(ctx, code, pkce)
  +-- ProfileCredentialStore.Save(tuple)
  +-- render token-free success page and close listener
```

The command must bind loopback before building the redirect URL. It must not use a wildcard callback, accept a callback from a non-loopback host, or print authorization-code/token query values. A headless mode may print the authorization URL and wait for a local callback, but it may not weaken state or redirect validation.

## 8. Decision records

### Decision: Store OAuth token state in a typed profile extension

- **Context:** The user requires access/refresh/expiry data in profile YAML; `InferenceSettings` is runtime configuration and its API key maps are broadly consumed.
- **Options considered:** Add fields to `InferenceSettings`; reserve typed `extensions."pinocchio.oauth@v1"`; external sidecar/keyring only.
- **Decision:** Use a reserved typed extension for the first release, with a typed parser/store and strict redaction.
- **Rationale:** It travels with the profile while avoiding accidental engine configuration propagation.
- **Consequences:** Existing generic extension display paths require audit/redaction; future keyring migration can retain the non-secret OAuth policy fields.
- **Status:** proposed.

### Decision: Pinocchio owns browser and storage; Geppetto owns protocol mechanics

- **Context:** Geppetto is reusable across hosts, while profile locations, CLI UX, and redirect/client policy are application-specific.
- **Options considered:** Put all OAuth code in Geppetto; put all code in Pinocchio; split protocol from host lifecycle.
- **Decision:** Use Geppetto `credentials/oauth` for standard PKCE/code/refresh operations and Pinocchio for profile/browser/store behavior.
- **Rationale:** The split is reusable without making Geppetto depend on YAML or CLI packages.
- **Consequences:** Pinocchio needs thin adapters and explicit integration tests rather than duplicating protocol HTTP.
- **Status:** accepted.

### Decision: OAuth profiles require an explicit writable registry source

- **Context:** Pinocchio can compose inline and imported registry sources; not every resolved profile has a safe file to mutate.
- **Options considered:** Write to whichever source supplied the field; write an override automatically; require an explicit local registry path.
- **Decision:** Require a direct local writable registry path for `auth login` and refresh persistence in the first release.
- **Rationale:** It makes file permissions, locking, provenance, and user intent auditable.
- **Consequences:** Imported/read-only profiles must be copied or explicitly overridden before login.
- **Status:** proposed.

### Decision: Refresh persists before cache update

- **Context:** Refresh-token rotation can invalidate the prior refresh token immediately.
- **Options considered:** Use new access token then save asynchronously; save first; never persist automatically.
- **Decision:** Reuse Geppetto’s save-before-cache source invariant.
- **Rationale:** A process restart cannot strand the user with an old refresh token after a successful in-memory refresh.
- **Consequences:** Profile writes occur on refresh paths and must be bounded, atomic, and concurrency-safe.
- **Status:** accepted.

## 9. Implementation plan

### Phase 0 — Discovery and contracts

- Confirm the exact on-disk YAML representation for direct Geppetto profile registries versus inline Pinocchio config profiles.
- Trace all list/show/provenance/debug output surfaces that can expose extensions.
- Select the first provider and record documented authorization URL, token URL, scopes, redirect URI, client authentication, and refresh rotation behavior.
- Define fixtures containing fake tokens only.

### Phase 1 — Typed OAuth profile model and redaction

- Add `pkg/oauthprofiles` typed parsing/validation for the reserved extension.
- Reject conflicting static-key and OAuth credential configurations.
- Add redaction helpers and wire them through profile inspection/reporting.
- Test layered extension merge behavior and no-secret output.

### Phase 2 — Writable YAML credential store

- Define direct writable registry selection.
- Implement lock/mode/check/read/patch/atomic-rename/fsync logic.
- Implement Geppetto `credentials.Store` and request identity validation.
- Test permissions, concurrent saves, refresh rotation, failed save recovery, and unrelated-profile preservation.

### Phase 3 — Source-aware Pinocchio runtime

- Bind a Geppetto OAuth client to the typed profile config.
- Implement a thin Geppetto `credentials.Refresher` adapter.
- Extend profile bootstrap/runtime engine construction to use `factory.WithBearerTokenSource` for OAuth profiles.
- Test proactive refresh and one bounded provider-401 replay against a fake OpenAI-compatible endpoint.

### Phase 4 — Browser login command

- Add `cmd/pinocchio/cmds/auth` and register it from `cmd/pinocchio/main.go`.
- Implement `auth login` with PKCE/state, loopback callback, code exchange, timeout/cancellation, and sanitized output.
- Test callback error, wrong path, state mismatch, duplicate callback, timeout, and success using a fake provider.

### Phase 5 — Operations and migration

- Document secure file permissions, backup handling, revoke/re-login, and profile migration.
- Add a non-destructive migration/repair command if needed.
- Run real-provider smoke only with local files excluded from Git, logs, and documentation uploads.

## 10. Testing and validation strategy

| Layer | Essential tests |
| --- | --- |
| Profile parser | valid config, invalid URLs, missing client ID, invalid expiry, conflicting static key, unknown kind |
| Extension merge | inherited policy, overridden token tuple, deep-copy/no aliasing, redacted report |
| Credential store | `0600` enforcement, load/save, tuple atomicity, lock behavior, write failure, profile identity mismatch |
| OAuth adapter | fake code exchange, forced refresh despite future expiry, preserve/require rotation policy, endpoint errors redacted |
| Browser command | PKCE parameters, state mismatch, loopback-only bind, wrong path, duplicate callback, timeout, success |
| Runtime | source overrides static key, source failure does not fall back, near-expiry refresh, first 401 replay, second 401 stop |
| Regression | existing Pinocchio configdoc/profile bootstrap/CLI tests and full race/lint/security suite |

Required validation commands will be finalized after implementation, but the intended baseline is:

```bash
cd /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio
gofmt -w <changed-go-files>
GOWORK=off go test ./... -count=1
GOWORK=off go test -race ./... -count=1
GOWORK=off make lint logcopter-check
GOWORK=off make gosec
```

## 11. Risks, alternatives, and open questions

1. **YAML comment/order preservation:** The chosen codec may rewrite formatting. Determine whether this is acceptable before using it as the secret store.
2. **OS portability:** `0600`, file locks, and directory fsync have different behavior outside POSIX. Define an explicit first-platform support policy.
3. **Provider variation:** Standard OAuth support does not prove a provider permits loopback redirects or public clients. Do not guess endpoint parameters.
4. **External registry sources:** A profile may be resolved from a read-only/imported/stacked source. The first release must fail clearly rather than silently create an override elsewhere.
5. **Static-key migration:** A migration needs an explicit command/operator action; automatic conversion could leave duplicate credentials.
6. **Plaintext-at-rest tradeoff:** `0600` YAML is user-requested but not encrypted. Keep the Store abstraction so a keyring/encrypted backend can be added later.
7. **llm-proxy distinction:** A server’s per-user OAuth bundles belong in its encrypted vault, not in Pinocchio’s local profile file. Its adapter will reuse Geppetto primitives but is separate work.

## 12. References

### Pinocchio

- `pkg/configdoc/types.go` — inline profile model and extension storage.
- `pkg/configdoc/merge.go` — extension merge behavior.
- `pkg/configdoc/profiles.go` — conversion from inline profiles to Geppetto registry profiles.
- `pkg/cmds/profilebootstrap/engine_settings.go` — profile resolution and standard engine factory construction.
- `pkg/cmds/profilebootstrap/profile_selection.go` — registry chain resolution.
- `cmd/pinocchio/cmds/profiles/root.go` — existing profile command group.
- `cmd/pinocchio/cmds/profiles/list.go` and `show.go` — profile inspection output surfaces.
- `cmd/pinocchio/main.go` — root command registration.

### Geppetto

- `pkg/steps/ai/credentials/bearer.go` — store/refresher/source contracts and renewable token behavior.
- `pkg/steps/ai/credentials/oauth/oauth.go` — PKCE, code exchange, forced refresh, and rotation policy.
- `pkg/inference/engine/factory/factory.go` — bearer-source factory option.
- `pkg/steps/ai/openai/chat_stream.go` and `pkg/steps/ai/openai_responses/streaming.go` — bounded 401 replay behavior.
