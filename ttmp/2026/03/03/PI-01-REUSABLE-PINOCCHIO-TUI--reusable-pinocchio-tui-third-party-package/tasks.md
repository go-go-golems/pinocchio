# Tasks

## TODO

- [x] Create ticket workspace (`pinocchio/ttmp` root)
- [x] Create primary design doc
- [x] Create investigation diary
- [x] Create copy/paste recipes doc
- [x] Relate key code files to design doc (docmgr)
- [x] Update ticket index overview + links
- [x] Run `docmgr doctor` for PI-01
- [x] Bundle-upload docs to reMarkable (dry-run + real)
- [x] (Optional) Add a tiny third-party TUI POC in `scripts/`
- [x] Write clean-break unified TUI design doc (simple chat + tool-loop unification)
- [x] Bundle-upload updated unified-design doc to reMarkable (dry-run + real)

## Backlog (not started; no implementation in this ticket yet)

- [ ] Record/confirm decision: keep `simple-chat-agent` agent-mode sidebar/host UI in `cmd/` (do not extract to `pkg/`)
- [ ] Decide and document canonical Watermill UI topic name (`"ui"` vs `"chat"`)
- [ ] Create `pinocchio/pkg/tui/...` package skeletons (`runtime/`, `backend/`, `projector/`, `toolui/`, `widgets/`, `renderers/`)
- [ ] Implement unified `backend.SessionBackend` (session + enginebuilder; `Registry=nil` => single-pass)
- [ ] Implement unified `projector.Projector` (superset event→timeline mapping; backend-only `BackendFinishedMsg`)
- [ ] Extract tool-loop backend wiring out of `pinocchio/cmd/...` into `pinocchio/pkg/tui/backend` (keep cmd-specific UX in cmd)
- [ ] Extract reusable tool-driven form pieces into `pkg/`:
  - `toolui.ToolUIRequest/Reply`
  - `toolui.RegisterGenerativeUITool`
  - `widgets.OverlayModel`
- [ ] Implement unified `tui/runtime.Builder` and migrate:
  - `pinocchio/pkg/cmds/cmd.go` chat mode
  - `pinocchio/cmd/agents/simple-chat-agent/main.go` (should keep its host/sidebar wrapper in cmd)
- [ ] Delete old simple-chat TUI stack (`pinocchio/pkg/ui/...`) after migration (clean break; no compatibility shims)
- [ ] Update `reference/02-third-party-pinocchio-tui-copy-paste-recipes.md` for the new `pkg/tui` API
- [ ] Extend timeline persistence to include tool/log entities (optional follow-up)
