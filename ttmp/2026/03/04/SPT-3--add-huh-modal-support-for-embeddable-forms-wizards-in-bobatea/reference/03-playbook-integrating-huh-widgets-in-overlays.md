---
Title: 'Playbook: Integrating huh Widgets in Overlays'
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
    - Path: pinocchio/pkg/tui/widgets/formoverlay/widget.go
      Note: FormOverlay widget — the core adapter wrapping huh.Form
    - Path: pinocchio/pkg/tui/widgets/formoverlay/config.go
      Note: Config struct and Placement enum
    - Path: pinocchio/pkg/tui/overlay/host.go
      Note: Overlay host — composites overlays using lipgloss v2 canvas
    - Path: pinocchio/pkg/tui/overlay/formoverlay_types.go
      Note: Message types (OpenFormOverlayMsg, etc.)
    - Path: pinocchio/pkg/ui/profileswitch/picker.go
      Note: Example factory — profile picker form
    - Path: pinocchio/cmd/switch-profiles-tui/main.go
      Note: Example integration — wiring overlay into a TUI app
ExternalSources: []
Summary: Step-by-step playbook for adding huh form overlays to pinocchio TUI applications
LastUpdated: 2026-03-04T00:00:00Z
WhatFor: Repeat the process of adding huh form overlays to TUI apps
WhenToUse: When adding a new modal form/dialog to a pinocchio TUI application
---

# Playbook: Integrating huh Widgets in Overlays

## Goal

Step-by-step guide for adding a huh form as a modal overlay to any pinocchio TUI application. Follow this playbook to create a new overlay from scratch in about 15 minutes.

## Prerequisites

- A pinocchio TUI application using `tea.Model` (e.g., a chat model from bobatea)
- The `formoverlay` and `overlay` packages (already in pinocchio)

Import paths:
```go
import (
    "github.com/go-go-golems/pinocchio/pkg/tui/widgets/formoverlay"
    "github.com/go-go-golems/pinocchio/pkg/tui/overlay"
)
```

## Step 1: Create a Form Factory

A factory is a `func() *huh.Form` that creates a **fresh form** each time the overlay opens. This is mandatory — huh forms are dead after `StateCompleted` and cannot be reused.

```go
func myFormFactory() func() *huh.Form {
    return func() *huh.Form {
        var name string
        var temperature float64
        return huh.NewForm(
            huh.NewGroup(
                huh.NewInput().Title("Name").Value(&name),
                huh.NewInput().Title("Temperature").Value(&temperature),
            ),
        )
    }
}
```

**Key rules:**
- Declare value variables (`var name string`) inside the factory closure, OR pass pointers from outside (like the profile picker does with `&selectedSlug`)
- Never reuse a form instance — always return `huh.NewForm(...)`
- The factory is called on every `Show()` — keep it cheap (no I/O)

**Dynamic data example** (like profile picker):
```go
var selected string
factory := func() *huh.Form {
    items := fetchItems() // called each time overlay opens
    opts := make([]huh.Option[string], len(items))
    for i, item := range items {
        opts[i] = huh.NewOption(item.Label, item.Value)
    }
    return huh.NewForm(
        huh.NewGroup(
            huh.NewSelect[string]().Title("Pick one").Options(opts...).Value(&selected),
        ),
    )
}
```

## Step 2: Create the FormOverlay

```go
fo := formoverlay.New(formoverlay.Config{
    Title:     "My Dialog",           // shown in the title bar
    Factory:   myFormFactory(),       // from Step 1
    Placement: formoverlay.PlacementCenter, // Center, Top, or TopRight
    MaxWidth:  50,                    // max rendered width (0 = default 60)
    MaxHeight: 20,                    // max rendered height (0 = default 20)
    OnSubmit: func(form *huh.Form) {
        // Called when user completes the form (Enter on last field).
        // Extract values via bound pointers or form.Get().
        // NOTE: This runs synchronously — cannot return tea.Cmd.
    },
    OnCancel: func() {
        // Called when user presses Esc or ctrl+c.
        // Optional — leave nil if no cleanup needed.
    },
    // Optional: custom styles (leave zero for defaults)
    // BorderStyle: lipgloss.NewStyle().Border(lipgloss.DoubleBorder()),
    // TitleStyle:  lipgloss.NewStyle().Bold(true),
})
```

**Placement options:**
| Placement | Position |
|-----------|----------|
| `PlacementCenter` | Centered on screen (default) |
| `PlacementTop` | Centered horizontally, 2 rows from top |
| `PlacementTopRight` | Right-aligned, 2 rows from top |

## Step 3: Wrap Your Model with the Overlay Host

```go
chatModel := chat.InitialModel(backend, /* options... */)

host := overlay.NewHost(chatModel, overlay.Config{
    FormOverlay: fo, // from Step 2
})

program := tea.NewProgram(host, tea.WithAltScreen())
```

