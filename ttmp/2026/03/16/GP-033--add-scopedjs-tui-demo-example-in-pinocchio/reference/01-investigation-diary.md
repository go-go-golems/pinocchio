---
Title: Investigation diary
Ticket: GP-033
Status: active
Topics:
    - pinocchio
    - ui
    - js-bindings
    - tools
    - architecture
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/README.md
      Note: First concrete reference for the shape and teaching goals of the new demo
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/main.go
      Note: Main Bubble Tea wiring precedent
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/renderers.go
      Note: Timeline renderer precedent for custom tool visualization
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/tool.go
      Note: Primary reusable registration surface the demo should exercise
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/eval.go
      Note: Eval contract and console-capture behavior that the demo renderer should surface
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/cmd/examples/scopedjs-dbserver/main.go
      Note: Small composed scopedjs example used as the closest non-TUI seed
ExternalSources: []
Summary: Short diary of the initial GP-033 investigation and why the recommended demo is a fake but concrete project-ops runtime rather than a real webserver or pure filesystem example.
LastUpdated: 2026-03-16T16:40:00-04:00
WhatFor: Preserve the concrete file reads and design decisions that shaped the scopedjs TUI demo recommendation.
WhenToUse: Use when continuing GP-033 implementation or reviewing why this demo was scoped the way it was.
---

# Investigation diary

## Goal

Create a new Pinocchio ticket for a `scopedjs` TUI demo, analyze the existing demo precedent, and write enough design material that a new intern can implement it without re-discovering the architecture from scratch.

## What I inspected first

- `pinocchio/cmd/examples/scopeddb-tui-demo/README.md`
- `pinocchio/cmd/examples/scopeddb-tui-demo/main.go`
- `pinocchio/cmd/examples/scopeddb-tui-demo/dataset.go`
- `pinocchio/cmd/examples/scopeddb-tui-demo/renderers.go`
- `pinocchio/pkg/ui/backends/toolloop/backend.go`
- `pinocchio/pkg/ui/forwarders/agent/forwarder.go`
- `geppetto/pkg/inference/tools/scopedjs/schema.go`
- `geppetto/pkg/inference/tools/scopedjs/tool.go`
- `geppetto/pkg/inference/tools/scopedjs/eval.go`
- `geppetto/cmd/examples/scopedjs-dbserver/main.go`
- `geppetto/ttmp/2026/03/15/GP-34--create-reusable-scoped-javascript-tool-runtime-for-llm-eval/index.md`

## Main findings

- The existing `scopeddb` demo is valuable because it is not only a package example. It is also a UI teaching tool. The new `scopedjs` demo should serve the same purpose.
- The right mental model is not "show raw eval JSON." The right mental model is "show a composed runtime in action," which means the timeline needs custom renderers for JavaScript input and structured eval output.
- A real server or a real Obsidian process would make the first demo noisy and brittle. Fake or test-double modules are the correct choice for a teaching demo.
- A pure `fs` demo would undersell `scopedjs`. The demo should visibly combine multiple capability classes in one tool call.

## Recommendation that came out of the read

Use a scoped "project workspace ops" demo:

- fake workspace files on disk,
- a scoped `db` global with notes/tasks,
- fake `obsidian` and `webserver` modules,
- `fs` for file reads and writes,
- bootstrap helpers such as `joinPath(...)`,
- one tool such as `eval_project_ops`.

That demo is realistic enough for prompt design and UI rendering, while still being deterministic and safe to run locally.

## Tooling note

The first attempt to create the ticket with `docmgr ticket create-ticket --root ttmp` followed the workspace-level default and created the ticket under `geppetto/ttmp`. Re-running the command with an absolute `--root /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/ttmp` created the correct ticket in the `pinocchio` repo.

## 2026-03-16 Slice 1 and Slice 2 Start

### Goal

Land the first executable checkpoint for the new demo without yet copying the full Bubble Tea shell. The first code slice should prove that:

- the new example directory exists,
- fake workspaces are deterministic,
- a scopedjs environment can be built in Pinocchio,
- and one direct tool execution works end to end.

### What was added

