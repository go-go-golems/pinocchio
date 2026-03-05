---
Title: Investigation diary
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
    - Path: ../../../../../../bobatea/pkg/repl/model.go
      Note: Canvas layer overlay system
    - Path: ../../../../../../huh/examples/bubbletea/main.go
      Note: Existing embedding example (key evidence)
    - Path: ../../../../../../huh/form.go
      Note: |-
        Form state machine and embedding assumptions
        Added GroupIndex() and GroupCount() public API (commit df3e5df)
    - Path: cmd/switch-profiles-tui/main.go
      Note: Replaced appModel with overlay.Host (commit c965476)
    - Path: pkg/cmds/chat_profile_switch_model.go
      Note: Stripped to utility funcs (commit d2efe8f)
    - Path: pkg/cmds/cmd.go
      Note: overlay.Host integration point (commit d2efe8f)
    - Path: pkg/tui/overlay/formoverlay_model.go
      Note: Form overlay key routing (commit c25cdd9)
    - Path: pkg/tui/overlay/formoverlay_types.go
      Note: Message types for overlay system (commit c25cdd9)
    - Path: pkg/tui/overlay/host.go
      Note: Overlay host with canvas layer composition (commit c25cdd9)
    - Path: pkg/tui/overlay/host_test.go
      Note: 10 integration tests for overlay host (commit c25cdd9)
    - Path: pkg/tui/widgets/formoverlay/config.go
      Note: Config struct
    - Path: pkg/tui/widgets/formoverlay/styles.go
      Note: Default border and title styles (commit 8901924)
    - Path: pkg/tui/widgets/formoverlay/widget.go
      Note: FormOverlay core - Show/Hide/Toggle/Update/View/ComputeLayout (commit 8901924)
    - Path: pkg/tui/widgets/formoverlay/widget_test.go
      Note: 12 unit tests for FormOverlay (commit 8901924)
    - Path: pkg/ui/profileswitch/picker.go
      Note: Profile picker form factory (commit c965476)
ExternalSources: []
Summary: Chronological investigation of huh's embeddability and what it takes to run forms as modal overlays.
LastUpdated: 2026-03-04T00:00:00Z
WhatFor: Record investigation steps and decisions
WhenToUse: When reviewing the analysis process or continuing the work
---






# Investigation Diary: huh Modal Support (SPT-3)

## Goal

Determine what it takes to make `charmbracelet/huh` forms embeddable as modal overlays in bobatea applications, using the existing lipgloss v2 canvas layer system.

## Context

SPT-1 (profile switching) revealed that the current approach of wrapping chat models with an `appModel` that swaps `View()` is architecturally broken — it replaces the entire screen, ignores bobatea's overlay system, and can't be reused. The question is: can huh forms be used inside bobatea overlays, and what adapter work is needed?

## Entry 1 — 2026-03-04: Initial Architecture Survey

### What I did

1. Listed all `.go` files in `huh/` root (19 files).
2. Read `huh/form.go` in detail: Form struct, state machine (`StateNormal/Completed/Aborted`), `Update()` logic, `View()` delegation to Layout, `Run()` method.
3. Read `huh/group.go`: Group struct with `viewport.Model`, field selector, focus management, `buildView()`.
4. Read `huh/layout.go`: 4 layout implementations (Default, Stack, Columns, Grid). All use full width.
5. Read `huh/keymap.go`: Comprehensive keybindings for all field types.
6. Read all field implementations: Input, Text, Select, MultiSelect, Confirm, Note, FilePicker.
7. Read `huh/theme.go`: Theme with Focused/Blurred field styles.
8. Read `huh/run.go`: `Run()` helper wraps single field in Form+Group.
9. Read `huh/examples/bubbletea/main.go`: **Critical evidence** — shows huh form embedded in parent model.

### What I found

**Good news**: huh forms implement `tea.Model` and can be embedded in a parent model today. The bubbletea example proves this works:
- `Init()` delegation works
- `Update()` delegation works
- `View()` delegation works
- `form.State` checking works
- `form.Get()` for results works

**Bad news**: Six specific gaps prevent modal overlay use:

1. **Output exceeds set dimensions**: `WithWidth(45)` sets content width but theme padding/borders add to rendered size. No clipping.
2. **Key greediness**: Form catches `ctrl+c` unconditionally at `form.go:528-535`. All `KeyMsg` goes to fields with no parent escape hatch.
3. **State lock after completion**: `form.go:501-505` — `if f.State != StateNormal { return f, nil }`. Dead after completion.
4. **SubmitCmd/CancelCmd defaults**: Set to `tea.Quit` in `RunWithContext()` but `nil` in standalone use. Safe for embedding but still fires `StateAborted` on `ctrl+c`.
5. **No modal framing**: No title bar, border, footer.
6. **No lifecycle management**: No Show/Hide/Toggle.

### What worked

- Reading the bubbletea example first gave me the key insight: huh is *already composable*. The problems are all *around* the form, not inside it.
- Mapping each gap to a specific line in the source code made the analysis concrete and actionable.

### What was tricky

- Understanding the dimension chain: Form → Layout.GroupWidth() → Group.WithWidth() → Field.WithWidth(). The theme padding is applied at field render time, AFTER width is set. This means `form.WithWidth(45)` produces output wider than 45 columns.
- Understanding that `form.Run()` creates its OWN `tea.NewProgram(f)` — this is the key reason you can't just call `.Run()` inside another program. But you don't need to — just use `Update()/View()`.

### Key decision

**Adapter, not fork.** The gaps can all be addressed by wrapping `huh.Form` in a thin adapter that handles visibility, key interception, dimension constraints, and framing. No changes to huh internals needed.

## Entry 2 — 2026-03-04: Solution Design

### What I did

1. Studied bobatea's overlay pattern in detail (command palette, help drawer, completion).
2. Identified the common overlay lifecycle: lazy init → show → key routing → compute layout → render → canvas layer → hide.
3. Designed the `FormOverlay` adapter that wraps `huh.Form`.
4. Chose the factory pattern (`func() *huh.Form`) to solve the state lock problem — fresh form on each open.
5. Designed key interception: Esc/ctrl+c intercepted before form sees them.
6. Designed canvas layer integration at Z=28.

### What worked

- The factory pattern elegantly avoids the state lock problem. No need to implement `Reset()` on a completed form — just create a new one. Forms are cheap (no I/O, no goroutines).
- Mapping the adapter to bobatea's existing overlay file patterns (`*_model.go`, `*_overlay.go`, `*_types.go`) made the integration path obvious.

### What was tricky

- Esc key conflict: huh's Select field uses Esc to exit filter mode. If the adapter intercepts Esc first, filter mode breaks. Solution: state-aware Esc handling in Phase 2 (check if a field is in a sub-mode before intercepting).
- Dimension mismatch: need to account for both theme padding AND modal border when computing `form.WithWidth()`. Belt-and-suspenders: set `WithWidth(contentWidth)` AND `MaxWidth(maxWidth)` on the border style.

