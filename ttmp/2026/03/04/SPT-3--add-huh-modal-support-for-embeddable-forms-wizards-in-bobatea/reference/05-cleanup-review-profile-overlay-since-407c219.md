---
Title: "Cleanup review: profile-switch overlay work since 407c219"
Ticket: SPT-3
Status: active
Topics:
    - tui
    - overlays
    - profile-switch
    - cleanup
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/overlay/host.go
      Note: "Overlay message routing and modal composition behavior"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/widget.go
      Note: "Height/width constraints, clipping behavior, and rendering pipeline"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/config.go
      Note: "Factory API and stale defaults comment"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/ui/profileswitch/picker.go
      Note: "Picker option formatting and hard-coded height cap"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/cmds/cmd.go
      Note: "/profile integration path in pinocchio command"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/cmds/chat_profile_switch_model.go
      Note: "Post-migration helper file with stale naming/comments"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/cmd/switch-profiles-tui/main.go
      Note: "Parallel implementation of profile-switch overlay flow"
ExternalSources: []
Summary: Detailed cleanup-oriented audit of the profile-switch overlay implementation from commit 407c219 to HEAD, with commit-churn analysis, concrete defects/risks, and a prioritized refactor plan.
LastUpdated: 2026-03-05T00:00:00Z
WhatFor: Decide what to clean up after rapid iteration on profile switch overlays and sizing behavior.
WhenToUse: Use when planning stabilization/refactor work for overlay routing, sizing, picker UX, and duplicate profile-switch integration code.
---

# Cleanup Review: Profile-Switch Overlay Work Since `407c219`

## Executive Summary

This feature area moved fast and solved the primary UX shift (modal overlay instead of full-screen replacement), but it now carries accumulated cleanup debt from iterative fixes.

The highest-impact issues are:

1. Modal routing currently swallows non-key messages for the inner chat model while the overlay is visible.
2. Overlay height clipping is applied after border render, which can cut the modal border in small terminals.
3. Picker height remains heuristic and unstable because labels are multiline and factory sizing is not terminal-aware.

These should be treated as stabilization work before adding more profile-management UI.

## Scope and Method

### Baseline and commit range

- Baseline commit: `407c21907c1088936fc5155cea0b29aa90663c18`
- Reviewed range: `407c219..HEAD` in `pinocchio`
- Focus paths:
  - `pkg/cmds/cmd.go`
  - `pkg/cmds/chat_profile_switch_model.go`
  - `pkg/tui/overlay/*`
  - `pkg/tui/widgets/formoverlay/*`
  - `pkg/ui/profileswitch/picker.go`

### What was analyzed

- Commit timeline and file churn
- Runtime control flow: event forwarding -> host routing -> overlay view/layout -> picker form behavior
- Tests currently covering these packages and missing regression coverage

### Validation command run

```bash
go test ./pkg/tui/overlay ./pkg/tui/widgets/formoverlay ./pkg/ui/profileswitch
```

Result: all passing.

## Commit Timeline (Feature-Specific)

From baseline to `HEAD`, the relevant history is:

| Commit | Description |
|---|---|
| `12f87df` | Enable `/profile` switching in TUI |
| `8901924` | Add `FormOverlay` widget |
| `c25cdd9` | Add overlay host with canvas layers |
| `c965476` | Replace appModel flow with overlay picker |
| `b2464bf` | Route non-key messages to overlay when visible |
| `c6246b4` | Add multi-group wizard support |
| `bbf4726` | Add uhoh embedding tests |
| `eb90661` | Add helper constructors and double-Esc behavior |
| `f32ef12` | Enhance picker labels/details and add diff-related profile UX |
| `27c214f` | Revert/disable step-progress block while fixing unrelated issue |
| `d2efe8f` | Replace old `profileSwitchModel` integration in `pkg/cmds` |
| `146da14` | Add terminal-aware sizing (`SetTerminalSize`) |

## Churn Map

Within the reviewed scope, the most touched files were:

| File | Added | Deleted | Total touched |
|---|---:|---:|---:|
| `pkg/tui/widgets/formoverlay/widget_test.go` | 485 | 17 | 502 |
| `pkg/tui/widgets/formoverlay/widget.go` | 334 | 19 | 353 |
| `pkg/cmds/chat_profile_switch_model.go` | 180 | 130 | 310 |
| `pkg/cmds/cmd.go` | 225 | 49 | 274 |
| `pkg/tui/overlay/host_test.go` | 248 | 0 | 248 |
| `pkg/tui/overlay/host.go` | 132 | 1 | 133 |
| `pkg/ui/profileswitch/picker.go` | 82 | 14 | 96 |

Interpretation: the shape of behavior is now mostly set, but high-churn files are signaling where cleanup and simplification will pay down risk fastest.

## Runtime Flow Map (Current)

1. User enters `/profile`.
2. Interceptor in `pkg/cmds/cmd.go` emits `overlay.OpenFormOverlayMsg`.
3. `overlay.Host.Update()` opens the form overlay and starts routing messages with modal priority.
4. `FormOverlay` creates a fresh `huh.Form` from factory and applies width via `WithWidth`.
5. Overlay view is rendered with border style and max constraints.
6. On selection submit, callback switches profile and emits profile-switched info event.

This architecture is directionally correct, but several implementation details below are now fragile.

## Findings

### 1) Inner chat updates can be dropped while modal is open (P1)

Problem:
`overlay.Host` currently routes many non-key messages exclusively to the overlay when visible, and returns without forwarding to `inner.Update`.

Where to look:
- `pinocchio/pkg/tui/overlay/host.go`, `Host.Update` default branch

Example:

```go
if h.formOverlay != nil && h.formOverlay.IsVisible() {
    cmd := h.formOverlay.Update(msg)
    return h, cmd
}
```

Why it matters:
The inner chat model may miss UI timeline/update messages during overlay visibility windows. This is especially risky during streaming or high-frequency event forwarding.

Cleanup sketch:

```go
if overlayVisible {
    overlayCmd := h.formOverlay.Update(msg)
    innerModel, innerCmd := h.inner.Update(msg)
    h.inner = innerModel
    return h, tea.Batch(overlayCmd, innerCmd)
}
```

Add a regression test that asserts inner receives non-key messages while overlay is visible.

### 2) Height clipping is applied post-border render (P1)

Problem:
`FormOverlay.View()` applies `MaxHeight` on the full bordered rendering output.

Where to look:
- `pinocchio/pkg/tui/widgets/formoverlay/widget.go`, `View()`

Example:

```go
rendered := o.borderStyle.
    MaxWidth(o.effectiveMaxWidth()).
    MaxHeight(o.effectiveMaxHeight()).
    Render(frame.String())
```

Why it matters:
On short terminals, line truncation can remove the closing border row, causing broken chrome and overlap artifacts.

Cleanup sketch:

1. Compute vertical frame cost first (`border + padding + title`).
2. Compute content height budget (`effectiveMaxHeight - frameCost`).
3. Clip content to budget.
4. Render border around clipped content without applying `MaxHeight` to final bordered string.

### 3) Picker height is still heuristic and unstable (P1/P2)

Problem:
The picker uses a hard-coded `Select.Height(min(len(opts)+2, N))` and multiline labels. Current local uncommitted change reduces cap `15 -> 10` but does not solve deterministic fit.

Where to look:
- `pinocchio/pkg/ui/profileswitch/picker.go`

Example:

```go
Height(min(len(opts)+2, 10))
...
parts = append(parts, fmt.Sprintf("    %s", desc))
```

Why it matters:
Row count and rendered line count diverge when option labels include descriptions/newlines. This undermines any fixed-height cap.

Cleanup sketch:

- Use single-line picker options in modal list.
- Move description/details to a dedicated side/preview section or contextual footer.
- Derive height from terminal budget rather than a fixed cap.

### 4) Factory API lacks size context (P2)

Problem:
`Config.Factory func() *huh.Form` has no terminal/available-size inputs.

Where to look:
- `pinocchio/pkg/tui/widgets/formoverlay/config.go`
- `pinocchio/pkg/tui/widgets/formoverlay/widget.go` (`Show`)

Why it matters:
The factory cannot compute field heights (`Select.Height`) based on real layout budget.

Cleanup sketch:

```go
type SizedFactory func(availableWidth, availableHeight int) *huh.Form
```

Keep `Factory` for compatibility and prefer `SizedFactory` when present.

### 5) Duplicate profile-switch UI/event helper logic across entrypoints (P2)

Problem:
Similar profile-switch event publication and overlay setup logic exists in both:
- `pkg/cmds/*`
- `cmd/switch-profiles-tui/main.go`

Where to look:
- `pkg/cmds/chat_profile_switch_model.go`
- `cmd/switch-profiles-tui/main.go`

Why it matters:
Behavior drift risk is high; fixes must be patched in two places.

Cleanup sketch:

- Extract shared `profileswitchui` helper package:
  - `BuildProfileOverlay(...)`
  - `PublishProfileSwitchedInfo(...)`
  - small helper for success/error notice generation

### 6) Stale artifacts/comments from rapid iteration (P3)

Problem:
Several stale pieces now mislead maintainers:

- `config.go` comment still says defaults are `60` and `20`; code is `80` and `30`.
- `widget.go` contains a commented-out step progress block.
- `cmd.go` contains stale compatibility comment.
- `chat_profile_switch_model.go` filename no longer reflects current contents.

Where to look:
- `pinocchio/pkg/tui/widgets/formoverlay/config.go`
- `pinocchio/pkg/tui/widgets/formoverlay/widget.go`
- `pinocchio/pkg/cmds/cmd.go`
- `pinocchio/pkg/cmds/chat_profile_switch_model.go`

Why it matters:
Raises cognitive load and makes future cleanup harder.

Cleanup sketch:

- Align comments with code.
- Remove dead/commented blocks.
- Rename helper file to intent-based name (for example `profile_switch_events.go`).

### 7) Regression test coverage misses observed sizing failure modes (P2)

Problem:
Existing tests cover many lifecycle paths but not the known small-terminal clipping regression and not dual-routing while modal is visible.

Where to look:
- `pinocchio/pkg/tui/overlay/host_test.go`
- `pinocchio/pkg/tui/widgets/formoverlay/widget_test.go`

Why it matters:
The exact production problems can reappear without failing CI.

Cleanup sketch:

- Add a test proving inner model receives a non-key message while overlay is visible.
- Add a test for bounded-height render preserving top and bottom border rows.
- Add a picker stress test with 20+ options in ~25-row terminal assumptions.

## Prioritized Cleanup Plan

### Phase A: Stabilize behavior (high value, low/medium risk)

1. Fix host routing so non-key messages are forwarded to both overlay and inner model when modal is visible.
2. Refactor overlay height rendering to clip content before border.
3. Add regression tests for the above.

### Phase B: Make sizing deterministic (medium value, medium risk)

1. Add `SizedFactory` support.
2. Update profile picker to derive height from available content budget.
3. Remove multiline labels from select options.

### Phase C: Hygiene and deduplication (medium value, low risk)

1. Normalize stale comments and remove dead commented code.
2. Rename helper file to match current responsibility.
3. Consolidate duplicated profile-switch helper logic.

## Suggested Acceptance Criteria for Cleanup PR

1. Overlay on an `80x25` terminal never renders a cut border in manual smoke tests.
2. While overlay is visible, streamed events still update the underlying chat timeline.
3. Picker list height is computed from available screen budget, not magic constants.
4. No stale comments about defaults, removed behavior, or disabled feature remnants.
5. Regression tests added for routing and constrained-height rendering.

## Open Questions

1. Should overlay fully pause inner model state transitions while visible, or should inner continue updating in background? Current behavior is mixed.
2. Should profile picker be optimized for dense single-line scan (more robust) or richer multiline context (more expressive)?
3. Should the switch-profiles example and main pinocchio command share one small reusable integration package now, or after stabilization?

## Immediate Hack Inventory (Current State)

1. Local uncommitted tweak: picker cap reduced from `15` to `10` in `pkg/ui/profileswitch/picker.go`.
2. `FormOverlay` still relies on `lipgloss.MaxHeight` on full frame output.
3. Step-progress title block remains commented out instead of removed or feature-flagged.
4. Defaults comment in `config.go` is outdated relative to current constants.
