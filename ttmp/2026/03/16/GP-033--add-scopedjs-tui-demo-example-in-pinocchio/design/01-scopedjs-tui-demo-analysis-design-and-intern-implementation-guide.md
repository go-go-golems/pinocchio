---
Title: Scopedjs TUI demo analysis, design, and intern implementation guide
Ticket: GP-033
Status: active
Topics:
    - pinocchio
    - ui
    - js-bindings
    - tools
    - architecture
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/main.go
      Note: Existing Bubble Tea demo skeleton to mirror
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/dataset.go
      Note: Existing demo spec and fake-scope precedent
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/renderers.go
      Note: Existing custom timeline renderer patterns
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/pkg/ui/backends/toolloop/backend.go
      Note: Tool-loop backend used by the demo
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/pkg/ui/forwarders/agent/forwarder.go
      Note: Event forwarding path from Geppetto to Bubble Tea timeline
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/schema.go
      Note: Public scopedjs API types
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/builder.go
      Note: Builder API used to register modules, globals, and bootstrap code
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/runtime.go
      Note: Runtime construction logic
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/eval.go
      Note: Eval execution, promise waiting, and console capture
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/tool.go
      Note: Prebuilt and lazy tool registration helpers
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/cmd/examples/scopedjs-tool/main.go
      Note: Minimal runnable scopedjs example
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/cmd/examples/scopedjs-dbserver/main.go
      Note: Composed runnable scopedjs example with fake modules
ExternalSources: []
Summary: Detailed architecture and implementation guide for a new Pinocchio example that demonstrates a composed scopedjs runtime in the Bubble Tea timeline UI.
LastUpdated: 2026-03-16T15:18:00-04:00
WhatFor: Give a new intern enough system context and implementation detail to build the `scopedjs-tui-demo` example correctly on the first pass.
WhenToUse: Use when implementing GP-033 or onboarding to the interaction between Pinocchio's TUI stack and Geppetto's scopedjs tool package.
---

# Scopedjs TUI demo analysis, design, and intern implementation guide

## Executive summary

This guide explains how to build a new Pinocchio example that demonstrates the new `geppetto/pkg/inference/tools/scopedjs` package in a full end-to-end TUI flow. If you are a new intern, the core idea is simple:

- Pinocchio already has a working TUI pattern for tool-using chat demos.
- Geppetto now has a reusable `scopedjs` package that can expose a prepared JavaScript runtime as one tool.
- GP-033 is about connecting those two things in one clean demo binary.

The best demo is a scoped "project workspace ops" assistant. The model gets one tool like `eval_project_ops` and can use JavaScript to:

- read and write scoped files,
- query fake project data,
- create note metadata,
- register fake web routes,
- and return one structured result.

The UI should show the whole story, not just the final text response.

## What this demo is trying to teach

The demo is not mainly about JavaScript syntax. It is teaching three things at once:

1. how Pinocchio hosts Geppetto tool-calling in a Bubble Tea app,
2. how `scopedjs` packages a reusable JavaScript runtime as a single tool,
3. how to render structured tool activity so the user can understand what happened.

That means the demo needs to succeed at three levels:

- backend architecture,
- runtime composition,
- and UI presentation.

If one of those three is weak, the demo will feel confusing.

## The systems involved

There are four subsystems you need to understand.

### 1. Pinocchio example application layer

This is the command-line binary itself. It is responsible for:

- Cobra flags,
- engine/profile resolution,
- building the tool registry,
- starting the Bubble Tea program,
- and registering renderers.

Primary file precedent:

- `pinocchio/cmd/examples/scopeddb-tui-demo/main.go`

### 2. Pinocchio tool-loop backend and event forwarding

The TUI does not directly call tools. Instead:

- the backend runs inference and tool-calling,
- Geppetto emits typed events,
- and the agent forwarder translates those events into timeline entities.

Primary files:

- `pinocchio/pkg/ui/backends/toolloop/backend.go`
- `pinocchio/pkg/ui/forwarders/agent/forwarder.go`