- `pinocchio/cmd/examples/scopedjs-tui-demo/fake_data.go`
- `pinocchio/cmd/examples/scopedjs-tui-demo/environment.go`
- `pinocchio/cmd/examples/scopedjs-tui-demo/environment_test.go`
- `pinocchio/cmd/examples/scopedjs-tui-demo/main.go`
- `pinocchio/cmd/examples/scopedjs-tui-demo/renderers.go`
- `pinocchio/cmd/examples/scopedjs-tui-demo/README.md`

### Main implementation decisions

- I kept the first checkpoint non-UI-heavy and focused on the runtime-building path first.
- The demo uses a scoped project-workspace fixture model instead of account-scoped SQL fixtures.
- `fs` is the real native module; `obsidian` and `webserver` are fake native modules; `db` is a scoped global.
- The workspace is materialized into a temp directory so file writes are real and visible.
- `main.go` and `renderers.go` are thin placeholders in this checkpoint so the example directory is structurally complete before full TUI wiring.

### Tooling fix discovered during implementation

The earlier `go.work` mismatch was blocking local sibling-module resolution. Running:

```bash
go work use .
```

from the shared workspace root updated `go.work` from `go 1.26` to `go 1.26.1`, which removed the workspace-level blocker and let `pinocchio` resolve the local `geppetto` checkout containing `pkg/inference/tools/scopedjs`.

### Verification

I verified the first code slice with:

```bash
go test ./cmd/examples/scopedjs-tui-demo
go run ./cmd/examples/scopedjs-tui-demo
```

The first test run also exposed one real behavior bug: the smoke test wrote to `artifacts/summary.json`, but the seeded workspace did not create the `artifacts/` directory yet. I fixed that by materializing the directory up front in `fake_data.go`.

### Outcome

The new example now has:

- deterministic demo workspaces,
- a working `eval_project_ops` scopedjs environment,
- direct smoke tests for runtime composition,
- and a runnable placeholder command entry point.

The next slice should replace the placeholder `main.go` with the actual Bubble Tea shell and then make the timeline useful with scopedjs-specific renderers.

### Commit checkpoint

- `61a1b61` — `feat(scopedjs-demo): scaffold runtime fixtures and smoke tests`

## 2026-03-16 Slice 3 Checkpoint

### Goal

Replace the placeholder command entry point with the real Pinocchio Bubble Tea shell so the demo is structurally aligned with `scopeddb-tui-demo` before custom renderers are added.

### What changed

- `pinocchio/cmd/examples/scopedjs-tui-demo/main.go` now mirrors the existing scopeddb demo's outer shape:
  - Cobra flags,
  - profile resolution,
  - engine creation,
  - registry/bootstrap wiring,
  - Watermill event router setup,
  - `toolloopbackend.NewToolLoopBackend(...)`,
  - Bubble Tea chat model initialization,
  - status bar setup,
  - and `agentforwarder.MakeUIForwarder(program)`.

### Verification

I verified this command-wiring slice with:

```bash
go test ./cmd/examples/scopedjs-tui-demo
go run ./cmd/examples/scopedjs-tui-demo --list-workspaces
```

The `--list-workspaces` path returned:

- `apollo`
- `mercury`

That was the safest non-engine path to confirm the command surface and fixture selection logic before moving on to timeline renderer work.

### Outcome

The example is no longer a placeholder binary. It now has the real outer Pinocchio shell. The next slice should make the timeline useful by replacing the no-op renderer registration with scopedjs-specific tool-call and tool-result renderers.

### Commit checkpoint

- `7313e2b` — `feat(scopedjs-demo): wire pinocchio command shell`

## 2026-03-16 Slice 4 Checkpoint

### Goal

Replace the no-op renderer registration with useful scopedjs-specific timeline renderers so the demo shows JavaScript and structured eval output instead of raw JSON noise.

### What changed

- `pinocchio/cmd/examples/scopedjs-tui-demo/renderers.go` now registers:
  - base LLM text rendering,
  - plain fallback rendering,
  - a scopedjs tool-call renderer,
  - a scopedjs tool-result renderer,
  - and the existing log event renderer.
