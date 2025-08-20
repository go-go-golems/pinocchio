---
Title: Brainstorm: Pretty‑print interfaces and embeddable Bubbletea models for tools/middlewares
Slug: brainstorming-pretty-print-and-embedded-models
Short: Proposal to let tools/middlewares expose pretty‑print data views or full Bubbletea models for rendering inside the parent UI
Topics:
- pinocchio
- tools
- middleware
- ui
- bubbletea
- interfaces
ShowPerDefault: true
SectionType: DesignDoc
---

## Motivation

We want tools (and optionally middlewares) to render data nicely inside the agent UI—similar to the calculator sidebar—but in a reusable, composable way. Two complementary capabilities:

1) Pretty‑print views: a standardized interface to provide structured content the parent UI can display in a panel (table/list/text/JSON snippets) without the tool managing Bubbletea itself.
2) Embedded UI models: advanced tools/middlewares can expose a full Bubbletea model, which the parent UI can host in a panel/drawer to customize settings and interact mid‑run.

## Interfaces

### Pretty‑print provider

```go
// PrettyRenderable describes a renderable view that the parent UI knows how to display.
type PrettyRenderable interface {
    // Kind returns a hint ("table", "list", "text", "json", "markdown", "progress").
    Kind() string
    // Data returns structured payload; parent renders with standard components.
    Data() any
    // Title/ID used for panel headers and routing
    Title() string
    ID() string
}

// PrettyPrinter is implemented by tools/middlewares to supply views at checkpoints.
type PrettyPrinter interface {
    // Views returns zero or more renderables; can change during execution.
    Views() []PrettyRenderable
}
```

The parent UI includes adapters:

- table -> bubbles table
- list  -> bubbles list
- text/markdown/json -> viewport + syntax highlight (simple)
- progress -> progress bar

Tools/middlewares that implement `PrettyPrinter` can publish UI control events (`UIOpenPanel`, `UIUpdatePanel`) carrying `PrettyRenderable` data. The UI will render or refresh the panel with consistent styling.

### Embeddable Bubbletea model

```go
// EmbeddableModel lets a tool/middleware provide a full Bubbletea model for a panel.
type EmbeddableModel interface {
    // Model constructs the tea.Model to be mounted; parent owns lifecycle.
    Model() tea.Model
    // Bounds hint for layout (min width/height; optional).
    MinWidth() int
    MinHeight() int
    Title() string
    ID() string
}
```

The parent panel manager will mount/unmount this model inside a container (sidebar/drawer/overlay). Events from the tool continue to flow normally; user interactions inside the embedded model can emit domain events or a small control bus.

## Event flow

1) Tool/middleware emits `UIOpenPanel` with either `PrettyRenderable` or `EmbeddableModel` metadata.
2) Parent creates a panel, chooses a renderer:
   - Pretty: `PrettyRenderer` with mapping by Kind
   - Embeddable: `EmbeddedModelPanel` mounting `tea.Model`
3) Tool/middleware emits `UIUpdatePanel` to update content (Pretty) or forwards messages to the embedded model (Embeddable).
4) Close via `UIClosePanel`.

## Calculator example (pretty)

- Tool implements `PrettyPrinter` and returns a list/table with rows: ID, A, Op, B, Result.
- Parent renders in the right sidebar panel using the existing calculator style (border, padding) unified with the standard theme.

## Settings editor example (embedded model)

- Middleware exposes an `EmbeddableModel` containing a small form (Bubbletea + `huh` widgets) to tweak runtime settings (temperature, tool choice, limits) while a run is paused or before it starts.
- Parent mounts it in a drawer; on submit/cancel, panel closes and events update the pipeline settings.

## Lifecycle and ownership

- Parent UI owns the Bubbletea loop; embedded models are children.
- Messages from tool → UI go through the router → forwarding handler → parent update.
- For embedded models, define a small adapter to forward custom messages between parent and child models safely.

## Theming & consistency

- Provide a shared `Theme` (Lip Gloss styles) used by `PrettyRenderer` and panel containers.
- Clamp widths for small terminals; prefer line‑wrapping + scrolling in viewports.

## API sketch (parent side)

```go
type PanelKind int
const (
    PanelPretty PanelKind = iota
    PanelEmbedded
)

type Panel struct {
    ID, Title string
    Kind      PanelKind
    Pretty    *PrettyRenderable
    Embedded  tea.Model
    WidthHint int
}

// PanelManager manages open panels, layout, and updates.
type PanelManager interface {
    Open(p Panel)
    Update(id string, payload any)
    Close(id string)
    View() string // returns right/overlay/drawer markup
}
```

## Migration plan

1) Define `uievents` constructors and message types; wire UI forwarding.
2) Implement `PanelManager` in the simple agent UI.
3) Add `PrettyRenderer` adapters (table/list/text/markdown/json/progress).
4) Add `EmbeddedModelPanel` to host a child `tea.Model`.
5) Convert calculator to `PrettyPrinter` (as a first reference implementation).
6) Create an example middleware exposing an embedded model for runtime settings.

## Risks

- Complexity creep in parent model: mitigate with a small, sealed `PanelManager` submodel and focused adapters.
- Back‑pressure from chat events while an embedded model is open: keep them on separate panels; do not block main viewport.

## Success criteria

- Tools can register a pretty view with minimal code.
- Middlewares can open a panel to show progress/controls.
- Parent UI stays responsive and readable at 80×24.