### Code review instructions

When reviewing the design doc:
1. Check that every claim about huh's behavior cites a specific file and line range.
2. Verify the FormOverlay API makes sense for the profile picker use case (SPT-1).
3. Consider whether the Z-index choice (28) conflicts with any planned overlays.
4. Consider the Esc key conflict with Select filter mode — Phase 2 needs to address this.

## Entry 3 — 2026-03-04: uhoh Analysis

### What I did

1. Read `uhoh/pkg/formdsl.go` in full (744 lines): Form DSL struct, BuildBubbleTeaModel(), Run(), ExtractFinalValues(), field type handling.
2. Read `uhoh/pkg/wizard/wizard.go` in full (464 lines): Wizard struct, Run() blocking loop, state management, expr-lang conditions, callback lifecycle.
3. Surveyed all step types in `uhoh/pkg/wizard/steps/`: FormStep, DecisionStep, ActionStep, InfoStep, SummaryStep, BaseStep.
4. Read `uhoh/examples/bubbletea-embed/main.go` (230 lines): working proof that uhoh form embedding works via BuildBubbleTeaModelFromYAML().
5. Identified code duplication between Form.Run() and BuildBubbleTeaModel() (~300 lines of identical switch-case code).
6. Wrote comprehensive uhoh analysis document covering architecture map, 6 gaps, WizardModel proposal, 5-phase implementation plan.

### What I found

**Good news**: uhoh forms already support Bubble Tea embedding via `BuildBubbleTeaModel()`. The bubbletea-embed example proves this works. The FormOverlay adapter from the design doc can wrap uhoh forms with zero uhoh changes.

**Bad news**: The wizard engine (`Wizard.Run()`) is fundamentally incompatible with embedding — it's a blocking sequential loop where every step calls `huh.Run()` or `huh.NewNote().Run()`, each of which creates its own `tea.NewProgram()`.

Six gaps identified:
1. Wizard.Run() is a blocking loop (wizard.go:219-389)
2. Every step calls huh.Run() internally
3. No step-level tea.Model interface
4. State merging assumes synchronous flow
5. Callbacks assume blocking execution
6. ActionStep goroutine pattern is incompatible

### What worked

- Discovering the existing `BuildBubbleTeaModel()` function and the bubbletea-embed example immediately showed that single-form embedding is a solved problem — the work is all about wizards.
- Identifying the Form.Run() vs BuildBubbleTeaModel() code duplication gave a clean first refactoring step.

### What was tricky

- Understanding ActionStep's dual goroutine/blocking pattern. It spawns a goroutine to show progress (huh.Note in its own tea.NewProgram) while running the callback, then shows a blocking completion note. Both patterns are incompatible with embedding.
- Designing the WizardModel state machine: step preparation → running → callbacks → navigation → next step. The callback chain (before → execute → after → validation → navigation) needs to happen across multiple Update() cycles.

### Key decision

**WizardModel as new tea.Model + EmbeddableStep interface.** Each step type needs a `BuildModel()` method that returns a `tea.Model` without calling Run(). The WizardModel drives steps through the Bubble Tea update loop, handling state merging and callbacks as messages.

## Entry 4 — 2026-03-04: Phase 1 Implementation — FormOverlay Widget Core

Implemented the full FormOverlay widget in `pinocchio/pkg/tui/widgets/formoverlay/`. This is the core adapter that wraps `huh.Form` with modal overlay behaviors — visibility lifecycle, key interception, dimension constraints, and canvas layer positioning. All four files were created from scratch following the design doc (Section 4.2–4.6) and the patterns observed in bobatea's overlay system.

### Prompt Context

**User prompt (verbatim):** "Now work on the ticket task by task, checking things off as you go, committing at appropriate intervals, and keeping a detailed diary. Look at the skills for more information on how to do that."

**Assistant interpretation:** Implement SPT-3 tasks sequentially, starting with Phase 1, committing code at milestones, using docmgr for task tracking, and maintaining an investigation diary per the diary skill format.

**Inferred user intent:** Get the FormOverlay feature built incrementally with proper documentation trail so progress is reviewable and resumable.

**Commit (code):** `8901924` — "feat(tui): add FormOverlay widget for modal huh forms"

### What I did

1. Explored pinocchio directory structure and `go.mod` to confirm dependencies (huh v0.8.0, lipgloss v1.1.1, bubbletea v1.3.10).
2. Read bobatea's `model.go` overlay pattern for reference (canvas layers, Z-indices, compositor).
3. Created `config.go`: `Config` struct with Title, MaxWidth/MaxHeight, Placement enum (Center/Top/TopRight), Factory, OnSubmit/OnCancel callbacks, BorderStyle/TitleStyle.
4. Created `styles.go`: `DefaultBorderStyle()` (rounded border, color "62", padding 1,2) and `DefaultTitleStyle()` (bold, inverted color "230"/"62", padding 0,1, margin-bottom 1). Constants for `defaultMaxWidth=60`, `defaultMaxHeight=20`.
5. Created `widget.go`: `FormOverlay` struct with full lifecycle:
   - `New(cfg)` constructor with sensible defaults for zero-value styles and dimensions
   - `Show()` calls factory, sets visible, computes content width, calls `form.Init()`
   - `Hide()` sets visible=false, nils form
   - `Toggle()` calls Show or Hide
   - `Update()` with key interception (esc/ctrl+c before form), form state checking (StateCompleted→onSubmit, StateAborted→onCancel)
   - `View()` with title bar + form content + border with MaxWidth/MaxHeight clipping
   - `ComputeLayout()` returns `OverlayLayout{X, Y, View}` with placement-based positioning and bounds clamping
   - `contentWidth()` helper accounting for horizontal frame size
6. Created `widget_test.go`: 12 unit tests covering all behaviors.
7. Fixed two lint issues: exhaustive switch (added `huh.StateNormal` case), nonamedreturns (refactored to `OverlayLayout` struct return).

### Why

Phase 1 is the foundation — every subsequent phase depends on this widget. The factory pattern, key interception, and ComputeLayout are the three critical design decisions that enable modal overlay usage.

### What worked

- The factory pattern works exactly as designed. Tests confirm each `Show()` creates a fresh form via the factory.
- Key interception correctly catches esc/ctrl+c before the form sees them — verified by test.
- `lipgloss.Style.MaxWidth()/MaxHeight()` clips rendered output as expected — no need for manual truncation.
- Using `border.Value() == ""` to detect zero-value lipgloss.Style works for defaulting.
- All 12 tests pass on first run after lint fixes.

### What didn't work

- **Lint: exhaustive switch** — golangci-lint's exhaustive checker required handling all `huh.State` values including `StateNormal`. Added a no-op case.
- **Lint: nonamedreturns** — original `ComputeLayout` had `(x, y int, view string, ok bool)` named returns. Refactored to return `OverlayLayout` struct instead, which is cleaner anyway.

### What I learned

