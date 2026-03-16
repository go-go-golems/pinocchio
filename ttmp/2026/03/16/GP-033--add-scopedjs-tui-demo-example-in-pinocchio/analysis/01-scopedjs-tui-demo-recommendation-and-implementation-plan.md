---
Title: Scopedjs TUI demo recommendation and implementation plan
Ticket: GP-033
Status: active
Topics:
    - pinocchio
    - ui
    - js-bindings
    - tools
    - architecture
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/main.go
      Note: Closest existing implementation pattern in Pinocchio
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/renderers.go
      Note: Closest existing custom renderer pattern
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/tool.go
      Note: Registration API the demo should showcase
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/eval.go
      Note: Eval behavior and result envelope the demo should render
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/cmd/examples/scopedjs-dbserver/main.go
      Note: Closest existing composed scopedjs example
ExternalSources: []
Summary: Recommends the concrete shape of the scopedjs TUI demo and breaks the work into small implementation slices with acceptance criteria and test guidance.
LastUpdated: 2026-03-16T15:18:00-04:00
WhatFor: Give the implementer a high-signal decision record and a practical phase-by-phase build plan.
WhenToUse: Use when starting GP-033 implementation or reviewing whether the demo scope is still correct.
---

# Scopedjs TUI demo recommendation and implementation plan

## Executive recommendation

Build a new example binary at:

```text
pinocchio/cmd/examples/scopedjs-tui-demo
```

The demo should mirror the structure and teaching role of `pinocchio/cmd/examples/scopeddb-tui-demo`, but instead of a single SQL query tool it should demonstrate one composed JavaScript runtime tool, for example:

```text
eval_project_ops
```

The runtime should expose:

- `fs` for local workspace file I/O,
- a scoped `db` global with fake project tasks and notes,
- a fake `obsidian` module that returns note metadata,
- a fake `webserver` module that records registered routes,
- and one or two bootstrap helpers such as `joinPath(...)`.

The model should be able to write one JavaScript program that composes those capabilities in a single tool call. The Bubble Tea UI should then render:

- the assistant text,
- the JavaScript tool call as formatted JS,
- console output if any,
- and the eval result as structured information rather than a raw JSON blob.

## Why this demo and not a different one

The demo needs to prove why `scopedjs` exists. A pure filesystem example is too small and does not show the benefit of one prepared environment. A real server demo is too operationally noisy and introduces failure modes unrelated to the package itself. A real Obsidian integration demo is also too environment-dependent for a first example.

The recommended project-ops demo sits in the right place:

- realistic enough that the LLM can do meaningful work,
- deterministic enough that prompts and outputs are stable,
- rich enough to show why one composed runtime beats many tiny tools,
- and contained enough that it fits in one example directory without hidden infrastructure.

## User experience the demo should create

The user should be able to run the demo and enter prompts like:

- `Read the workspace notes and summarize the three most important open tasks.`
- `Create a dashboard note for the open tasks and save it in the workspace.`
- `Set up a demo /tasks route that returns the open tasks.`
- `Write a note that links the latest task summary and mention the route path.`

The UI should make it obvious that the agent used JavaScript against a prepared runtime. It should not feel like an opaque black box.

## High-level design

```text
prompt in Bubble Tea
        |
        v
ToolLoopBackend
        |
        v
LLM chooses eval_project_ops
        |
        v
scopedjs tool executes JS against prepared runtime
        |
        v
EventToolCall / EventToolResult
        |
        v
agent forwarder -> timeline
        |
        v
custom renderers show JS + result summary
```

## Proposed file layout

```text
pinocchio/cmd/examples/scopedjs-tui-demo/
  README.md
  main.go
  environment.go
  fake_data.go
  renderers.go
  environment_test.go      (optional first test file)
```

### `main.go`

Responsibilities:

- Cobra flags
- engine/profile resolution
- tool registry construction
- event router and Watermill in-memory bus setup
- Bubble Tea program startup
- timeline renderer registration
- status bar and system prompt wiring

### `environment.go`

Responsibilities:

- define the demo scope and demo metadata
- build the `scopedjs.EnvironmentSpec`
- register modules and globals
- create bootstrap helper scripts
- build the registry through `scopedjs.BuildRuntime(...)` plus `scopedjs.RegisterPrebuilt(...)`

### `fake_data.go`

Responsibilities:

- contain deterministic project/task/note fixtures
- generate a scoped temp workspace with a few seed files
- provide fixture lookup helpers

### `renderers.go`

Responsibilities:

- render tool input as JavaScript, not raw JSON
- parse `scopedjs.EvalOutput`
- render console lines and errors cleanly
- render structured results such as:
  - files written,
  - routes registered,
  - note metadata,
  - task rows or summaries

## Detailed implementation phases

## Phase 1: Copy the outer TUI shell

Start from the shape of the scopeddb demo, not from zero.

Copy and adapt:

- main command setup
- profile resolution
- event router setup
- backend creation
- Bubble Tea program lifecycle
- status bar pattern

Keep these unchanged in spirit:

- use `toolloopbackend.NewToolLoopBackend(...)`
- use `agentforwarder.MakeUIForwarder(program)`
- use the same in-memory Watermill setup
- keep flags simple and demo-oriented

Acceptance criteria:

- the new command starts and shows the chat UI
- it resolves the engine the same way the scopeddb demo does
- the status bar mentions the demo scope

## Phase 2: Build the scoped runtime environment

Create one reusable environment builder function.

Suggested scope type:

```go
type demoScope struct {
    WorkspaceID string
}
```

Suggested meta type:

