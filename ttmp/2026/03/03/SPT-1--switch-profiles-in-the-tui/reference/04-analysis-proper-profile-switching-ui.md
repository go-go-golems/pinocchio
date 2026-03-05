---
Title: 'Analysis: Proper Profile Switching UI with Canvas Layer Overlays'
Ticket: SPT-1
Status: active
Topics:
    - tui
    - profiles
    - overlays
    - lipgloss-v2
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: bobatea/pkg/repl/command_palette_model.go
      Note: Reference overlay lifecycle (open/close/key routing)
    - Path: bobatea/pkg/repl/command_palette_overlay.go
      Note: |-
        Reference overlay implementation (canvas layers)
        Reference overlay implementation pattern
    - Path: bobatea/pkg/repl/completion_overlay.go
      Note: Reference overlay with computed positioning
    - Path: bobatea/pkg/repl/helpdrawer_overlay.go
      Note: Reference overlay with drawer layout
    - Path: bobatea/pkg/repl/model.go
      Note: REPL model with canvas layer overlay composition
    - Path: geppetto/pkg/profiles/service.go
      Note: Profile resolution pipeline
    - Path: geppetto/pkg/profiles/types.go
      Note: Profile domain model (all fields)
    - Path: pinocchio/cmd/switch-profiles-tui/main.go
      Note: |-
        Current (broken) appModel wrapper with huh.Form
        Current appModel hack to be replaced
    - Path: pinocchio/pkg/ui/profileswitch/backend.go
      Note: Profile-aware backend with session.Builder swapping
    - Path: pinocchio/pkg/ui/profileswitch/manager.go
      Note: Profile loading, listing, resolution
ExternalSources: []
Summary: Deep analysis of why the current profile switching UI is architecturally wrong and how to rebuild it as a proper lipgloss v2 canvas layer overlay, with ASCII mockups for picker, editor, and creation flows.
LastUpdated: 2026-03-04T00:00:00Z
WhatFor: Guide the reimplementation of the profile switching UI
WhenToUse: When planning or implementing the profile switching overlay
---



# Analysis: Proper Profile Switching UI with Canvas Layer Overlays

## Executive Summary

The current profile switching UI in `switch-profiles-tui` is architecturally wrong. It uses a `huh.Form` modal that **replaces the entire screen** when active, implemented as a wrapper `appModel` that short-circuits `View()`. This is a hack that ignores the existing overlay infrastructure in bobatea, which already implements three production overlays (command palette, help drawer, completion) using **lipgloss v2 canvas layers** with proper Z-index compositing.

This document analyzes the problems, proposes a proper architecture using the bobatea overlay pattern, and provides detailed ASCII mockups for the profile picker, profile editor, and profile creation flows.

---

## Part 1: What's Wrong with the Current Implementation

### 1.1 The appModel Hack

The current code in `pinocchio/cmd/switch-profiles-tui/main.go` wraps the chat model:

```go
type appModel struct {
    inner   tea.Model       // the chat model
    active  *huh.Form       // modal form, nil when hidden
    // ...
}

func (m appModel) View() string {
    if m.active != nil {
        return m.active.View()   // ← REPLACES entire screen
    }
    return m.inner.View()
}
```

**Problems:**

1. **Total screen takeover**: When the picker opens, the entire chat disappears. The user loses context of their conversation. In a real TUI, modals float *over* content—they don't obliterate it.

2. **No canvas layer integration**: bobatea already has a compositor with Z-indexed layers. The current code ignores this entirely and implements its own parallel View() dispatch.

3. **Wrong architectural layer**: Profile switching UI is hardcoded in a single application binary (`main.go`) instead of being a reusable overlay component. Every pinocchio TUI app that wants profiles would need to copy-paste this wrapper.

4. **Primitive picker UI**: `huh.Form` with a `Select` widget gives you a flat dropdown list. No profile details, no preview of what changes, no visual feedback about current selection.

5. **No editing or creation**: The current UI can only switch between existing profiles. You can't edit a profile's system prompt, change its model, or create new profiles—operations the backend (`geppetto/pkg/profiles`) fully supports (YAML and SQLite sources are writable).

6. **Brittle key routing**: The appModel checks `m.active != nil` to decide key routing. The bobatea REPL uses a proper priority chain: `Command Palette → Help Drawer → Completion → Input`. Adding profile switching to the appModel creates a parallel routing system that doesn't compose.

### 1.2 What the User Actually Sees

Current flow when user types `/profile`:

```
┌─────────────────────────────────────┐
│ profile=mento-haiku  runtime=mento  │  ← header
│                                     │
│ User: Tell me about Rust            │  ← chat
│                                     │
│ Assistant: Rust is a systems...     │
│                                     │
│ > /profile                          │  ← user types
└─────────────────────────────────────┘
         ↓ Enter ↓
┌─────────────────────────────────────┐
│                                     │
│   Switch profile                    │  ← huh.Form REPLACES screen
│                                     │
│   > mento-haiku-4.5                 │
│     mento-sonnet-4.6                │
│     mento-opus-4.6                  │
│                                     │
│                                     │
│                                     │
└─────────────────────────────────────┘
```

