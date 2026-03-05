---
Title: Embeddable huh forms and wizards as bobatea canvas layer overlays
Ticket: SPT-3
Status: active
Topics:
    - tui
    - huh
    - overlays
    - modals
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: bobatea/pkg/repl/command_palette_model.go
      Note: Reference overlay lifecycle (open/close/key routing)
    - Path: bobatea/pkg/repl/command_palette_overlay.go
      Note: |-
        Reference overlay with placement/positioning
        Reference overlay placement/positioning pattern
    - Path: bobatea/pkg/repl/model.go
      Note: |-
        REPL model with canvas layer overlay composition (View lines 278-362)
        REPL canvas layer overlay system (View lines 278-362)
    - Path: huh/examples/bubbletea/main.go
      Note: |-
        Existing example of embedding huh in a parent Bubble Tea model
        Existing embedding example — key evidence that forms are composable
    - Path: huh/form.go
      Note: |-
        Form struct, Init/Update/View, Run(), state machine, dimension handling
        Form struct
    - Path: huh/group.go
      Note: |-
        Group struct with viewport, field focus management, buildView
        Group with viewport and field focus management
    - Path: huh/keymap.go
      Note: Key bindings for form/group/field navigation
    - Path: huh/layout.go
      Note: |-
        Layout interface and 4 implementations (Default/Stack/Columns/Grid)
        Layout interface — Default/Stack/Columns/Grid width computation
    - Path: huh/run.go
      Note: Run() helper that wraps single field in Form
    - Path: huh/theme.go
      Note: Theme struct with Focused/Blurred field styles
ExternalSources: []
Summary: Analysis of what it takes to make huh forms embeddable as modal overlays in pinocchio TUI applications using lipgloss v2 canvas layers (referencing bobatea's overlay patterns), with concrete API proposals and phased implementation plan.
LastUpdated: 2026-03-04T00:00:00Z
WhatFor: Guide the implementation of huh modal support in pinocchio
WhenToUse: When implementing embeddable form overlays for pinocchio TUI applications
---



# Embeddable huh Forms and Wizards as Canvas Layer Overlays

## 1. Executive Summary

The `huh` form library already implements the `tea.Model` interface and can be embedded in a parent Bubble Tea model today — the `huh/examples/bubbletea/main.go` example demonstrates this. However, the current embedding story has significant gaps that make it unsuitable for modal overlay use:

1. **No output clipping**: `View()` output can exceed the intended modal dimensions
2. **Key greediness**: The form consumes all `KeyMsg` events with no escape hatch for the parent
3. **State lock after completion**: Form blocks all `Update()` calls once completed
4. **No visual modal framing**: No built-in border/title/footer for modal presentation
5. **SubmitCmd/CancelCmd default to `tea.Quit`**: Embedded forms try to quit the parent program

This document proposes a **thin adapter layer** in pinocchio — not a fork of huh — that wraps `huh.Form` with the missing modal behaviors: dimension constraints, key interception, state lifecycle management, and canvas layer integration. The adapter follows the same patterns as bobatea's existing overlays (command palette, help drawer, completion popup), which serve as the reference architecture.

**Important architectural note**: All new overlay/widget code lives in **pinocchio**, not bobatea. Bobatea's REPL overlay system (`bobatea/pkg/repl/`) is the *reference implementation* that demonstrates the canvas layer pattern — pinocchio reuses the same lipgloss v2 primitives and follows the same overlay lifecycle, but the FormOverlay widget and all profile-related UI are pinocchio components. This keeps pinocchio-specific concerns out of the generic bobatea library while making the overlay infrastructure reusable across pinocchio TUI applications.

The key insight is that huh forms are *already composable* at the model level. What's missing is the *overlay infrastructure* around them.

## 2. Problem Statement and Scope

### 2.1 The Problem

Pinocchio TUI applications need modal forms for operations like profile switching (SPT-1), settings editing, and wizard flows. Currently, the only option is the `appModel` hack pattern seen in `pinocchio/cmd/switch-profiles-tui/main.go`:

```go
func (m appModel) View() string {
    if m.active != nil {
        return m.active.View()   // ← replaces ENTIRE screen
    }
    return m.inner.View()
}
```

This approach:
- Loses the chat context behind the form
- Doesn't use bobatea's canvas layer system
- Must be reimplemented per-application
- Can't compose with other overlays

### 2.2 Scope

**In scope:**
- Adapter that wraps `huh.Form` for use as a canvas layer overlay in pinocchio TUI apps
- Key interception for Esc/close without quitting the parent program
- Dimension constraints (max width/height, centering/placement)
- State lifecycle: open → interact → complete/cancel → close → reopen
- Integration into pinocchio's TUI overlay system (following bobatea's REPL overlay pattern for key routing and canvas layers)
- Multi-step wizard support (Form with multiple Groups)

