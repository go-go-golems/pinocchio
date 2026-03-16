# Changelog

## 2026-03-16

- Initial workspace created
- Inspected the existing `scopeddb` TUI demo in Pinocchio plus the new `scopedjs` package and runnable examples in Geppetto.
- Chose the recommended demo shape: a scoped project-ops runtime with `fs`, fake `db`, fake `obsidian`, fake `webserver`, and bootstrap helpers, exposed as one eval tool.
- Added a detailed recommendation and implementation plan document.
- Added a long intern-facing architecture, design, and implementation guide.
- Added an investigation diary capturing the repository reads and design decisions.
- Uploaded the bundled ticket packet to reMarkable as `GP-033 scopedjs tui demo plan` under `/ai/2026/03/16/GP-033`.
- Started implementation in `pinocchio/cmd/examples/scopedjs-tui-demo/` with deterministic fixtures, workspace materialization, a first `scopedjs` environment builder, direct runtime smoke tests, and placeholder `main.go` plus renderer entry points.
- Updated the shared workspace with `go work use .`, which raised `go.work` to `go 1.26.1` and restored local sibling-module resolution for `pinocchio` against the new `geppetto/pkg/inference/tools/scopedjs` package.
- Committed the initial scaffold and runtime checkpoint as `61a1b61` (`feat(scopedjs-demo): scaffold runtime fixtures and smoke tests`) after the repo pre-commit hook passed full test and lint checks.
- Replaced the placeholder demo entry point with the real Pinocchio command shell, including profile resolution, registry/bootstrap wiring, the event router, the Bubble Tea model, a demo-specific system prompt, and a workspace-oriented status bar.
- Verified the command wiring with `go test ./cmd/examples/scopedjs-tui-demo` and `go run ./cmd/examples/scopedjs-tui-demo --list-workspaces`.