The entire chat is gone. The user can't see what they were doing. There's no visual connection between "I was chatting" and "now I'm picking a profile."

---

## Part 2: How bobatea's Overlay System Works

### 2.1 The Canvas Layer Architecture

bobatea's REPL model (`bobatea/pkg/repl/model.go:278-362`) uses lipgloss v2 canvas layers:

```go
func (m *Model) View() string {
    // 1. Render base REPL (header + timeline + input + help)
    base := lipgloss.JoinVertical(...)

    // 2. Compute overlay layouts (each returns position + content + ok)
    completionLayout, completionOK := m.computeCompletionOverlayLayout(...)
    drawerLayout, drawerOK := m.computeHelpDrawerOverlayLayout(...)
    paletteLayout, paletteOK := m.computeCommandPaletteOverlayLayout()

    // 3. Build layer stack
    layers := []*lipglossv2.Layer{
        lipglossv2.NewLayer(base).X(0).Y(0).Z(0).ID("repl-base"),
    }
    if drawerOK {
        layers = append(layers,
            lipglossv2.NewLayer(drawerPanel).X(x).Y(y).Z(15).ID("help-drawer"),
        )
    }
    if completionOK {
        layers = append(layers,
            lipglossv2.NewLayer(popup).X(x).Y(y).Z(20).ID("completion"),
        )
    }
    if paletteOK {
        layers = append(layers,
            lipglossv2.NewLayer(palette).X(x).Y(y).Z(30).ID("command-palette"),
        )
    }

    // 4. Composite everything on a single canvas
    comp := lipglossv2.NewCompositor(layers...)
    canvas := lipglossv2.NewCanvas(m.width, m.height)
    canvas.Compose(comp)
    return canvas.Render()
}
```

This gives us:
- **True overlapping**: Modals render on top of the chat, which remains visible
- **Z-ordering**: Higher Z values render on top and receive input first
- **Pixel-perfect positioning**: Each layer has X,Y coordinates computed from terminal dimensions
- **Composability**: Adding a new overlay is adding a new layer to the stack

### 2.2 The Overlay Lifecycle Pattern

Every overlay follows the same pattern:

```
1. Types file        → define overlay-specific types
2. Model file        → lazy init, open/close, key handling, state sync
3. Overlay file      → layout computation, rendering
4. Wire into model   → add to View() layers, Update() key routing
5. Wire into config  → add to Config struct, keymap, styles
```

Key routing priority in `model_input.go`:

```go
func (m *Model) updateInput(k tea.KeyMsg) (tea.Model, tea.Cmd) {
    if handled, cmd := m.handleCommandPaletteInput(k); handled { return m, cmd }
    if handled, cmd := m.handleHelpDrawerShortcuts(k); handled { return m, cmd }
    if handled, cmd := m.handleCompletionNavigation(k); handled { return m, cmd }
    // ... regular input ...
}
```

