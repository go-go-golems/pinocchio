# Changelog

## 2026-05-07

Added devctl support for launching Pinocchio web-chat from the repository root and retained the existing `cmd/web-chat` workflow. The launcher now uses devctl profiles (`web-chat`, `web-chat-observe`), threads profile-aware web-chat flags from profile env into the backend command, builds the Go web-chat binary, prepares the Vite frontend, and returns backend/Vite service plans for devctl supervision.

Validation completed with `devctl profiles list`, `devctl plugins list`, `devctl plan --dry-run`, `devctl plan --profile web-chat-observe --dry-run`, `devctl up --dry-run --timeout 60s`, and a real `devctl up --force --timeout 180s` smoke that fetched `GET /api/chat/profiles` successfully before `devctl down`.

### Related Files

- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/.devctl.yaml — Root devctl profiles and plugin entry point.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/.devctl.yaml — Subdirectory devctl profiles and plugin entry point.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/plugins/webchat.py — devctl protocol plugin for config, validation, build, prepare, launch, and commands.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/.gitignore — Ignores root `.devctl/` state/logs.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/07/PINO-DEVCTL-WEBCHAT--add-devctl-support-for-launching-pinocchio-web-chat-with-profiles/design-doc/01-pinocchio-devctl-web-chat-launch-guide.md — Intern-facing analysis/design/implementation guide.
