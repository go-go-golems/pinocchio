package ui

import (
    bspinner "github.com/charmbracelet/bubbles/spinner"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/huh"
    "github.com/charmbracelet/lipgloss"
    "github.com/go-go-golems/geppetto/pkg/events"
    toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
    uhohdsl "github.com/go-go-golems/uhoh/pkg"
    "github.com/rs/zerolog/log"
    "strings"
)

// ChatAppModel wraps the new chat timeline model with a status header and the existing
// sidebar that aggregates tool info. This keeps the tool info sidebar while adopting
// the TimelineShell + input UX from bobatea/chat.
type ChatAppModel struct {
    spinner  bspinner.Model
    uiEvents <-chan interface{}

    // embedded chat model
    chat tea.Model

    // Tool-driven form integration (reused from REPL app)
    toolReqCh  <-chan toolspkg.ToolUIRequest
    activeForm *huh.Form
    formVals   map[string]interface{}
    formReply  chan toolspkg.ToolUIReply

    // Status line
    status      string
    isStreaming bool

    // Live streamed assistant output (cleared on final)
    live string

    // Tool call/result compact log shown above the chat (cleared on final)
    toolEvents     string
    toolEntryIndex map[string]int
    toolEntries    []events.ToolEventEntry

    // status bar fields
    currentMode string
    runID       string
    turnID      string

    // Sidebar (toggle with Ctrl+G)
    showSidebar bool
    sidebar     SidebarModel

    // Layout
    totalWidth  int
    totalHeight int
    leftWidth   int
    rightWidth  int
}

func NewChatAppModel(uiCh <-chan interface{}, chat tea.Model, toolReqCh <-chan toolspkg.ToolUIRequest) ChatAppModel {
    sp := bspinner.New()
    sp.Spinner = bspinner.Line
    sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
    return ChatAppModel{
        spinner:        sp,
        uiEvents:       uiCh,
        chat:           chat,
        toolReqCh:      toolReqCh,
        sidebar:        NewSidebarModel(),
        toolEntryIndex: map[string]int{},
        toolEntries:    []events.ToolEventEntry{},
        currentMode:    "teacher",
    }
}

// waitForUIEvent and waitForToolReq are defined in app.go for this package; reuse them here

func (m ChatAppModel) Init() tea.Cmd {
    return tea.Batch(m.spinner.Tick, m.chat.Init(), waitForUIEvent(m.uiEvents), waitForToolReq(m.toolReqCh))
}