Profile switching would slot in at the same priority level as the command palette (or higher, since it's a modal that blocks all other input).

---

## Part 3: Proposed Architecture

### 3.1 Where the Profile Overlay Lives

The overlay lives in **pinocchio** — profile switching is a pinocchio concern. Bobatea's REPL overlay system (`bobatea/pkg/repl/`) serves as the **reference architecture** for the canvas layer pattern, but all new overlay/widget code is in pinocchio, making it reusable across pinocchio TUI applications.

```
pinocchio/pkg/tui/widgets/profilepicker/
    ├── widget.go       ← core widget (list, detail, edit views)
    ├── types.go        ← ProfileItem, ProfileDetail, actions
    ├── styles.go       ← visual styling
    └── keymap.go       ← navigation keys

pinocchio/pkg/tui/overlay/
    ├── host.go                ← overlay host/compositor (lipgloss v2 canvas layers)
    ├── profile_overlay.go     ← layout computation for canvas layer
    ├── profile_model.go       ← lifecycle: ensure, open, close, key routing
    └── profile_types.go       ← provider interface

pinocchio/pkg/ui/profileswitch/
    ├── overlay_provider.go    ← implements overlay provider interface
    └── (existing: manager.go, backend.go)
```

**Why pinocchio, not bobatea?** Bobatea is a generic TUI toolkit. Profile switching, form overlays, and the overlay host are pinocchio-specific infrastructure. Keeping them in pinocchio avoids coupling bobatea to huh or profile domain types, while still making the overlay reusable across all pinocchio TUI binaries (switch-profiles-tui, simple-chat-agent, etc.).

### 3.2 The Provider Interface

```go
// In pinocchio/pkg/tui/overlay/profile_types.go

type ProfilePickerProvider interface {
    // List available profiles with summary info
    ListProfiles(ctx context.Context) ([]ProfileListItem, error)

    // Get full detail for a profile (for preview/edit)
    GetProfileDetail(ctx context.Context, slug string) (*ProfileDetail, error)

    // Switch to a profile (returns new active profile info)
    SwitchProfile(ctx context.Context, slug string) (*ProfileSwitchResult, error)

    // Update a profile's fields
    UpdateProfile(ctx context.Context, slug string, patch ProfilePatch) error

    // Create a new profile
    CreateProfile(ctx context.Context, profile NewProfileInput) error

    // Delete a profile
    DeleteProfile(ctx context.Context, slug string) error

    // Get current active profile slug
    CurrentProfile() string
}

type ProfileListItem struct {
    Slug        string
    DisplayName string
    Description string
    IsDefault   bool
    IsActive    bool      // currently selected
    ModelName   string    // extracted from step settings for quick display
    Provider    string    // "claude", "openai", "gemini"
    Registry    string    // which registry it comes from
}

type ProfileDetail struct {
    Slug           string
    DisplayName    string
    Description    string
    SystemPrompt   string
    ModelName      string
    Provider       string
    Temperature    float64
    Tools          []string
    Middlewares     []string
    StackParents   []string  // inherited profiles
    Registry       string
    ReadOnly       bool      // from policy
    AllowOverrides bool
    Version        uint64
    Tags           []string
    // Raw patch for advanced view
    StepSettingsPatch map[string]any
}

type ProfilePatch struct {
    DisplayName    *string
    Description    *string
    SystemPrompt   *string
    ModelName      *string
    Provider       *string
    Temperature    *float64
    Tools          []string
    Tags           []string
}

type NewProfileInput struct {
    Slug           string
    DisplayName    string
    Description    string
    SystemPrompt   string
    ModelName      string
    Provider       string
    StackParents   []string
}

type ProfileSwitchResult struct {
    Slug               string
    RuntimeKey         string
    RuntimeFingerprint string
}
```

### 3.3 The Widget State Machine

The profile overlay widget has multiple views (modes):

```
                 ┌──────────┐
                 │  CLOSED  │
                 └────┬─────┘
                      │ open (Ctrl+P or /profile)
                      ▼
                 ┌──────────┐
          ┌──────│  PICKER  │──────┐
          │      └────┬─────┘      │
          │           │            │
     e (edit)    Enter (switch)    n (new)
          │           │            │
          ▼           ▼            ▼
    ┌──────────┐ ┌─────────┐ ┌──────────┐
    │  EDITOR  │ │ (close) │ │ CREATOR  │
    └────┬─────┘ └─────────┘ └────┬─────┘
         │                        │
    Ctrl+S (save)            Ctrl+S (save)
    Esc (cancel)             Esc (cancel)
         │                        │
         └────────┬───────────────┘
                  ▼
             ┌──────────┐
             │  PICKER  │  (refreshed)
             └──────────┘
```

### 3.4 Canvas Layer Integration

```go
// Z-index assignments:
// Z=0   : Base REPL/Chat
// Z=15  : Help Drawer
// Z=20  : Completion popup
// Z=25  : Profile overlay  ← NEW
// Z=30  : Command Palette

// In View():
profileLayout, profileOK := m.computeProfileOverlayLayout()
if profileOK {
    layers = append(layers,
        lipglossv2.NewLayer(profileLayout.View).
            X(profileLayout.PanelX).
            Y(profileLayout.PanelY).
            Z(25).
            ID("profile-overlay"),
    )
}
```

---

## Part 4: ASCII Mockups

### 4.1 Profile Picker Modal (Main View)

The picker appears as a centered modal overlay. The chat is visible but dimmed behind it.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ profile=mento-haiku-4.5  runtime=mento-haiku-4.5                       │
│─────────────────────────────────────────────────────────────────────────│
│                                                                         │
│ User: Tell me about Rust generics                                       │
│                                                                         │
│ As┌───────────────────────────────────────────────────┐ systems         │
│ pr│           Switch Profile  (Esc to close)          │                 │
│ la│───────────────────────────────────────────────────│                 │
│   │  Filter: _                                        │                 │
│   │───────────────────────────────────────────────────│                 │
│   │                                                   │                 │
│   │  ● mento-haiku-4.5          Claude Haiku 4.5     │                 │
│   │    mento-sonnet-4.6         Claude Sonnet 4.6    │                 │
│   │    mento-opus-4.6           Claude Opus 4.6      │                 │
│   │    openai-gpt-4o            GPT-4o               │                 │
│   │    openai-gpt-4o-mini       GPT-4o mini          │                 │
│   │    gemini-2.5-pro           Gemini 2.5 Pro       │                 │
│   │                                                   │                 │
│   │───────────────────────────────────────────────────│                 │
│   │  ↑↓ navigate  Enter switch  e edit  n new  d del │                 │
│   └───────────────────────────────────────────────────┘                 │
│                                                                         │
│ > _                                                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

**Features visible:**
- Title bar with escape hint
- Filter/search input at top (type to fuzzy-filter profiles)
- Profile list with slug + display name/model
- `●` marker on the currently active profile
- Footer with key hints
- Chat content visible behind the modal (dimmed)

### 4.2 Profile Picker with Detail Preview (Split Layout)

When the terminal is wide enough (>100 cols), show a split view:

```
┌─────────────────────────────────────────────────────────────────────────┐
│ profile=mento-haiku-4.5  runtime=mento-haiku-4.5                       │
│─────────────────────────────────────────────────────────────────────────│
│                                                                         │
│ ┌───────────────────────────────────────────────────────────────────┐   │
│ │           Switch Profile  (Esc to close)                          │   │
│ │───────────────────────────────────────────────────────────────────│   │
│ │ Filter: _                                                         │   │
│ │───────────────────────────────────┬───────────────────────────────│   │
│ │                                   │                               │   │
│ │  ● mento-haiku-4.5               │  mento-sonnet-4.6             │   │
│ │  ▸ mento-sonnet-4.6              │  ─────────────────            │   │
│ │    mento-opus-4.6                │  Provider: Claude              │   │
│ │    openai-gpt-4o                 │  Model:    claude-sonnet-4.6   │   │
│ │    openai-gpt-4o-mini            │  Temp:     0.7                 │   │
│ │    gemini-2.5-pro                │                                │   │
│ │                                   │  System Prompt:               │   │
│ │                                   │  "You are a helpful           │   │
│ │                                   │   assistant..."               │   │
│ │                                   │                                │   │
│ │                                   │  Tools: calculator, web       │   │
│ │                                   │  Stack: [provider-claude]     │   │
│ │                                   │  Registry: default            │   │
│ │───────────────────────────────────┴───────────────────────────────│   │
│ │  ↑↓ navigate  Enter switch  e edit  n new  d del  Tab preview    │   │
│ └───────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│ > _                                                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

**Features:**
- Left pane: profile list (same as narrow view)
- Right pane: detail preview of highlighted profile
- `▸` marks the highlighted (cursor) item, `●` marks the active profile
- Detail shows key fields: provider, model, temperature, system prompt excerpt, tools, stack parents, registry
- Preview updates as user navigates up/down

### 4.3 Profile Picker (Narrow Terminal, < 80 cols)

For narrow terminals, show a compact single-column list:

```
┌────────────────────────────────────────┐
│ profile=mento-haiku-4.5                │
│────────────────────────────────────────│
│                                        │
│ ┌────────────────────────────────────┐ │
│ │ Switch Profile          Esc close  │ │
│ │────────────────────────────────────│ │
│ │ Filter: _                          │ │
│ │────────────────────────────────────│ │
│ │ ● mento-haiku-4.5    Haiku 4.5    │ │
│ │ ▸ mento-sonnet-4.6   Sonnet 4.6   │ │
│ │   mento-opus-4.6     Opus 4.6     │ │
│ │   openai-gpt-4o      GPT-4o       │ │
│ │────────────────────────────────────│ │
│ │ ↑↓ nav  Enter switch  e edit      │ │
│ └────────────────────────────────────┘ │
│                                        │
│ > _                                    │
└────────────────────────────────────────┘
```

### 4.4 Profile Editor Modal

When user presses `e` on a profile, the picker transitions to the editor view:

```
┌─────────────────────────────────────────────────────────────────────────┐
│ profile=mento-haiku-4.5  runtime=mento-haiku-4.5                       │
│─────────────────────────────────────────────────────────────────────────│
│                                                                         │
│ ┌───────────────────────────────────────────────────────────────────┐   │
│ │           Edit Profile: mento-sonnet-4.6                          │   │
│ │───────────────────────────────────────────────────────────────────│   │
│ │                                                                   │   │
│ │  Display Name : [Sonnet 4.6                              ]       │   │
│ │  Description  : [Claude Sonnet 4.6 for general tasks     ]       │   │
│ │                                                                   │   │
│ │  ── Runtime ──────────────────────────────────────────────        │   │
│ │  Provider     : claude   ▾                                        │   │
│ │  Model        : [claude-sonnet-4.6                       ]       │   │
│ │  Temperature  : [0.7                                     ]       │   │
│ │                                                                   │   │
│ │  System Prompt:                                                   │   │
│ │  ┌───────────────────────────────────────────────────────┐       │   │
│ │  │ You are a helpful assistant. Answer questions         │       │   │
│ │  │ concisely and accurately.                             │       │   │
│ │  │                                                       │       │   │
│ │  └───────────────────────────────────────────────────────┘       │   │
│ │                                                                   │   │
│ │  Tools        : [calculator, web_search                  ]       │   │
│ │  Tags         : [general, fast                           ]       │   │
│ │                                                                   │   │
│ │───────────────────────────────────────────────────────────────────│   │
│ │  Tab next field  Ctrl+S save  Esc cancel                         │   │
│ └───────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│ > _                                                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

**Features:**
- Form fields for all editable profile attributes
- Provider dropdown (claude/openai/gemini)
- Multi-line system prompt editor (inner textarea)
- Tools as comma-separated input
- Tab to navigate between fields
- Ctrl+S to save, Esc to cancel (returns to picker)
- Read-only profiles show fields but block editing (grayed out)

### 4.5 Profile Editor for Read-Only Profile

When the profile has `Policy.ReadOnly = true`:

```
┌───────────────────────────────────────────────────────────────────┐
│           View Profile: mento-opus-4.6  (read-only)              │
│───────────────────────────────────────────────────────────────────│
│                                                                   │
│  Display Name : Opus 4.6                                          │
│  Description  : Claude Opus 4.6 for complex reasoning             │
│                                                                   │
│  ── Runtime ──────────────────────────────────────────────        │
│  Provider     : claude                                            │
│  Model        : claude-opus-4.6                                   │
│  Temperature  : 0.5                                               │
│                                                                   │
│  System Prompt:                                                   │
│  ┌───────────────────────────────────────────────────────┐       │
│  │ You are a careful reasoning assistant. Think step     │       │
│  │ by step before answering.                             │       │
│  └───────────────────────────────────────────────────────┘       │
│                                                                   │
│  Tools        : calculator, web_search, code_exec                 │
│  Stack        : provider-claude → base-reasoning                  │
│  Registry     : default                                           │
│  Version      : 3                                                 │
│                                                                   │
│───────────────────────────────────────────────────────────────────│
│  ↑↓ scroll   c clone as new   Esc back                           │
│                                                                   │
│  ⚠ This profile is read-only. Press 'c' to clone and edit.      │
└───────────────────────────────────────────────────────────────────┘
```

**Features:**
- Same layout but fields displayed as text, not inputs
- Notice at bottom that profile is read-only
- `c` to clone as new profile (pre-fills creator with this profile's values)

### 4.6 New Profile Creator

When user presses `n` from picker or `c` from read-only viewer:

```
┌───────────────────────────────────────────────────────────────────┐
│           New Profile                                             │
│───────────────────────────────────────────────────────────────────│
│                                                                   │
│  Slug          : [my-custom-profile                      ]       │
│  Display Name  : [My Custom Profile                      ]       │
│  Description   : [Custom profile for code review         ]       │
│                                                                   │
│  ── Inherit From (optional) ─────────────────────────────        │
│  Stack         : [mento-sonnet-4.6                       ]       │
│                   (comma-separated profile slugs)                 │
│                                                                   │
│  ── Runtime Overrides ───────────────────────────────────        │
│  Provider      : claude   ▾                                       │
│  Model         : [claude-sonnet-4.6                      ]       │
│  Temperature   : [0.3                                    ]       │
│                                                                   │
│  System Prompt :                                                  │
│  ┌───────────────────────────────────────────────────────┐       │
│  │ You are a senior code reviewer. Review code for       │       │
│  │ correctness, performance, and readability. Be         │       │
│  │ specific about line numbers and suggest fixes.        │       │
│  └───────────────────────────────────────────────────────┘       │
│                                                                   │
│  Tools         : [                                       ]       │
│  Tags          : [code-review, custom                    ]       │
│                                                                   │
│───────────────────────────────────────────────────────────────────│
│  Tab next field  Ctrl+S create  Esc cancel                       │
└───────────────────────────────────────────────────────────────────┘
```

**Features:**
- Slug field (validated on save: lowercase alphanumeric + ./-_)
- Stack field for inheritance (optional, references existing profiles)
- Same runtime fields as editor
- When cloning, fields pre-filled from source profile
- Ctrl+S validates and creates, Esc returns to picker

### 4.7 Inline Profile Indicator (Header Bar)

The header should be richer than the current plain text:

```
Current (plain):
┌─────────────────────────────────────────────────────────────────────────┐
│ profile=mento-haiku-4.5  runtime=mento-haiku-4.5                       │

Proposed (styled):
┌─────────────────────────────────────────────────────────────────────────┐
│ ◆ mento-haiku-4.5 │ Claude Haiku 4.5 │ T=0.7 │ Ctrl+P: switch        │

Or when just switched (with brief flash/highlight):
┌─────────────────────────────────────────────────────────────────────────┐
│ ◆ mento-sonnet-4.6 │ Claude Sonnet 4.6 │ T=0.7 │ ✓ switched          │
```

### 4.8 Profile Switch Confirmation in Timeline

When switching, instead of just a plain text entity, show a styled marker:

```
│                                                                         │
│ User: Tell me about Rust generics                                       │
│                                                                         │
│ Assistant: Rust uses a system of generics that allows you to write...   │
│                                                                         │
│ ── profile switched: mento-haiku-4.5 → mento-sonnet-4.6 ──────────── │
│                                                                         │
│ User: Now explain lifetimes                                             │
│                                                                         │
│ Assistant: Lifetimes in Rust are the mechanism by which the compiler... │
```

### 4.9 Profile Diff on Switch (Optional Enhancement)

When switching profiles, briefly show what changed:

```
┌───────────────────────────────────────────────────────────────────┐
│           Switching Profile                                       │
│───────────────────────────────────────────────────────────────────│
│                                                                   │
│  From: mento-haiku-4.5      →  To: mento-sonnet-4.6             │
│                                                                   │
│  Model:       haiku-4.5     →  sonnet-4.6                        │
│  Temperature:  0.7          →  0.7          (unchanged)          │
│  Sys Prompt:  (same)        →  (same)                            │
│  Tools:       calc          →  calc, web    (+web)               │
│                                                                   │
│───────────────────────────────────────────────────────────────────│
│  Enter confirm   Esc cancel                                      │
└───────────────────────────────────────────────────────────────────┘
```

This is optional but valuable—shows the user what will actually change before confirming.

---

## Part 5: Implementation Plan

### Phase 1: Profile Picker Widget (pinocchio)

**Goal**: Reusable profile picker widget with list, filter, and detail preview.

**Files to create:**

```
pinocchio/pkg/tui/widgets/profilepicker/
    widget.go       - Main widget: state machine, Update(), View()
    types.go        - ProfileListItem, ProfileDetail, actions
    styles.go       - Styles struct with sensible defaults
    keymap.go       - Navigation, filter, open/close keys
```

**Widget responsibilities:**
- Maintain a filterable list of `ProfileListItem`
- Track cursor position, scroll offset
- Render list view (narrow) or split view (wide)
- Fetch detail on cursor change (debounced)
- Return actions: `SwitchAction{Slug}`, `EditAction{Slug}`, `NewAction{}`, `DeleteAction{Slug}`, `CloseAction{}`

**State:**
```go
type Widget struct {
    provider    Provider
    items       []ProfileListItem
    filtered    []ProfileListItem
    cursor      int
    filter      string
    filterInput textinput.Model
    detail      *ProfileDetail   // for current cursor item
    mode        Mode             // List, Edit, Create, View

    width, height int
    styles        Styles
    keyMap        KeyMap
}
```

### Phase 2: Profile Editor Sub-Widget

**Goal**: Form for editing/creating profiles, embedded inside the picker widget.

**Approach:**
- Use `textinput.Model` for each editable field
- Use `textarea.Model` for system prompt (multi-line)
- Tab-navigation between fields
- Validation on save (slug format, required fields)
- The editor is a *mode* of the picker widget, not a separate overlay

**Fields:**
- `slug` (only for creation, read-only for edit)
- `displayName` (text input)
- `description` (text input)
- `provider` (select: claude/openai/gemini)
- `modelName` (text input)
- `temperature` (text input, validated as float)
- `systemPrompt` (textarea)
- `tools` (text input, comma-separated)
- `stackParents` (text input, comma-separated slugs)
- `tags` (text input, comma-separated)

### Phase 3: Overlay Host Integration (pinocchio)

**Goal**: Wire the picker widget into pinocchio's overlay system (following bobatea's REPL overlay pattern).

**Files to create/modify:**

```
pinocchio/pkg/tui/overlay/
    host.go               - Overlay host: canvas layer compositor, key routing
    profile_types.go      - Provider interface
    profile_model.go      - ensureProfileWidget(), open/close, key routing
    profile_overlay.go    - computeProfileOverlayLayout()

pinocchio/cmd/switch-profiles-tui/
    main.go               - Wire overlay host into the TUI model
```

**Key routing in pinocchio's TUI model (follows bobatea's priority chain pattern):**

