# Tasks

## Phase 1: FormOverlay Widget Core (pinocchio)

Create the FormOverlay widget that wraps huh.Form with modal overlay behaviors.
All code in `pinocchio/pkg/tui/widgets/formoverlay/`.

**Design doc references**: Section 4.2 (FormOverlay Widget), 4.3 (Visibility Lifecycle), 4.4 (Update with Key Interception), 4.5 (View with Modal Framing), 4.6 (Layout Computation)

- [x] 1.1 Create `widget.go` — FormOverlay struct with Show/Hide/Toggle visibility lifecycle. Factory pattern (`func() *huh.Form`) creates fresh form on each Show() to avoid huh's state lock (Gap 3, Section 6.2). Show() calls factory + form.Init(). Hide() nils out form.
- [x] 1.2 Create `config.go` — Config struct with Title, MaxWidth, MaxHeight, Placement enum (Center/Top/TopRight), Factory, OnSubmit/OnCancel callbacks, BorderStyle, TitleStyle. See Section 4.2 for full struct.
- [x] 1.3 Implement `Update()` with key interception — Esc and ctrl+c intercepted BEFORE form sees them (solves Gap 2 and Gap 4, Section 4.4). Delegate all other messages to inner form. Check form.State after each Update: StateCompleted triggers onSubmit + Hide, StateAborted triggers onCancel + Hide.
- [x] 1.4 Implement `View()` with modal framing and dimension constraints — Title bar + form content + border. Use `lipgloss.Style.MaxWidth()/MaxHeight()` on border to clip output (solves Gap 1 and Gap 5, Section 4.5). Set `form.WithWidth(contentWidth)` accounting for border + theme padding (Section 6.5).
- [x] 1.5 Implement `ComputeLayout()` — Returns (x, y, view, ok) for canvas layer positioning. Compute position based on Placement enum and terminal dimensions. Clamp to bounds. See Section 4.6.
- [x] 1.6 Create `styles.go` — Default border and title styles. Sensible defaults for modal appearance: rounded border, subtle title styling, optional shadow.
- [x] 1.7 Unit tests — Show/Hide lifecycle, key interception (Esc/ctrl+c don't reach inner form), key passthrough (regular keys reach form), state completion triggers onSubmit, factory freshness (each Show creates new form), ComputeLayout placement calculations. See Section 11 (Testing Strategy).

## Phase 2: Overlay Host (pinocchio)

Create the overlay host that composes canvas layers, following bobatea's REPL overlay pattern.
All code in `pinocchio/pkg/tui/overlay/`.

**Design doc references**: Section 3.3 (bobatea Overlay System — Reference Architecture), 4.7 (Overlay Host Integration)

- [x] 2.1 Create `host.go` — OverlayHost struct that wraps an inner `tea.Model` (the chat model) and adds canvas layer composition. Init/Update/View delegate to inner model, then compose overlay layers on top in View() using `lipglossv2.NewCanvas` + `lipglossv2.NewCompositor`. Follow the pattern from bobatea's `model.go:278-362`.
- [x] 2.2 Implement key routing priority chain in host — When form overlay is visible, route ALL key messages to it first. Only pass to inner model if overlay doesn't handle them. Follow bobatea's `model_input.go` pattern. See Section 4.7 key routing example.
- [x] 2.3 Create `formoverlay_model.go` — Wire FormOverlay into the host: `ensureFormOverlay()` for lazy init, `HandleFormOverlayInput()` exported method for key routing, open/close triggers (keybinding or programmatic message).
- [x] 2.4 Create `formoverlay_overlay.go` — `ComputeFormOverlayLayout()` that calls FormOverlay.ComputeLayout() and returns the lipgloss v2 layer at Z=28.
- [x] 2.5 Create `formoverlay_types.go` — FormOverlayConfig for overlay host configuration, FormOverlayProvider interface, message types (OpenFormOverlayMsg, FormOverlayCompletedMsg, FormOverlayCancelledMsg).
- [x] 2.6 Expose overlay registration API — OverlayHost should accept FormOverlay instances via config or runtime registration, so pinocchio TUI apps can add overlays without modifying the host.
- [x] 2.7 Integration test — Overlay host wraps a mock inner model. Open form overlay, verify key routing goes to overlay, verify View() contains both base layer and overlay layer, verify overlay close returns key routing to inner model.

## Phase 3: Profile Picker Overlay (pinocchio)

Replace the appModel hack in switch-profiles-tui with a FormOverlay-based profile picker.

**Design doc references**: Section 8 Phase 3 (Profile Picker Using FormOverlay). Also see SPT-1 analysis Part 4 (ASCII Mockups) for the UI vision.

- [x] 3.1 Create profile picker form factory — Function that takes `profileswitch.Manager` and returns `func() *huh.Form` that builds a huh.Select populated with profile list items (slug + display name). Use `huh.NewSelect[string]().Title("Switch Profile").Options(...)`.
- [x] 3.2 Wire profile picker into overlay host in main.go — Create OverlayHost wrapping the chat model. Register FormOverlay with the profile picker factory. Connect OnSubmit callback to `Backend.SwitchProfile()`.
- [x] 3.3 Remove appModel struct from main.go — Delete the `appModel` wrapper entirely. The overlay host now handles visibility, key routing, and View() composition. Remove the huh form creation and `m.active` logic.
- [x] 3.4 Wire /profile command to open overlay — The submit interceptor for `/profile` should send an `OpenFormOverlayMsg` instead of creating a huh.Form inline. `/profile <slug>` can still switch directly without the overlay.
- [x] 3.5 Smoke test — Run switch-profiles-tui with real profile registries. Open picker with /profile, navigate with arrow keys, select profile, verify switch happens and chat remains visible behind modal. Test Esc to cancel.

## Phase 4: Multi-Step Wizard Support

Support huh Forms with multiple Groups as wizard-style modals.

**Design doc references**: Section 8 Phase 4, Section 5.3 (Multi-Step Wizard as Modal ASCII mockup)

- [x] 4.1 Track current group index in FormOverlay — When the inner form has multiple groups, update the overlay title to show progress ("Step 2 of 3"). Listen for huh's group navigation messages.
- [x] 4.2 Handle variable height across groups — Different groups may have different heights. FormOverlay should recompute layout when the active group changes. Call ComputeLayout() after group transitions.
- [x] 4.3 Test multi-group form in overlay — Create a test wizard YAML (using uhoh's form DSL) with 3 groups. Embed via BuildBubbleTeaModelFromYAML(). Verify step navigation, title updates, and completion.

## Phase 5: uhoh Integration

Make uhoh forms and wizards work inside the overlay system.

**Design doc references**: Reference doc 02 (uhoh analysis), Sections 3 (Form DSL), 7 (Proposed WizardModel), 9 (Implementation Phases)

- [x] 5.1 Deduplicate uhoh Form.Run() — Refactor `pkg/formdsl.go` Form.Run() to delegate to BuildBubbleTeaModel() + RunWithContext(). Eliminates ~300 lines of duplicated switch-case code. See uhoh analysis Section 3.3 and Phase 1.
- [x] 5.2 Test uhoh form embedding in FormOverlay — Use BuildBubbleTeaModelFromYAML() to create a huh.Form from YAML, wrap in FormOverlay, verify it works end-to-end. The existing `examples/bubbletea-embed/` pattern should work with zero uhoh changes.
- [x] 5.3 Add EmbeddableStep interface to wizard steps — New interface with `BuildModel(state) (tea.Model, map[string]interface{}, error)`. Implement for FormStep (delegates to FormData.BuildBubbleTeaModel()), DecisionStep, InfoStep, SummaryStep. See uhoh analysis Section 7.3.
- [x] 5.4 Create WizardModel (tea.Model) — Non-blocking wizard that drives steps through the Bubble Tea update loop. Maintains step index, builds step models, handles StepCompletedMsg, runs callbacks during transitions, emits WizardCompletedMsg. See uhoh analysis Section 7.1-7.4.
- [x] 5.5 Redesign ActionStep for async embedding — ActionStep's embedded model shows spinner via View() (not a separate tea.Program), runs callback as tea.Cmd, transitions to completion view on result. See uhoh analysis Section 5.5 and Gap 6.
- [x] 5.6 Create bubbletea-embed-wizard example — New `uhoh/examples/bubbletea-embed-wizard/` demonstrating a multi-step wizard embedded in a parent Bubble Tea model, similar to the existing bubbletea-embed example for forms.

## Phase 6: Convenience API and Documentation

Make it easy for developers to create form overlays.

**Design doc references**: Section 8 Phase 5

- [x] 6.1 Quick helpers — `formoverlay.NewSelect(title, options, onSelect)`, `formoverlay.NewConfirm(title, onConfirm)`, `formoverlay.NewWizard(title, groups, onComplete)`. Thin wrappers that create Config + Factory.
- [x] 6.2 Example application — `pinocchio/examples/form-overlay/` showing a minimal TUI with overlay host + form overlay.
- [x] 6.3 Esc key refinement for Select filter mode — Phase 2 deferred issue: huh's Select field uses Esc to exit filter mode. Add state-aware Esc handling that checks if a field is in a sub-mode before intercepting. See Section 6.3 and Risk 1 in Section 10.

## Phase 7: Profile Management Enhancements (from SPT-1 analysis)

These extend the form overlay into a full profile management UI.
See SPT-1 analysis Parts 4-7 for ASCII mockups and detailed specs.

- [x] 7.1 Profile picker with detail preview (split layout) — When terminal >100 cols, show left pane (profile list) + right pane (profile detail). See SPT-1 Part 4.2 mockup.
- [ ] 7.2 Profile editor sub-view — Form for editing profile fields (display name, system prompt, model, temperature). Uses huh fields inside the overlay. See SPT-1 Part 4.4 mockup.
- [ ] 7.3 Profile creator sub-view — Form for creating new profiles with slug validation, stack inheritance, runtime overrides. See SPT-1 Part 4.6 mockup.
- [x] 7.4 Profile switch diff confirmation — Brief modal showing what changes between current and target profile before confirming switch. See SPT-1 Part 4.9 mockup.
- [x] 7.5 Enhanced header bar — Richer profile indicator showing display name + model + temperature + switch hint. See SPT-1 Part 4.7.
- [ ] 7.6 Command palette integration — Add "Switch Profile" and "New Profile" as command palette entries that open the profile overlay. See SPT-1 Part 7.

## Done

<!-- Move items here as they are completed -->
- [x] Rip out huh from formoverlay: replace FormOverlay with a generic Overlay widget that accepts any tea.Model content
- [x] Fix host routing: forward non-key messages to BOTH overlay and inner model when overlay is visible
- [x] Build custom ProfilePicker tea.Model with keyboard nav, filtering, height-aware rendering
- [x] Wire ProfilePicker into overlay host, replacing huh-based picker in cmd.go and main.go
- [x] Remove huh helpers (NewSelect/NewConfirm/NewInput) and huh-dependent tests
- [x] Rename chat_profile_switch_model.go to profile_switch_events.go, clean stale comments
- [x] Fix height clipping: clip content before border render, not after
