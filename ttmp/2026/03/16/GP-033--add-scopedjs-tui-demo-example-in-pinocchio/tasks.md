# Tasks

## TODO

### Research and framing

- [x] Inspect the existing `scopeddb` TUI demo in `pinocchio/cmd/examples/scopeddb-tui-demo`.
- [x] Inspect the new `geppetto/pkg/inference/tools/scopedjs` package and the small runnable scopedjs examples in Geppetto.
- [x] Decide on one demo scenario that is more than `fs` alone but still deterministic and safe.
- [x] Create the GP-033 ticket workspace in `pinocchio/ttmp`.
- [x] Write a recommendation and implementation-plan document.
- [x] Write an intern-facing design and implementation guide.
- [x] Add an investigation diary for the ticket.
- [x] Upload the ticket bundle to reMarkable.

### Proposed implementation slices

#### Slice 1: demo scaffold and fixtures

- [x] Create `pinocchio/cmd/examples/scopedjs-tui-demo/`.
- [x] Add `main.go`, `environment.go`, `fake_data.go`, `renderers.go`, and `README.md`.
- [x] Define the scope and meta types for the demo.
- [x] Define deterministic fake workspace fixtures, task rows, and note rows.
- [x] Add helpers for listing available workspaces and loading fixtures by workspace id.
- [x] Record the Slice 1 start and finish in the diary.
- [x] Commit Slice 1 as the initial example scaffold checkpoint.

#### Slice 2: runtime environment and tool registration

- [x] Define the `scopedjs.EnvironmentSpec` for `eval_project_ops`.
- [x] Register the real `fs` native module.
- [x] Implement fake `obsidian` and `webserver` native modules.
- [x] Inject `workspaceRoot` and `db` globals.
- [x] Add bootstrap helpers such as `joinPath(...)` or `writeJSON(...)`.
- [x] Build the prebuilt runtime and register it into an in-memory tool registry.
- [x] Add at least one direct runtime/tool smoke test that executes the tool without the TUI.
- [x] Record the Slice 2 work in the diary.
- [x] Commit Slice 2 once the environment builds and tests pass.

#### Slice 3: Pinocchio command wiring

- [x] Adapt the `scopeddb-tui-demo` command shell to the new `scopedjs` demo.
- [x] Add flags for workspace selection, profile resolution, and listing available fixture workspaces.
- [x] Build the event router, Watermill sink, backend, and Bubble Tea model.
- [x] Add a demo-specific system prompt that explicitly teaches the LLM when to use `eval_project_ops`.
- [x] Add a status bar showing workspace/project counts and a prompt hint.
- [x] Verify the command compiles and starts.
- [x] Record the Slice 3 work in the diary.
- [x] Commit Slice 3 after the command wiring compiles cleanly.

#### Slice 4: custom renderers and result formatting

- [x] Add a tool-call renderer that parses `scopedjs.EvalInput`.
- [x] Render JavaScript code as fenced JS and any tool `input` payload as YAML or JSON.
- [x] Add a tool-result renderer that parses `scopedjs.EvalOutput`.
- [x] Render console output lines distinctly from the final returned result.
- [x] Render common structured result shapes such as routes, notes, file paths, and task lists.
- [x] Keep a safe YAML or text fallback for unexpected result shapes.
- [x] Add focused tests for any pure result-formatting helper functions.
- [x] Record the Slice 4 work in the diary.
- [x] Commit Slice 4 when the timeline output is readable.

#### Slice 5: README, prompts, and manual validation

- [x] Write the README run instructions and the "what it shows" section.
- [x] Add at least four suggested prompts that should trigger meaningful composed runtime usage.
- [x] Manually run the demo against a configured engine/profile.
- [x] Verify one successful file-writing flow, one route-registration flow, and one error or fallback path.
- [x] Update the changelog and diary with the exact commands used for validation.
- [x] Mark the acceptance criteria complete.
- [x] Commit the final polish slice after manual validation.

### Acceptance criteria

- [x] The demo is runnable with `go run ./cmd/examples/scopedjs-tui-demo`.
- [x] The tool call appears as formatted JavaScript in the timeline.
- [x] The tool result is rendered as structured output, not raw JSON noise.
- [x] The demo visibly shows at least three capability classes in one eval run: file I/O, scoped data access, and route/note composition.
- [x] The README includes prompts that reliably trigger the composed runtime.
