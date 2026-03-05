---
Title: 'uhoh analysis: wizard and form embedding for modal overlays'
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
    - Path: uhoh/examples/bubbletea-embed/main.go
      Note: |-
        Existing proof that uhoh form embedding works at huh level
        Existing proof that uhoh form embedding works
    - Path: uhoh/pkg/formdsl.go
      Note: |-
        Form DSL — YAML to huh.Form translation, BuildBubbleTeaModel(), Run()
        Form DSL — YAML to huh.Form
    - Path: uhoh/pkg/wizard/steps/action_step.go
      Note: ActionStep — uses huh.Note, partial goroutine support
    - Path: uhoh/pkg/wizard/steps/decision_step.go
      Note: DecisionStep — creates huh.Select, calls form.Run() (blocking)
    - Path: uhoh/pkg/wizard/steps/form_step.go
      Note: FormStep — delegates to pkg.Form.Run() (blocking)
    - Path: uhoh/pkg/wizard/steps/info_step.go
      Note: InfoStep — displays huh.Note, blocking Run()
    - Path: uhoh/pkg/wizard/steps/step.go
      Note: BaseStep — abstract base with lifecycle callbacks
    - Path: uhoh/pkg/wizard/steps/summary_step.go
      Note: SummaryStep — displays collected data via huh.Note
    - Path: uhoh/pkg/wizard/wizard.go
      Note: |-
        Wizard orchestrator — blocking Run() loop, state management, callbacks
        Wizard orchestrator — blocking Run() loop
ExternalSources: []
Summary: Deep analysis of uhoh's YAML form DSL and wizard orchestrator, identifying what needs to change to support embeddable forms and wizards inside bobatea modal overlays.
LastUpdated: 2026-03-04T00:00:00Z
WhatFor: Guide implementation of embeddable uhoh forms and wizards for modal overlays
WhenToUse: When implementing WizardModel or extending uhoh for non-blocking Bubble Tea embedding
---



# uhoh Analysis: Wizard and Form Embedding for Modal Overlays

## 1. Executive Summary

uhoh is a YAML-driven form DSL and multi-step wizard orchestrator built on top of `charmbracelet/huh`. It provides two layers of abstraction above huh: a **form DSL** (`pkg/formdsl.go`) that translates YAML field definitions into `huh.Form` instances, and a **wizard engine** (`pkg/wizard/wizard.go`) that sequences multiple steps — each of which may contain a form, a decision prompt, an action, or an informational display.

The form DSL layer **already supports Bubble Tea embedding** via `BuildBubbleTeaModel()`, which returns a `*huh.Form` (a `tea.Model`) plus a values map. This is proven by the working `examples/bubbletea-embed/` example. The same FormOverlay adapter proposed in the SPT-3 design doc can wrap these forms without any uhoh changes.

The wizard layer is a different story. `Wizard.Run()` is a **blocking sequential loop** that calls `step.Execute(ctx, state)` for each step, where every step internally calls `huh.Run()` or `huh.NewNote().Run()` — each of which creates its own `tea.NewProgram()`. This is fundamentally incompatible with embedding inside another Bubble Tea program. Making wizards embeddable requires a new `WizardModel` that implements `tea.Model` and drives steps through the Bubble Tea update loop.

This document maps uhoh's architecture in detail, identifies every gap, and proposes a concrete path to embeddable wizards.


## 2. Architecture Map

### 2.1 Layer Diagram

```
┌─────────────────────────────────────────────────┐
│  Application Code                               │
│  (CLI main.go or parent Bubble Tea model)       │
└──────────────┬──────────────────┬───────────────┘
               │                  │
       ┌───────▼───────┐  ┌──────▼────────┐
       │  Form DSL     │  │  Wizard Engine │
       │  formdsl.go   │  │  wizard.go     │
       └───────┬───────┘  └──────┬────────┘
               │                  │
               │          ┌──────▼────────┐
               │          │  Step Types   │
               │          │  steps/*.go   │
               │          └──────┬────────┘
               │                  │
       ┌───────▼──────────────────▼───────┐
       │          charmbracelet/huh       │
       │  Form, Select, Input, Note, etc. │
       └──────────────────────────────────┘
```

### 2.2 Two Execution Paths

uhoh provides two distinct ways to use forms:

| Path | Entry Point | Creates tea.Program? | Embeddable? |
|------|------------|---------------------|-------------|
| **Standalone** | `Form.Run(ctx)` | Yes — calls `huhForm.RunWithContext(ctx)` | No |
| **Embedded** | `Form.BuildBubbleTeaModel()` | No — returns `*huh.Form` for caller to drive | Yes |

The wizard engine currently only has the standalone path:

| Path | Entry Point | Creates tea.Program? | Embeddable? |
|------|------------|---------------------|-------------|
| **Standalone** | `Wizard.Run(ctx, initialState)` | Yes — each step calls `.Run()` internally | No |
| **Embedded** | Does not exist yet | — | — |


## 3. Form DSL Analysis (`pkg/formdsl.go`)

### 3.1 YAML Schema

The form DSL defines a simple hierarchy:

```yaml
name: "Example Form"
theme: "Charm"          # Optional: Charm, Dracula, Catppuccin, Base16, Default
groups:
  - name: "Group 1"
    fields:
      - type: input      # input, text, select, multiselect, confirm, note, filepicker
        key: "username"
        title: "Username"
        value: "default"  # Optional default
        # Type-specific attributes via nested structs
```

Supported field types: `input`, `text`, `select`, `multiselect`, `confirm`, `note`, `filepicker`.

Each field type has an optional attributes struct (e.g., `InputAttributes`, `SelectAttributes`) for type-specific configuration like `prompt`, `char_limit`, `placeholder`, `inline`, `height`, `filterable`.

### 3.2 BuildBubbleTeaModel() — The Embedding API

`BuildBubbleTeaModel()` (lines 117–358) is the key function for embedding. It:

1. Creates a `values` map with typed pointers (`*string`, `*[]string`, `*bool`) keyed by field key.
2. Iterates groups → fields, creating the corresponding `huh.Field` instances bound to the pointer values.
3. Applies theme if specified.
4. Returns `(*huh.Form, map[string]interface{}, error)`.

The convenience wrapper `BuildBubbleTeaModelFromYAML(src []byte)` unmarshals YAML then calls `BuildBubbleTeaModel()`.

After the form completes, callers use `ExtractFinalValues(values)` (lines 360–385) to dereference the pointers into a plain `map[string]interface{}`.

### 3.3 Code Duplication: Run() vs BuildBubbleTeaModel()

`Form.Run()` (lines 399–716) duplicates the entire field-building logic from `BuildBubbleTeaModel()`. The two methods share approximately 300 lines of identical switch-case code for creating huh fields from the DSL. The only differences are:

- `Run()` calls `huhForm.RunWithContext(ctx)` at the end (line 678).
- `Run()` extracts final values inline (lines 692–713) instead of returning the values map.
- `Run()` uses `log.Printf` where `BuildBubbleTeaModel()` uses `fmt.Printf` for warnings.

This duplication is a maintenance risk. A refactor should make `Run()` delegate to `BuildBubbleTeaModel()`:

```go
func (f *Form) Run(ctx context.Context) (map[string]interface{}, error) {
    huhForm, values, err := f.BuildBubbleTeaModel()
    if err != nil {
        return nil, err
    }
    if err := huhForm.RunWithContext(ctx); err != nil {
        return nil, errors.Wrap(err, "error running huh form")
    }
    return ExtractFinalValues(values)
}
```

### 3.4 Validation Gap

`addValidation()` (line 108–110) returns `errors.New("not implemented")`. All validation specifications in YAML are silently ignored with a warning log. This affects both standalone and embedded modes equally.


## 4. Wizard Engine Analysis (`pkg/wizard/wizard.go`)

### 4.1 Wizard Struct

```go
type Wizard struct {
    Name        string
    Description string
    Steps       steps.WizardSteps      // Custom YAML unmarshalling
    Theme       string
    GlobalState map[string]interface{} // From YAML

    exprFunctions   map[string]ExprFunc       // Expr-lang condition functions
    callbacks       map[string]WizardCallbackFunc  // Lifecycle callbacks
    actionCallbacks map[string]ActionCallbackFunc  // Action-step-specific callbacks
    initialState    map[string]interface{}          // Runtime initial state
}
```

Configuration uses the functional options pattern: `WithExprFunction()`, `WithCallback()`, `WithActionCallback()`, `WithInitialState()`.