The host:
- Delegates `Init()` to the inner model
- Routes `tea.KeyMsg` to the overlay first when it's visible (modal behavior)
- Routes ALL other messages to the overlay when visible (critical for huh Init)
- Composites the overlay on top of the inner model's View using lipgloss v2 canvas layers at Z=28

## Step 4: Trigger the Overlay

Send an `overlay.OpenFormOverlayMsg{}` to open the overlay:

```go
// From a submit interceptor:
interceptor := func(input string) (bool, tea.Cmd) {
    if input == "/settings" {
        return true, func() tea.Msg { return overlay.OpenFormOverlayMsg{} }
    }
    return false, nil
}

// Or from a keybinding handler:
case tea.KeyMsg:
    if key.Matches(msg, myKeyMap.OpenSettings) {
        return m, func() tea.Msg { return overlay.OpenFormOverlayMsg{} }
    }
```

The overlay closes automatically when:
- User presses **Esc** or **ctrl+c** (calls `OnCancel`)
- User completes the form (calls `OnSubmit`)

## Step 5: Runtime Registration (Optional)

You can add or swap overlays at runtime:

```go
// After host creation:
host.SetFormOverlay(newOverlay)
```

## Gotchas and Pitfalls

### 1. Message routing is critical

The overlay host must route **all messages** (not just key messages) to the overlay when it's visible. huh forms emit internal messages from `Init()` (focus, cursor setup) that must be processed, or the form renders but doesn't respond to input.

### 2. OnSubmit is synchronous

`OnSubmit` is a plain `func()` — it cannot return `tea.Cmd`. If you need to emit Bubble Tea messages after form completion (e.g., timeline events), you'll need to use a workaround:
- Call your logic directly in OnSubmit (works for most cases)
- Or use a channel/callback pattern to bridge back to the event loop

### 3. Factory must create fresh forms

Never cache or reuse a `*huh.Form` instance. After `StateCompleted`, huh forms refuse all `Update()` calls. The factory pattern avoids this entirely.

### 4. The selectedSlug pattern

When using `huh.Select` with `Value(&selected)`, the pointer must outlive the form. Declare it in the outer scope (not inside the factory):

```go
// CORRECT: pointer lives in outer scope
var selected string
factory := func() *huh.Form {
    return huh.NewForm(huh.NewGroup(
        huh.NewSelect[string]().Options(opts...).Value(&selected),
    ))
}
onSubmit := func(form *huh.Form) {
    fmt.Println("Selected:", selected) // reads from same pointer
}
```

### 5. Esc key conflict with Select filter mode

huh's `Select` field uses Esc to exit filter mode (`/` to enter filter). The FormOverlay intercepts Esc before the form sees it, which means **pressing Esc while filtering will close the overlay instead of exiting filter mode**. This is a known limitation — a future enhancement will add state-aware Esc handling.

### 6. Tab key in chat models

If your inner model uses Tab to submit messages (like bobatea's chat model), Tab will NOT reach the overlay. The overlay only receives messages when it's visible, and Tab is typically handled by the chat's submit interceptor before the overlay host sees it. This is correct behavior — the interceptor sends `OpenFormOverlayMsg`, and subsequent keys go to the overlay.

## Complete Example

See `pinocchio/cmd/switch-profiles-tui/main.go` for a full working example. The key sections:

1. **Factory**: `profileswitch.PickerFormFactory(mgr, &selectedSlug)` (lines ~294)
2. **FormOverlay creation**: `formoverlay.New(formoverlay.Config{...})` (lines ~292-313)
3. **Host wiring**: `overlay.NewHost(chatModel, overlay.Config{FormOverlay: profileOverlay})` (lines ~316-318)
4. **Trigger**: interceptor returns `overlay.OpenFormOverlayMsg{}` for `/profile` command (line ~245)

## Quick Reference: File Locations

| What | Path |
|------|------|
| FormOverlay widget | `pinocchio/pkg/tui/widgets/formoverlay/` |
| Overlay host | `pinocchio/pkg/tui/overlay/` |
| Config + Placement | `pinocchio/pkg/tui/widgets/formoverlay/config.go` |
| Default styles | `pinocchio/pkg/tui/widgets/formoverlay/styles.go` |
| Message types | `pinocchio/pkg/tui/overlay/formoverlay_types.go` |
| Profile picker example | `pinocchio/pkg/ui/profileswitch/picker.go` |
| Full integration example | `pinocchio/cmd/switch-profiles-tui/main.go` |

## Related

- Design doc: `design-doc/01-embeddable-huh-forms-and-wizards-as-bobatea-canvas-layer-overlays.md`
- Investigation diary: `reference/01-investigation-diary.md`
- uhoh analysis: `reference/02-uhoh-analysis-wizard-and-form-embedding-for-modal-overlays.md`