**Out of scope:**
- Modifying `huh` itself (changes should be minimal and upstreamed separately)
- Building custom form fields
- Replacing huh with a different library

### 2.3 What huh Already Provides

Evidence from `huh/examples/bubbletea/main.go` (lines 109-146):

```go
func (m Model) Init() tea.Cmd {
    return m.form.Init()                          // ✓ Delegated init
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    form, cmd := m.form.Update(msg)               // ✓ Delegated update
    if f, ok := form.(*huh.Form); ok {
        m.form = f
    }
    if m.form.State == huh.StateCompleted {        // ✓ State check
        return m, tea.Quit
    }
    return m, tea.Batch(cmds...)
}

func (m Model) View() string {
    v := strings.TrimSuffix(m.form.View(), "\n\n") // ✓ Delegated view
    form := m.lg.NewStyle().Margin(1, 0).Render(v)
    // ... compose with other content ...
}
```

This proves huh's `Init()`/`Update()`/`View()` work as a sub-model. The missing pieces are all *around* the form, not *inside* it.

## 3. Current-State Architecture Analysis

### 3.1 huh Form Internals (Evidence-Based)

**Form state machine** (`huh/form.go:35-47`):
```
StateNormal → (user completes all groups) → StateCompleted
StateNormal → (user presses Quit key)    → StateAborted
```

**Form.Update() key behavior** (`huh/form.go:501-598`):
- Line 501-505: **Blocks all updates when State != StateNormal** — once done, form is dead
- Line 510-527: Handles `tea.WindowSizeMsg` — resizes groups (only if width/height not explicitly set)
- Line 528-535: **Catches Quit keymap globally** — sets `StateAborted` + returns `CancelCmd`
- Line 536-598: Routes `nextFieldMsg`, `nextGroupMsg`, `prevGroupMsg` etc.
- All other messages pass through to the active Group

**Form.View() delegation** (`huh/form.go:609-615`):
```go
func (f *Form) View() string {
    if f.quitting { return "" }
    return f.layout.View(f)
}
```
Delegates to `Layout.View()` which renders the active Group(s).

**Layout width computation** (`huh/layout.go:37-39`):
```go
func (l *layoutDefault) GroupWidth(_ *Form, _ *Group, w int) int {
    return w   // Uses full available width, no constraints
}
```

**Group viewport** (`huh/group.go:28, 114`):
- Each Group has a `viewport.Model` for scrolling
- Width and height propagated via `WithWidth()/WithHeight()`
- `buildView()` assembles field views into viewport content

### 3.2 Six Specific Gaps for Modal Overlay Use

**Gap 1: Output exceeds set dimensions**
- `Form.WithWidth(45)` sets the *content width* but theme padding/borders add to the rendered size
- `huh/theme.go`: `Focused.Base` has `PaddingLeft(1).BorderStyle(ThickBorder).BorderLeft(true)` — adds width
- No clipping or truncation in `View()`

**Gap 2: Key greediness**
- `huh/form.go:528-535` catches `ctrl+c` unconditionally
- All `tea.KeyMsg` goes to the active field — parent never sees unhandled keys
- No mechanism for "pass this key back to parent"

**Gap 3: State lock on completion**
- `huh/form.go:501-505`: `if f.State != StateNormal { return f, nil }`
- After `StateCompleted`, form ignores all messages including `WindowSizeMsg`
- No `Reset()` method to return to `StateNormal`

**Gap 4: SubmitCmd/CancelCmd defaults**
- `huh/form.go:624-625`: `f.SubmitCmd = tea.Quit; f.CancelCmd = tea.Quit`
- These are set in `RunWithContext()` but NOT in standalone use
- When not using `Run()`, they're `nil` by default — safe for embedding, but the `StateAborted` check on `ctrl+c` still fires

**Gap 5: No modal framing**
- No built-in title bar, border, or footer for the form-as-modal
- No scrim/dimming for the background

**Gap 6: No lifecycle management**
- No Show()/Hide()/Toggle() like the command palette has
- No concept of "form is a transient overlay that opens and closes"

### 3.3 bobatea Overlay System — Reference Architecture

The bobatea REPL model (`bobatea/pkg/repl/model.go:278-362`) serves as the **reference implementation** for canvas layer overlays. Pinocchio's FormOverlay follows the same patterns using the same lipgloss v2 primitives:

```go
layers := []*lipglossv2.Layer{
    lipglossv2.NewLayer(base).X(0).Y(0).Z(0).ID("repl-base"),
}
if drawerOK   { layers = append(layers, lipglossv2.NewLayer(...).Z(15)) }
if completionOK { layers = append(layers, lipglossv2.NewLayer(...).Z(20)) }
if paletteOK  { layers = append(layers, lipglossv2.NewLayer(...).Z(30)) }

canvas := lipglossv2.NewCanvas(m.width, m.height)
canvas.Compose(lipglossv2.NewCompositor(layers...))
return canvas.Render()
```

