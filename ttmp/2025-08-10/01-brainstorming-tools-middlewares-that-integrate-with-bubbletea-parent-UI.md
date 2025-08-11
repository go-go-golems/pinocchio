---
Title: Brainstorm: Building tools/middlewares that integrate with the Bubbletea parent UI
Slug: brainstorming-bubbletea-integrated-tools
Short: Design notes and practical patterns for tools/middlewares that render widgets directly in the main agent UI
Topics:
- pinocchio
- ui
- bubbletea
- geppetto
- tools
- middleware
IsTopLevel: false
ShowPerDefault: true
SectionType: DesignDoc
---

## Purpose

This guide explains how to build a tool or middleware that can open and control UI widgets directly inside the main Bubbletea agent UI (as opposed to launching a separate, temporary “generative UI” form). The goal is to let long‑running, tool‑driven flows surface progress, controls, and structured results inside the parent UI (e.g., show a sidebar panel, a progress bar, or an embedded viewer).

## High‑level architecture

- Engines/middlewares publish events via Watermill sinks (Geppetto `events.Event`), already wired to the UI.
- The Bubbletea app subscribes and converts events into `tea.Msg`, which update UI state.
- For “UI control” (open panel, update content, close panel), define lightweight UI control events and handle them in the parent UI.

This keeps the engine/tool layer headless (no terminal coupling) and the UI reactive.

## Two integration paths

- Middleware level (cross‑cutting):
  - Sits in front of/around the engine; emits UI control events (open/close panel, progress) and domain events (records, previews, charts).
  - Good for telemetry, progress, session‑wide widgets.

- Tool level (per‑Turn tool):
  - The tool implementation sends UI control events while it runs. Useful for task‑specific widgets (e.g., calculator history, SQL table preview, diff viewer).

Both share the same event contract and UI wiring.

## Event contract (UI control)

Define a small set of UI control events carried over the existing sink (topic: `ui`):

- UIOpenPanel: { panel: "sidebar|overlay|drawer", id: string, title?: string, width?: number }
- UIUpdatePanel: { id: string, content: string | structured }
- UIClosePanel: { id: string }
- UIProgress: { id: string, current: number, total?: number, note?: string }

Represent them as Geppetto events (either new `events.EventType` or a generic `EventText`/`EventPartialCompletion` with a `StepMetadata.Type = "ui-control"` and payload in `Metadata`).

Recommended: add a small helper shim in a new package `github.com/go-go-golems/pinocchio/pkg/ui/events` with constructors:

```go
func NewUIOpenPanelEvent(id, panel, title string, width int) events.Event
func NewUIUpdatePanelEvent(id string, content any) events.Event
func NewUIClosePanelEvent(id string) events.Event
func NewUIProgressEvent(id string, cur, total int, note string) events.Event
```

## UI wiring in the parent app

In the main program where the Watermill router forwards chat events to the Bubbletea program (see `ui.StepChatForwardFunc(p)`), add a handler for `ui-control` events:

1) Dispatch to Bubbletea as domain‑specific messages:

```go
type UIOpenPanelMsg struct{ ID, Panel, Title string; Width int }
type UIUpdatePanelMsg struct{ ID string; Content any }
type UIClosePanelMsg struct{ ID string }
type UIProgressMsg struct{ ID string; Current, Total int; Note string }
```

2) In your root model (e.g., the simple agent app), add a dedicated panel manager:

- Keep a map `id -> PanelModel`, where `PanelModel` exposes `Update(msg)` and `View()`.
- Render panels in reserved areas (e.g., right sidebar, bottom drawer) and compute left container width accordingly (as already done in the simple agent).

3) Update loop handling:

- On `UIOpenPanelMsg`: create/register the panel and recompute layout.
- On `UIUpdatePanelMsg`: forward to the target panel model (or set content on a simple text panel).
- On `UIClosePanelMsg`: remove the panel and recompute layout.
- On `UIProgressMsg`: forward to a progress panel or update a status line.

## Emitting UI control events from middleware

Inside a middleware (example pseudo‑code):

```go
func (mw *UIMiddleware) Handle(ctx context.Context, next middleware.HandlerFunc, t *turns.Turn) (*turns.Turn, error) {
    id := uuid.New().String()
    // open panel
    mw.publish(ctx, uievents.NewUIOpenPanelEvent(id, "sidebar", "Tool run", 28))
    // progress
    mw.publish(ctx, uievents.NewUIProgressEvent(id, 0, 100, "starting"))

    // call next
    out, err := next(ctx, t)
    if err != nil { return out, err }

    // update panel with results (e.g., summary text or table)
    mw.publish(ctx, uievents.NewUIUpdatePanelEvent(id, map[string]any{"summary": "done"}))
    mw.publish(ctx, uievents.NewUIClosePanelEvent(id))
    return out, nil
}
```

Use the engine’s configured sink: `engine.WithSink(watermillSink)` is already wired; your middleware can receive it via context or compose the engine inside a decorated engine that shares the sink.

## Emitting UI control events from tools

If a tool needs to interact live:

1) Provide the sink via Turn/Data (context‑attach with `events.WithEventSinks`).

2) In the tool function, publish UI events at checkpoints:

```go
func MyTool(req Input) (Output, error) {
    sinks := events.EventSinksFromContext(req.Ctx)
    for _, s := range sinks { _ = s.PublishEvent(uievents.NewUIOpenPanelEvent(...)) }
    // compute …
    for _, s := range sinks { _ = s.PublishEvent(uievents.NewUIUpdatePanelEvent(...)) }
    // done
    for _, s := range sinks { _ = s.PublishEvent(uievents.NewUIClosePanelEvent(...)) }
    return out, nil
}
```

3) The parent UI will render/animate based on those messages.

## Widget patterns to re‑use

- Sidebar (right) for structured, scrollable content (e.g., calculator history, SQL preview)
- Drawer (bottom) for progress logs or streaming text
- Overlay (center) for temporary modal previews
- Tabs inside a panel for multi‑view (Summary / Table / Raw JSON)

Prefer composing existing Bubbles: `viewport`, `table`, `list`, `progress` rather than bespoke renderers. Keep a thin `PanelModel` that maps events to state, and `View()` to markup.

## State & layout tips

- Keep panel state normalized (e.g., `map[string]PanelState`) and derive views in `View()`.
- Compute the left container width by subtracting panel total width including border/padding (already implemented in the simple agent).
- Always handle small terminals: clamp panel widths, fall back to single‑column.

## Testing & tracing

- Unit‑test panel models by sending synthetic `tea.Msg` events.
- Use debug logs (`zerolog`) to inspect event flow.
- In a dev profile, log UI control events alongside chat events for auditing.

## Minimal end‑to‑end checklist

- [ ] Define UI control events and helper constructors
- [ ] Add router→Bubbletea forwarding for `ui-control`
- [ ] Add panel manager to parent UI (open/update/close handlers)
- [ ] Implement one demo middleware that emits `open → progress → update → close`
- [ ] Implement one demo tool that streams updates into the same panel
- [ ] Verify in tmux; resize, toggle sidebar, and ensure rendering stays correct


