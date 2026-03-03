# Changelog

## 2026-03-03

- Initial workspace created
- Added primary design doc + diary + copy/paste recipes
- Documented current reusable “basic chat” API (`pinocchio/pkg/ui/runtime.ChatBuilder`) and identified agent/tool-loop backend stuck under `pinocchio/cmd/...` as the main reuse blocker
- Uploaded bundle PDF to reMarkable: `/ai/2026/03/03/PI-01-REUSABLE-PINOCCHIO-TUI/PI-01 Reusable Pinocchio TUI.pdf`
- Added clean-break unification design doc (unifies simple chat + tool-loop into one `pinocchio/pkg/tui` surface; no compatibility shims)
- Uploaded clean-break design doc bundle to reMarkable: `/ai/2026/03/03/PI-01-REUSABLE-PINOCCHIO-TUI/PI-01 Unified Pinocchio TUI (Clean Break).pdf`
- Adjusted unification plan: keep `simple-chat-agent` agent-mode sidebar/host UI in `cmd/` (too specialized); added implementation backlog tasks