### 4.2 Run() — The Blocking Loop

`Wizard.Run()` (lines 166–394) is a sequential `for` loop:

```
for currentStepIndex < len(w.Steps):
    1. Evaluate skip condition (expr-lang)
    2. Execute "before" callback
    3. Call step.Execute(ctx, wizardState)     ← BLOCKING
    4. Execute "after" callback
    5. Merge step results into wizardState
    6. Execute "validation" callback
    7. Execute "navigation" callback → may override next step
    8. Advance to next step (linear or jump)
```

Every `step.Execute()` call blocks until the user completes that step's UI. This is the fundamental incompatibility with Bubble Tea embedding — you cannot block inside an `Update()` method.

### 4.3 State Management

The wizard maintains a flat `map[string]interface{}` state that flows through the entire execution:

1. Initialized from `GlobalState` (YAML).
2. Merged with `initialState` (runtime option).
3. Merged with `initialState` parameter (from `Run()` call).
4. Each step's results are merged after execution.
5. State is passed to expr-lang conditions and callbacks.

### 4.4 Navigation Model

Navigation supports three modes:
- **Linear**: Default, `currentStepIndex++`.
- **Skip**: Steps with `skip_condition` are evaluated against state via expr-lang.
- **Jump**: Navigation callbacks return a `*string` step ID to jump to.


## 5. Step Types Analysis

### 5.1 Step Interface

All steps implement a common interface defined in `steps/step.go`:

```go
type Step interface {
    ID() string
    Type() string
    Execute(ctx context.Context, state map[string]interface{}) (map[string]interface{}, error)
    SkipCondition() string
    BeforeCallback() string
    AfterCallback() string
    ValidationCallback() string
    NavigationCallback() string
}
```

### 5.2 Step Type Inventory

| Step Type | File | huh Usage | Blocking? | Embeddable? |
|-----------|------|-----------|-----------|-------------|
| **FormStep** | `form_step.go` | Delegates to `pkg.Form.Run()` | Yes | No — calls Run() |
| **DecisionStep** | `decision_step.go` | Creates `huh.Select` + `huh.Form`, calls `.Run()` | Yes | No — calls Run() |
| **ActionStep** | `action_step.go` | Uses `huh.NewNote()`, partial goroutine | Partial | Partially — already uses goroutine for progress |
| **InfoStep** | `info_step.go` | Uses `huh.NewNote().Run()` | Yes | No — calls Run() |
| **SummaryStep** | `summary_step.go` | Uses `huh.NewNote().Run()` | Yes | No — calls Run() |

Every step that shows UI calls `.Run()`, which creates its own `tea.NewProgram()`. This is the root cause of the embedding incompatibility.

### 5.3 FormStep Detail

FormStep deserializes a nested `pkg.Form` from YAML and calls `FormData.Run(ctx)`:

```go
func (fs *FormStep) Execute(ctx context.Context, state map[string]interface{}) (map[string]interface{}, error) {
    results, err := fs.FormData.Run(ctx)
    // ... merge results ...
}
```

To make this embeddable, FormStep needs a parallel method that calls `FormData.BuildBubbleTeaModel()` instead of `FormData.Run()`.

### 5.4 DecisionStep Detail

DecisionStep builds a `huh.Select` from its options and runs it:

```go
selectField := huh.NewSelect[string]().Title(ds.Prompt).Options(huhOpts...).Value(&selected)
form := huh.NewForm(huh.NewGroup(selectField))
err := form.Run()
```

To make this embeddable, it needs a `BuildBubbleTeaModel()` that returns the `*huh.Form` without calling `.Run()`.

### 5.5 ActionStep Detail

ActionStep is the most complex step. It:
1. Spawns a goroutine to show a progress `huh.Note` while the callback runs.
2. Executes the registered `ActionCallbackFunc`.
3. Shows a completion `huh.Note` (blocking).

The goroutine pattern is interesting because it already shows awareness that blocking UI and async work need to coexist. However, spawning a goroutine that creates its own `tea.NewProgram()` inside another `tea.NewProgram()` would cause terminal corruption.

### 5.6 InfoStep and SummaryStep

Both are display-only steps that show `huh.Note` content and wait for the user to press Enter. Embedding requires converting these to return a `tea.Model` that the parent drives.