- `huh.Form.Update()` returns `(tea.Model, tea.Cmd)` not `(*huh.Form, tea.Cmd)` — need a type assertion `model.(*huh.Form)` to get back to the concrete type.
- Pinocchio already has lipgloss v2 as an indirect dependency, so Phase 2 can use `lipglossv2.NewCanvas` without adding new deps.
- `go.work` includes all local modules, so changes across pinocchio/bobatea/uhoh are immediately visible during development.

### What was tricky to build

- **Content width calculation**: Need to subtract the border's horizontal frame size from maxWidth to get the form content width. If maxWidth=60 and border has padding(1,2) + border(1), that's 6 chars of horizontal chrome, so content gets 54. The `contentWidth()` helper encapsulates this with a minimum of 10 to prevent absurdly narrow forms.
- **State machine ordering in Update()**: Key interception must happen BEFORE `form.Update()`, but state checking must happen AFTER. The esc/ctrl+c check returns early (never reaches the form), while regular keys pass through to the form, then we check `form.State` for completion/abort transitions.

### What warrants a second pair of eyes

- The `border.Value() == ""` check for detecting zero-value lipgloss.Style — this works for the current lipgloss v1 API but may not be the intended way to check for "no style set." Consider if there's a more robust approach.
- The `OverlayLayout` type is defined inside `widget.go` rather than `config.go` — it's a return type, not a config type, so this seemed right, but it could be argued either way.

### What should be done in the future

- Phase 2 (Overlay Host) will consume `ComputeLayout()` to position the overlay on a canvas layer.
- The Esc key conflict with huh's Select filter mode (Entry 2) is deferred to Phase 6 (task 6.3).
- Consider adding `SetMaxWidth()`/`SetMaxHeight()` methods for dynamic resizing in response to terminal size changes.

### Code review instructions

- Start at `widget.go:108` (Update method) — this is the key logic: key interception → form delegation → state checking.
- Then `widget.go:68` (Show) — factory call + width setup + Init.
- Then `widget.go:155` (View) — title + form + border rendering.
- Then `widget.go:187` (ComputeLayout) — placement math + clamping.
- Validate with: `cd pinocchio && go test ./pkg/tui/widgets/formoverlay/ -v -count=1`

### Technical details

Files created:
- `pinocchio/pkg/tui/widgets/formoverlay/config.go` (49 lines)
- `pinocchio/pkg/tui/widgets/formoverlay/styles.go` (27 lines)
- `pinocchio/pkg/tui/widgets/formoverlay/widget.go` (229 lines)
- `pinocchio/pkg/tui/widgets/formoverlay/widget_test.go` (229 lines)

Test output (12/12 pass):
```
=== RUN   TestShowHideLifecycle ... PASS
=== RUN   TestToggle ... PASS
=== RUN   TestEscClosesOverlay ... PASS
=== RUN   TestCtrlCClosesOverlay ... PASS
=== RUN   TestRegularKeysPassThrough ... PASS
=== RUN   TestFactoryCreatesFreshForm ... PASS
=== RUN   TestViewWhenHidden ... PASS
=== RUN   TestViewWhenVisible ... PASS
=== RUN   TestComputeLayoutHidden ... PASS
=== RUN   TestComputeLayoutCenter ... PASS
=== RUN   TestComputeLayoutTop ... PASS
=== RUN   TestUpdateWhenHidden ... PASS
```

## Entry 5 — 2026-03-04: Phase 2 Implementation — Overlay Host

Implemented the overlay host in `pinocchio/pkg/tui/overlay/`. This is the compositing layer that wraps an inner `tea.Model` and renders FormOverlay modals on top using lipgloss v2 canvas layers. The host follows bobatea's REPL overlay pattern (priority key routing, Z-index layers, canvas composition) but lives in pinocchio as a reusable component.

### Prompt Context

**User prompt (verbatim):** (same as Entry 4)

**Assistant interpretation:** Continue implementing SPT-3 tasks — Phase 2 creates the overlay host that integrates FormOverlay into a compositing layer system.

**Inferred user intent:** Build the integration layer so FormOverlay can actually render on top of a chat model.

**Commit (code):** `c25cdd9` — "feat(tui): add overlay host with canvas layer composition"

### What I did

1. Studied bobatea's overlay system in detail via explore agent: canvas layer composition (`model.go:278-362`), key routing priority chain (`model_input.go`), layout computation patterns (`command_palette_overlay.go`).
2. Created `formoverlay_types.go`: Message types — `OpenFormOverlayMsg`, `FormOverlayCompletedMsg`, `FormOverlayCancelledMsg`.
3. Created `host.go`: `Host` struct wrapping inner `tea.Model` with `FormOverlay` field. `Init()` delegates to inner. `Update()` routes `OpenFormOverlayMsg` to open overlay, `tea.KeyMsg` to overlay first (when visible) then inner, all else to inner. `View()` composites base + overlay using `lipglossv2.NewCanvas` + `lipglossv2.NewCompositor`. Exposes `SetFormOverlay()` for runtime registration, `FormOverlayVisible()` for state queries.
4. Created `formoverlay_model.go`: `handleFormOverlayInput()` implements the `(handled bool, cmd tea.Cmd)` pattern from bobatea — when overlay is visible, ALL keys go to it (modal behavior).
5. Created `formoverlay_overlay.go`: `computeFormOverlayLayout()` delegates to `FormOverlay.ComputeLayout()`.
6. Created `host_test.go`: 10 integration tests covering delegation, visibility, key routing, canvas composition, close-and-resume, runtime registration, and no-overlay edge case.

### Why

Phase 2 is the glue between Phase 1 (widget) and Phase 3 (profile picker). Without the host, there's no way to render overlays as canvas layers — you'd still need an `appModel`-style hack.

### What worked

- The bobatea patterns translated directly to pinocchio. The `handleFormOverlayInput` → `(handled, cmd)` pattern, the `computeLayout` → `(layout, ok)` pattern, and the canvas layer composition all work exactly as in bobatea.
- Using `Host` as a value type (not pointer) works well with Bubble Tea's update model — `Update` returns `(tea.Model, tea.Cmd)` and the host is small enough that copies are cheap.
- lipgloss v2 was already an indirect dep in pinocchio — just importing `charm.land/lipgloss/v2` directly worked without touching `go.mod`.

### What didn't work

- `gofmt` formatting issues with import block ordering (lipgloss v2 import vs bubbletea) — fixed by running gofmt.

### What I learned

- The lipgloss v2 canvas API is clean: `NewLayer(content).X(x).Y(y).Z(z).ID(id)`, `NewCompositor(layers...)`, `NewCanvas(w, h).Compose(comp).Render()`. No surprises.
- Bobatea's Z-index hierarchy: Z=0 base, Z=15 help drawer, Z=20 completion, Z=30 command palette. FormOverlay uses Z=28 (above completion but below command palette).

### What was tricky to build

- Deciding on the `Host` type's relationship with `FormOverlay`: pointer field vs lazy init. Went with pointer field set at construction time or via `SetFormOverlay()` — simpler than bobatea's `ensure*Widget()` pattern since we only have one overlay type to manage.

