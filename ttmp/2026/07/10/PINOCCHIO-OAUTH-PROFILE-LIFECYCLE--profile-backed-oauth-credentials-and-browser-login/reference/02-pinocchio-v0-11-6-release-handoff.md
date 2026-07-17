---
Title: Pinocchio v0.11.6 release handoff
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
Owners: []
RelatedFiles:
    - Path: repo://.github/workflows/release.yml
      Note: Tag-triggered artifacts and docs publication
    - Path: repo://.goreleaser.yaml
      Note: Snapshot and production artifact configuration
    - Path: repo://changelog.md
      Note: v0.11.6 release scope and validation boundary
    - Path: repo://pkg/doc/topics/oauth-profile-login.md
      Note: OAuth operator help exported to docs.yolo
    - Path: repo://pkg/oauthprofiles/store.go
      Note: POSIX credential persistence and Windows fail-closed boundary
ExternalSources: []
Summary: Validated release candidate scope, dry-run results, and manual tag/publish checklist for Pinocchio v0.11.6.
LastUpdated: 2026-07-17T11:28:36.612631046-04:00
WhatFor: Prepare and manually publish Pinocchio v0.11.6 after the OAuth lifecycle and dependency updates.
WhenToUse: Use before tagging v0.11.6 and while verifying its GitHub release and docs.yolo publication.
---


# Pinocchio v0.11.6 release handoff

## Goal

Provide a reviewable, copy/paste-ready handoff for publishing Pinocchio `v0.11.6` without conflating offline OAuth lifecycle validation with a real-provider login claim.

## Candidate scope

The candidate is the first patch release after `v0.11.5`. It includes:

- profile-backed OAuth login, status, logout, redaction, and POSIX YAML persistence;
- Geppetto `v0.13.7` renewable bearer integration for normal commands and Go-hosted JavaScript engines;
- Claude OAuth profile binding and static-key/source conflict enforcement;
- explicit fail-closed Windows behavior for OAuth YAML persistence;
- dependency updates for go-go-goja, sessionstream, Redis, tokenizer, and `golang.org/x` modules;
- GitHub Actions version updates.

The release does **not** claim a real-provider OAuth smoke. Provider selection, exact redirect registration, scopes, public-client policy, refresh semantics, account selection, and live smoke remain separately approval-gated.

## Validation completed during preparation

- `svu current` returned `v0.11.5`; `svu patch` returned `v0.11.6`.
- Focused OAuth/profile/auth tests passed under `GOWORK=off`.
- The merged PR series passed repository tests, lint, GoSec, govulncheck, dependency review, secret scanning, and CodeQL.
- `GOWORK=off make goreleaser` produced a `0.11.6-next` Linux snapshot, archive, DEB, and RPM.
- Help export produced a valid SQLite database containing `oauth-profile-login`.
- Windows OAuth persistence is intentionally unsupported; Windows-target test packages compile and contain an unsupported-platform assertion.

The known repository-wide race baseline in web-chat/appserver remains unrelated to OAuth. Focused OAuth race tests pass.

## Manual release checklist

Run only after the release-preparation PR is merged and local `main` is clean:

```bash
git switch main
git pull --ff-only origin main
git status --short
svu current
svu patch
GOWORK=off go test ./... -count=1
GOWORK=off make lint logcopter-check gosec govulncheck
rm -rf /tmp/pinocchio-docs-export
mkdir -p /tmp/pinocchio-docs-export
GOWORK=off go run ./cmd/pinocchio help export \
  --format sqlite \
  --output-path /tmp/pinocchio-docs-export/help.sqlite
sqlite3 /tmp/pinocchio-docs-export/help.sqlite \
  "SELECT slug,title FROM sections WHERE slug = 'oauth-profile-login';"
GOWORK=off make goreleaser
```

Expected version before tagging: `v0.11.6`.

Tag and publish are intentionally manual:

```bash
make tag-patch
git show --no-patch v0.11.6
make release
```

`make release` pushes tags and verifies module proxy resolution. The tag-triggered release workflow builds Linux and Darwin artifacts, signs checksums, publishes packages/Homebrew metadata, and then publishes the Pinocchio help database to docs.yolo.

## Post-publish verification

```bash
gh run list --workflow release.yml --limit 5
gh release view v0.11.6
curl -fsSL https://docs.yolo.scapegoat.dev/api/packages
```

Verify that:

1. the `v0.11.6` GitHub release and checksums exist;
2. Linux and Darwin artifacts are present;
3. the release workflow’s `publish-docs` job succeeded;
4. docs.yolo lists Pinocchio `v0.11.6`;
5. the OAuth help page is discoverable without exposing credential values.

## Rollback boundary

Do not move or reuse the tag if publication partially fails. Preserve the immutable tag, diagnose the failed workflow/job, and rerun the supported release continuation or publish job. Do not add a live credential test to release automation.

## Related

- `pkg/doc/topics/oauth-profile-login.md`
- `pkg/oauthprofiles/store.go`
- `.github/workflows/release.yml`
- `.goreleaser.yaml`
- `changelog.md`