Key routing in `model_input.go` uses a priority chain:
```go
if handled, cmd := m.handleCommandPaletteInput(k); handled { return m, cmd }
if handled, cmd := m.handleHelpDrawerShortcuts(k); handled { return m, cmd }
if handled, cmd := m.handleCompletionNavigation(k); handled { return m, cmd }
```

Each overlay follows the pattern:
1. `ensure*Widget()` — lazy initialization
2. `compute*OverlayLayout()` — returns (layout, ok bool)
3. `render*Panel()` — returns view string
4. Key handler returns `(handled bool, cmd tea.Cmd)`

## 4. Proposed Solution: The FormOverlay Adapter

### 4.1 Architecture Overview

```
┌─────────────────────────────────────────────────┐
│ pinocchio/pkg/tui/widgets/formoverlay/          │
│                                                  │
│  FormOverlay                                     │
│  ├── wraps: *huh.Form                           │
│  ├── adds:  visibility (Show/Hide/Toggle)       │
│  ├── adds:  key interception (Esc to close)     │
│  ├── adds:  dimension constraints               │
│  ├── adds:  modal framing (border, title)       │
│  ├── adds:  state lifecycle (reset on reopen)   │
│  └── adds:  placement (center, top, etc.)       │
│                                                  │
│  Does NOT modify huh internals.                  │
│  Does NOT re-implement form logic.               │
│  Delegates Init/Update/View to the inner form.   │
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│ pinocchio/pkg/tui/overlay/                      │
│  (follows bobatea/pkg/repl/ overlay pattern)    │
│                                                  │
│  host.go                — overlay host/compositor│
│  formoverlay_model.go   — lifecycle, key routing │
│  formoverlay_overlay.go — layout computation     │
│  formoverlay_types.go   — config, callbacks      │
└─────────────────────────────────────────────────┘
```

**Why pinocchio, not bobatea?** Bobatea is a generic TUI toolkit with no domain knowledge. The FormOverlay wraps huh forms for modal use — a pattern needed by pinocchio TUI applications (profile switching, settings editing, wizard flows). Keeping it in pinocchio avoids coupling bobatea to huh, and makes the overlay reusable across all pinocchio-based TUI binaries (switch-profiles-tui, simple-chat-agent, etc.).

### 4.2 The FormOverlay Widget

```go
// pinocchio/pkg/tui/widgets/formoverlay/widget.go

package formoverlay

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/huh"
    "github.com/charmbracelet/lipgloss"
)

// FormOverlay wraps a huh.Form for use as a modal overlay.
type FormOverlay struct {
    form    *huh.Form
    factory func() *huh.Form  // Creates fresh form on each open

    visible   bool
    title     string
    placement Placement

    maxWidth  int
    maxHeight int

    // Callbacks
    onSubmit func(form *huh.Form)  // Called when form completes
    onCancel func()                 // Called when form is dismissed

    // Styling
    borderStyle lipgloss.Style
    titleStyle  lipgloss.Style

    // Internal
    initialized bool
}

type Placement int

const (
    PlacementCenter Placement = iota
    PlacementTop
    PlacementTopRight
)

type Config struct {
    Title     string
    MaxWidth  int
    MaxHeight int
    Placement Placement

    // Factory creates a fresh form each time the overlay opens.
    // This avoids the "state lock" problem — each open gets a new form.
    Factory   func() *huh.Form

    OnSubmit  func(form *huh.Form)
    OnCancel  func()

    BorderStyle lipgloss.Style
    TitleStyle  lipgloss.Style
}

func New(cfg Config) *FormOverlay {
    return &FormOverlay{
        factory:     cfg.Factory,
        title:       cfg.Title,
        maxWidth:    cfg.MaxWidth,
        maxHeight:   cfg.MaxHeight,
        placement:   cfg.Placement,
        onSubmit:    cfg.OnSubmit,
        onCancel:    cfg.OnCancel,
        borderStyle: cfg.BorderStyle,
        titleStyle:  cfg.TitleStyle,
    }
}
```

### 4.3 Visibility Lifecycle

```go
// Show creates a fresh form from the factory and makes the overlay visible.
func (o *FormOverlay) Show() tea.Cmd {
    o.form = o.factory()
    o.form.WithShowHelp(true)
    // Do NOT set SubmitCmd/CancelCmd — we handle state externally
    o.visible = true
    o.initialized = false
    return o.form.Init()
}

// Hide closes the overlay without triggering callbacks.
func (o *FormOverlay) Hide() {
    o.visible = false
    o.form = nil
}

// Toggle opens or closes the overlay.
func (o *FormOverlay) Toggle() tea.Cmd {
    if o.visible {
        o.Hide()
        return nil
    }
    return o.Show()
}

func (o *FormOverlay) IsVisible() bool {
    return o.visible
}
```

