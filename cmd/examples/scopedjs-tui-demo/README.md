# Scopedjs TUI Demo

This example shows how Pinocchio can drive one scoped JavaScript tool, `eval_project_ops`, through the normal chat loop and render the tool activity in a readable timeline.

The demo is intentionally fake but concrete:

- the workspace is materialized into a temp directory so file writes are real,
- `db` exposes deterministic task and note data,
- `require("obsidian")` creates markdown notes,
- `require("webserver")` records routes instead of opening sockets,
- `require("fs")` is the real native filesystem module from `go-go-goja`.

## What it shows

In one tool call, the model can combine:

- scoped data access from `db`,
- real file I/O through `fs`,
- note creation through `obsidian`,
- route registration through `webserver`.

The timeline should show:

- the tool call as fenced JavaScript,
- any auxiliary tool input payload,
- structured tool results such as note paths, routes, rows, and console output,
- then the assistant's plain-English explanation.

## Run

List available fixture workspaces:

```bash
go run ./cmd/examples/scopedjs-tui-demo --list-workspaces
```

Start the TUI against the default `apollo` workspace:

```bash
go run ./cmd/examples/scopedjs-tui-demo
```

Start the TUI against a specific workspace:

```bash
go run ./cmd/examples/scopedjs-tui-demo --workspace mercury
```

If you rely on a profile registry instead of base `PINOCCHIO_*` settings, pass it explicitly:

```bash
go run ./cmd/examples/scopedjs-tui-demo \
  --profile demo-openai \
  --profile-registries sqlite:profiles.db
```

## Suggested prompts

- `Summarize the open tasks and tell me which one looks most urgent.`
- `Create a dashboard note from the open tasks, save it in the workspace, and mention the note path.`
- `Register a /tasks route that returns the current open tasks and tell me which route path you created.`
- `Use the JavaScript tool to create a note and a /tasks route in one step, then return both the note path and the registered routes.`
- `Try to read dashboard/missing.md, explain the failure cleanly, and do not invent a successful write if it fails.`

## Workspace fixtures

- `apollo`: dashboard-refresh flavored notes, tasks, and route config
- `mercury`: notes-cleanup flavored notes, tasks, and plan docs

Each run creates a fresh temp workspace and deletes it on exit.

## Files to read first

- `fake_data.go`: fixture data and temp workspace materialization
- `environment.go`: scopedjs environment definition, fake modules, and direct eval helpers
- `renderers.go`: tool-call and tool-result timeline rendering
- `main.go`: Bubble Tea shell, backend wiring, and status bar