- The tool-call renderer parses `scopedjs.EvalInput` and renders:
  - JavaScript as fenced `js`,
  - and any auxiliary `input` payload as YAML.
- The tool-result renderer parses `scopedjs.EvalOutput` and renders:
  - errors,
  - console output,
  - summary paths,
  - note metadata,
  - routes,
  - row summaries,
  - and remaining payloads through a YAML fallback.
- `pinocchio/cmd/examples/scopedjs-tui-demo/renderers_test.go` adds focused tests for:
  - JS tool-call formatting,
  - structured result formatting,
  - and error rendering.

### Verification

I verified the renderer slice with:

```bash
go test ./cmd/examples/scopedjs-tui-demo
go run ./cmd/examples/scopedjs-tui-demo --list-workspaces
```

The command still reports:

- `apollo`
- `mercury`

which confirms the renderer additions did not regress the command shell or package build.

### Outcome

The next remaining work is demo polish and a true manual run against a configured engine/profile so the rendered timeline can be validated interactively.

### Commit checkpoint

- `2f7be40` — `feat(scopedjs-demo): render eval calls and results`

## 2026-03-16 Slice 5 Checkpoint

### Goal

Finish the demo as something a reviewer can actually run and trust. That meant tightening the fake runtime behavior discovered during the live TUI pass, expanding the README from a placeholder into a runnable guide, and then validating one successful composed flow plus one error-oriented flow against a real configured engine.

### What changed

- `pinocchio/cmd/examples/scopedjs-tui-demo/fake_data.go`
  - pre-creates `dashboard/` in the temp workspace so prompts that write dashboard notes do not fail on a missing directory.
- `pinocchio/cmd/examples/scopedjs-tui-demo/environment.go`
  - sanitizes fake `webserver` payloads recursively so callback-style registrations become stable serializable placeholders such as `"[function]"`.
- `pinocchio/cmd/examples/scopedjs-tui-demo/environment_test.go`
  - adds a direct callback-style route test,
  - adds a direct non-empty error-path test for JavaScript eval failures.
- `pinocchio/cmd/examples/scopedjs-tui-demo/README.md`
  - now documents the actual runtime shape, run commands, fixture workspaces, and concrete prompts that trigger composed behavior.

### Validation commands

Package and command checks:

```bash
go test ./cmd/examples/scopedjs-tui-demo
go run ./cmd/examples/scopedjs-tui-demo --list-workspaces
```

Interactive TUI validation:

```bash
go run ./cmd/examples/scopedjs-tui-demo --workspace apollo
```

### Manual prompt validation

Successful composed flow:

```text
Use the JavaScript tool to create a dashboard note with require("obsidian").createNote from the open tasks, and register a /tasks route using the open task list as plain JSON data, not a callback. Return the note path and routes.
```

Observed outcome:

- the timeline rendered the tool call as formatted JavaScript,
- the tool result rendered structured note/route output,
- the assistant answered with a note path and the registered `/tasks` route.

Error/fallback-oriented flow:

```text
Try to read dashboard/missing.md, explain the failure cleanly, and do not invent a successful write if it fails.
```

Observed outcome:

- the model used the JavaScript tool,
- the timeline showed a structured error section,
- the assistant explained that `dashboard/missing.md` did not exist instead of inventing a successful file read.

### Important behavior note

The direct eval error-path test exposed one sharp edge in the lower-level `scopedjs` stack: raw JavaScript exceptions currently collapse into a generic string such as `Promise rejected: map[]` rather than preserving the original error message. That does not block the Pinocchio demo, because the demo still surfaces an error state and the assistant can explain the failure, but it is worth a second look in Geppetto if better JS exception fidelity matters.

### Outcome

At this point the demo is in the state the ticket originally asked for:

- runnable from `go run ./cmd/examples/scopedjs-tui-demo`,
- visible JavaScript in the timeline,
- structured result rendering instead of raw tool JSON,
- composed workspace behavior across `db`, `fs`, `obsidian`, and `webserver`,
- and enough README guidance that a reviewer can reproduce the good paths quickly.

### Commit checkpoint

- `e65d08f` — `feat(scopedjs-demo): polish runtime behavior and demo guide`
