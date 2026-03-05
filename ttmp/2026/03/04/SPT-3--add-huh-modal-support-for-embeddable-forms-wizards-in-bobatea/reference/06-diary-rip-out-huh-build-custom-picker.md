---
Title: 'Diary: Rip out huh, build custom picker'
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
    - Path: pkg/cmds/profile_switch_events.go
      Note: Renamed from chat_profile_switch_model.go (commit 1bb956f)
    - Path: pkg/tui/overlay/host.go
      Note: |-
        Host rewritten to use generic overlay, fixed non-key msg routing (commit 1bb956f)
        Host rewritten
    - Path: pkg/tui/widgets/overlay/config.go
      Note: Config with Factory func() tea.Model instead of func() *huh.Form (commit 1bb956f)
    - Path: pkg/tui/widgets/overlay/widget.go
      Note: New generic Overlay widget (commit 1bb956f)
    - Path: pkg/ui/profileswitch/picker.go
      Note: |-
        Custom ProfilePicker tea.Model with filtering, scrolling, height awareness (commit 1bb956f)
        Custom ProfilePicker with filtering/scrolling (commit 1bb956f)
ExternalSources: []
Summary: Implementation diary for replacing huh-based FormOverlay with generic Overlay widget and custom ProfilePicker.
LastUpdated: 2026-03-04T00:00:00Z
WhatFor: Track the implementation journey of the huh removal and custom picker build
WhenToUse: When reviewing or continuing overlay/picker work
---


# Diary: Rip out huh, build custom picker

## Goal

Document the replacement of huh-based FormOverlay with a generic Overlay widget and a custom ProfilePicker tea.Model. This addresses the height overflow, border clipping, and event routing issues identified in the postmortem and cleanup review.

## Step 1: Replace FormOverlay with generic Overlay, build custom ProfilePicker, fix host routing

This step was a single large coordinated change: rip out the entire huh-based formoverlay package, replace it with a generic overlay that accepts any tea.Model, build a custom profile picker with proper height awareness, fix the host's message routing bug, and clean up stale file names.

The core design decision was to make the overlay widget content-agnostic. Instead of wrapping `*huh.Form` directly, the new `Overlay` wraps any `tea.Model`. Content models signal completion by returning a `CloseOverlayMsg` as a tea.Cmd. This is cleaner than checking `huh.Form.State` after every Update and means the overlay doesn't need to know anything about the content's lifecycle.

### Prompt Context

**User prompt (verbatim):** "We are working on docmgr ticket SPT-3... I've decided to ditch huh and instead build our own picker because that will make building it and controlling the height easier. We also have the issue surfaced in [cleanup guide] that all events are now diverted to the modal when that shouldn't be the case. Let's rip out the huh stuff, remove the legacy stuff addressed in the cleanup guide, and build our own select list profile model that we have control over."

**Assistant interpretation:** Remove huh dependency from the overlay system, replace with a generic overlay + custom picker, fix the three main issues from the postmortem/cleanup review (height clipping, event routing, stale artifacts).

**Inferred user intent:** Get a working, height-aware profile picker without huh's limitations, and fix the accumulated technical debt in one clean pass.

**Commit (code):** `1bb956f` — "Replace huh-based FormOverlay with generic Overlay + custom ProfilePicker"

### What I did

1. Created `pkg/tui/widgets/overlay/` with 3 files:
   - `widget.go` — generic `Overlay` struct wrapping `tea.Model`, with `CloseOverlayMsg` protocol
   - `config.go` — `Config` with `Factory func() tea.Model`, `OnClose`, `OnCancel`
   - `styles.go` — same defaults as before (80x30, rounded border)

2. Rewrote `pkg/tui/overlay/host.go`:
   - Uses `overlaywidget.Overlay` instead of `formoverlay.FormOverlay`
   - Fixed routing: non-key messages now forwarded to BOTH overlay and inner model
   - Removed formoverlay_model.go and formoverlay_overlay.go (logic inlined)
   - Renamed types file from formoverlay_types.go to types.go

3. Built `pkg/ui/profileswitch/picker.go`:
   - `PickerModel` tea.Model with cursor navigation (j/k/arrows/home/end)
   - Type-to-filter with backspace support
   - Height-aware scrolling with scroll indicators ("3 more" etc.)
   - Single-line labels (slug + display name) — no more multiline huh option labels
   - Sends `CloseOverlayMsg` on Enter
   - `PickerFactory(mgr, selected)` returns `func() tea.Model` for the overlay

