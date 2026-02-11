---
Title: Integrating Bubble Tea and Lip Gloss in a Streaming Agent
Short: How to render a spinner and a streaming viewport during inference without clearing the terminal
---

## Overview

This guide shows how to integrate Bubble Tea and Lip Gloss UI components into a streaming agent built with Geppetto. The approach preserves your normal REPL input/output while running a Bubble Tea program that renders a spinner and a scrolling viewport for streaming tokens and tool logs. It avoids clearing the whole terminal by rendering only within Bubble Tea’s frame.

The implementation hooks into the event stream produced by the Geppetto engine via a Watermill-backed router. We forward events into a channel consumed by a Bubble Tea model, and run that program concurrently with inference.

See the reference code in `pinocchio/cmd/agents/simple-chat-agent/main.go`:
- `addUIForwarder` to forward events into a channel
- `streamUIModel` Bubble Tea model with a spinner and viewport
- Wiring in `RunIntoWriter` to start the program and the tool-calling loop in parallel

## Key Ideas

- Use the existing event router and sink. Do not change the engine—just forward events to the UI.
- Keep stdin/out REPL logic outside Bubble Tea. Start/stop a short-lived Bubble Tea program per inference turn.
- Use a channel to deliver events to the Bubble Tea model. Avoid blocking by using a buffered channel.
- Keep your own content buffer instead of reading from `viewport.Model`; set content after changes.

## Minimal Steps

1) Forward events to a UI channel

```go
func addUIForwarder(router *events.EventRouter, ch chan<- interface{}) {
    router.AddHandler("ui-forwarder", "chat", func(msg *message.Message) error {
        defer msg.Ack()
        e, err := events.NewEventFromJson(msg.Payload)
        if err != nil { return err }
        select { case ch <- e: default: }
        return nil
    })
}
```

2) Create a Bubble Tea model with a spinner and viewport

```go
type streamUIModel struct {
    spinner     bspinner.Model
    viewport    viewport.Model
    uiEvents    <-chan interface{}
    content     string
    quitWhenDone bool
}

func newStreamUIModel(ch <-chan interface{}) streamUIModel { /* init spinner+viewport */ }

func (m streamUIModel) Init() tea.Cmd { return tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents)) }
func waitForUIEvent(ch <-chan interface{}) tea.Cmd { /* read one event, return as msg */ }

func (m streamUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch ev := msg.(type) {
    case *events.EventPartialCompletionStart: /* set streaming flag */
    case *events.EventPartialCompletion: m.content += ev.Delta; m.viewport.SetContent(m.content)
    case *events.EventFinal: if m.quitWhenDone { return m, tea.Quit }
    }
    return m, tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
}

func (m streamUIModel) View() string { /* header + spinner + viewport */ }
```

3) Run the UI concurrently with the inference loop

```go
uiCh := make(chan interface{}, 1024)
addUIForwarder(router, uiCh)
uiModel := newStreamUIModel(uiCh)
pgm := tea.NewProgram(uiModel, tea.WithOutput(w))

egTurn, turnCtx := errgroup.WithContext(runCtx)
egTurn.Go(func() error { _, err := pgm.Run(); return err })
egTurn.Go(func() error {
    loop := toolloop.New(
        toolloop.WithEngine(eng),
        toolloop.WithRegistry(registry),
        toolloop.WithLoopConfig(loopCfg),
        toolloop.WithToolConfig(toolCfg),
    )
    updated, err := loop.RunLoop(turnCtx, conv)
    // append updates to conversation
    return err
})
_ = egTurn.Wait()
```

## Notes

- You can replace the spinner with `github.com/charmbracelet/huh/spinner` for accessible spinners; see `huh/spinner/examples/accessible/main.go`.
- Bubble Tea is started per turn here to keep REPL input outside the Bubble Tea program; you can also build a persistent TUI and route keyboard input explicitly if you prefer.
- If you need to print raw lines to stdout while Bubble Tea is running, either use `tea.WithAltScreen()` appropriately or print within the Bubble Tea model’s `View` to avoid visual conflicts.

## References

- Bubble Tea: https://github.com/charmbracelet/bubbletea
- Bubbles spinner: https://github.com/charmbracelet/bubbles/tree/master/spinner
- Bubbles viewport: https://github.com/charmbracelet/bubbles/tree/master/viewport
- Huh spinner accessible example: `huh/spinner/examples/accessible/main.go`
