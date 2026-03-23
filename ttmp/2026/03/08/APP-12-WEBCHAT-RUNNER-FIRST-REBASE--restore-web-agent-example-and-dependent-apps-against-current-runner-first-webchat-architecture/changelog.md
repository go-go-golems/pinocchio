# Changelog

## 2026-03-08

- Created ticket `APP-12-WEBCHAT-RUNNER-FIRST-REBASE`, the primary design doc, and the investigation diary.
- Confirmed from the GP-031 playbook that downstream fixes must preserve the runner-first `ChatService` / `ConversationService` split and the deps-first router/server constructors.
- Repaired local workspace composition by adding a root `go.work`, updating `wesen-os/go.work` to `go 1.26.1`, and mirroring the `github.com/go-go-golems/go-go-os-chat v0.0.0` local replacement semantics needed by the app stack.
- Validated successful builds for:
  - `web-agent-example`
  - `go-go-os-chat/pkg/...`
  - `wesen-os/pkg/assistantbackendmodule`
  - `wesen-os/workspace-links/go-go-app-inventory`
  - `wesen-os/workspace-links/go-go-app-arc-agi-3`
- Added `scripts/validate-workspace-builds.sh` so the repaired compile matrix can be replayed directly from the ticket workspace.

## 2026-03-08

Restored the local workspace overlays required by the current runner-first pinocchio branch, validated downstream builds, and documented the architecture and reasoning for future maintainers.

### Related Files

- /home/manuel/workspaces/2026-03-02/os-openai-app-server/go.work — Root workspace fix
- /home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/08/APP-12-WEBCHAT-RUNNER-FIRST-REBASE--restore-web-agent-example-and-dependent-apps-against-current-runner-first-webchat-architecture/design-doc/01-runner-first-webchat-rebase-analysis-and-implementation-guide.md — Primary design and implementation guide
- /home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/08/APP-12-WEBCHAT-RUNNER-FIRST-REBASE--restore-web-agent-example-and-dependent-apps-against-current-runner-first-webchat-architecture/reference/01-investigation-diary.md — Chronological record of the investigation and validation
- /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/go.work — Nested workspace Go version fix