```go
func (m *Model) updateInput(k tea.KeyMsg) (tea.Model, tea.Cmd) {
    // Profile overlay takes highest priority (it's a modal)
    if handled, cmd := m.overlayHost.HandleProfileOverlayInput(k); handled {
        return m, cmd
    }
    // ... other key handlers ...
}
```

**Canvas layer composition in View() (same lipgloss v2 primitives as bobatea):**

```go
profileLayout, profileOK := m.overlayHost.ComputeProfileOverlayLayout()
if profileOK {
    layers = append(layers,
        lipglossv2.NewLayer(profileLayout.View).
            X(profileLayout.PanelX).
            Y(profileLayout.PanelY).
            Z(25).
            ID("profile-overlay"),
    )
}
```

### Phase 4: Provider Implementation (pinocchio)

**Goal**: Implement the overlay provider interface using pinocchio's profileswitch package.

**Files to create/modify:**

```
pinocchio/pkg/ui/profileswitch/
    overlay_provider.go   - Implements overlay.ProfilePickerProvider
```

**The provider wraps:**
- `Manager.ListProfiles()` → `ProfileListItem` mapping
- `Manager.Resolve()` → `ProfileDetail` mapping
- `Backend.SwitchProfile()` → `ProfileSwitchResult`
- Profile CRUD via geppetto's `Registry` service (for edit/create/delete)