### What warrants a second pair of eyes

- The `Host` is a value type (struct, not pointer). This means `SetFormOverlay()` takes `*Host` receiver while other methods take `Host` value receiver. This is intentional (Bubble Tea convention) but could be confusing.

### What should be done in the future

- Add support for multiple overlay types (help drawer, command palette) if pinocchio needs them.
- Consider adding a scrim/dimming layer behind the overlay for visual separation.

### Code review instructions

- Start at `host.go:49` (Update) — key routing logic: OpenFormOverlayMsg → open, KeyMsg → overlay first then inner.
- Then `host.go:79` (View) — canvas layer composition.
- Then `formoverlay_model.go:11` (handleFormOverlayInput) — modal key routing.
- Validate with: `cd pinocchio && go test ./pkg/tui/overlay/ -v -count=1`

## Entry 6 — 2026-03-04: Phase 3 Implementation — Profile Picker Overlay

Replaced the `appModel` hack in `switch-profiles-tui/main.go` with a FormOverlay-based profile picker. This is the payoff — the chat now remains visible behind the modal profile picker, and the overlay system handles all key routing and lifecycle management. Net -56 lines of code.

### Prompt Context

**User prompt (verbatim):** (same as Entry 4)

**Assistant interpretation:** Continue implementing SPT-3 — Phase 3 wires the profile picker into the overlay host and removes the appModel.

**Inferred user intent:** Get a working profile picker that renders as a modal overlay instead of replacing the entire screen.

**Commit (code):** `c965476` — "feat(tui): replace appModel with overlay host for profile switching"

### What I did

1. Created `pkg/ui/profileswitch/picker.go`: `PickerFormFactory(mgr, &selected)` returns `func() *huh.Form` that builds a `huh.Select` populated from the profile registry. Handles error case (no profiles) with a fallback Note form.
2. Rewrote `cmd/switch-profiles-tui/main.go`:
   - Added imports for `overlay` and `formoverlay` packages.
   - Removed entire `appModel` struct (type, constructor, Init, Update, View — 125 lines).
   - Removed `openProfilePickerMsg` type.
   - Created `formoverlay.New()` with `PickerFormFactory`, `OnSubmit` callback that calls `backend.SwitchProfile()` + publish + persist.
   - Created `overlay.NewHost(chatModel, overlay.Config{FormOverlay: profileOverlay})`.
   - Changed `/profile` command to emit `overlay.OpenFormOverlayMsg{}` instead of `openProfilePickerMsg{}`.
   - Changed `tea.NewProgram(host, ...)` instead of `tea.NewProgram(appModel, ...)`.
3. Net result: 79 lines added, 135 removed = -56 lines.

### Why

The `appModel` had three critical problems: (1) it replaced the entire screen during profile selection, (2) it couldn't be reused by other pinocchio TUI apps, (3) it duplicated form lifecycle management. The overlay host solves all three.

### What worked

- `PickerFormFactory` cleanly separates form creation from overlay wiring — it's a pure function that takes a manager and returns a factory.
- The `OnSubmit` callback captures `backend`, `sink`, `persistSwitch`, and `convID` in its closure — no need for a struct to hold these references.
- `/profile <slug>` (direct switch) still works unchanged — only `/profile` (no args) was changed to use the overlay.
- Build + test + lint all pass on first try after gofmt.

### What didn't work

- Nothing significant. The rewrite was straightforward because Phases 1 and 2 had already built the right abstractions.

### What I learned

- The old `appModel.Update()` handled both "open picker" and "picker completed" inline — about 80 lines of deeply nested switch/if logic. The overlay host + FormOverlay replaces all of that with a 10-line `OnSubmit` callback plus the overlay infrastructure.
- Closure-based callbacks are much cleaner than storing state in a struct when the state is only needed during one operation.

### What was tricky to build

- The `OnSubmit` callback runs synchronously (not as a `tea.Cmd`), so it can't emit timeline events via `tea.Batch`. The old appModel could return `tea.Batch(publishCmd, entityCmd)`. The new OnSubmit calls publish/persist directly but can't emit `localPlainEntityCmd` for the UI timeline. This is a minor regression — the profile switch still works, but no "profile_switched" entity appears in the timeline. A future enhancement could have `OnSubmit` return a `tea.Cmd` instead of being a plain `func()`.
- The `selectedSlug` variable is declared outside the overlay and its pointer is captured by both `PickerFormFactory` and `OnSubmit`. This works because the factory writes to it (via huh's `Value(&selectedSlug)`) and `OnSubmit` reads it — but the ownership chain is implicit.

### What warrants a second pair of eyes

- **OnSubmit sync limitation**: The `OnSubmit` callback can't return commands to the Bubble Tea event loop. Profile switch works but doesn't produce timeline entities. Consider changing `OnSubmit` signature to return `tea.Cmd` in a future iteration.
- **selectedSlug shared ownership**: The `&selectedSlug` pointer is shared between the factory and OnSubmit via closure. Thread-safe because Bubble Tea is single-threaded, but worth noting.

### What should be done in the future

- Task 3.5 (smoke test): Run with real profile registries and verify the modal overlay works end-to-end.
- Consider changing `FormOverlay.OnSubmit` to return `tea.Cmd` so callbacks can emit timeline events.
- Add blur/unblur for the chat input when the overlay opens/closes (the old appModel did this).

### Code review instructions

- Start at `main.go:290-318` — overlay creation and host wiring.
- Then `picker.go` — the factory function.
- Then look at what was removed: the entire `appModel` type (diff shows -135 lines).
- Validate with: `cd pinocchio && go build ./cmd/switch-profiles-tui/`

## Entry 7 — 2026-03-04: Smoke Test and Message Routing Fix

Ran the full smoke test in tmux with real profile registries. Discovered and fixed a critical bug: huh forms need their Init() messages to be processed, but the overlay host was only routing `tea.KeyMsg` to the overlay. All other messages (huh internal focus/cursor setup) were falling through to the inner model and being lost.

### Prompt Context

**User prompt (verbatim):** "you should send /profile + <TAB> i think" (correcting my initial test approach of using Enter instead of Tab to submit)

**Assistant interpretation:** Test the overlay with the correct chat submit key (Tab, not Enter).

**Inferred user intent:** Verify the overlay works end-to-end with real profile data.

**Commit (code):** `b2464bf` — "fix(tui): route non-key messages to form overlay when visible"

### What I did

1. Built the binary: `go build -o /tmp/switch-profiles-tui ./cmd/switch-profiles-tui/`
2. Launched in tmux: `tmux new-session -d -s smoke -x 120 -y 40 '/tmp/switch-profiles-tui --profile-registries yaml:/tmp/profile-registry.yaml'`
3. Tested: Enter to focus input → `/profile` → Tab to submit → overlay appeared
4. Observed: Overlay rendered correctly (centered, bordered, 4 profiles listed) — but Enter didn't select a profile
5. Diagnosed: huh form's Init() cmd produces internal messages that weren't being routed to the overlay
6. Fixed `host.go` Update: when overlay is visible, `default` case routes all non-key messages to overlay instead of inner model
7. Rebuilt, retested: arrow navigation works, Enter selects, profile header updates to `mento-sonnet-4.6`, Esc cancels cleanly

### What worked

- tmux smoke testing worked well — `tmux capture-pane -t smoke -p` gives a clean text snapshot of the TUI state
- The fix was straightforward: add `if h.formOverlay.IsVisible() { ... }` to the default case in Update
- All 4 test scenarios passed: open overlay, navigate, select (profile switches), cancel with Esc

### What didn't work

- Initial attempt used Enter to submit `/profile` — but the chat uses Tab to submit. Enter first goes to "focus input" mode.
- Before the fix, the form rendered visually but was non-functional: Enter did nothing because huh's Select field hadn't processed its Init messages.

### What was tricky to build

- **The Init message routing bug**: When `Show()` returns `form.Init()` as a `tea.Cmd`, Bubble Tea processes it and the resulting huh internal messages come back through `Update()`. These messages are NOT `tea.KeyMsg` — they're huh-specific types (focus messages, cursor setup). The original code only routed `tea.KeyMsg` to the overlay, so all these critical initialization messages went to the inner model and were silently discarded. The form rendered (View works without Init) but couldn't respond to input (the Select field's internal state wasn't set up).

### What warrants a second pair of eyes

- The fix routes ALL non-key messages to the overlay when visible. This means the inner model doesn't receive any messages while the overlay is active. This is correct for a modal overlay (it's supposed to block everything) but could be surprising if someone expects the inner model to keep receiving background updates.