**Key design decision**: The factory pattern solves Gap 3 (state lock). Instead of trying to reset a completed form, we create a fresh one each time the overlay opens. This is cheap (forms are lightweight structs) and avoids any hidden state leakage.

### 4.4 Update with Key Interception

```go
// Update routes messages to the inner form with key interception.
func (o *FormOverlay) Update(msg tea.Msg) tea.Cmd {
    if !o.visible || o.form == nil {
        return nil
    }

    // Intercept keys BEFORE form sees them
    if k, ok := msg.(tea.KeyMsg); ok {
        switch k.String() {
        case "esc":
            // Esc always closes the overlay
            o.Hide()
            if o.onCancel != nil {
                o.onCancel()
            }
            return nil
        case "ctrl+c":
            // Prevent form from aborting — just close the overlay
            o.Hide()
            if o.onCancel != nil {
                o.onCancel()
            }
            return nil
        }
    }

    // Delegate to the inner form
    model, cmd := o.form.Update(msg)
    if f, ok := model.(*huh.Form); ok {
        o.form = f
    }

    // Check for form completion
    switch o.form.State {
    case huh.StateCompleted:
        if o.onSubmit != nil {
            o.onSubmit(o.form)
        }
        o.Hide()
        return cmd
    case huh.StateAborted:
        if o.onCancel != nil {
            o.onCancel()
        }
        o.Hide()
        return cmd
    }

    return cmd
}
```

**This solves Gap 2 and Gap 4**: Esc/ctrl+c are intercepted before the form sees them. The form never sends `tea.Quit` because we don't set `SubmitCmd`/`CancelCmd`.

### 4.5 View with Modal Framing and Dimension Constraints

```go
// View renders the form inside a modal frame, constrained to max dimensions.
func (o *FormOverlay) View() string {
    if !o.visible || o.form == nil {
        return ""
    }

    // Get raw form output
    content := strings.TrimSuffix(o.form.View(), "\n\n")

    // Build modal frame
    var frame strings.Builder

    // Title bar
    if o.title != "" {
        titleBar := o.titleStyle.Render(o.title)
        frame.WriteString(titleBar)
        frame.WriteString("\n")
    }

    frame.WriteString(content)

    // Apply border and dimension constraints
    rendered := o.borderStyle.
        MaxWidth(o.maxWidth).
        MaxHeight(o.maxHeight).
        Render(frame.String())

    return rendered
}
```

**This solves Gap 1 and Gap 5**: Output is constrained via `MaxWidth`/`MaxHeight` on the border style, and the modal has a visible frame.

### 4.6 Layout Computation for Canvas Layer

```go
// ComputeLayout calculates position for the canvas layer.
func (o *FormOverlay) ComputeLayout(termWidth, termHeight int) (x, y int, view string, ok bool) {
    if !o.visible || o.form == nil {
        return 0, 0, "", false
    }

    view = o.View()
    panelWidth := lipgloss.Width(view)
    panelHeight := lipgloss.Height(view)

    if panelWidth <= 0 || panelHeight <= 0 {
        return 0, 0, "", false
    }

    switch o.placement {
    case PlacementCenter:
        x = (termWidth - panelWidth) / 2
        y = (termHeight - panelHeight) / 2
    case PlacementTop:
        x = (termWidth - panelWidth) / 2
        y = 2  // Small margin from top
    case PlacementTopRight:
        x = termWidth - panelWidth - 2
        y = 2
    }

    // Clamp to bounds
    x = max(0, min(x, termWidth-panelWidth))
    y = max(0, min(y, termHeight-panelHeight))

    return x, y, view, true
}
```

### 4.7 Overlay Host Integration (pinocchio)

The overlay host lives in pinocchio, following the same pattern as bobatea's REPL overlay system but scoped to pinocchio's TUI needs. Any pinocchio TUI application that embeds a chat model can also embed the overlay host.

**formoverlay_types.go:**
```go
package overlay

// FormOverlayProvider creates forms on demand.
type FormOverlayProvider interface {
    // CreateForm builds a fresh huh.Form for the overlay.
    CreateForm() *huh.Form
}
```

**formoverlay_model.go:**
```go
package overlay

func (h *Host) handleFormOverlayInput(k tea.KeyMsg) (bool, tea.Cmd) {
    if h.formOverlay == nil {
        return false, nil
    }

    // Open trigger
    if !h.formOverlay.IsVisible() {
        if key.Matches(k, h.keyMap.FormOverlayOpen) {
            cmd := h.formOverlay.Show()
            return true, cmd
        }
        return false, nil
    }

    // While visible: route all keys to overlay
    cmd := h.formOverlay.Update(k)
    return true, cmd
}
```