### 3. Geppetto tool system

Tools in Geppetto are registered into a `ToolRegistry`. The new `scopedjs` package exists to make one special kind of tool easy to build: one tool backed by one prepared JavaScript runtime.

Primary files:

- `geppetto/pkg/inference/tools/definition.go`
- `geppetto/pkg/inference/tools/registry.go`
- `geppetto/pkg/inference/tools/scopedjs/tool.go`

### 4. The scopedjs runtime package

This is the core package being demonstrated. It does the heavy lifting:

- collects modules, globals, and bootstrap files,
- builds a goja runtime,
- executes eval code,
- captures console output,
- and returns a structured result envelope.

Primary files:

- `geppetto/pkg/inference/tools/scopedjs/schema.go`
- `geppetto/pkg/inference/tools/scopedjs/builder.go`
- `geppetto/pkg/inference/tools/scopedjs/runtime.go`
- `geppetto/pkg/inference/tools/scopedjs/eval.go`
- `geppetto/pkg/inference/tools/scopedjs/tool.go`

## The best precedent to copy

Do not design the first `scopedjs` TUI demo from zero. Copy the overall structure of:

```text
pinocchio/cmd/examples/scopeddb-tui-demo
```

That demo is the correct precedent because it already solved:

- how to wire a small Pinocchio example,
- how to add a system prompt,
- how to build a small fake scoped dataset,
- how to create custom timeline renderers,
- and how to keep the example constrained and understandable.

The `scopedjs` demo should be a sibling, not a cousin.

## Recommended demo concept

## Concept: scoped project workspace ops

Use one fake project workspace with:

- a temp root directory,
- a small set of markdown and JSON files,
- open tasks and notes in fake in-memory data,
- and a project-specific status bar.

The runtime should expose:

- `require("fs")`
- `require("obsidian")`
- `require("webserver")`
- `workspaceRoot`
- `db`
- helper functions like `joinPath(...)`

The tool should be named something like:

```text
eval_project_ops
```

### Why this concept is strong

It shows composition. One tool call can:

- query tasks,
- read or write a file,
- create a note,
- and register a route.

That is exactly what `scopedjs` is for.

### Why not a real webserver

A real server would shift the demo away from the package and toward process management, ports, and cleanup. That is the wrong abstraction for a first example. The fake `webserver` module should only record routes in memory.

### Why not a real Obsidian process

A real Obsidian dependency would make the example machine-specific and much harder to test. The first demo should use a fake `obsidian` module that returns note metadata and optionally writes markdown files through `fs`.

## Proposed runtime contents

## Scope

The scope can be tiny:

```go
type demoScope struct {
    WorkspaceID string
}
```

The scope exists so the example teaches that the runtime is not ambient and global. It is prepared for one bounded workspace.

## Meta

The meta should drive the status bar and system prompt:

```go
type demoMeta struct {
    WorkspaceID string
    ProjectName string
    FileCount   int
    TaskCount   int
    NoteCount   int
}
```

This mirrors how the scopeddb demo returns meta like account name and ticket counts.

## Modules and globals

### Native module: `fs`

Use the real `go-go-goja` `fs` module. This is important because it proves the runtime can use actual native modules.

### Fake module: `obsidian`

Recommended exports:

- `createNote(title, body)`
- `link(path)`

These should return plain data structures. Keep them boring.

### Fake module: `webserver`

Recommended exports:

- `get(path, payload)`
- `routes()`

It should store route registrations in a slice and return them.

### Global: `workspaceRoot`

This is the writable root directory for the scoped workspace.

### Global: `db`

Recommended surface:

- `db.query(sql)`
- or `db.openTasks()`, `db.latestNotes()`

For a demo, either approach is fine. `db.query(sql)` looks more realistic, while higher-level helpers are easier for the model.

## Bootstrap helpers

Add at least one helper file so the demo teaches bootstrap behavior too.

Examples:

- `joinPath(a, b)`
- `writeJSON(path, value)`
- `renderTaskSummary(rows)`