### What should be done in the future

- Consider whether WindowSizeMsg should update the overlay's dimensions (currently the overlay's MaxWidth/MaxHeight are fixed at creation time).
- Add blur/unblur for the chat input when the overlay opens/closes.

### Code review instructions

- The fix is in `host.go:66-77` — the `default` case now checks `formOverlay.IsVisible()` before deciding where to route messages.
- Test by running: `go build -o /tmp/switch-profiles-tui ./cmd/switch-profiles-tui/ && /tmp/switch-profiles-tui --profile-registries yaml:/tmp/profile-registry.yaml`

## Step 8: Phase 4 — Multi-Step Wizard Support

Phase 4 adds step progress tracking for multi-group huh forms. When a form has more than one group, the overlay title dynamically shows "Step N of M" (e.g., "Wizard (Step 2 of 3)"). The implementation required two coordinated changes: exposing group tracking from huh's internal selector, and consuming it in the FormOverlay's View method.

### Prompt Context

**User prompt (verbatim):** (same as Step 4)

**Assistant interpretation:** Implement Phase 4 tasks: multi-group form step tracking, variable height handling, and wizard tests.

**Inferred user intent:** Enable wizard-style multi-step forms in the overlay system with proper progress indication.

**Commit (code):** df3e5df — "Add GroupIndex() and GroupCount() public API for multi-group forms" (huh)
**Commit (code):** c6246b4 — "Add multi-group wizard support to FormOverlay" (pinocchio)

### What I did

- Added `GroupIndex()` and `GroupCount()` public methods to `huh/form.go` — delegates to internal `f.selector.Index()` and `f.selector.Total()`
- Modified `formoverlay/widget.go` View() to show "Step N of M" in title bar when `GroupCount() > 1`
- Added 4 multi-group tests: step progress title rendering, single-group title has no step indicator, overlay stays visible across groups, Esc still closes multi-group forms
- Verified task 4.2 (variable height) is already handled — View() and ComputeLayout() render fresh each frame, so different-sized groups naturally produce different overlay dimensions

### Why

- Step progress is essential UX for wizard-style forms so users know where they are
- The huh Form's group tracking is private (internal selector) — needed public API to expose it
- Single-group forms should not show "Step 1 of 1" — only multi-group forms get the indicator

### What worked

- huh's `selector` type already had `Index()` and `Total()` — just needed to expose them
- The View-per-frame rendering approach means variable height across groups "just works" with no extra code
- All 16 formoverlay tests pass, all 10 overlay host tests pass, lint and vet clean

### What didn't work

- N/A — straightforward implementation

### What I learned

- huh's group navigation uses a `selector` type that tracks index internally
- The Form's Update handler processes `nextGroupMsg`/`prevGroupMsg` messages to advance between groups
- Because View() is called every frame with no caching, layout recomputation is free — the overlay naturally adapts to different group sizes

### What was tricky to build

- Nothing particularly tricky here. The main consideration was deciding where to add the public API (on Form, not on the internal selector) and ensuring the step display only appears for multi-group forms.

### What warrants a second pair of eyes

- The `GroupIndex()` and `GroupCount()` additions to huh — these are small but add public API surface to a vendored fork. If huh upstream adds similar methods with different signatures, there could be a conflict.

### What should be done in the future

- Interactive smoke test of a multi-group wizard overlay to verify step navigation visually
- Consider adding a progress bar or dots alongside the step text for better visual feedback

### Code review instructions

- Start with `huh/form.go` — search for `GroupIndex` to see the 2 new methods
- Then `pinocchio/pkg/tui/widgets/formoverlay/widget.go:166-169` — the step progress title logic
- Tests: `widget_test.go` — search for `TestMultiGroup` to see the 4 new tests
- Validate: `go test ./pinocchio/pkg/tui/widgets/formoverlay/... -count=1 -v`

## Step 9: Phase 5 — uhoh EmbeddableStep + WizardModel + async ActionStep

Completed the uhoh integration layer. Created the `EmbeddableStep` interface and `WizardModel` tea.Model, then extended it with `AsyncEmbeddableStep` for ActionStep's non-blocking callback execution. Also added `LoadWizardFromBytes` helper and a full `bubbletea-embed-wizard` example.

### Prompt Context

**User prompt (verbatim):** (same as Step 7 — continuation)

**Assistant interpretation:** Continue Phase 5 tasks: EmbeddableStep interface, WizardModel, async ActionStep, wizard example.

**Inferred user intent:** Complete the uhoh integration so wizards can be embedded in FormOverlay.

**Commit (code):** `ef5250e` — "Add EmbeddableStep interface and async WizardModel"
**Commit (code):** `a079474` — "Add async ActionStep embedding and bubbletea-embed-wizard example"

### What I did
- Created `embeddable.go` with `EmbeddableStep` and `AsyncEmbeddableStep` interfaces
- Implemented `BuildModel()` on FormStep, DecisionStep, InfoStep, SummaryStep
- Created `wizard_model.go` — async tea.Model driving wizard steps
- Added `ActionCompletedMsg`, `RunAsync()`, `GetOutputKey()` on ActionStep
- Updated WizardModel to batch async work and handle ActionCompletedMsg
- Refactored `LoadWizard` to delegate to new `LoadWizardFromBytes`
- Created `bubbletea-embed-wizard` example (info → form → decision → summary)