**Key routing in the pinocchio TUI model (follows bobatea's priority chain pattern):**
```go
func (m *Model) updateInput(k tea.KeyMsg) (tea.Model, tea.Cmd) {
    // Form overlay has highest priority when visible (it's a modal)
    if handled, cmd := m.overlayHost.HandleFormOverlayInput(k); handled {
        return m, cmd
    }
    // ... other key handlers (chat input, etc.) ...
}
```

**Canvas layer composition in View() (same lipgloss v2 primitives as bobatea):**
```go
formX, formY, formView, formOK := m.overlayHost.ComputeFormOverlayLayout(m.width, m.height)
if formOK {
    layers = append(layers,
        lipglossv2.NewLayer(formView).
            X(formX).Y(formY).Z(28).
            ID("form-overlay"),
    )
}
```

Z=28 puts the form overlay above any help/completion layers but below command palette-level overlays. The form blocks other input when visible.

## 5. ASCII Mockups

### 5.1 Simple Select Form as Modal

```
┌─────────────────────────────────────────────────────────────────────────┐
│ ◆ mento-haiku-4.5 │ Claude Haiku 4.5 │ T=0.7                          │
│─────────────────────────────────────────────────────────────────────────│
│                                                                         │
│ User: Tell me about Rust                                                │
│                                                                         │
│ As┌─────────────────────────────────────────┐ systems programming       │
│ pr│  Switch Profile                         │ that...                   │
│ la│─────────────────────────────────────────│                           │
│   │                                         │                           │
│   │  Choose your profile                    │                           │
│   │                                         │                           │
│   │  > mento-haiku-4.5                      │                           │
│   │    mento-sonnet-4.6                     │                           │
│   │    mento-opus-4.6                       │                           │
│   │    openai-gpt-4o                        │                           │
│   │                                         │                           │
│   │  ↑/↓ navigate  enter select  esc close  │                           │
│   └─────────────────────────────────────────┘                           │
│                                                                         │
│ > _                                                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

The huh `Select` field renders inside the modal frame. The chat is visible behind. The canvas layer composites the modal on top.

### 5.2 Multi-Field Form as Modal (e.g., Settings Editor)

```
┌─────────────────────────────────────────────────────────────────────────┐
│ ◆ mento-haiku-4.5 │ Claude Haiku 4.5                                   │
│─────────────────────────────────────────────────────────────────────────│
│                                                                         │
│  ┌──────────────────────────────────────────────────┐                   │
│  │  Edit Profile                                    │                   │
│  │──────────────────────────────────────────────────│                   │
│  │                                                  │                   │
│  │  Display Name                                    │                   │
│  │  ┌────────────────────────────────────────────┐  │                   │
│  │  │ My Custom Profile                          │  │                   │
│  │  └────────────────────────────────────────────┘  │                   │
│  │                                                  │                   │
│  │  System Prompt                                   │                   │
│  │  ┌────────────────────────────────────────────┐  │                   │
│  │  │ You are a helpful assistant.               │  │                   │
│  │  │ Answer concisely.                          │  │                   │
│  │  │                                            │  │                   │
│  │  └────────────────────────────────────────────┘  │                   │
│  │                                                  │                   │
│  │  Temperature                                     │                   │
│  │  ┌────────────────────────────────────────────┐  │                   │
│  │  │ 0.7                                        │  │                   │
│  │  └────────────────────────────────────────────┘  │                   │
│  │                                                  │                   │
│  │  tab next  shift+tab prev  enter submit          │                   │
│  └──────────────────────────────────────────────────┘                   │
│                                                                         │
│ > _                                                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

This uses huh's `Input` and `Text` fields inside a single Group. The overlay constrains the form to ~50 columns and centers it.

### 5.3 Multi-Step Wizard as Modal

```
Step 1: Choose base profile

┌──────────────────────────────────────────────────┐
│  New Profile (Step 1 of 3)                       │
│──────────────────────────────────────────────────│
│                                                  │
│  Inherit from                                    │
│                                                  │
│  > mento-sonnet-4.6                              │
│    mento-opus-4.6                                │
│    openai-gpt-4o                                 │
│    (none — start from scratch)                   │
│                                                  │
│  enter next  esc cancel                          │
└──────────────────────────────────────────────────┘

Step 2: Set identity

┌──────────────────────────────────────────────────┐
│  New Profile (Step 2 of 3)                       │
│──────────────────────────────────────────────────│
│                                                  │
│  Slug                                            │
│  ┌──────────────────────────────────────────┐    │
│  │ my-code-reviewer                         │    │
│  └──────────────────────────────────────────┘    │
│                                                  │
│  Display Name                                    │
│  ┌──────────────────────────────────────────┐    │
│  │ Code Reviewer                            │    │
│  └──────────────────────────────────────────┘    │
│                                                  │
│  System Prompt                                   │
│  ┌──────────────────────────────────────────┐    │
│  │ Review code for correctness and          │    │
│  │ readability. Be specific.                │    │
│  └──────────────────────────────────────────┘    │
│                                                  │
│  shift+tab back  tab/enter next  esc cancel      │
└──────────────────────────────────────────────────┘

Step 3: Confirm

┌──────────────────────────────────────────────────┐
│  New Profile (Step 3 of 3)                       │
│──────────────────────────────────────────────────│
│                                                  │
│  Create profile "my-code-reviewer"?              │
│                                                  │
│  Base: mento-sonnet-4.6                          │
│  Model: claude-sonnet-4.6                        │
│  Temperature: 0.7                                │
│  System prompt: "Review code for..."             │
│                                                  │
│        [ Create ]    [ Cancel ]                  │
│                                                  │
│  shift+tab back  enter confirm  esc cancel       │
└──────────────────────────────────────────────────┘
```

This uses huh's multi-Group Form with `LayoutDefault` (one group at a time). Each group becomes a wizard step. The overlay title updates to show progress. The adapter intercepts navigation commands to update the title.

### 5.4 Confirmation Dialog as Minimal Modal

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│ User: Tell me about ┌────────────────────────────┐                     │
│                      │  Delete profile?           │                     │
│ Assistant: Rust is   │                            │                     │
│ a systems programm   │  mento-haiku-4.5           │                     │
│ ing language that    │                            │                     │
│ focuses on safety    │   [ Yes ]     [ No ]       │                     │
│ and performance...   │                            │                     │
│                      └────────────────────────────┘                     │
│                                                                         │
│ > _                                                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

A single-field `Confirm` in a tiny modal. Shows how the adapter handles minimal forms too.

## 6. Design Decisions

### 6.1 Adapter, Not Fork

**Decision**: Wrap `huh.Form` rather than forking/modifying huh.

**Rationale**:
- huh is actively maintained by Charm; forking creates maintenance burden
- The `tea.Model` interface is sufficient for embedding
- All gaps can be addressed externally
- Minimal upstream changes (if any) can be contributed back

### 6.2 Factory Pattern for State Reset

**Decision**: Use `func() *huh.Form` factory to create fresh forms on each open, rather than implementing a `Reset()` method.

**Rationale**:
- `huh/form.go:501-505` blocks all updates after completion — no clean way to reset internal selector state, group state, field state
- Factory is idiomatic Go and trivially correct
- Applications already know how to construct their forms
- Forms are cheap to create (no I/O, no goroutines)

### 6.3 Key Interception Before Form

**Decision**: The adapter intercepts Esc and ctrl+c before passing messages to the form.

**Rationale**:
- Form's Quit keymap (`huh/form.go:528-535`) would set `StateAborted` and return `CancelCmd`
- We want Esc to close the overlay, not abort the form's internal state
- This matches how bobatea's command palette works (reference pattern): `handleCommandPaletteInput()` checks close keys before delegating

**Trade-off**: Some huh-internal key behaviors (like Esc in a filter mode) won't work. We may need to check form state more carefully:

```go
// Refined: only intercept Esc at the form level, not during field-level Esc
case "esc":
    if o.form.State == huh.StateNormal {
        // Check if any field is in a sub-mode (filtering, etc.)
        // If yes, pass through; if no, close overlay
    }
```

This is a known subtlety that Phase 2 should address.

### 6.4 Z-Index = 28

**Decision**: Form overlay at Z=28, between completion (20) and command palette (30).

**Rationale**:
- Form overlays are modal — they should block completion popups
- Command palette should still be accessible (Ctrl+Shift+P to escape a stuck form)
- Matches the existing overlay hierarchy pattern

### 6.5 Width Constraint via WithWidth + MaxWidth

**Decision**: Set form width via `huh.Form.WithWidth()` AND constrain rendered output via `lipgloss.Style.MaxWidth()`.

**Rationale**:
- `WithWidth()` tells huh how wide to make content (fields, viewports)
- But theme padding/borders can exceed this (Gap 1)
- `MaxWidth` on the modal border style clips the final output
- Belt-and-suspenders approach handles both content sizing and visual clipping

## 7. Alternatives Considered

### 7.1 Fork huh and Add Modal Support Internally

**Approach**: Add `WithModal(bool)`, `WithParentKeyHandler()`, `Reset()` directly to huh.

**Why rejected**:
- Maintenance burden of a fork
- huh's API surface is large; invasive changes risk breaking existing users
- The adapter approach achieves the same result with less risk
- If upstream wants modal support, we can contribute the adapter pattern

### 7.2 Build Forms from Scratch Using Bubble Tea Primitives

**Approach**: Use `textinput.Model`, `textarea.Model`, `viewport.Model` directly without huh.

**Why rejected**:
- Reimplements form navigation, validation, theming, accessibility
- huh already handles all of this correctly
- Significant development time for equivalent functionality
- No access to huh's dynamic evaluation, option loading, filter modes

### 7.3 Use the appModel Wrapper Pattern (Status Quo)

**Approach**: Keep wrapping chat model with an outer model that swaps View().

**Why rejected**:
- Loses chat context (screen replacement)
- Doesn't compose with other overlays
- Must be reimplemented per-application
- No canvas layer integration
- This is exactly the problem we're solving

### 7.4 Make huh Forms Work as Command Palette Entries

**Approach**: Each form field becomes a command palette action.

**Why rejected**:
- Command palette is flat (single list of actions)
- Forms need field-to-field navigation, validation, multi-step
- Would need to fundamentally change the command palette widget
- Wrong abstraction level

## 8. Implementation Plan

### Phase 1: FormOverlay Core (pinocchio)

**Goal**: Minimal adapter that wraps `huh.Form` with Show/Hide, key interception, and framing.

**Files to create:**
```
pinocchio/pkg/tui/widgets/formoverlay/
    widget.go       — FormOverlay struct, Show/Hide/Toggle, Update, View
    config.go       — Config, Placement enum
    styles.go       — Default border/title styles
```

**Acceptance criteria:**
- Can wrap any `huh.Form` via factory
- Show() creates fresh form, Hide() dismisses
- Esc closes overlay without quitting program
- View() returns bordered, titled form content
- ComputeLayout() returns X, Y for canvas positioning

**Test approach:**
- Unit test: Show/Hide lifecycle
- Unit test: Key interception (Esc closes, other keys pass through)
- Unit test: State completion triggers onSubmit callback
- Unit test: ComputeLayout returns valid coordinates for various placements

### Phase 2: Overlay Host Integration (pinocchio)

**Goal**: Wire FormOverlay into pinocchio's TUI overlay system (following bobatea's REPL overlay pattern).

