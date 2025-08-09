package ui

import (
    bspinner "github.com/charmbracelet/bubbles/spinner"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/huh"
    "github.com/charmbracelet/lipgloss"
    "github.com/go-go-golems/bobatea/pkg/repl"
    "github.com/go-go-golems/geppetto/pkg/events"
    toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
    uhohdsl "github.com/go-go-golems/uhoh/pkg"
)

var (
    headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
    subHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
    toolNameStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
    jsonStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
    finalStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("118"))
    errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

type AppModel struct {
    spinner  bspinner.Model
    uiEvents <-chan interface{}

    repl repl.Model

    // Tool-driven form integration
    toolReqCh  <-chan toolspkg.ToolUIRequest
    activeForm *huh.Form
    formVals   map[string]interface{}
    formReply  chan toolspkg.ToolUIReply

    // Status line
    status      string
    isStreaming bool

    // Live streamed assistant output (cleared on final)
    live string
}

func NewAppModel(uiCh <-chan interface{}, replModel repl.Model, toolReqCh <-chan toolspkg.ToolUIRequest) AppModel {
    sp := bspinner.New()
    sp.Spinner = bspinner.Line
    sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
    return AppModel{
        spinner:  sp,
        uiEvents: uiCh,
        repl:     replModel,
        toolReqCh: toolReqCh,
    }
}

func waitForUIEvent(ch <-chan interface{}) tea.Cmd {
    return func() tea.Msg {
        e, ok := <-ch
        if !ok {
            return nil
        }
        return e
    }
}

func waitForToolReq(ch <-chan toolspkg.ToolUIRequest) tea.Cmd {
    return func() tea.Msg {
        req, ok := <-ch
        if !ok {
            return nil
        }
        return req
    }
}

func (m AppModel) Init() tea.Cmd {
    return tea.Batch(m.spinner.Tick, m.repl.Init(), waitForUIEvent(m.uiEvents), waitForToolReq(m.toolReqCh))
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if m.activeForm != nil {
        fm, cmd := m.activeForm.Update(msg)
        if f, ok := fm.(*huh.Form); ok {
            m.activeForm = f
        }
        if m.activeForm.State == huh.StateCompleted && m.formReply != nil {
            vals, err := uhohdsl.ExtractFinalValues(m.formVals)
            reply := m.formReply
            m.activeForm = nil
            m.formVals = nil
            m.formReply = nil
            go func() { reply <- toolspkg.ToolUIReply{Values: vals, Err: err} }()
        }
        return m, tea.Batch(cmd, waitForUIEvent(m.uiEvents), waitForToolReq(m.toolReqCh))
    }

    switch ev := msg.(type) {
    case tea.WindowSizeMsg:
        m.repl.SetWidth(ev.Width)
        return m, nil
    case toolspkg.ToolUIRequest:
        m.activeForm = ev.Form
        m.formVals = ev.Values
        m.formReply = ev.ReplyCh
        return m, nil
    case *events.EventPartialCompletionStart:
        m.isStreaming = true
        m.status = "Streaming…"
        return m, tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
    case *events.EventPartialCompletion:
        // append raw deltas; REPL will show final answers post-loop as well
        m.live += ev.Delta
        return m, tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
    case *events.EventToolCall:
        m.status = "Tool: " + ev.ToolCall.Name
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolCallExecute:
        m.status = "Executing: " + ev.ToolCall.Name
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolResult:
        m.status = ""
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolCallExecutionResult:
        m.status = ""
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventFinal:
        m.isStreaming = false
        m.live = ""
        m.status = ""
        return m, nil
    case *events.EventError:
        m.isStreaming = false
        m.live = ""
        m.status = ""
        return m, nil
    }

    var cmds []tea.Cmd
    var cmd tea.Cmd
    m.spinner, cmd = m.spinner.Update(msg)
    cmds = append(cmds, cmd)
    var replModel tea.Model
    replModel, cmd = m.repl.Update(msg)
    if rm, ok := replModel.(repl.Model); ok {
        m.repl = rm
    }
    cmds = append(cmds, cmd)
    return m, tea.Batch(append(cmds, waitForUIEvent(m.uiEvents), waitForToolReq(m.toolReqCh))...)
}

func (m AppModel) View() string {
    if m.activeForm != nil {
        return m.activeForm.View()
    }
    var header string
    if m.status != "" || m.isStreaming {
        header = headerStyle.Render(m.status)
        // show spinner while streaming or when a status is set (tool running)
        header += " " + m.spinner.View()
        // show live streamed output as a temporary block above the REPL
        if m.live != "" {
            return header + "\n" + jsonStyle.Render(m.live) + "\n" + m.repl.View()
        }
        return header + "\n" + m.repl.View()
    }
    if m.live != "" {
        return jsonStyle.Render(m.live) + "\n" + m.repl.View()
    }
    return m.repl.View()
}