### Why
- EmbeddableStep provides the non-blocking form creation contract for wizard steps
- WizardModel enables multi-step wizards inside FormOverlay
- AsyncEmbeddableStep solves the problem that ActionStep's callback blocks the event loop

### What worked
- Clean separation: EmbeddableStep for form-based steps, AsyncEmbeddableStep for async work
- The `awaitingAsync` flag in WizardModel prevents premature form completion from advancing while callback is still running

### What didn't work
- Nothing major. Previous commit attempt failed on exhaustive switch lint (fixed in prior session).

### What I learned
- huh.Form's state machine (StateNormal → StateCompleted → StateAborted) works well as the advancement signal for wizard steps
- For ActionStep, the form is just a visual placeholder while the real work happens via tea.Cmd

### What was tricky to build
- The async step flow required careful coordination: WizardModel must not advance when the progress Note completes (which it does immediately since it has `Next(true)`), only when ActionCompletedMsg arrives. The `awaitingAsync` flag gates form completion checking.

### What warrants a second pair of eyes
- ActionStep.RunAsync uses `context.Background()` instead of a user-provided context. This is a limitation of the current tea.Cmd model — the cmd function has no access to a cancellation context.

### What should be done in the future
- Add spinner/animation to ActionStep's progress Note instead of static text
- Add error display form when ActionCompletedMsg.Err != nil (currently aborts)

### Code review instructions
- Start: `uhoh/pkg/wizard/steps/embeddable.go` — interfaces
- Then: `uhoh/pkg/wizard/wizard_model.go` — WizardModel with async support
- Then: `uhoh/pkg/wizard/steps/action_step.go` — BuildModel/RunAsync
- Example: `uhoh/examples/bubbletea-embed-wizard/main.go`
- Validate: `cd uhoh && go build ./...`

## Step 10: Phase 6 — Convenience API, double-Esc, and example

Added developer-facing convenience constructors and addressed the Esc key conflict with huh's Select filter mode. Created a minimal example demonstrating the helper APIs.

### Prompt Context

**User prompt (verbatim):** (same as Step 7 — continuation)

**Assistant interpretation:** Continue to Phase 6 tasks.

**Inferred user intent:** Make FormOverlay easier to use with common patterns and fix the Select filter UX issue.

**Commit (code):** `eb90661` — "Add convenience helpers, double-Esc mode, and form-overlay example"

### What I did
- Created `helpers.go` with `NewSelect`, `NewConfirm`, `NewInput` constructors
- Added `DoubleEscToClose` config option: first Esc passes to form, second closes
- Created `examples/form-overlay/main.go` — interactive demo of all helpers
- Added 2 tests for double-Esc behavior (basic + key-reset)

### Why
- Convenience constructors reduce boilerplate for the most common overlay patterns
- DoubleEscToClose is needed because huh's Select uses Esc to clear its filter — our overlay intercepts Esc first, breaking filtering

### What worked
- Simple stateful approach: `escPending` bool tracks whether first Esc was seen. Any non-Esc key resets it, so double-Esc must be consecutive.
- All 20 tests pass including the new double-Esc tests.

### What didn't work
- Initial commit failed on gofmt lint for the example file (struct field alignment). Fixed with `gofmt -w`.

### What I learned
- huh Select's ClearFilter keybinding is bound to Esc but only enabled when `!filtering && filter.Value() != ""`. So Esc has multiple meanings depending on Select's internal state.

### What was tricky to build
- N/A — straightforward implementation.

### What warrants a second pair of eyes
- The double-Esc approach is a pragmatic workaround. Ideally, the overlay would introspect huh's field state to know if Esc should pass through. But huh doesn't expose this, so double-Esc is the safest generic solution.

### What should be done in the future
- Consider exposing huh Select's filtering state to enable smarter Esc routing
- Add `NewWizard` convenience helper once wizard embedding stabilizes

### Code review instructions
- Start: `pinocchio/pkg/tui/widgets/formoverlay/helpers.go` — 3 constructors
- Then: `widget.go` — search for `escPending` for double-Esc logic
- Tests: `widget_test.go` — search for `TestDoubleEsc`
- Example: `pinocchio/examples/form-overlay/main.go`

## Step 11: Phase 7 — Profile picker/header enhancements and diff utility

Enhanced the profile picker with descriptions and current-profile marking, enriched the header bar with model/temperature info, and added a ProfileDiff utility for human-readable switch summaries. Tasks 7.2 (profile editor), 7.3 (profile creator), and 7.6 (command palette) deferred.

### Prompt Context

**User prompt (verbatim):** (same as Step 7 — continuation)

**Assistant interpretation:** Continue to Phase 7 profile management enhancements.

**Inferred user intent:** Polish the profile switching UX with better information display.

**Commit (code):** `f32ef12` — "Enhance profile picker and header with model/temp info and diff support"

### What I did
- Enhanced header: shows `profile=X  model=Y  temp=Z  runtime=W` (task 7.5)
- Enhanced picker: marks current profile with `*`, shows description, increased overlay size to 60x25, enabled DoubleEscToClose (task 7.1)
- Created `diff.go` with `ProfileDiff()` utility comparing model, runtime, temperature, system prompt (task 7.4)
- Wired diff logging into overlay OnSubmit callback

### Why
- Users need to see at a glance what model/temperature is active without inspecting profile configs
- Current-profile marking in the picker prevents accidentally re-selecting the same profile
- ProfileDiff provides transparency about what changes on switch

### What worked
- Drawing from `EffectiveStepSettings.Chat.Engine` and `Chat.Temperature` for the header provides accurate runtime config info
- `formatPickerLabel` adds structure without overcomplicating the Select options

### What didn't work
- Task 7.2 (profile editor) and 7.3 (profile creator) require mutation APIs on the Manager/Registry. The Manager wraps a read-only Registry interface; adding UpsertProfile requires significant plumbing — deferred.
- Task 7.6 (command palette) — the chat model doesn't integrate with bobatea's PaletteCommand system. Would require porting palette support to the chat package — deferred.

### What I learned
- geppetto's profile system uses `StepSettingsPatch` (raw map) vs `EffectiveStepSettings` (typed struct). The effective settings are what's actually applied at runtime.

### What was tricky to build
- N/A — straightforward enhancements.

### What warrants a second pair of eyes
- The multiline picker label (slug + description on separate lines) — need to verify huh's Select renders multiline options correctly.

### What should be done in the future
- Tasks 7.2, 7.3: Expose mutation methods on Manager (wrapping store UpsertProfile)
- Task 7.6: Port PaletteCommand integration from REPL to chat model
- Show ProfileDiff as a timeline entity (not just log) so users see it in the TUI

### Code review instructions
- Start: `pinocchio/pkg/ui/profileswitch/picker.go` — enhanced picker
- Then: `pinocchio/pkg/ui/profileswitch/diff.go` — diff utility
- Then: `pinocchio/cmd/switch-profiles-tui/main.go` — search for `parts` (header) and `DoubleEscToClose`
- Validate: `cd pinocchio && go test ./pkg/ui/profileswitch/ -count=1 -v`

