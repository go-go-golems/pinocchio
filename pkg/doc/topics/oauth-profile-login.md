---
Title: "OAuth profile login and renewable credentials"
Slug: "oauth-profile-login"
Short: "Log into a profile with PKCE and keep renewable OAuth state in one owner-only registry."
Topics:
- oauth
- credentials
- profiles
- security
Commands:
- pinocchio
Flags:
- profile
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

Pinocchio owns the browser, loopback callback, and persistence lifecycle for OAuth profiles. Geppetto receives only a host-injected renewable bearer source; it does not read profile YAML, own browser state, or persist credentials.

## Configure one direct registry owner

An OAuth profile uses `extensions."pinocchio.oauth@v1"` in exactly one explicit direct YAML registry. The registry file must be mode `0600` and its parent must not be group- or world-writable. Inline Pinocchio profiles, composed registries, SQLite registries, and remote sources cannot own OAuth credentials because a refresh needs one auditable atomic-write target.

Do not put access tokens, refresh tokens, client secrets, or OAuth state in `inference_settings.api.api_keys`.

## Start browser login

Use the Glazed command:

```bash
pinocchio auth login --profile workspace/assistant
```

The command binds an exact loopback callback, uses PKCE S256 and state validation, exchanges the authorization code, and atomically saves the resulting tuple. It prints only sanitized status; it never prints authorization codes or credentials.

## Runtime behavior

At runtime Pinocchio resolves the typed extension, rejects an overlapping static provider key, creates Geppetto's renewable bearer source, and injects it into the factory. Proactive renewal and one bounded pre-stream 401 replay remain inside the Geppetto provider path. This integration targets Geppetto `v0.13.7` or newer.

The profile extension is deliberately provider-configured rather than a claim that every provider shares one OAuth contract. Before enabling a real provider, document its authorization endpoint, token endpoint, exact registered loopback redirect, scopes, public-client policy, and refresh-token rotation behavior. The repository's offline tests use synthetic issuers only; no real provider smoke is implied by this help entry.

The current JavaScript engine builder cannot receive this host-owned bearer source. Pinocchio must inject a Go-created engine for JavaScript execution rather than passing credential material through JavaScript.

## Backup, recovery, and migration

The direct registry is plaintext secret storage protected by filesystem ownership and mode `0600`; it is not encryption. Back up the file only through an equally protected private channel, never into logs, tickets, shell history, or shared configuration. Restore the file with the same owner-only permissions before using it.

`pinocchio auth logout --profile workspace/assistant` removes only the selected local credential tuple. It does not revoke a provider grant. If a provider-side revocation contract is later approved, perform that provider operation separately; otherwise use logout followed by browser login to recover locally. Static API-key-to-OAuth migration is explicit: remove the overlapping static key and add the typed OAuth extension rather than relying on automatic conversion.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| Login rejects the profile source | Profile is inline, composed, or non-YAML | Move the OAuth extension to one direct owner-only YAML registry. |
| Runtime rejects a static key | OAuth source and static key overlap | Remove the provider API key; the dynamic source is authoritative. |
| Login cannot persist credentials | File mode or parent directory is unsafe | Set the registry to `0600` and secure its parent directory. |
| JavaScript-built engine lacks OAuth | JS builder has no host bearer-source hook | Use a Go-created, source-injected engine. |

## See Also

- [Pinocchio profile resolution and runtime switching](pinocchio-profile-resolution-and-runtime-switching.md)
- [Migrating Legacy Pinocchio Config to Unified Profile Documents](../tutorials/08-migrating-legacy-pinocchio-config-to-unified-profile-documents.md)
- Geppetto `use-renewable-bearer-credentials` help entry
