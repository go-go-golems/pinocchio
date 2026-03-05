# Changelog

## 2026-03-04

Deep analysis of why the current profile switching UI is architecturally wrong (appModel wrapper with huh.Form that replaces entire screen) and how to rebuild it as a proper lipgloss v2 canvas layer overlay. Includes ASCII mockups for profile picker (narrow/wide), editor, read-only viewer, creator, and switch diff confirmation. Proposes phased implementation: widget in bobatea → REPL overlay integration → pinocchio provider → remove appModel hack.

### Related Files

- reference/04-analysis-proper-profile-switching-ui.md — Full analysis with mockups and implementation plan
- bobatea/pkg/repl/model.go — REPL model with existing canvas layer overlay system
- bobatea/pkg/repl/command_palette_overlay.go — Reference overlay pattern
- pinocchio/cmd/switch-profiles-tui/main.go — Current appModel hack to be replaced


## 2026-03-03

- Initial workspace created
- Added intern-oriented design doc + investigation diary for profile switching

## 2026-03-03

Created intern-first design + diary for profile switching (/profile modal, runtime composition from profile registries, persistence attribution plan).

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/design-doc/01-design-profile-switching-in-switch-profiles-tui.md — Primary design doc
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/reference/01-investigation-diary.md — Chronological investigation diary


## 2026-03-03

Step 5: make tmux smoke + persistence verification reliable (Watermill gochannel buffering/non-blocking publish, serialized SQLite timeline upserts, and remove backend.Kill() cancellation); add scripts for tmux smoke + DB assertions (commits bobatea 34b05be; pinocchio ba0623d; pinocchio 4f719cd).

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/bobatea/pkg/chat/model.go — Stop canceling backend on completion (prevents missing persisted turns).
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/cmd/switch-profiles-tui/main.go — Harden event routing + direct profile_switch persistence.
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/scripts/switch-profiles-tui-smoke-and-verify.sh — Repeatable real-inference regression harness.


## 2026-03-03

Docs: added intern-facing postmortem and deep dive on event context/concurrency (Watermill + Bubble Tea + SQLite)

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/reference/02-postmortem-profile-switching-in-switch-profiles-tui.md — Runbook + lessons + failure analysis.
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/reference/03-event-context-concurrency-watermill-bubble-tea-and-sqlite.md — Context + concurrency reasoning guide.


## 2026-03-04

Updated analysis: moved all proposed overlay/widget code from bobatea to pinocchio. Profile picker widget, overlay host, and provider all live in pinocchio/pkg/tui/. Bobatea REPL is reference architecture only.

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui — New location for profile picker widget and overlay host