```go
type demoMeta struct {
    WorkspaceID string
    ProjectName string
    FileCount   int
    TaskCount   int
    NoteCount   int
}
```

Suggested environment responsibilities:

- create temp dir and seed files
- register `fs`
- register fake `obsidian`
- register fake `webserver`
- inject `workspaceRoot`
- inject `db`
- add helper functions like `joinPath(...)`

Pseudo-code:

```go
spec := scopedjs.EnvironmentSpec[demoScope, demoMeta]{
    RuntimeLabel: "project-ops-demo",
    Tool: scopedjs.ToolDefinitionSpec{
        Name: "eval_project_ops",
        Description: scopedjs.ToolDescription{
            Summary: "...",
            Notes: []string{...},
            StarterSnippets: []string{...},
        },
    },
    DefaultEval: scopedjs.DefaultEvalOptions(),
    Configure: func(ctx context.Context, b *scopedjs.Builder, scope demoScope) (demoMeta, error) {
        // seed workspace
        // add modules
        // add globals
        // add bootstrap helpers
        return meta, nil
    },
}
```

Acceptance criteria:

- the environment can be built without the UI
- the tool registry contains `eval_project_ops`
- a direct test invocation returns a non-empty `scopedjs.EvalOutput`

## Phase 3: Decide what the fake modules should actually do

Keep them tiny and honest.

### Fake `db` global

Expose simple helpers like:

- `db.query(sql)`
- `db.openTasks()`
- `db.latestNotes()`

This does not need a real SQL parser. The main requirement is that the LLM sees data-shaped operations.

### Fake `obsidian` module

Expose:

- `createNote(title, body)`
- `link(path)`

Return metadata objects only, or optionally write markdown files into the temp workspace if that makes the result more visual.

### Fake `webserver` module

Expose:

- `get(path, payload)`
- `routes()`

Do not start an actual HTTP server. Store route registrations in memory and return them.

### `fs`

Reuse the real `go-go-goja` `fs` module. This is important because it gives the demo one true native module alongside the fake ones.

Acceptance criteria:

- all fake modules are deterministic
- they can be explained in one paragraph each
- the result payload is visually renderable in the TUI

## Phase 4: Render the tool call and tool result well

This is the part that will make or break the teaching value of the demo.

The scopeddb demo renders SQL nicely. The scopedjs demo should do the equivalent for JavaScript.

### Tool-call renderer

Input shape:

- Pinocchio tool call events carry `name` and `input`
- `input` is raw JSON
- for scopedjs, that JSON should decode into `scopedjs.EvalInput`

Renderer behavior:

- if the tool name is `eval_project_ops`, parse the JSON
- show `code` as a fenced `js` block
- if `input` is non-empty, render it beneath as YAML or JSON

Pseudo-code:

```go
var in scopedjs.EvalInput
if err := json.Unmarshal([]byte(raw), &in); err == nil {
    render("```js\n" + strings.TrimSpace(in.Code) + "\n```")
    if len(in.Input) > 0 {
        render(asYAML(in.Input))
    }
}
```

### Tool-result renderer

Result shape:

- tool results arrive as JSON text
- parse into `scopedjs.EvalOutput`

Renderer behavior:

- if `Error` is set, show it prominently
- if `Console` has lines, render them in a compact console block
- render `Result` as structured markdown:
  - routes table if routes exist
  - file list if files exist
  - note metadata if note exists
  - otherwise YAML/JSON fallback

Acceptance criteria:

- JavaScript is readable in the timeline
- result output is scannable in one screen
- console lines are visible but not noisy

## Phase 5: Add README and prompts

The README should explain:

- what the demo teaches
- how to run it
- what prompts to try
- which files matter

Suggested prompts:

- `List the open tasks and tell me which note should be updated first.`
- `Create a dashboard note for the current open tasks.`
- `Set up a demo /tasks route and return the registered routes.`
- `Write a summary file into the workspace and tell me where you put it.`

Acceptance criteria:

- a new engineer can run the demo without reading the whole ticket
- at least three prompts reliably trigger the tool

## Phase 6: Manual validation

Manual validation is necessary because this is a UI example and the core value is visual.

Checklist:

- run the demo with a real configured engine
- send at least four prompts
- observe one no-tool answer and several tool-using answers
- confirm JS tool-call rendering
- confirm result renderer behavior for:
  - success,
  - console output,
  - error path

## Testing strategy

The first implementation does not need full end-to-end TUI automation. It does need a few targeted tests.

Recommended tests:

- fixture loader returns deterministic scope data
- environment builder registers expected modules/globals
- direct tool execution returns parseable `scopedjs.EvalOutput`
- any markdown/result formatting helper behaves deterministically

Not required initially:

- full Bubble Tea golden tests
- provider-specific LLM behavior tests
- networked integration tests

## Risks and controls

### Risk: the LLM writes sloppy JS

Control:

- keep system prompt specific
- add strong starter snippets
- make the runtime description explicit

### Risk: result rendering becomes unreadable

Control:

- prefer summary-oriented renderers
- collapse or truncate large blobs
- only fall back to raw YAML when necessary

### Risk: fake modules feel too fake

Control:

- use file side effects through real `fs`
- include data with real-looking task/note objects
- make the modules domain-realistic even if they are in-memory

## Recommendation summary

The correct first demo is not "just run JS." It is "show one coherent scoped runtime doing useful project-ops work inside Pinocchio's existing TUI architecture." If the implementer follows the same outer shell as `scopeddb-tui-demo` and focuses most effort on `environment.go` plus `renderers.go`, the result should be both understandable and demoable.
