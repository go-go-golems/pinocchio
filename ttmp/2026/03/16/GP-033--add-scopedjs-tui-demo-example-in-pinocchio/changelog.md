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
- Committed the command-wiring checkpoint as `7313e2b` (`feat(scopedjs-demo): wire pinocchio command shell`).
- Replaced the no-op renderer registration with scopedjs-specific tool-call and tool-result renderers that show JavaScript source, optional eval input payloads, console output, structured result summaries, and fallback YAML for unexpected result shapes.
- Added focused renderer tests and re-verified the package with `go test ./cmd/examples/scopedjs-tui-demo` plus `go run ./cmd/examples/scopedjs-tui-demo --list-workspaces`.
- Committed the renderer checkpoint as `2f7be40` (`feat(scopedjs-demo): render eval calls and results`).
- Polished the demo runtime after an interactive TUI pass by pre-creating the `dashboard/` directory in temp workspaces and sanitizing fake route payloads so callback-style route registrations render as stable structured output instead of leaking raw function values into the result.
- Expanded the example README with run instructions, fixture descriptions, and prompt suggestions aimed at composed runtime usage.
- Added direct tests for callback-style route registration and non-empty JavaScript error reporting from the scoped eval path.
- Manually validated the demo with `go run ./cmd/examples/scopedjs-tui-demo --workspace apollo`, including:
  - a successful note-plus-route flow driven by the prompt `Use the JavaScript tool to create a dashboard note with require("obsidian").createNote from the open tasks, and register a /tasks route using the open task list as plain JSON data, not a callback. Return the note path and routes.`
  - a missing-file/error flow driven by the prompt `Try to read dashboard/missing.md, explain the failure cleanly, and do not invent a successful write if it fails.`
- Re-verified the final slice with `go test ./cmd/examples/scopedjs-tui-demo` and `go run ./cmd/examples/scopedjs-tui-demo --list-workspaces`.
- Committed the final polish slice as `e65d08f` (`feat(scopedjs-demo): polish runtime behavior and demo guide`).
