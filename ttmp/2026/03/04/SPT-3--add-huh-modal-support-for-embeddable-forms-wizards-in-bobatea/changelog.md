# Changelog

## 2026-03-04

- Initial workspace created


## 2026-03-04

Initial analysis: mapped huh form architecture (state machine, key routing, layout system, 6 embedding gaps) and designed FormOverlay adapter for bobatea canvas layer integration. Includes ASCII mockups for select modal, multi-field editor, multi-step wizard, and confirmation dialog.

### Related Files

- bobatea/pkg/repl/model.go — Canvas layer system that FormOverlay will integrate with
- huh/examples/bubbletea/main.go — Key evidence that forms are already composable
- huh/form.go — Form state machine and embedding assumptions analyzed


## 2026-03-04

Added uhoh analysis document: wizard and form embedding for modal overlays. Covers architecture map, gap analysis (6 gaps), proposed WizardModel, step-level BuildModel methods, 5-phase implementation plan.

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/uhoh/pkg/formdsl.go — Form DSL analysis
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/uhoh/pkg/wizard/wizard.go — Wizard engine analysis


## 2026-03-04

Updated design doc: moved all proposed code from bobatea to pinocchio. FormOverlay widget and overlay host now live in pinocchio/pkg/tui/. Bobatea is reference architecture only.

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui — New location for FormOverlay widget and overlay host


## 2026-03-04

Rewrote tasks.md with 7 phases, 33 detailed tasks, each with short explanation and design doc section references. Phases: FormOverlay widget, overlay host, profile picker, wizard support, uhoh integration, convenience API, profile management enhancements.


## 2026-03-04

Phase 1 complete: FormOverlay widget core (tasks 1.1-1.7). Created config.go, styles.go, widget.go, widget_test.go in pinocchio/pkg/tui/widgets/formoverlay/. All 12 tests pass, lint clean. (commit 8901924)

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/widget.go — FormOverlay core implementation


## 2026-03-04

Phase 2 complete: Overlay host with canvas layer composition (tasks 2.1-2.7). Created host.go, formoverlay_model.go, formoverlay_overlay.go, formoverlay_types.go, host_test.go in pinocchio/pkg/tui/overlay/. All 10 tests pass, lint clean. (commit c25cdd9)

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/overlay/host.go — Overlay host with canvas layer composition


## 2026-03-04

Phase 3 complete: Profile picker overlay replaces appModel (tasks 3.1-3.4). Created picker.go, rewrote main.go. Net -56 lines. (commit c965476)

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/cmd/switch-profiles-tui/main.go — Replaced appModel with overlay.Host
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/ui/profileswitch/picker.go — Reusable profile picker form factory


## 2026-03-04

Smoke test passed: profile picker overlay works end-to-end. Fixed critical bug: non-key messages must route to overlay when visible (huh Init messages). (commit b2464bf)

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/overlay/host.go — Fixed message routing - all msgs go to overlay when visible


## 2026-03-04

Phase 4: Multi-step wizard support — added GroupIndex/GroupCount to huh (df3e5df), step progress title + tests in formoverlay (c6246b4)

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/huh/form.go — Added GroupIndex() and GroupCount() public API
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/widget.go — Step progress title for multi-group forms
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/widget_test.go — 4 multi-group wizard tests


## 2026-03-04

Phase 5: uhoh wizard integration — EmbeddableStep interface, WizardModel, async ActionStep, bubbletea-embed-wizard example (commits ef5250e, a079474)

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/uhoh/examples/bubbletea-embed-wizard/main.go — Multi-step wizard example embedded in parent model
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/uhoh/pkg/wizard/steps/action_step.go — BuildModel/RunAsync for async callback execution
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/uhoh/pkg/wizard/steps/embeddable.go — EmbeddableStep + AsyncEmbeddableStep interfaces
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/uhoh/pkg/wizard/wizard_model.go — Async WizardModel tea.Model with ActionCompletedMsg handling


## 2026-03-04

Phase 6: Convenience API + double-Esc + example (commit eb90661 in pinocchio)

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/examples/form-overlay/main.go — Minimal example demonstrating all helper types
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/helpers.go — NewSelect/NewConfirm/NewInput convenience constructors
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/widget.go — DoubleEscToClose support for Select filter compatibility


## 2026-03-04

Phase 7 (partial): Enhanced picker with descriptions/current marker, header with model/temp, ProfileDiff utility (commit f32ef12). Tasks 7.2/7.3 (profile editor/creator) deferred — requires registry mutation plumbing. Task 7.6 (command palette) deferred — chat model lacks palette integration.

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/cmd/switch-profiles-tui/main.go — Enhanced header (model/temp)
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/ui/profileswitch/diff.go — ProfileDiff utility for profile switch comparison
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/ui/profileswitch/picker.go — Enhanced picker with description


## 2026-03-04

Step 12: Replaced profileSwitchModel with overlay.Host + FormOverlay in pinocchio cmd.go (commit d2efe8f). Net -103 lines.

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/cmds/chat_profile_switch_model.go — Stripped to utility functions only
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/cmds/cmd.go — Integration point - overlay.Host replaces profileSwitchModel


## 2026-03-04

Step 13: Added dynamic terminal-aware sizing to FormOverlay (commit 146da14). Discovered height overflow issue with lipgloss.MaxHeight clipping borders. Postmortem written.

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/overlay/host.go — SetTerminalSize forwarding on WindowSizeMsg
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/widget.go — SetTerminalSize and effectiveMaxWidth/Height


## 2026-03-05

Added detailed cleanup review for the profile-switch overlay work since baseline commit `407c219`. The review includes commit-range timeline, file churn analysis, concrete defect/risk findings with file-level evidence, and a prioritized stabilization/refactor plan.

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/ttmp/2026/03/04/SPT-3--add-huh-modal-support-for-embeddable-forms-wizards-in-bobatea/reference/05-cleanup-review-profile-overlay-since-407c219.md — Main analysis/review document
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/overlay/host.go — Message routing finding (overlay visible path)
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/widget.go — Height clipping and rendering findings
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/ui/profileswitch/picker.go — Height cap and multiline label findings

## 2026-03-04

Replace huh-based FormOverlay with generic Overlay + custom ProfilePicker (commit 1bb956f). Rip out huh dependency, fix host routing for non-key msgs, fix height clipping, add picker with filtering/scrolling, rename stale files.

### Related Files

- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/cmds/profile_switch_events.go — Renamed from chat_profile_switch_model.go
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/overlay/host.go — Fixed routing: non-key msgs now reach inner model while overlay visible
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/overlay/widget.go — New generic Overlay widget replacing FormOverlay
- /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/ui/profileswitch/picker.go — Custom ProfilePicker tea.Model with filtering and scrolling