**Files to create/modify:**
```
pinocchio/pkg/tui/overlay/
    host.go                 — OverlayHost: canvas layer compositor, key routing
    formoverlay_model.go    — ensureFormOverlay(), handleFormOverlayInput()
    formoverlay_overlay.go  — computeFormOverlayLayout()
    formoverlay_types.go    — FormOverlayConfig

pinocchio/cmd/switch-profiles-tui/
    main.go                 — Wire overlay host into the TUI model
```

**Acceptance criteria:**
- Form overlay appears as a canvas layer at Z=28
- Key routing: form overlay blocks all other input when visible
- Overlay host is reusable across pinocchio TUI binaries

### Phase 3: Profile Picker Using FormOverlay (pinocchio)

**Goal**: Replace the `appModel` hack in `switch-profiles-tui` with a FormOverlay-based picker.

**Files to modify:**
```
pinocchio/cmd/switch-profiles-tui/main.go
    — Remove appModel struct
    — Create FormOverlay with profile Select form
    — Wire into overlay host
```

**Acceptance criteria:**
- `/profile` opens a form overlay with profile Select
- Chat remains visible behind the modal
- Profile switching works end-to-end
- Existing smoke tests still pass

### Phase 4: Multi-Step Wizard Support

**Goal**: Support huh Forms with multiple Groups as wizard-style modals.