Keep helpers short. The point is to show that runtime bootstrap exists, not to hide the interesting work.

## Architecture diagram

```text
+-----------------------------+
| Bubble Tea chat UI          |
| - title                     |
| - status bar                |
| - timeline                  |
+-------------+---------------+
              |
              v
+-----------------------------+
| ToolLoopBackend             |
| - session                   |
| - enginebuilder             |
| - tool registry             |
+-------------+---------------+
              |
              v
+-----------------------------+
| Geppetto tool loop          |
| - LLM decides tool use      |
| - emits tool call events    |
| - executes eval_project_ops |
+-------------+---------------+
              |
              v
+-----------------------------+
| scopedjs runtime            |
| - fs module                 |
| - obsidian module           |
| - webserver module          |
| - db global                 |
| - workspaceRoot global      |
| - bootstrap helpers         |
+-------------+---------------+
              |
              v
+-----------------------------+
| Event forwarder + renderers |
| - JS input renderer         |
| - console/result renderer   |
+-----------------------------+
```

## End-to-end flow

Here is what happens from a user prompt to timeline rendering.

1. The user enters a prompt in the TUI.
2. The Pinocchio backend appends a new turn and starts inference.
3. The LLM sees the tool schema and tool description for `eval_project_ops`.
4. If it decides to use the tool, Geppetto emits an `EventToolCall`.
5. The scopedjs tool executes the provided JavaScript.
6. The runtime may read files, query data, create note metadata, and register routes.
7. `scopedjs` returns `EvalOutput`.
8. Geppetto emits `EventToolResult`.
9. The Pinocchio forwarder turns those events into timeline entities.
10. Custom renderers display:
    - formatted JS,
    - any console lines,
    - and a structured summary of the returned object.

## Important API references

## `scopedjs` public types

From `geppetto/pkg/inference/tools/scopedjs/schema.go`:

- `ToolDescription`
- `ToolDefinitionSpec`
- `EvalOptions`
- `EnvironmentSpec`
- `BuildResult`
- `EvalInput`
- `EvalOutput`

These types define almost the whole demo boundary.

### `EnvironmentSpec`

This is the central host-side API. It lets the app say:

- what the runtime is called,
- what tool metadata the LLM sees,
- what default eval options exist,
- and how to configure the runtime.

Pseudo-code:

```go
spec := scopedjs.EnvironmentSpec[demoScope, demoMeta]{
    RuntimeLabel: "project-ops-demo",
    Tool: scopedjs.ToolDefinitionSpec{...},
    DefaultEval: scopedjs.DefaultEvalOptions(),
    Configure: func(ctx context.Context, b *scopedjs.Builder, scope demoScope) (demoMeta, error) {
        // register modules/globals/bootstrap
        return meta, nil
    },
}
```

### `Builder`

From `builder.go`, the builder collects what goes into the runtime:

- `AddNativeModule(...)`
- `AddModule(...)`
- `AddGlobal(...)`
- `AddInitializer(...)`
- `AddBootstrapSource(...)`
- `AddBootstrapFile(...)`
- `AddHelper(...)`

For the demo, you will mostly use:

- `AddNativeModule`
- `AddGlobal`
- `AddBootstrapSource`
- `AddHelper`

### `RegisterPrebuilt(...)`

From `tool.go`, this is likely the right registration path for the demo.

Why:

- the example is deterministic,
- the runtime can be built once up front,
- and the scopeddb demo already uses a prebuilt pattern.

Pseudo-code:

```go
handle, err := scopedjs.BuildRuntime(ctx, spec, scope)
registry := tools.NewInMemoryToolRegistry()
err = scopedjs.RegisterPrebuilt(registry, spec, handle, scopedjs.EvalOptions{})
```

### `EvalInput` and `EvalOutput`

From `schema.go`, the tool payload is intentionally small.

Input:

```json
{
  "code": "const rows = db.query(...); return rows;",
  "input": { "limit": 5 }
}
```

Output:

```json
{
  "result": { "...": "..." },
  "console": [{ "level": "log", "text": "..." }],
  "error": "",
  "durationMs": 12
}
```

These shapes matter because your renderer needs to parse them.

## What to copy from the scopeddb demo

## Copy almost directly

- Cobra flag pattern in `main.go`
- profile resolution helper approach
- event router plus Watermill gochannel setup
- `toolloopbackend.NewToolLoopBackend(...)`
- `agentforwarder.MakeUIForwarder(program)`
- title and status bar structure
- renderer registration entry point

## Adapt carefully

- data spec file becomes environment spec file
- SQL renderer becomes JS renderer
- query result table renderer becomes eval result renderer
- scopeddb system prompt becomes scopedjs system prompt

## Do not copy blindly

- SQL-specific markdown formatting
- result-table assumptions
- scopeddb-specific starter prompt language

## Proposed file-by-file implementation

## `main.go`

Responsibilities:

- accept flags such as:
  - `--workspace`
  - `--profile`
  - `--profile-registries`
  - `--log-level`
  - `--list-workspaces`
- resolve engine settings
- build the runtime registry
- build the backend
- run the program

Pseudo-code sketch:

```go
registry, meta, cleanup, err := buildDemoRegistry(ctx, demoScope{WorkspaceID: workspaceID})
backend := toolloopbackend.NewToolLoopBackend(engineInstance, middlewares, registry, sink, nil)
model := chat.InitialModel(backend,
    chat.WithTitle("scopedjs project ops demo"),
    chat.WithTimelineRegister(registerDemoRenderers),
    chat.WithStatusBarView(makeStatusBar(meta)),
)
```

## `environment.go`

This file is the heart of the demo.

Suggested functions:

- `demoEnvironmentSpec() scopedjs.EnvironmentSpec[demoScope, demoMeta]`
- `buildDemoRegistry(ctx, scope) (...)`
- `systemPrompt(meta demoMeta) string`

Pseudo-code:

```go
func buildDemoRegistry(ctx context.Context, scope demoScope) (*tools.InMemoryToolRegistry, demoMeta, func() error, error) {
    spec := demoEnvironmentSpec()
    handle, err := scopedjs.BuildRuntime(ctx, spec, scope)
    ...
    err = scopedjs.RegisterPrebuilt(registry, spec, handle, scopedjs.EvalOptions{})
    ...
}
```

## `fake_data.go`

Keep this file declarative. It should mostly hold fixtures and helper constructors.

Recommended content:

- workspace fixture names
- seed task rows
- seed note rows
- initial file contents
- maybe one helper that materializes the workspace directory

## `renderers.go`

This file is the second heart of the demo.

Recommended renderer kinds:

- tool call renderer for JS
- tool result renderer for structured `scopedjs.EvalOutput`

### JS tool-call renderer

Responsibilities:

- parse raw JSON into `scopedjs.EvalInput`
- render `code` as fenced JS
- render `input` map beneath it if present

Markdown sketch:

```md
```js
const rows = db.query("SELECT * FROM tasks");
const webserver = require("webserver");
webserver.get("/tasks", rows);
return webserver.routes();
```

input:
```yaml
limit: 5
```
```

### Eval-result renderer

Responsibilities:

- parse raw JSON into `scopedjs.EvalOutput`
- render errors prominently
- render console lines in a compact block
- detect common result shapes and summarize them

Suggested pattern:

- if `result.routes` exists, render a compact table
- if `result.note` exists, render note path/title
- if `result.previewPath` exists, display it
- otherwise fall back to YAML

## Suggested system prompt

The system prompt should be explicit about when to use the tool.

Example shape:

```text
You are a project-ops assistant.

Use the JavaScript tool when the user asks you to inspect workspace files,
summarize tasks, create notes, prepare dashboard content, or register demo routes.

The tool runs in a scoped runtime that already exposes:
- fs
- db
- require("obsidian")
- require("webserver")
- workspaceRoot

Prefer one coherent tool call that does the needed work and returns a concise structured result.
```

The goal is to make tool use natural but disciplined.