func (m ChatAppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if m.activeForm != nil {
        fm, cmd := safeFormUpdate(m.activeForm, msg)
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
    case *events.EventLog:
        switch ev.Message {
        case "agentmode: user prompt inserted":
            if mode, ok := ev.Fields["mode"].(string); ok && mode != "" { m.currentMode = mode }
        }
        m.sidebar, _ = m.sidebar.Update(ev)
    case *events.EventInfo:
        switch ev.Message {
        case "agentmode: mode switched", "Mode changed":
            if to, ok := ev.Data["to"].(string); ok && to != "" { m.currentMode = to }
        }
        m.sidebar, _ = m.sidebar.Update(ev)
    case *events.EventPartialCompletionStart:
        meta := ev.Metadata()
        if meta.RunID != "" { m.runID = meta.RunID }
        if meta.TurnID != "" { m.turnID = meta.TurnID }
        m.isStreaming = true
    case tea.WindowSizeMsg:
        m.totalWidth = ev.Width
        m.totalHeight = ev.Height
        if m.showSidebar {
            desiredRightTotal := int(float64(ev.Width) * 0.25)
            if desiredRightTotal < 24 { desiredRightTotal = 24 }
            if desiredRightTotal > ev.Width/2 { desiredRightTotal = ev.Width / 2 }
            m.rightWidth = maxInt(0, desiredRightTotal-4)
        } else {
            m.rightWidth = 0
        }
        m.leftWidth = ev.Width - (m.rightWidth + 4)
        if m.leftWidth < 0 { m.leftWidth = 0 }
        if m.rightWidth > 0 { m.sidebar, _ = m.sidebar.Update(SetSidebarSizeMsg{Width: m.rightWidth}) }
        // Forward size to chat model
    case tea.KeyMsg:
        if ev.String() == "ctrl+g" {
            m.showSidebar = !m.showSidebar
            // recompute based on stored totalWidth
            if m.showSidebar {
                desiredRightTotal := int(float64(m.totalWidth) * 0.25)
                if desiredRightTotal < 24 { desiredRightTotal = 24 }
                if desiredRightTotal > m.totalWidth/2 { desiredRightTotal = m.totalWidth / 2 }
                m.rightWidth = maxInt(0, desiredRightTotal-4)
            } else {
                m.rightWidth = 0
            }
            m.leftWidth = m.totalWidth - (m.rightWidth + 4)
            if m.leftWidth < 0 { m.leftWidth = 0 }
            if m.rightWidth > 0 { m.sidebar, _ = m.sidebar.Update(SetSidebarSizeMsg{Width: m.rightWidth}) }
        }
    case toolspkg.ToolUIRequest:
        m.activeForm = ev.Form
        m.formVals = ev.Values
        m.formReply = ev.ReplyCh
        return m, nil
    case *events.EventPartialCompletion:
        m.live += ev.Delta
        return m, tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
    case *events.EventToolCall:
        m.status = "Tool: " + ev.ToolCall.Name
        log.Debug().Str("id", ev.ToolCall.ID).Str("name", ev.ToolCall.Name).Msg("UI: EventToolCall")
        idx, found := m.toolEntryIndex[ev.ToolCall.ID]
        if !found { idx = len(m.toolEntries); m.toolEntryIndex[ev.ToolCall.ID] = idx; m.toolEntries = append(m.toolEntries, events.ToolEventEntry{ID: ev.ToolCall.ID}) }
        entry := &m.toolEntries[idx]
        entry.ProviderCalled = true
        entry.Name = ev.ToolCall.Name
        if ev.ToolCall.Input != "" { entry.Input = ev.ToolCall.Input }
        m.renderToolEvents()
        m.sidebar, _ = m.sidebar.Update(ev)
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolCallExecute:
        m.status = "Executing: " + ev.ToolCall.Name
        log.Debug().Str("id", ev.ToolCall.ID).Str("name", ev.ToolCall.Name).Msg("UI: EventToolCallExecute")
        idx, ok := m.toolEntryIndex[ev.ToolCall.ID]
        if !ok { idx = len(m.toolEntries); m.toolEntryIndex[ev.ToolCall.ID] = idx; m.toolEntries = append(m.toolEntries, events.ToolEventEntry{ID: ev.ToolCall.ID, Name: ev.ToolCall.Name}) }
        m.toolEntries[idx].ExecStarted = true
        if ev.ToolCall.Input != "" && m.toolEntries[idx].Input == "" { m.toolEntries[idx].Input = ev.ToolCall.Input }
        m.renderToolEvents()
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolResult:
        m.status = ""
        res := ev.ToolResult.Result
        log.Debug().Str("id", ev.ToolResult.ID).Int("entries", len(m.toolEntries)).Msg("UI: EventToolResult")
        idx, ok := m.toolEntryIndex[ev.ToolResult.ID]
        if !ok { idx = len(m.toolEntries); m.toolEntryIndex[ev.ToolResult.ID] = idx; m.toolEntries = append(m.toolEntries, events.ToolEventEntry{ID: ev.ToolResult.ID}) }
        m.toolEntries[idx].Result = res
        m.renderToolEvents()
        m.addCoalescedToolLineToSidebar(ev.ToolResult.ID)
        m.sidebar, _ = m.sidebar.Update(ev)
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolCallExecutionResult:
        m.status = ""
        res := ev.ToolResult.Result
        log.Debug().Str("id", ev.ToolResult.ID).Int("entries", len(m.toolEntries)).Msg("UI: EventToolCallExecutionResult")
        idx, ok := m.toolEntryIndex[ev.ToolResult.ID]
        if !ok { idx = len(m.toolEntries); m.toolEntryIndex[ev.ToolResult.ID] = idx; m.toolEntries = append(m.toolEntries, events.ToolEventEntry{ID: ev.ToolResult.ID}) }
        m.toolEntries[idx].Result = res
        m.renderToolEvents()
        m.addCoalescedToolLineToSidebar(ev.ToolResult.ID)
        m.sidebar, _ = m.sidebar.Update(ev)
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventFinal:
        m.isStreaming = false
        m.live = ""
        m.toolEvents = ""
        m.toolEntryIndex = map[string]int{}
        m.toolEntries = nil
        m.status = ""
        return m, nil
    case *events.EventError:
        m.isStreaming = false
        m.live = ""
        m.toolEvents = ""
        m.toolEntryIndex = map[string]int{}
        m.toolEntries = nil
        m.status = ""
        return m, nil
    }

    var cmds []tea.Cmd
    var cmd tea.Cmd
    m.spinner, cmd = m.spinner.Update(msg)
    cmds = append(cmds, cmd)
    var child tea.Model
    child, cmd = m.chat.Update(msg)
    if child != nil { m.chat = child }
    cmds = append(cmds, cmd)
    return m, tea.Batch(append(cmds, waitForUIEvent(m.uiEvents), waitForToolReq(m.toolReqCh))...)
}

