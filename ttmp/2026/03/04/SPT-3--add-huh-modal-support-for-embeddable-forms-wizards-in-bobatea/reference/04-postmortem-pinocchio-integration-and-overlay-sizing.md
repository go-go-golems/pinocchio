---
Title: 'Postmortem: pinocchio integration and overlay sizing'
Ticket: SPT-3
Status: active
Topics:
    - tui
    - huh
    - overlays
    - modals
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/cmds/cmd.go
      Note: "Integration point: overlay.Host replaces profileSwitchModel (commit d2efe8f)"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/cmds/chat_profile_switch_model.go
      Note: "Stripped to utility functions only, profileSwitchModel removed (commit d2efe8f)"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/widget.go
      Note: "Added SetTerminalSize, effectiveMaxWidth/Height for dynamic sizing (commit 146da14)"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/overlay/host.go
      Note: "Added SetTerminalSize forwarding on WindowSizeMsg (commit 146da14)"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/tui/widgets/formoverlay/styles.go
      Note: "Defaults increased from 60x20 to 80x30 (commit 146da14)"
    - Path: /home/manuel/workspaces/2026-03-03/switch-profiles-tui/pinocchio/pkg/ui/profileswitch/picker.go
      Note: "Select height cap reduced 15 to 10 (uncommitted)"
ExternalSources: []
Summary: Postmortem covering the pinocchio integration of overlay.Host + FormOverlay, dynamic sizing implementation, and remaining height/overflow issues discovered during tmux testing.
LastUpdated: 2026-03-04T00:00:00Z
WhatFor: Document what was accomplished, what failed, and what remains unresolved
WhenToUse: When continuing work on overlay sizing or pinocchio integration
---

# Postmortem: Pinocchio Integration and Overlay Sizing

## Summary

This session accomplished two things and left one unresolved:

1. **Done**: Replaced pinocchio's crude `profileSwitchModel` with `overlay.Host` + `FormOverlay` (commit `d2efe8f`). The profile picker now renders as a floating modal on top of the chat instead of replacing the entire screen.

2. **Done**: Added dynamic terminal-aware sizing to `FormOverlay` (commit `146da14`). The overlay now adapts to terminal dimensions via `SetTerminalSize()`, computing effective width as `min(maxWidth, termWidth - 4)` and height as `min(maxHeight, termHeight - 4)`.

3. **Unresolved**: The overlay height still overflows on small terminals (25 rows). With many profiles (15+), the huh Select renders all items, the overlay exceeds the terminal height, and `lipgloss.Style.MaxHeight()` clips the border, producing a broken visual.

## What Worked

### profileSwitchModel replacement

The migration from `profileSwitchModel` to `overlay.Host` was clean:

- **Interceptor change**: `/profile` command sends `overlay.OpenFormOverlayMsg{}` instead of `bobatea_chat.OpenProfilePickerMsg{}`
- **Model wiring**: `overlay.NewHost(model, overlay.Config{FormOverlay: profileOverlay})` replaces `newProfileSwitchModel(model, backend, mgr, sink, chatConvID)`
- **OnSubmit callback**: Captures `backend`, `sink`, `chatConvID` in closure, calls `SwitchProfile()` + `publishProfileSwitchedInfo()`
- **File cleanup**: `profileSwitchModel` struct and all methods removed from `chat_profile_switch_model.go` (130 lines deleted), utility functions `systemNoticeEntityCmd` and `publishProfileSwitchedInfo` retained
- Net change: +29 / -132 lines

All tests pass, lint clean, build clean.

### Dynamic sizing

The `SetTerminalSize()` approach works correctly for **width**:
- Host forwards `WindowSizeMsg` to `FormOverlay.SetTerminalSize()`
- `effectiveMaxWidth()` returns `min(maxWidth, termWidth - 4)`, minimum 20
- `contentWidth()` uses effective width minus border horizontal frame
- When visible, `SetTerminalSize` also updates `form.WithWidth()` so the huh form re-renders at the new width

## What Failed

### Height overflow on small terminals

On an 80x25 terminal with ~20 profiles:

```
  +--------------------------------------------------------------+
  |   Switch Profile                                             |
  |  | Switch profile                                            |
  |  | >   cerebras-llama-3.1-8b                                 |
  |  |     codestral                                             |
  |  |   * default                                               |
  |  |     gemini-2.5-flash                                      |
  |  |     ...15+ more items...                                  |
  |  |     index-sonnet                                          |
  |                                                              |
  |  up/down/filter/enter submit                                 |
  [bottom border clipped -- overlaps chat input]
```

**Root cause chain:**
1. `PickerFormFactory` creates a `huh.Select` with `.Height(min(len(opts)+2, 15))` -- up to 15 visible items
2. 15 items + title + help + overlay chrome (border, padding, title bar) = ~23 rendered rows
3. `effectiveMaxHeight = min(30, 25 - 4) = 21` -- clipping kicks in
4. `lipgloss.Style.MaxHeight(21)` truncates the rendered string at 21 lines -- this **removes the bottom border row**, producing a visually broken overlay
5. `ComputeLayout` centers based on the clipped height, but the overlay still overlaps the chat's bottom chrome (input area, status bar)

**Why MaxHeight clipping doesn't work for borders:**
`lipgloss.MaxHeight()` operates on the final rendered string -- it truncates lines after the border is drawn. This means the border's closing row gets cut off. The clipping is content-unaware.

### Attempted fix (partial)

Reduced Select height cap from 15 to 10:
```go
Height(min(len(opts)+2, 10))
```
This helps for moderate profile counts but doesn't solve the fundamental problem: the overlay doesn't know how tall the form content will be when it sets `form.WithWidth()` in `Show()`, and huh's `Form` doesn't support `WithHeight()`.

### tmux testing difficulties

- tmux Tab key was intercepted by the shell rather than sent to the TUI app in some scenarios when the app hadn't fully launched yet
- Persistent DBs (`/tmp/switch-profiles-tui.*.db`) caused old conversation state to load on restart, requiring `--conv-id` or DB cleanup
- tmux `new-session -x W -y H` doesn't force pane size when an existing client is attached with a different terminal size; need `set-option window-size manual` + `resize-window`

## Root Causes

### Architectural gap: no height feedback loop

The overlay sizing has a one-way data flow:
1. `FormOverlay` computes `contentWidth()` and tells the form via `form.WithWidth()`
2. The form renders at that width, producing content of **unknown height**
3. The overlay wraps it in a border, applies `MaxHeight()` clip
4. If content + chrome > maxHeight, the border gets truncated

What's missing: the form needs to know the **available height** so it can limit its own rendering (e.g., fewer Select items). But `huh.Form` has no `WithHeight()` method. The `huh.Select.Height()` is the only height control, and it's set at form creation time (in the factory), before terminal dimensions are known.

### Architectural gap: factory creates form before sizing

`PickerFormFactory` runs on `Show()`, which does know `termWidth/termHeight`. But the factory is a `func() *huh.Form` with no parameters -- it can't receive the available height.

## Recommendations

### Short-term (immediate)

1. **Reduce Select height cap to 8** (safe for 20-row terminals)
2. **Increase effectiveMaxHeight margin** from 4 to 8 rows (accounts for chat header + input box + status line + breathing room)
3. **Render border separately from content**: Instead of relying on `lipgloss.MaxHeight()` to clip the entire bordered frame, compute content max height as `effectiveMaxHeight - verticalChrome`, clip the content string to that height, then wrap in the border. This guarantees the border is never truncated.

### Medium-term

4. **Pass available height to factory**: Change factory signature to `func(availableWidth, availableHeight int) *huh.Form` or add a `SizedFactory` alternative field. The factory can then set `Select.Height()` based on actual available space.

### Long-term

5. **Add `WithHeight()` to huh.Form**: Upstream enhancement that would make height-constrained embedding a first-class feature.

## Commits This Session

| Hash | Description |
|------|-------------|
| `d2efe8f` | Replace profileSwitchModel with overlay.Host + FormOverlay in pinocchio |
| `146da14` | Make FormOverlay dynamically size to fit terminal |

## Uncommitted Changes

| File | Change |
|------|--------|
| `pkg/ui/profileswitch/picker.go` | Select height cap 15 to 10 (needs further testing before committing) |

## Related

- Diary entries 12-13 in `reference/01-investigation-diary.md`
- Design doc: `design-doc/01-embeddable-huh-forms-and-wizards-as-bobatea-canvas-layer-overlays.md`
