# Scopeddb TUI Demo

This example is a small Bubble Tea application that demonstrates how to expose a scoped read-only SQLite snapshot as a Geppetto tool using `geppetto/pkg/inference/tools/scopeddb`.

## What it shows

- a typed `scopeddb.DatasetSpec`
- fake account-scoped support-ticket data
- `Meta` returned from snapshot materialization
- tool registration with `scopeddb.RegisterPrebuilt`
- a Bobatea TUI that shows:
  - assistant text
  - tool calls with highlighted SQL
  - tool results rendered as a compact table

## Run

If your default Pinocchio config already resolves an engine:

```bash
go run ./cmd/examples/scopeddb-tui-demo
```

If you want to use an explicit profile registry:

```bash
go run ./cmd/examples/scopeddb-tui-demo \
  --profile-registries /path/to/profiles.yaml \
  --profile default
```

Choose a different fake account:

```bash
go run ./cmd/examples/scopeddb-tui-demo --account northwind
```

List available fixture accounts:

```bash
go run ./cmd/examples/scopeddb-tui-demo --list-accounts
```

## Suggested prompts

- `List the open tickets ordered by newest first.`
- `Show the event timeline for ACME-102.`
- `Which issue is the highest priority right now?`
- `Summarize what changed most recently for this account.`

## Files

- `dataset.go` defines the scoped database schema, scope type, meta type, and materializer.
- `fake_data.go` contains the literal fixtures.
- `renderers.go` renders SQL inputs and query results in a TUI-friendly way.
- `main.go` wires the engine, tool registry, event router, backend, and Bubble Tea program.