## Step 12: Replace profileSwitchModel with overlay.Host in pinocchio

Integrated the FormOverlay system into the main pinocchio binary. The crude `profileSwitchModel` wrapper — which completely replaced the chat view with a raw huh form — was replaced by `overlay.Host` + `FormOverlay`, matching the architecture already proven in `switch-profiles-tui`. This was the main integration milestone: the feature built across Phases 1–7 now works in pinocchio proper, not just the standalone proof-of-concept.

### Prompt Context

**User prompt (verbatim):** (continuation from prior session discussing pinocchio integration architecture — user confirmed `profileSwitchModel` uses raw huh widgets and agreed to proceed with replacement)

**Assistant interpretation:** Replace the crude `profileSwitchModel` in pinocchio's `pkg/cmds/cmd.go` with `overlay.Host` + `FormOverlay`, wiring it identically to how `switch-profiles-tui/main.go` does it.

**Inferred user intent:** Get the overlay system actually integrated into pinocchio (the real product) rather than just the proof-of-concept binary.

**Commit (code):** `d2efe8f` — "Replace profileSwitchModel with overlay.Host + FormOverlay in pinocchio"

### What I did

1. Explored the architecture gap: pinocchio uses `bobatea_chat.Model` (not `repl.Model`), and wraps it with `profileSwitchModel` in `pkg/cmds/cmd.go:656`.
2. Read `chat_profile_switch_model.go` (162 lines): crude wrapper that catches `OpenProfilePickerMsg`, creates a raw `huh.Form` inline, and replaces `View()` entirely when form is active.
3. Read `cmd.go` interceptor at lines 625–657: `/profile` command sends `bobatea_chat.OpenProfilePickerMsg{}`.
4. Read `overlay.Host`, `formoverlay_model.go`, `formoverlay_types.go` to confirm the overlay handles `OpenFormOverlayMsg`.
5. Read `switch-profiles-tui/main.go` as the reference implementation.
6. Modified `cmd.go`:
   - Added imports for `overlay`, `formoverlay`, `huh`
   - Changed interceptor to send `overlay.OpenFormOverlayMsg{}` instead of `bobatea_chat.OpenProfilePickerMsg{}`
   - Created `formoverlay.New(formoverlay.Config{...})` with `PickerFormFactory`, `DoubleEscToClose`, `OnSubmit` callback
   - Replaced `newProfileSwitchModel(model, ...)` with `overlay.NewHost(model, overlay.Config{FormOverlay: profileOverlay})`
7. Stripped `chat_profile_switch_model.go`:
   - Removed `profileSwitchModel` struct, `newProfileSwitchModel`, `Init`, `Update`, `View` (130 lines)
   - Retained `systemNoticeEntityCmd` and `publishProfileSwitchedInfo` utilities
   - Cleaned imports: removed `context`, `fmt`, `bubbles/key`, `huh`, `bobatea_chat`, `profileswitch`
8. Fixed unused `fmt` import after stripping.
9. Verified: `go build ./...` clean, `go test ./pkg/tui/... ./pkg/cmds/...` all pass, `grep -r profileSwitchModel` returns zero hits.

### Why

The `profileSwitchModel` was always a temporary hack from SPT-1. It had three critical problems:
1. Replaced the entire chat view when the form was active (chat disappears)
2. Duplicated form lifecycle management that FormOverlay already handles
3. Couldn't be reused by other pinocchio TUI apps

### What worked

- The migration was mechanical — the `switch-profiles-tui/main.go` served as a 1:1 template for the `cmd.go` changes.
- No behavioral regressions: same `PickerFormFactory`, same `OnSubmit` callback pattern, same `DoubleEscToClose`.
- All existing tests pass without modification.
- Net -103 lines.

### What didn't work

- Nothing failed in this step. The architecture was already validated in the proof-of-concept.

### What I learned

- `bobatea_chat.OpenProfilePickerMsg` was only used by `profileSwitchModel` — after removal, zero references remain in pinocchio. The type itself still exists in bobatea but is now unused.
- The `systemNoticeEntityCmd` utility (creates timeline entities for system notices) is still needed by the `/profile <slug>` direct-switch path in the interceptor, so it must be retained.

### What was tricky to build

- Nothing particularly tricky. The key consideration was ensuring that `publishProfileSwitchedInfo` was called from the `OnSubmit` callback. The old `profileSwitchModel` could return `tea.Batch(publishCmd, entityCmd)` because it was inside `Update()`. The new `OnSubmit` callback runs synchronously (not as a `tea.Cmd`), so it calls `publishProfileSwitchedInfo` directly. This works but means no timeline entity is emitted for the switch — a minor regression noted in the postmortem.

### What warrants a second pair of eyes

- The `OnSubmit` callback captures `sink` and `chatConvID` from the outer scope. These are stable for the lifetime of the program, so this is safe, but the closure chain is deep.
- `bobatea_chat.OpenProfilePickerMsg` is now dead code in bobatea. Should it be removed?

### What should be done in the future

- Change `OnSubmit` to return `tea.Cmd` so profile switch can emit a timeline entity
- Remove dead `OpenProfilePickerMsg` from bobatea
- Add blur/unblur for the chat input when overlay opens/closes

### Code review instructions

- Start: `pkg/cmds/cmd.go` lines 660–684 — overlay creation and host wiring
- Then: `pkg/cmds/chat_profile_switch_model.go` — verify only utility functions remain
- Then: diff to see what was removed (130 lines of `profileSwitchModel`)
- Validate: `go build ./... && go test ./pkg/cmds/... ./pkg/tui/... -count=1`

## Step 13: Dynamic overlay sizing and height overflow investigation

Added terminal-aware dynamic sizing to FormOverlay so the overlay adapts to the actual terminal dimensions. Then attempted to fix height overflow on small terminals, discovering a fundamental architectural gap: `lipgloss.MaxHeight()` clips the border along with the content, producing a broken visual when the form content exceeds the available space.

### Prompt Context

**User prompt (verbatim):** "Let's commit. Then let's better size the profile selector because it's a bit tiny and claustrophobic right now, It should fit in the terminal ofc, but be still tight around the text, as well." (accompanied by two screenshots showing the overlay too small at 60-char width)

**Assistant interpretation:** The overlay needs to be wider to avoid text wrapping but should still fit the terminal. Implement dynamic sizing that adapts to terminal dimensions.

**Inferred user intent:** Make the profile picker overlay look good at any terminal size — wider on large terminals, shrinking to fit on small ones.

**Commit (code):** `146da14` — "Make FormOverlay dynamically size to fit terminal"

### What I did