## 6. Gap Analysis: What Prevents Wizard Embedding

### Gap 1: Wizard.Run() is a blocking loop

**Impact**: Cannot embed a wizard inside a Bubble Tea `Update()` — would block the entire event loop.

**Evidence**: `wizard.go:219-389` — `for currentStepIndex < len(w.Steps)` with `step.Execute()` calls.

**Solution**: Create `WizardModel` implementing `tea.Model` that maintains step index as state and advances on step completion messages.

### Gap 2: Every step calls huh.Run() internally

**Impact**: Each step creates its own `tea.NewProgram()`, which takes over the terminal. Nested programs corrupt terminal state.

**Evidence**: `form_step.go` calls `FormData.Run(ctx)`, `decision_step.go` calls `form.Run()`, `info_step.go` calls `note.Run()`, `summary_step.go` calls `note.Run()`.

**Solution**: Each step needs a `BuildBubbleTeaModel()` method that returns a `tea.Model` without running it.

### Gap 3: No step-level tea.Model interface

**Impact**: Steps only expose `Execute()` (blocking). No way to get a `tea.Model` from a step for composable embedding.

**Evidence**: `step.go` interface has only `Execute(ctx, state)`.

**Solution**: Extend the `Step` interface (or add a parallel `EmbeddableStep` interface) with `BuildModel(state) (tea.Model, error)`.

### Gap 4: State merging assumes synchronous flow

**Impact**: The wizard merges step results into state immediately after `Execute()` returns. In an async model, step completion is a message, and state merging must happen in `Update()`.

**Evidence**: `wizard.go:306-317` — `for k, v := range stepResult { wizardState[k] = v }`.

**Solution**: `WizardModel.Update()` handles a `StepCompletedMsg` that carries the step's results, merges into state, then advances to the next step.

### Gap 5: Callbacks assume blocking execution

**Impact**: Before/after/validation/navigation callbacks are called synchronously between steps. In an async model, callbacks must be invoked as part of the step transition in `Update()`.

**Evidence**: `wizard.go:251-263` (before), `wizard.go:289-303` (after), `wizard.go:319-334` (validation), `wizard.go:339-358` (navigation).

**Solution**: The `WizardModel` must invoke callbacks during step transitions in `Update()`, potentially wrapping long-running callbacks as `tea.Cmd` that return results via messages.

### Gap 6: ActionStep goroutine pattern is incompatible

**Impact**: ActionStep spawns a goroutine that runs `huh.NewNote()` as a parallel `tea.NewProgram()`. This cannot work inside another `tea.Program`.

**Evidence**: `action_step.go` — goroutine spawning `note.Run()` for progress display.

**Solution**: ActionStep's embedded model should show progress via its own `View()` output (e.g., a spinner), run the callback as a `tea.Cmd`, and transition to completion view on callback result message.


## 7. Proposed Solution: WizardModel

### 7.1 Architecture

```
┌──────────────────────────────────────────┐
│ WizardModel (tea.Model)                  │
│                                          │
│  state: map[string]interface{}           │
│  currentStep: int                        │
│  currentModel: tea.Model  ← active step  │
│  wizard: *Wizard          ← definition   │
│  phase: preparing|running|callbacks|done  │
│                                          │
│  Init()   → init first step model        │
│  Update() → route to current step model  │
│            → handle step completion      │
│            → run callbacks               │
│            → advance to next step        │
│  View()   → delegate to current model    │
└──────────────────────────────────────────┘
```

### 7.2 State Machine

```
┌─────────┐   skip     ┌─────────┐
│ Prepare │──────────→│ Prepare │  (next step)
│  Step   │           │  Step   │
└────┬────┘           └─────────┘
     │ build model
     ▼
┌─────────┐  step done  ┌──────────┐  callbacks  ┌─────────┐
│ Running │────────────→│ After    │────────────→│ Prepare │
│  Step   │             │ Callbacks│             │  Step   │
└─────────┘             └──────────┘             └─────────┘
                                                      │
                                                      │ no more steps
                                                      ▼
                                                 ┌─────────┐
                                                 │  Done   │
                                                 └─────────┘
```

### 7.3 Step-Level BuildModel Methods

Each step type needs a new method:

```go
// New interface for embeddable steps
type EmbeddableStep interface {
    Step
    BuildModel(state map[string]interface{}) (tea.Model, map[string]interface{}, error)
}
```

| Step Type | BuildModel() Implementation |
|-----------|---------------------------|
| **FormStep** | Call `FormData.BuildBubbleTeaModel()` — already exists |
| **DecisionStep** | Build `huh.NewForm(huh.NewGroup(huh.NewSelect(...)))`, return without `.Run()` |
| **ActionStep** | Return custom model with spinner + callback-as-Cmd + completion view |
| **InfoStep** | Build `huh.NewForm(huh.NewGroup(huh.NewNote(...)))`, return without `.Run()` |
| **SummaryStep** | Build `huh.NewForm(huh.NewGroup(huh.NewNote(...)))`, return without `.Run()` |

### 7.4 Message Types

```go
type StepCompletedMsg struct {
    StepID  string
    Results map[string]interface{}
}

type StepAbortedMsg struct {
    StepID string
    Err    error
}

type CallbackCompletedMsg struct {
    Phase    string // "before", "after", "validation", "navigation"
    StepID   string
    NextStep *string // For navigation callbacks
    Err      error
}

type WizardCompletedMsg struct {
    FinalState map[string]interface{}
}
```

### 7.5 API Comparison: Standalone vs Embedded

```go
// --- Standalone (existing) ---
wizard, _ := wizard.LoadWizard("wizard.yaml",
    wizard.WithCallback("validate", myValidator),
    wizard.WithActionCallback("deploy", myDeployer),
)
results, err := wizard.Run(ctx, initialState)

// --- Embedded (proposed) ---
wizard, _ := wizard.LoadWizard("wizard.yaml",
    wizard.WithCallback("validate", myValidator),
    wizard.WithActionCallback("deploy", myDeployer),
)
model, _ := wizard.BuildBubbleTeaModel(initialState)
// Use model.Init(), model.Update(), model.View() in parent
// Check for WizardCompletedMsg in parent's Update()
```


## 8. Integration with FormOverlay (SPT-3 Design Doc)

The `FormOverlay` adapter proposed in the main SPT-3 design doc wraps a `*huh.Form`. Since uhoh's `BuildBubbleTeaModel()` already returns a `*huh.Form`, single-form embedding works today with zero uhoh changes:

```go
// uhoh form in a FormOverlay — works today
yamlBytes := loadFormYAML()
huhForm, values, _ := uhoh.BuildBubbleTeaModelFromYAML(yamlBytes)
overlay := NewFormOverlay(func() *huh.Form { return huhForm })
```

For wizards, the `WizardModel` itself would be used as the overlay's content model, replacing the `*huh.Form`:

```go
// uhoh wizard in a WizardOverlay — requires WizardModel
wizard, _ := wizard.LoadWizard("wizard.yaml", ...)
wizModel, _ := wizard.BuildBubbleTeaModel(initialState)
overlay := NewWizardOverlay(func() tea.Model { return wizModel })
```

This means the overlay adapter should be generalized to accept `tea.Model` rather than just `*huh.Form`.


## 9. Implementation Phases

### Phase 1: Deduplicate Form.Run() (low risk, high value)

Refactor `Form.Run()` to delegate to `BuildBubbleTeaModel()`. This eliminates ~300 lines of duplicated code and ensures both paths stay in sync.

**Files**: `pkg/formdsl.go`
**Effort**: Small — mechanical refactor.
**Risk**: Low — existing tests cover behavior.

### Phase 2: Add BuildModel() to Step Types (medium risk)

Add `BuildModel(state) (tea.Model, map[string]interface{}, error)` to each step type. FormStep already has the building blocks; DecisionStep, InfoStep, and SummaryStep need to construct `huh.Form` instances without calling `.Run()`.

**Files**: `pkg/wizard/steps/form_step.go`, `decision_step.go`, `info_step.go`, `summary_step.go`, `action_step.go`, `step.go`
**Effort**: Medium — each step needs a parallel code path.
**Risk**: Medium — ActionStep's goroutine pattern needs careful redesign.

### Phase 3: Create WizardModel (medium-high risk)