### Phase 5: Remove appModel Wrapper (pinocchio)

**Goal**: Delete the `appModel` hack from `main.go`.

**Changes:**
- Remove `appModel` struct entirely
- Remove `huh` dependency for profile picking
- Pass the profile provider to the REPL/chat model instead
- The interceptor for `/profile` now opens the overlay instead of emitting `openProfilePickerMsg`
- Profile switching events/persistence are triggered by the overlay's action callbacks

### Phase 6: Enhanced Header & Timeline Markers

**Goal**: Improve visual feedback.

**Changes:**
- Richer header showing profile display name + model + temperature
- Styled timeline markers for profile switches (horizontal rule with from→to)
- Brief highlight/flash on the header when profile changes

---

## Part 6: Key Design Decisions

### 6.1 Overlay vs. Command Palette Integration

**Option A**: Profile picker as its own overlay (recommended)
- Pro: Full-screen modal with detail view, editor, creator
- Pro: Clean separation, dedicated Z-layer
- Pro: Can show profile details which require significant space
- Con: Another overlay to manage

**Option B**: Profile switching as a command palette action
- Pro: Reuses existing infrastructure
- Con: Command palette is flat (just a command list)—no room for detail preview, editing, creation
- Con: Would need sub-menus which the palette doesn't support

**Decision**: Option A. Profile management is complex enough to warrant its own overlay. The command palette can still have a "Switch Profile" command that *opens* the profile overlay.