**Changes:**
- FormOverlay tracks current group index for title updates ("Step 2 of 3")
- Group navigation (shift+tab to go back) works within the overlay
- Overlay resizes if groups have different heights

### Phase 5: Convenience API and Documentation

**Goal**: Make it easy for application developers to create form overlays.

**API additions:**
```go
// Quick helpers
formoverlay.NewSelect("Pick a profile", options, onSelect)
formoverlay.NewConfirm("Delete profile?", onConfirm)
formoverlay.NewWizard("New Profile", groups, onComplete)
```

**Documentation:**
- Example in `pinocchio/examples/form-overlay/`
- README section on form overlay usage

## 9. Minimal Changes to huh (Optional Upstream Contributions)

While the adapter approach avoids modifying huh, a few small changes would make embedding smoother. These could be contributed upstream:

### 9.1 Form.Reset() Method

```go
func (f *Form) Reset() {
    f.State = StateNormal
    f.quitting = false
    f.aborted = false
    f.selector.SetIndex(0)
    // Re-initialize all groups/fields
}
```

This would eliminate the need for the factory pattern (but the factory is fine as-is).

### 9.2 Form.HandleKey() with Handled Return

```go
func (f *Form) HandleKey(k tea.KeyMsg) (handled bool, cmd tea.Cmd) {
    // Returns false if the key wasn't consumed by the form
}
```

This would let the parent know when to handle keys itself.

### 9.3 Form.ContentView() Without Framing