Implement `WizardModel` as a `tea.Model` that:
- Maintains wizard state and current step index.
- Builds and drives step models through Init/Update/View.
- Handles step completion, callback execution, navigation, and skip conditions.
- Emits `WizardCompletedMsg` when all steps finish.

**Files**: New `pkg/wizard/wizard_model.go`
**Effort**: Medium-large — state machine with multiple phases.
**Risk**: Medium-high — callback execution timing, navigation jumps, and error handling all need async equivalents.

### Phase 4: ActionStep Async Redesign (high risk)

Redesign ActionStep's embedded model to:
- Show spinner/progress via View() instead of spawning a separate note program.
- Execute the callback as a `tea.Cmd`.
- Transition to completion view on callback result.

**Files**: `pkg/wizard/steps/action_step.go`, new `action_step_model.go`
**Effort**: Medium — requires understanding of Bubble Tea command patterns.
**Risk**: High — long-running callbacks need timeout handling, error display, and cancellation.

### Phase 5: Integration Tests and Example

Create a `examples/bubbletea-embed-wizard/` example that demonstrates an embedded wizard inside a parent Bubble Tea model, similar to the existing `examples/bubbletea-embed/` for forms.

**Files**: New `examples/bubbletea-embed-wizard/main.go`, `wizard.yaml`
**Effort**: Small-medium.
**Risk**: Low — follows established pattern.


## 10. Risks and Open Questions

### Risks

1. **Callback execution model**: Synchronous callbacks that modify state are simple in the blocking loop but complex in async. Long-running callbacks (e.g., API calls in ActionStep) need to be `tea.Cmd` with proper error handling. Short callbacks (validation, navigation) could remain synchronous in `Update()` — but this blocks the event loop.

2. **Navigation jumps in async context**: The blocking loop can jump to any step by index. In the async model, jumping backwards means the old step's model must be discarded and a new one built, potentially with different state. Forward jumps skip step model construction entirely.

3. **Terminal corruption from nested programs**: Any step that accidentally calls `.Run()` (which creates a `tea.NewProgram()`) inside an existing program will corrupt the terminal. The `EmbeddableStep` interface must make it impossible to accidentally call the blocking path.

4. **ActionStep complexity**: ActionStep is the most complex step type, combining async callbacks, progress display, and completion UI. Its embedded model is the riskiest piece.

### Open Questions

1. **Should WizardModel support back navigation?** The current wizard engine only moves forward (or jumps). Embedded wizards in modals often want a "Back" button. This would require step models to support re-initialization with previous state.

2. **Should callbacks become tea.Cmd universally?** Making all callbacks async (via `tea.Cmd`) is cleaner but adds complexity. An alternative is to allow synchronous callbacks for fast operations (validation, navigation) and only require `tea.Cmd` for slow operations (action callbacks).

3. **Should the existing Run() path be preserved?** The blocking `Wizard.Run()` works well for CLI usage. The question is whether to maintain both paths or migrate everything to the async model with a `Run()` wrapper that creates a `tea.NewProgram(wizardModel)`.

4. **Generalize FormOverlay to ModelOverlay?** Since `WizardModel` is a `tea.Model` (not a `*huh.Form`), the bobatea overlay adapter needs to accept any `tea.Model`. This might mean renaming `FormOverlay` to `ModelOverlay` or having two adapter types.


## 11. References

| File | Role |
|------|------|
| `uhoh/pkg/formdsl.go` | Form DSL: YAML → huh.Form translation |
| `uhoh/pkg/wizard/wizard.go` | Wizard orchestrator: blocking Run() loop |
| `uhoh/pkg/wizard/steps/step.go` | Step interface (BaseStep) |
| `uhoh/pkg/wizard/steps/form_step.go` | FormStep: delegates to Form.Run() |
| `uhoh/pkg/wizard/steps/decision_step.go` | DecisionStep: huh.Select → form.Run() |
| `uhoh/pkg/wizard/steps/action_step.go` | ActionStep: callbacks + huh.Note progress |
| `uhoh/pkg/wizard/steps/info_step.go` | InfoStep: huh.Note.Run() |
| `uhoh/pkg/wizard/steps/summary_step.go` | SummaryStep: huh.Note.Run() |
| `uhoh/examples/bubbletea-embed/main.go` | Proof that uhoh form embedding works |
| SPT-3 design doc | FormOverlay adapter proposal |