### 6.2 Where to Store Created Profiles

Profiles created in the TUI need a writable backend:

- **SQLite source**: If any registry source is SQLite, write there
- **YAML source**: YAML files can be rewritten (but more brittle)
- **New SQLite**: If all sources are YAML (read-only), auto-create a `~/.config/pinocchio/local-profiles.db` and add it to the chain

The `geppetto` registry already supports mixed sources with `ChainedRegistry`. The TUI would check `ProfileDetail.ReadOnly` to determine if editing is allowed.

### 6.3 Profile Editing Granularity

**Minimal viable editor** (Phase 2):
- Display name, description, system prompt, model name
- These cover 90% of what users want to change

**Full editor** (later):
- Temperature, tools, middlewares, stack parents, tags
- Provider selection dropdown
- Advanced: raw JSON patch editor for `StepSettingsPatch`

### 6.4 Keyboard Shortcuts

```
Global:
  Ctrl+P           Open profile picker (also: /profile command)

In Picker:
  ↑/k  ↓/j         Navigate profile list
  Enter             Switch to highlighted profile
  e                 Edit highlighted profile
  n                 New profile
  d                 Delete highlighted profile (with confirmation)
  /                 Focus filter input
  Esc               Close picker
  Tab               Toggle detail panel (in wide mode)

In Editor/Creator:
  Tab / Shift+Tab   Next/previous field
  Ctrl+S            Save changes
  Esc               Cancel, return to picker

In Read-Only Viewer:
  c                 Clone as new profile
  ↑/↓              Scroll content
  Esc               Back to picker
```