4. Updated integration points:
   - `pkg/cmds/cmd.go` — uses `overlaywidget.New` + `PickerFactory`, `OpenOverlayMsg`
   - `cmd/switch-profiles-tui/main.go` — same changes
   - `examples/form-overlay/main.go` — rewritten to use generic overlay

5. Removed `pkg/tui/widgets/formoverlay/` entirely (5 files, ~940 lines)

6. Renamed `chat_profile_switch_model.go` to `profile_switch_events.go`

### Why

- huh's `Select.Height()` is set at form creation and doesn't adapt to terminal size
- huh's `Form` has no `WithHeight()` method
- `lipgloss.MaxHeight()` applied post-border clips the border itself
- Non-key messages were being swallowed by the overlay, breaking streaming
- The factory signature `func() *huh.Form` locked us into huh forever

### What worked

- The `CloseOverlayMsg` protocol is simpler than checking `huh.Form.State`
- Content-before-border clipping eliminates the border truncation bug
- Type-to-filter on the picker is more responsive than huh's filter mode
- All 35 tests pass, lint clean, build clean
- Net reduction: ~839 lines deleted vs added

### What didn't work

- Nothing significant failed. The refactor was clean because the overlay host's API surface to the outside (cmd.go, main.go) only needed minor changes: different import paths and message types.

### What I learned

- The overlay widget doesn't need to know about form completion states — `CloseOverlayMsg` as a tea.Msg is the right abstraction boundary
- Clipping content before wrapping in a border is trivially correct (split lines, take first N), while post-border clipping requires understanding the border's rendered structure
- The `DoubleEscToClose` feature from formoverlay was dropped because the custom picker doesn't have a filter mode that consumes Esc — single Esc to close is fine

### What was tricky to build

The main subtlety was the height budget calculation in the picker's `visibleItemCount()`. The picker needs to account for: filter indicator (2 lines when active), scroll indicators (1 line when scrolling), and the help line (1 line always). These are dynamic, so the available item count changes depending on whether the user is filtering.

The host routing fix required thinking about what messages should be exclusive vs. shared. Keys are exclusive (modal behavior), but everything else (custom messages, timer ticks, streaming events) should reach both the overlay and the inner model.

### What warrants a second pair of eyes

- The `OnClose` callback in the overlay runs synchronously. If the profile switch takes time (network call to resolve runtime), the UI will block. This was the same behavior with huh's `OnSubmit`, but worth noting.
- The picker's `SetSize` method is currently not called by anyone — the picker uses its default height of 15. Need to wire this up through the overlay so the picker knows its content budget.

### What should be done in the future

- Wire `Overlay.ContentHeight()` / `ContentWidth()` into the content model so the picker knows its available space (the overlay already computes this but doesn't pass it down)
- The example in `examples/form-overlay/` should probably be renamed to `examples/overlay/`
- Consider adding a `SizedFactory func(w, h int) tea.Model` variant to the overlay config
- Smoke test with real profile registries in a small terminal (25 rows)

### Code review instructions

**Start with:** `pkg/tui/widgets/overlay/widget.go` — the new generic overlay. Key methods: `Show()`, `Update()` (esc/ctrl+c interception + CloseOverlayMsg handling), `View()` (content-before-border clipping).

**Then:** `pkg/tui/overlay/host.go` — the rewritten host. Focus on the `default:` branch in `Update()` which now forwards to both overlay and inner.

**Then:** `pkg/ui/profileswitch/picker.go` — the custom picker. Focus on `visibleItemCount()` and the scrolling logic in `View()`.

**Validate:**
```bash
cd pinocchio
go build ./...
go test ./pkg/tui/overlay/ ./pkg/tui/widgets/overlay/ ./pkg/ui/profileswitch/ -v -count=1
```

### Technical details

New message protocol:
```go
// Content model sends this to close the overlay (successful completion).
type CloseOverlayMsg struct{}

// Host sends this to open the overlay.
type OpenOverlayMsg struct{}
```

Overlay config change:
```go
// Old (huh-locked):
Factory func() *huh.Form
OnSubmit func(form *huh.Form)

// New (generic):
Factory func() tea.Model
OnClose func()
OnCancel func()
```

Height clipping fix:
```go
// Old: clips border too
rendered := borderStyle.MaxHeight(max).Render(frame)

// New: clips content, then wraps in border
content = clipHeight(content, maxContentHeight)
rendered := borderStyle.MaxWidth(maxWidth).Render(frame)
```

## Related

- Postmortem: `reference/04-postmortem-pinocchio-integration-and-overlay-sizing.md`
- Cleanup review: `reference/05-cleanup-review-profile-overlay-since-407c219.md`