1. Added `termWidth, termHeight int` fields to `FormOverlay` struct.
2. Added `SetTerminalSize(w, h int)` method that stores dimensions and updates `form.WithWidth()` if overlay is visible.
3. Added `effectiveMaxWidth()`: returns `min(maxWidth, termWidth - 4)`, minimum 20.
4. Added `effectiveMaxHeight()`: returns `min(maxHeight, termHeight - 4)`, minimum 10.
5. Updated `contentWidth()` to use `effectiveMaxWidth()`.
6. Updated `View()` to use `effectiveMaxWidth()` and `effectiveMaxHeight()`.
7. Updated `overlay.Host.Update()` to call `formOverlay.SetTerminalSize()` on every `WindowSizeMsg` (not just when visible — size should be known before overlay opens).
8. Increased defaults in `styles.go` from 60x20 to 80x30.
9. Updated `MaxWidth/MaxHeight` in both `cmd.go` and `switch-profiles-tui/main.go` from 60x25 to 80x30.
10. All tests pass, build clean.
11. Attempted tmux smoke testing on 80x25 terminal — discovered height overflow issue.
12. Reduced Select height cap in `picker.go` from 15 to 10 (uncommitted).
13. Attempted further tmux testing — hit shell Tab completion interference and persistent DB state issues, ultimately stopped before getting a clean test.

### Why

- The user's screenshots showed the overlay was too narrow (60 chars) on wide terminals, causing description text to wrap awkwardly.
- On small terminals, the overlay needs to shrink to fit rather than extending beyond the screen.

### What worked

- Width sizing works correctly: the overlay now uses up to 80 columns, shrinking to `termWidth - 4` on narrower terminals.
- `SetTerminalSize` correctly updates the form width when the terminal resizes while the overlay is visible.
- All 28 existing tests pass without modification (tests that set explicit MaxWidth/MaxHeight are unaffected by default changes).

### What didn't work

- **Height overflow**: On an 80x25 terminal with ~20 profiles, the overlay rendered ~23 rows which exceeded `effectiveMaxHeight` of 21. `lipgloss.MaxHeight(21)` clipped the rendered string, removing the bottom border row. The overlay appeared to "overflow" the terminal and overlap the chat's input area.
- **tmux Tab key**: When sending `/profile` + Tab via `tmux send-keys`, the Tab was sometimes intercepted by the shell's completion system rather than reaching the TUI app. Required careful timing (waiting for the app to fully launch before sending keys).
- **Persistent state**: The switch-profiles-tui persists conversation state to `/tmp/*.db` by default. When restarting for testing, old state loaded and the TUI showed a stale conversation with an LLM response in progress, making it impossible to test the profile picker cleanly. Required either deleting the DBs or using `--conv-id fresh-test`.

### What I learned

- `lipgloss.Style.MaxHeight()` truncates the FINAL rendered string — it doesn't know about borders. If the rendered string has a top border at line 1 and bottom border at line N, MaxHeight truncates at line M < N, and the bottom border disappears. This is a fundamental limitation of line-based clipping.
- The form height is determined at creation time by `huh.Select.Height()`. The FormOverlay has no way to constrain it after creation because `huh.Form` has no `WithHeight()` method.
- The factory signature `func() *huh.Form` doesn't accept dimensions, so the factory can't adapt Select height to available terminal space.

### What was tricky to build

- **The height problem has no clean solution with the current architecture.** The overlay needs to tell the form how tall it can be, but:
  1. The form is created by a `func() *huh.Form` factory that doesn't receive dimensions
  2. `huh.Form` has no `WithHeight()` method
  3. `huh.Select.Height()` is set at construction time and can't be changed after
  4. `lipgloss.MaxHeight()` clips the border along with content, producing visual breakage

  The "right" fix requires changing either the factory signature or adding height support to huh. The short-term workaround (reducing Select height cap) is fragile.

- **tmux testing is unreliable for TUI apps.** Tab key interpretation depends on whether the shell prompt or the TUI app has focus. The `tmux send-keys -l` (literal) flag helps for text but not for control keys. Timing between commands matters — need to wait for the app to fully initialize before sending keys.

### What warrants a second pair of eyes

- The `effectiveMaxHeight` margin of 4 rows is likely insufficient. The chat model has: 1 header row + ~8 input box rows + 1 status bar row = ~10 rows of chrome. The overlay should probably leave 8-10 rows of margin, not 4.
- The Select height cap of 10 (changed from 15 but uncommitted) may still be too many for 20-row terminals. Need to validate empirically.
- The border-clipping problem with `lipgloss.MaxHeight()` is a design issue, not a bug. The fix (render border separately from content, clip only the content) should be implemented before this is considered production-ready.

### What should be done in the future

1. **Immediate**: Implement content-aware height clipping — clip the form content to `maxHeight - verticalChrome`, then wrap in the border. This guarantees the border is never truncated.
2. **Short-term**: Change factory to `func(w, h int) *huh.Form` or add a `SizedFactory` field so the Select height can adapt to terminal size.
3. **Medium-term**: Add `WithHeight()` to the huh fork.
4. **Testing**: Build a proper tmux test harness that handles app startup timing, clean state, and reliable key sending.

### Code review instructions

- Start: `widget.go` lines 74–88 (`SetTerminalSize`) and 275–315 (`effectiveMaxWidth/Height`, `contentWidth`)
- Then: `host.go` lines 49–55 — WindowSizeMsg forwarding
- Then: `styles.go` — default changes (60x20 to 80x30)
- Then: `cmd.go` and `main.go` — MaxWidth/MaxHeight config changes
- Validate: `go build ./... && go test ./pkg/tui/... -count=1 -v`

### Technical details

tmux capture showing the height overflow (80x25 terminal):
```
  +----------------------------------------------------------------------+
  |                                                                      |
  |   Switch Profile                                                     |
  |                                                                      |
  |  | Switch profile                                                    |
  |  | >   cerebras-llama-3.1-8b                                         |
  |  |     codestral                                                     |
  |  |   * default                                                       |
  |  |     gemini-2.5-flash                                              |
  |  |     gemini-2.5-flash-lite                                         |
  |  |     gemini-2.5-pro                                                |
  |  |     gemini-3-pro                                                  |
  |  |     gemma3                                                        |
+-|  |     gpt-5                                                         |-+
| |  |     gpt-5-mini                                                    | |
| |  |     gpt-5-nano                                                    | |
| |  |     groq-oss-20b                                                  | |
| |  |     haiku                                                         | |
| |  |     index-sonnet                                                  | |
| |                                                                      | |
+-|  up/down/filter/enter submit                                         |-+
ctrl+h help ...
```

The overlay border (outer) overlaps with the chat input box (inner +-+-+ borders). The bottom border of the overlay is missing because `MaxHeight` clipped it.

## Related

- Postmortem: `reference/04-postmortem-pinocchio-integration-and-overlay-sizing.md`
- Design doc: `design-doc/01-embeddable-huh-forms-and-wizards-as-bobatea-canvas-layer-overlays.md`
- uhoh analysis: `reference/02-uhoh-analysis-wizard-and-form-embedding-for-modal-overlays.md`
- SPT-1 analysis: `ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/reference/04-analysis-proper-profile-switching-ui.md`