### 6.5 The "Chat Model vs. REPL Model" Question

The current `switch-profiles-tui` uses `chat.Model` directly (not `repl.Model`). The overlay system in bobatea's REPL is the reference pattern, but pinocchio builds its own overlay host.

**Path A**: Move to REPL model
- The REPL model already has an overlay system
- Would need to make the chat backend work as a REPL evaluator
- Couples pinocchio to bobatea's REPL architecture

**Path B**: Build pinocchio's own overlay host (recommended)
- Extract the canvas layer composition pattern into `pinocchio/pkg/tui/overlay/host.go`
- The overlay host wraps any inner `tea.Model` (chat, etc.) with canvas layer support
- Pinocchio TUI apps compose: `overlayHost(chatModel)` — chat renders base, host adds overlay layers
- Follows bobatea's pattern without depending on bobatea's REPL model
- Reusable across all pinocchio TUI binaries

**Path C**: Compose chat model inside REPL model
- REPL evaluator wraps the chat backend
- REPL provides the overlay infrastructure
- Chat handles the actual conversation rendering

**Recommendation**: Path B. Pinocchio builds a lightweight overlay host that follows bobatea's proven pattern (lipgloss v2 canvas layers, key routing priority chain) without coupling to bobatea's REPL model or evaluator interface. The overlay host is a thin compositor that wraps any `tea.Model` and adds overlay layers — profile picker, form overlays, etc. This keeps pinocchio TUI apps simple: `main.go` creates a chat model, wraps it in the overlay host, and registers overlays.

