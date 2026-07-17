# Changelog

## 2026-07-10

- Initial workspace created


## 2026-07-10

Created an evidence-backed intern guide and phased implementation plan for typed OAuth profile state, secure YAML persistence, browser PKCE login, and Geppetto source injection.

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/cmds/profilebootstrap/engine_settings.go — Runtime injection evidence
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/configdoc/types.go — Profile extension evidence

## 2026-07-10

Validated and published the initial intern-oriented OAuth profile lifecycle guide bundle to /ai/2026/07/10/PINOCCHIO-OAUTH-PROFILE-LIFECYCLE after a successful dry run.

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/ttmp/2026/07/10/PINOCCHIO-OAUTH-PROFILE-LIFECYCLE--profile-backed-oauth-credentials-and-browser-login/design-doc/01-profile-oauth-credential-lifecycle-analysis-design-and-implementation-guide.md — Published intern guide
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/ttmp/2026/07/10/PINOCCHIO-OAUTH-PROFILE-LIFECYCLE--profile-backed-oauth-credentials-and-browser-login/reference/01-implementation-diary.md — Validation and delivery evidence

## 2026-07-10

Implemented the versioned OAuth profile parser and locked atomic direct-YAML credential store; changed pre-commit lint targets to retain the active workspace while preserving pre-push GOWORK=off isolation (code commits adea466 and de6517c).

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/Makefile — Workspace-compatible lint execution
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/oauthprofiles/store.go — Secure credential tuple persistence

## 2026-07-10

Resolved OAuth profiles only from direct YAML sources, rejected static-key conflicts, and injected Geppetto renewable bearer sources into default Pinocchio engine construction (commit 457c65d).

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/cmds/profilebootstrap/engine_settings.go — Default runtime integration
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/cmds/profilebootstrap/oauth.go — OAuth source identity and factory wiring

## 2026-07-10

Added the Glazed auth login verb with loopback PKCE/state/code exchange and secret-safe persistence; redacted OAuth extensions from config provenance and web profile API output; expanded concurrency and login-failure tests (commits e078a41, c307235, 839e8c2).

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/cmd/pinocchio/cmds/auth/login.go — Glazed browser login
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/configdoc/explain.go — Provenance redaction

## 2026-07-10

Validated OAuth work with logcopter and gosec; added a 5-second callback ReadHeaderTimeout after gosec G112. Full repository race remains blocked by unrelated web-chat/sessionstream races (commit 9265a71).

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/cmd/pinocchio/cmds/auth/login.go — Bound loopback callback header reads
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/cmd/web-chat/internal/appserver/server_test.go — Existing full-race blocker

## 2026-07-10

Completed OAuth extension merge, clone-isolation, malformed credential, and secret-free-error coverage (commit 467ab31).

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/configdoc/merge_test.go — Layered extension merge coverage
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/oauthprofiles/profile_test.go — Parser and redaction coverage

## 2026-07-13

Published OAuth login/migration help that prohibits refresh material in inference settings and documents the direct-YAML owner rule.

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/doc/topics/oauth-profile-login.md — OAuth operator guide

## 2026-07-14

Consumed Geppetto v0.13.6; focused and full standalone non-race Pinocchio tests now pass (commit ef035f5).

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/go.mod — Published Geppetto renewable bearer dependency
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/cmds/profilebootstrap/oauth.go — Runtime source integration validated against release

## 2026-07-16

Consumed released Geppetto v0.13.7, added OAuth source-precedence factory coverage, documented secure backup/recovery/migration and provider-contract boundaries, and validated standalone tests plus lint/logcopter/gosec/govulncheck.

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/go.mod — Release dependency update
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/cmds/profilebootstrap/oauth_test.go — Runtime integration regression test
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/doc/topics/oauth-profile-login.md — Operator lifecycle documentation

## 2026-07-16

Addressed PR #184 P1 review findings: inject the OAuth-aware factory into normal Pinocchio command execution and implement Windows OAuth registry locking; Linux tests and Windows cross-compilation pass.

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/cmds/cmd.go — Normal command runtime factory injection
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/oauthprofiles/lock_windows.go — Windows persistence lock

## 2026-07-16

Replaced untested Windows OAuth YAML locking/ACL support with an explicit fail-closed platform policy. Windows is rejected at store construction before browser login; POSIX behavior is unchanged. POSIX success-path tests are excluded on Windows, which has a dedicated unsupported-platform assertion. PR #186 remains unmerged for user-controlled merge.

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/oauthprofiles/platform_windows.go — Unsupported-platform rejection
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/oauthprofiles/store.go — Early platform validation
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/oauthprofiles/platform_windows_test.go — Windows unsupported-platform contract test

## 2026-07-17

Reconciled completed offline OAuth lifecycle coverage, separated approval-gated live smoke, added failed-write cleanup coverage, and prepared a validated Pinocchio v0.11.6 release handoff without tagging or publishing.

### Related Files

- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/changelog.md — Release notes
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/pkg/oauthprofiles/store_test.go — Persistence failure coverage
- /home/manuel/workspaces/2026-07-10/refresh-oauth-token-geppetto/pinocchio/ttmp/2026/07/10/PINOCCHIO-OAUTH-PROFILE-LIFECYCLE--profile-backed-oauth-credentials-and-browser-login/reference/02-pinocchio-v0-11-6-release-handoff.md — Release checklist