```go
func (f *Form) ContentView() string {
    // Returns just the field content, no form-level styling
}
```

This would make it easier for the adapter to apply its own framing.

These are optional. The adapter works without them.

## 10. Risks and Mitigations

### Risk 1: huh Field Types with Sub-Modals

Some huh fields have internal modal-like behavior (Select filter mode, FilePicker). The Esc interception could interfere.

**Mitigation**: Phase 2 should add state-aware Esc handling. Check if the active field is in a sub-mode before intercepting Esc. huh fields expose state through their View() output (e.g., filter input visible), and we can detect this.

### Risk 2: Dimension Mismatch Between huh Content and Modal Frame

huh's theme padding adds to the rendered width beyond what `WithWidth()` sets. The modal frame border adds more.

**Mitigation**: Account for both when calling `form.WithWidth()`:
```go
contentWidth := maxWidth - borderHorizontalSize - themePaddingEstimate
form.WithWidth(contentWidth)
```

### Risk 3: huh Internal Message Types

huh uses internal message types (`nextFieldMsg`, `updateTitleMsg`, etc.) that are package-private. The adapter can't inspect or filter these.

**Mitigation**: This is fine — the adapter passes all non-key messages through to the form unchanged. Only `tea.KeyMsg` needs interception.

### Risk 4: Dynamic Option Loading (Select/MultiSelect)

huh fields with `OptionsFunc` send async commands and receive `updateOptionsMsg`. These commands must return to the form's Update(), not get lost.

**Mitigation**: The adapter's `Update()` returns the form's commands unchanged. The parent REPL must dispatch these commands back to the form overlay's `Update()`. This is the standard Bubble Tea pattern and works naturally as long as the REPL's main `Update()` case-matches the form overlay messages.

## 11. Testing Strategy

### Unit Tests (pinocchio)

1. **FormOverlay lifecycle**: Show → Update keys → complete → onSubmit called → Hide
2. **FormOverlay cancel**: Show → Esc → onCancel called → Hide
3. **Key interception**: Verify Esc/ctrl+c don't reach the inner form
4. **Key passthrough**: Verify regular keys reach the inner form
5. **ComputeLayout**: Verify placement calculations for Center/Top/TopRight at various terminal sizes
6. **Factory freshness**: Verify Show() creates a new form each time (no state leakage)

### Integration Tests (pinocchio)

1. **Profile picker overlay**: Open → navigate → select → switch happens → overlay closes
2. **Profile picker cancel**: Open → Esc → overlay closes → no switch
3. **Overlay + chat coexistence**: Chat content remains visible behind overlay
4. **Existing smoke tests**: All existing `switch-profiles-tui` smoke tests pass

### Manual Testing

1. Test at terminal sizes: 80x24, 120x40, 200x60
2. Test with long profile lists (>20 items, viewport scrolling)
3. Test wizard flow (multi-group form with back/forward navigation)
4. Test with both mouse and keyboard navigation

## 12. References

### Key Files

| File | Role |
|------|------|
| `huh/form.go` | Form struct, state machine, Update/View, Run() |
| `huh/group.go` | Group with viewport, field management |
| `huh/layout.go` | Layout interface (Default/Stack/Columns/Grid) |
| `huh/keymap.go` | Key bindings for all field types |
| `huh/theme.go` | Theme with Focused/Blurred styles |
| `huh/run.go` | Run() helper for single fields |
| `huh/field_select.go` | Select field with filter/viewport |
| `huh/field_input.go` | Input field with suggestions |
| `huh/field_text.go` | Textarea field with editor support |
| `huh/field_confirm.go` | Confirm field with buttons |
| `huh/examples/bubbletea/main.go` | Existing embedding example |
| `bobatea/pkg/repl/model.go` | Reference: REPL overlay system (canvas layers) |
| `bobatea/pkg/repl/command_palette_overlay.go` | Reference: overlay pattern |
| `bobatea/pkg/repl/command_palette_model.go` | Reference: overlay lifecycle |

### New Files (pinocchio)

| File | Role |
|------|------|
| `pinocchio/pkg/tui/widgets/formoverlay/widget.go` | FormOverlay widget |
| `pinocchio/pkg/tui/widgets/formoverlay/config.go` | Config, Placement |
| `pinocchio/pkg/tui/widgets/formoverlay/styles.go` | Default styles |
| `pinocchio/pkg/tui/overlay/host.go` | Overlay host/compositor |
| `pinocchio/pkg/tui/overlay/formoverlay_model.go` | Overlay lifecycle |
| `pinocchio/pkg/tui/overlay/formoverlay_overlay.go` | Layout computation |

### Related Tickets

- **SPT-1**: Profile switching in the TUI — first consumer of form overlays
- **SPT-1 reference/04**: Analysis of proper profile switching UI — describes the overlay-based profile picker that would use this infrastructure
