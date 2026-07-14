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