## Suggested prompts for README

- `List the open tasks and show me which file looks most relevant.`
- `Create a dashboard note summarizing the open tasks.`
- `Write a summary file in the workspace and tell me the path.`
- `Register a demo /tasks route and return the registered routes.`
- `Read the latest note and create a short follow-up note.`

## Recommended non-goals

Be explicit about what the demo is not.

- not a production webserver
- not a real Obsidian integration
- not a benchmarking harness
- not a general-purpose JS REPL product
- not a security hardening exercise beyond normal demo scoping

These non-goals matter because they keep the example compact.

## Detailed implementation sequence for an intern

This is the order I would want a new intern to follow.

### Step 1: Run the existing scopeddb demo

Before writing any code, run or at least read:

- `pinocchio/cmd/examples/scopeddb-tui-demo`

Goal:

- understand what a good Pinocchio example looks like,
- understand how the timeline feels,
- and understand what parts are generic versus domain-specific.

### Step 2: Run the small scopedjs examples in Geppetto

Read and, if possible, run:

- `geppetto/cmd/examples/scopedjs-tool/main.go`
- `geppetto/cmd/examples/scopedjs-dbserver/main.go`

Goal:

- understand how `EnvironmentSpec` and `RegisterPrebuilt(...)` are used,
- and understand what a composed runtime looks like without TUI complexity.

### Step 3: Create the new example directory

Create:

```text
pinocchio/cmd/examples/scopedjs-tui-demo/
```

Start with:

- `main.go`
- `environment.go`
- `fake_data.go`
- `renderers.go`
- `README.md`

### Step 4: Make the runtime buildable without the UI

Get the environment and registry building first.

Definition of done:

- direct helper can build the registry,
- direct helper can return demo meta,
- and cleanup works.

Do not start with renderer polish.

### Step 5: Wire the UI shell

Once the registry exists, wire the backend and Bubble Tea program using the same pattern as the scopeddb demo.

Definition of done:

- app starts,
- backend runs,
- tool is available to the LLM.

### Step 6: Build the custom renderers

Do this only after the backend works. Otherwise you will be debugging too many things at once.

Definition of done:

- JS appears as JS,
- eval results appear as structured output,
- errors are legible.

### Step 7: Write the README last

Once the prompts and visuals are real, write the README with actual commands and actual suggested prompts.

## Risks an intern is likely to hit

## Risk 1: too much logic in the fake modules

If the fake modules become large, the example loses clarity. Keep them narrow.

## Risk 2: raw JSON renderer fallback everywhere

If you do not build a proper result renderer, the demo will technically work but feel low-quality.

## Risk 3: trying to make the runtime too realistic

The demo does not need production semantics. It needs stable, visible semantics.

## Risk 4: mixing environment-building logic into `main.go`

Do not do this. Keep `main.go` thin and put most domain logic in `environment.go` and `fake_data.go`.

## Testing guidance

## Unit tests worth writing

- fixture creation helpers
- any pure formatting helpers in `renderers.go`
- environment builder smoke test if it can be done without TUI

## Manual tests you must do

- run the app
- trigger a successful tool call
- trigger a tool call with console output
- trigger a tool call that returns an error
- confirm the timeline remains readable

## Review checklist

- Is the example clearly analogous to `scopeddb-tui-demo`?
- Does it show why `scopedjs` is useful?
- Are the modules/globals/bootstrap helpers easy to understand?
- Is the tool call visually rendered as JavaScript?
- Is the result visually rendered as structured information?
- Can a reader find the important code quickly by file name?

## Final recommendation

Treat this example as a teaching artifact, not just a smoke test. That means:

- keep the runtime composition real enough to be interesting,
- keep the fake modules simple enough to be trustworthy,
- and spend extra care on the renderers because that is what turns backend behavior into understanding.

If you follow the `scopeddb-tui-demo` outer structure and use `geppetto/cmd/examples/scopedjs-dbserver/main.go` as the runtime-content seed, you will be working with the grain of both codebases instead of fighting them.