func (m ChatAppModel) View() string {
    if m.activeForm != nil {
        return safeFormView(m.activeForm)
    }
    // Build status bar: mode / run / turn
    var statusLeft string
    if m.currentMode != "" { statusLeft = "mode:" + m.currentMode } else { statusLeft = "mode:?" }
    if m.runID != "" { statusLeft += "  run:" + m.runID }
    if m.turnID != "" { statusLeft += "  turn:" + m.turnID }

    var header string
    if m.status != "" || m.isStreaming {
        header = headerStyle.Render(statusLeft + "  |  " + m.status)
        header += " " + m.spinner.View()
        top := header
        if m.toolEvents != "" { top = top + "\n" + m.toolEvents }
        if m.live != "" { return m.renderLayout(top+"\n"+jsonStyle.Render(m.live), m.chatView()) }
        return m.renderLayout(top, m.chatView())
    }
    if m.live != "" || m.toolEvents != "" {
        top := headerStyle.Render(statusLeft)
        if m.toolEvents != "" { top += "\n" + m.toolEvents }
        if m.live != "" {
            if top != "" { top += "\n" }
            top += jsonStyle.Render(m.live)
        }
        return m.renderLayout(top, m.chatView())
    }
    return m.renderLayout(headerStyle.Render(statusLeft), m.chatView())
}

func (m ChatAppModel) chatView() string {
    // Let the chat model render itself; it already manages its own viewport and sizes
    v := m.chat.View()
    if m.leftWidth > 0 {
        return lipgloss.NewStyle().Width(m.leftWidth).Render(v)
    }
    return v
}

func (m ChatAppModel) renderLayout(top string, main string) string {
    left := main
    if top != "" { left = top + "\n" + main }
    if !m.showSidebar || m.rightWidth <= 0 {
        return lipgloss.NewStyle().Width(m.leftWidth).Render(left)
    }
    leftView := lipgloss.NewStyle().Width(m.leftWidth).Render(left)
    rightView := m.sidebar.View()
    return lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
}

// renderToolEvents composes a compact, single-line-per-call view across provider call, local exec, and results.
func (m *ChatAppModel) renderToolEvents() {
    if len(m.toolEntries) == 0 { m.toolEvents = ""; return }
    log.Debug().Int("entries", len(m.toolEntries)).Msg("UI: renderToolEvents")
    var out []string
    for _, e := range m.toolEntries {
        if e.Name == "" && e.ID == "" { continue }
        name := e.Name
        if name == "" { name = e.ID }
        parts := []string{}
        if e.ProviderCalled { parts = append(parts, toolCallStyle.Render("→ "+name)) }
        if e.ExecStarted { parts = append(parts, toolCallStyle.Render("↳ exec")) }
        if e.Result != "" { parts = append(parts, toolResultStyle.Render("← ")+jsonStyle.Render(e.Result)) }
        if e.Input != "" { parts = append(parts, jsonStyle.Render(e.Input)) }
        out = append(out, strings.Join(parts, "  "))
    }
    m.toolEvents = strings.Join(out, "\n")
}

// addCoalescedToolLineToSidebar currently a no-op placeholder retained for parity with AppModel.
func (m *ChatAppModel) addCoalescedToolLineToSidebar(id string) {}


