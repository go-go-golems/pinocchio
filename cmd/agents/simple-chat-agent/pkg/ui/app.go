package ui

import (
    "strings"

    bspinner "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/viewport"
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
    viewport viewport.Model
    content  string
    uiEvents <-chan interface{}

    repl repl.Model

    // Tool-driven form integration
    toolReqCh  <-chan toolspkg.ToolUIRequest
    activeForm *huh.Form
    formVals   map[string]interface{}
    formReply  chan toolspkg.ToolUIReply
}

func NewAppModel(uiCh <-chan interface{}, replModel repl.Model, toolReqCh <-chan toolspkg.ToolUIRequest) AppModel {
    sp := bspinner.New()
    sp.Spinner = bspinner.Line
    sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
    vp := viewport.New(80, 6)
    vp.Style = lipgloss.NewStyle()
    return AppModel{
        spinner:  sp,
        viewport: vp,
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
        m.viewport.Width = ev.Width
        m.repl.SetWidth(ev.Width)
        return m, nil
    case toolspkg.ToolUIRequest:
        m.activeForm = ev.Form
        m.formVals = ev.Values
        m.formReply = ev.ReplyCh
        return m, nil
    case *events.EventPartialCompletionStart:
        m.content += "\n" + headerStyle.Render("— Inference started —")
        m.viewport.SetContent(m.content)
        return m, tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
    case *events.EventPartialCompletion:
        // append raw deltas; REPL will show final answers post-loop as well
        if ev.Delta != "" {
            m.content += ev.Delta
            m.viewport.SetContent(m.content)
        }
        return m, tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
    case *events.EventToolCall:
        m.content += "\n" + toolNameStyle.Render("Tool Call: ") + ev.ToolCall.Name
        if s := strings.TrimSpace(ev.ToolCall.Input); s != "" {
            m.content += "\n" + jsonStyle.Render(s)
        }
        m.viewport.SetContent(m.content)
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolCallExecute:
        m.content += "\n" + subHeaderStyle.Render("Executing: ") + ev.ToolCall.Name
        if s := strings.TrimSpace(ev.ToolCall.Input); s != "" {
            m.content += "\n" + jsonStyle.Render(s)
        }
        m.viewport.SetContent(m.content)
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolResult:
        m.content += "\n" + subHeaderStyle.Render("Tool Result:")
        if s := strings.TrimSpace(ev.ToolResult.Result); s != "" {
            m.content += "\n" + jsonStyle.Render(s)
        }
        m.viewport.SetContent(m.content)
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolCallExecutionResult:
        m.content += "\n" + subHeaderStyle.Render("Tool Exec Result:")
        if s := strings.TrimSpace(ev.ToolResult.Result); s != "" {
            m.content += "\n" + jsonStyle.Render(s)
        }
        m.viewport.SetContent(m.content)
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventFinal:
        if ev.Text != "" {
            m.content += "\n" + finalStyle.Render(ev.Text)
            m.viewport.SetContent(m.content)
        }
        return m, nil
    case *events.EventError:
        m.content += "\n" + errorStyle.Render("Error: ") + ev.ErrorString
        m.viewport.SetContent(m.content)
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
    header := headerStyle.Render("Streaming… ") + m.spinner.View()
    return header + "\n" + m.viewport.View() + "\n" + m.repl.View()
}


