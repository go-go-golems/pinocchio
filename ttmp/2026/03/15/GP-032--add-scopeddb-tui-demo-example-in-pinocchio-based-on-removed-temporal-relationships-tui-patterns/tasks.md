# Tasks

## Completed

- [x] Confirm the historical temporal-relationships TUI surfaces that were removed.
- [x] Inspect the current Pinocchio TUI and example entry points.
- [x] Inspect the extracted `geppetto/pkg/inference/tools/scopeddb` API.
- [x] Create ticket `GP-032`.
- [x] Write an intern-facing analysis, design, and implementation guide.
- [x] Write a chronological investigation diary.

## Proposed implementation backlog

- [x] Create `pinocchio/cmd/examples/scopeddb-tui-demo/`.
- [x] Add `main.go`.
- [x] Add `dataset.go`.
- [x] Add `fake_data.go`.
- [x] Add `renderers.go`.
- [x] Add `README.md`.
- [x] Choose a fake-data domain with an obvious scope boundary.
- [x] Define `demoScope`.
- [x] Define `demoMeta`.
- [x] Write the demo schema SQL.
- [x] Define `AllowedObjects`.
- [x] Define `DefaultQuery` limits.
- [x] Implement the `Materialize` callback.
- [x] Add a basic dataset test.
- [x] Build the scoped in-memory SQLite DB at startup.
- [x] Register the query tool with `scopeddb.RegisterPrebuilt`.
- [x] Display or log `Meta` during startup.
- [x] Resolve effective engine settings from default config and optional profile flags.
- [x] Create the Watermill router and sink.
- [x] Create the reusable tool-loop backend.
- [x] Build the Bobatea chat model and register demo renderers.
- [x] Add a SQL-focused tool-call renderer.
- [x] Add a query-result table renderer.
- [x] Add a small status bar that surfaces `Meta`.
- [x] Document example prompts in a README.
- [x] Run `gofmt` on the new example.
- [x] Verify `go test ./cmd/examples/scopeddb-tui-demo`.
- [x] Verify `go build ./cmd/examples/scopeddb-tui-demo`.
- [x] Run `go run ./cmd/examples/scopeddb-tui-demo --list-accounts` as a non-UI smoke check.
- [ ] Run the interactive TUI manually and confirm the query flow is legible.