---

## Part 7: Integration with Existing Command Palette

The REPL's command palette can act as a quick-access shortcut:

```go
// In the evaluator's PaletteCommandProvider:
func (e *ChatEvaluator) ListPaletteCommands(ctx context.Context) ([]PaletteCommand, error) {
    return []PaletteCommand{
        {
            ID:          "switch-profile",
            Title:       "Switch Profile",
            Description: "Open profile picker to switch AI profile",
            Action:      func() tea.Cmd { return openProfileOverlay },
        },
        {
            ID:          "new-profile",
            Title:       "New Profile",
            Description: "Create a new AI profile",
            Action:      func() tea.Cmd { return openProfileCreator },
        },
    }, nil
}
```

This means:
- `Ctrl+P` → opens profile picker directly
- `Ctrl+Shift+P` (command palette) → type "switch" → select "Switch Profile" → opens profile picker
- `/profile` in input → interceptor opens profile picker

Multiple entry points, single overlay.

---

## Part 8: Comparison with Current State

| Aspect | Current (appModel hack) | Proposed (canvas overlay) |
|--------|------------------------|--------------------------|
| Visual | Screen takeover (huh.Form) | Floating modal over chat |
| Chat visible | No | Yes (dimmed behind modal) |
| Profile list | Flat select dropdown | Filterable list with cursor |
| Profile details | None | Preview pane (wide mode) |
| Profile editing | Not possible | Full editor sub-view |
| Profile creation | Not possible | Creator sub-view |
| Profile deletion | Not possible | With confirmation |
| Architecture | appModel wrapper in main.go | Reusable widget in pinocchio |
| Key routing | Parallel if/else in appModel | Integrated in overlay host priority chain |
| Canvas layers | Not used | Proper Z=25 layer |
| Reusability | Copy-paste per app | Plug in provider interface |
| Code location | pinocchio/cmd/.../main.go | pinocchio/pkg/tui/ widget + overlay |

---

## Part 9: Risk Assessment

### Low Risk
- Profile picker list view: straightforward list widget
- Canvas layer integration: well-established pattern with 3 existing overlays
- Provider interface: clean abstraction, pinocchio already has the data

### Medium Risk
- Profile editor: form widgets in Bubble Tea are always finicky (focus management, Tab routing, multi-line textareas inside modals)
- Filter/search: needs debouncing and proper cursor management inside the overlay
- Wide-mode detail panel: responsive layout computation needs testing across terminal sizes

### High Risk
- Profile creation with validation: slug uniqueness checking, stack reference validation, real-time feedback
- Profile deletion: needs confirmation dialog (another sub-view) and handling of "deleted active profile" edge case
- Write-back to sources: YAML file rewriting is brittle; SQLite is safer but needs the source to be writable

### Mitigation
- Phase the work: picker first (most value), editor second, creator third
- Test at multiple terminal sizes (80x24, 120x40, 200x60)
- Use SQLite as the default writable backend for new profiles
- Always allow "clone as new" for read-only profiles (avoid needing YAML writes)

---

## Part 10: Summary

The current profile switching UI is a temporary hack that should be replaced with a proper canvas layer overlay following bobatea's established patterns. The key insight is that bobatea already solves the hard problems (canvas composition, key routing priority, overlay lifecycle management)—the current implementation just doesn't use any of it.

The proposed architecture:
1. **Widget in pinocchio**: Reusable `profilepicker.Widget` with list/edit/create modes (`pinocchio/pkg/tui/widgets/profilepicker/`)
2. **Overlay host in pinocchio**: Canvas layer compositor at Z=25, key routing priority chain (`pinocchio/pkg/tui/overlay/`)
3. **Provider in pinocchio**: Maps geppetto profile domain to the widget's interface (`pinocchio/pkg/ui/profileswitch/`)
4. **Delete the hack**: Remove `appModel` wrapper entirely

All new code lives in pinocchio — bobatea's REPL overlay system is the reference architecture (same lipgloss v2 canvas layer pattern), but pinocchio owns the overlay host and all profile UI. This makes the overlay infrastructure reusable across pinocchio TUI binaries without coupling bobatea to domain-specific concerns.

This gives us a profile management experience comparable to VS Code's settings editor or a terminal multiplexer's session picker—a first-class UI component, not a bolted-on afterthought.
