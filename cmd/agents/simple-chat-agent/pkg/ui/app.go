package ui

import (
	bspinner "github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/bobatea/pkg/repl"
	"github.com/go-go-golems/geppetto/pkg/events"
	toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	uhohdsl "github.com/go-go-golems/uhoh/pkg"
	"strings"
)

var (
	headerStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	jsonStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	toolCallStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	toolResultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("79"))
	// Container style for the REPL viewport
	replContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)
)

type AppModel struct {
	spinner  bspinner.Model
	uiEvents <-chan interface{}

	repl     repl.Model
	viewport viewport.Model

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

	// Tool call/result compact log shown above the REPL (cleared on final)
	toolEvents     string
	toolEntryIndex map[string]int
	toolEntries    []ToolEventEntry

	// status bar fields
	currentMode string
	runID       string
	turnID      string

	// When true, scroll viewport to bottom after updating content
	needsScrollBottom bool

	// Sidebar (toggle with Ctrl+G)
	showSidebar bool
	sidebar     SidebarModel

	// Layout
	totalWidth  int
	totalHeight int
	leftWidth   int
	rightWidth  int
}

// ToolEventEntry aggregates provider call, local execution, and result state by tool call ID.
type ToolEventEntry struct {
	ID             string
	Name           string
	Input          string
	ProviderCalled bool
	ExecStarted    bool
	Result         string
}

func NewAppModel(uiCh <-chan interface{}, replModel repl.Model, toolReqCh <-chan toolspkg.ToolUIRequest) AppModel {
	sp := bspinner.New()
	sp.Spinner = bspinner.Line
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	vp := viewport.New(0, 0)
	vp.Style = replContainerStyle
	return AppModel{
		spinner:        sp,
		uiEvents:       uiCh,
		repl:           replModel,
		viewport:       vp,
		toolReqCh:      toolReqCh,
		sidebar:        NewSidebarModel(),
		toolEntryIndex: map[string]int{},
		toolEntries:    []ToolEventEntry{},
		currentMode:    "teacher",
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
			if mode, ok := ev.Fields["mode"].(string); ok && mode != "" {
				m.currentMode = mode
			}
		}
		m.sidebar, _ = m.sidebar.Update(ev)
	case *events.EventInfo:
		switch ev.Message {
		case "agentmode: mode switched", "Mode changed":
			if to, ok := ev.Data["to"].(string); ok && to != "" {
				m.currentMode = to
			}
		}
		m.sidebar, _ = m.sidebar.Update(ev)
	case *events.EventTextSegmentStarted:
		// Extract run/turn from event metadata
		meta := ev.Metadata()
		if meta.SessionID != "" {
			m.runID = meta.SessionID
		}
		if meta.TurnID != "" {
			m.turnID = meta.TurnID
		}
		// Mark streaming active
		m.isStreaming = true
	case tea.WindowSizeMsg:
		m.totalWidth = ev.Width
		m.totalHeight = ev.Height
		if m.showSidebar {
			// Compute sidebar desired total width ~25% of screen, clamped
			desiredRightTotal := int(float64(ev.Width) * 0.25)
			if desiredRightTotal < 24 {
				desiredRightTotal = 24
			}
			if desiredRightTotal > ev.Width/2 {
				desiredRightTotal = ev.Width / 2
			}
			// Account for border(2) + padding(2) in sidebar style
			m.rightWidth = maxInt(0, desiredRightTotal-4)
		} else {
			m.rightWidth = 0
		}
		// Left width is the remainder, but actual rendering will re-measure the right view
		m.leftWidth = ev.Width - (m.rightWidth + 4)
		if m.leftWidth < 0 {
			m.leftWidth = 0
		}
		// Resize REPL to left panel width by forwarding a WindowSizeMsg
		_, _ = m.repl.Update(tea.WindowSizeMsg{Width: maxInt(0, m.leftWidth-2), Height: maxInt(0, ev.Height-3)})
		// Leave room for header line(s)
		m.viewport.Width = maxInt(0, m.leftWidth)
		m.viewport.Height = maxInt(0, ev.Height-3)
		m.viewport.SetContent(m.repl.View())
		if m.rightWidth > 0 {
			m.sidebar, _ = m.sidebar.Update(SetSidebarSizeMsg{Width: m.rightWidth})
		}
		return m, nil
	case tea.KeyMsg:
		// Gate REPL submissions while streaming or when tools are pending
		if ev.String() == "enter" {
			if m.isStreaming || m.hasPendingTools() {
				if m.status == "" {
					m.status = "Busy…"
				}
				return m, nil
			}
		}
		if ev.String() == "ctrl+g" {
			m.showSidebar = !m.showSidebar
			// Recompute widths based on stored totalWidth
			if m.showSidebar {
				desiredRightTotal := int(float64(m.totalWidth) * 0.25)
				if desiredRightTotal < 24 {
					desiredRightTotal = 24
				}
				if desiredRightTotal > m.totalWidth/2 {
					desiredRightTotal = m.totalWidth / 2
				}
				m.rightWidth = maxInt(0, desiredRightTotal-4)
			} else {
				m.rightWidth = 0
			}
			m.leftWidth = m.totalWidth - (m.rightWidth + 4)
			if m.leftWidth < 0 {
				m.leftWidth = 0
			}
			// Resize REPL to left panel width by forwarding a WindowSizeMsg
			_, _ = m.repl.Update(tea.WindowSizeMsg{Width: maxInt(0, m.leftWidth-2), Height: maxInt(0, m.totalHeight-3)})
			m.viewport.Width = maxInt(0, m.leftWidth)
			m.viewport.Height = maxInt(0, m.totalHeight-3)
			m.viewport.SetContent(m.repl.View())
			if m.rightWidth > 0 {
				m.sidebar, _ = m.sidebar.Update(SetSidebarSizeMsg{Width: m.rightWidth})
			}
			return m, nil
		}
	case toolspkg.ToolUIRequest:
		m.activeForm = ev.Form
		m.formVals = ev.Values
		m.formReply = ev.ReplyCh
		return m, nil
	case *events.EventTextDelta:
		// append raw deltas; REPL will show final answers post-loop as well
		m.live += ev.Delta
		m.needsScrollBottom = true
		return m, tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
	case *events.EventToolCallStarted:
		m.status = "Tool: " + ev.ToolName
		// Aggregate into single entry per ID
		log.Debug().Str("id", ev.ToolCallID).Str("name", ev.ToolName).Msg("UI: EventToolCallStarted")
		idx, found := m.toolEntryIndex[ev.ToolCallID]
		if !found {
			idx = len(m.toolEntries)
			m.toolEntryIndex[ev.ToolCallID] = idx
			m.toolEntries = append(m.toolEntries, ToolEventEntry{ID: ev.ToolCallID})
		}
		entry := &m.toolEntries[idx]
		entry.ProviderCalled = true
		entry.Name = ev.ToolName
		m.renderToolEvents()
		m.needsScrollBottom = true
		m.sidebar, _ = m.sidebar.Update(ev)
		return m, waitForUIEvent(m.uiEvents)
	case *events.EventToolCallRequested:
		m.status = "Tool: " + ev.ToolName
		log.Debug().Str("id", ev.ToolCallID).Str("name", ev.ToolName).Msg("UI: EventToolCallRequested")
		idx, found := m.toolEntryIndex[ev.ToolCallID]
		if !found {
			idx = len(m.toolEntries)
			m.toolEntryIndex[ev.ToolCallID] = idx
			m.toolEntries = append(m.toolEntries, ToolEventEntry{ID: ev.ToolCallID})
		}
		entry := &m.toolEntries[idx]
		entry.ProviderCalled = true
		entry.Name = ev.ToolName
		if ev.Input != "" {
			entry.Input = ev.Input
		}
		m.renderToolEvents()
		m.needsScrollBottom = true
		m.sidebar, _ = m.sidebar.Update(ev)
		return m, waitForUIEvent(m.uiEvents)
	case *events.EventToolExecutionStarted:
		m.status = "Executing: " + ev.ToolName
		// Aggregate into entry
		log.Debug().Str("id", ev.ToolCallID).Str("name", ev.ToolName).Msg("UI: EventToolExecutionStarted")
		idx, ok := m.toolEntryIndex[ev.ToolCallID]
		if !ok {
			idx = len(m.toolEntries)
			m.toolEntryIndex[ev.ToolCallID] = idx
			m.toolEntries = append(m.toolEntries, ToolEventEntry{ID: ev.ToolCallID, Name: ev.ToolName})
		}
		m.toolEntries[idx].ExecStarted = true
		if ev.Input != "" && m.toolEntries[idx].Input == "" {
			m.toolEntries[idx].Input = ev.Input
		}
		m.renderToolEvents()
		m.needsScrollBottom = true
		return m, waitForUIEvent(m.uiEvents)
	case *events.EventToolResultReady:
		m.status = ""
		res := ev.Result
		log.Debug().Str("id", ev.ToolCallID).Int("entries", len(m.toolEntries)).Msg("UI: EventToolResultReady")
		idx, ok := m.toolEntryIndex[ev.ToolCallID]
		if !ok {
			idx = len(m.toolEntries)
			m.toolEntryIndex[ev.ToolCallID] = idx
			m.toolEntries = append(m.toolEntries, ToolEventEntry{ID: ev.ToolCallID})
		}
		m.toolEntries[idx].Result = res
		if ev.ToolName != "" {
			m.toolEntries[idx].Name = ev.ToolName
		}
		m.renderToolEvents()
		m.addCoalescedToolLineToRepl(ev.ToolCallID)
		m.needsScrollBottom = true
		m.sidebar, _ = m.sidebar.Update(ev)
		return m, waitForUIEvent(m.uiEvents)
	case *events.EventTextSegmentFinished:
		m.isStreaming = false
		m.live = ""
		m.toolEvents = ""
		m.toolEntryIndex = map[string]int{}
		m.toolEntries = nil
		m.status = ""
		m.needsScrollBottom = true
		return m, nil
	case *events.EventError:
		m.isStreaming = false
		m.live = ""
		m.toolEvents = ""
		m.toolEntryIndex = map[string]int{}
		m.toolEntries = nil
		m.status = ""
		m.needsScrollBottom = true
		return m, nil
	case repl.EvaluationCompleteMsg:
		// New REPL entry added; ensure we scroll to bottom
		m.needsScrollBottom = true
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)
	var replModel tea.Model
	replModel, cmd = m.repl.Update(msg)
	if rm, ok := replModel.(*repl.Model); ok {
		m.repl = *rm
	}
	cmds = append(cmds, cmd)
	// Update viewport with latest content and pass through events for scrolling
	m.viewport.SetContent(m.repl.View())
	if m.needsScrollBottom {
		m.viewport.GotoBottom()
		m.needsScrollBottom = false
	}
	var vpcmd tea.Cmd
	// Forward only page up/down and mouse wheel events to the viewport for scrolling
	switch ev := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport, vpcmd = m.viewport.Update(msg)
		cmds = append(cmds, vpcmd)
	case tea.MouseMsg:
		// Allow mouse wheel and other mouse interactions to be handled by viewport
		m.viewport, vpcmd = m.viewport.Update(msg)
		cmds = append(cmds, vpcmd)
	case tea.KeyMsg:
		k := strings.ToLower(ev.String())
		if k == "pgup" || k == "pgdown" {
			m.viewport, vpcmd = m.viewport.Update(msg)
			cmds = append(cmds, vpcmd)
		}
		// Do not forward arrow keys/home/end to viewport; REPL handles them
	default:
		// Do not forward other messages to viewport to avoid stealing navigation keys
	}
	return m, tea.Batch(append(cmds, waitForUIEvent(m.uiEvents), waitForToolReq(m.toolReqCh))...)
}

func (m AppModel) View() string {
	if m.activeForm != nil {
		return safeFormView(m.activeForm)
	}
	// Build status bar: mode / run / turn
	var statusLeft string
	if m.currentMode != "" {
		statusLeft = "mode:" + m.currentMode
	} else {
		statusLeft = "mode:?"
	}
	if m.runID != "" {
		statusLeft += "  run:" + m.runID
	}
	if m.turnID != "" {
		statusLeft += "  turn:" + m.turnID
	}

	var header string
	if m.status != "" || m.isStreaming {
		header = headerStyle.Render(statusLeft + "  |  " + m.status)
		header += " " + m.spinner.View()
		top := header
		if m.toolEvents != "" {
			top = top + "\n" + m.toolEvents
		}
		if m.live != "" {
			return m.renderLayout(top+"\n"+jsonStyle.Render(m.live), m.viewport.View())
		}
		return m.renderLayout(top, m.viewport.View())
	}
	if m.live != "" || m.toolEvents != "" {
		top := headerStyle.Render(statusLeft)
		if m.toolEvents != "" {
			top += "\n" + m.toolEvents
		}
		if m.live != "" {
			if top != "" {
				top += "\n"
			}
			top += jsonStyle.Render(m.live)
		}
		return m.renderLayout(top, m.viewport.View())
	}
	return m.renderLayout(headerStyle.Render(statusLeft), m.viewport.View())
}

func (m AppModel) renderLayout(top string, main string) string {
	left := main
	if top != "" {
		left = top + "\n" + main
	}
	if !m.showSidebar || m.rightWidth <= 0 {
		return lipgloss.NewStyle().Width(m.leftWidth).Render(left)
	}
	leftView := lipgloss.NewStyle().Width(m.leftWidth).Render(left)
	rightView := m.sidebar.View()
	return lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
}

// renderToolEvents composes a compact, single-line-per-call view across provider call, local exec, and results.
func (m *AppModel) renderToolEvents() {
	if len(m.toolEntries) == 0 {
		m.toolEvents = ""
		return
	}
	log.Debug().Int("entries", len(m.toolEntries)).Msg("UI: renderToolEvents")
	var out []string
	for _, e := range m.toolEntries {
		if e.Name == "" && e.ID == "" {
			continue
		}
		name := e.Name
		if name == "" {
			name = e.ID
		}
		parts := []string{}
		if e.ProviderCalled {
			parts = append(parts, toolCallStyle.Render("→ "+name))
		}
		if e.ExecStarted {
			parts = append(parts, toolCallStyle.Render("↳ exec"))
		}
		if e.Result != "" {
			parts = append(parts, toolResultStyle.Render("← ")+jsonStyle.Render(e.Result))
		}
		if e.Input != "" {
			parts = append(parts, jsonStyle.Render(e.Input))
		}
		out = append(out, strings.Join(parts, "  "))
	}
	m.toolEvents = strings.Join(out, "\n")
}

// addCoalescedToolLineToRepl pushes the aggregated tool entry for the given ID into the REPL history.
func (m *AppModel) addCoalescedToolLineToRepl(id string) {
	idx, ok := m.toolEntryIndex[id]
	if !ok || idx < 0 || idx >= len(m.toolEntries) {
		log.Debug().Str("id", id).Msg("UI: addCoalescedToolLineToRepl missing entry")
		return
	}
	e := m.toolEntries[idx]
	name := e.Name
	if name == "" {
		name = e.ID
	}
	parts := []string{"→ " + name}
	if e.ExecStarted {
		parts = append(parts, "↳ exec")
	}
	if e.Result != "" {
		parts = append(parts, "← "+e.Result)
	}
	if e.Input != "" {
		parts = append(parts, e.Input)
	}
	line := strings.Join(parts, "  ")
	log.Debug().Str("id", id).Str("line", line).Msg("UI: addCoalescedToolLineToRepl")
	// History API changed in bobatea REPL; skip injecting as REPL history entry
}

// hasPendingTools returns true if there are tool entries without a final result
func (m *AppModel) hasPendingTools() bool {
	for _, e := range m.toolEntries {
		if (e.ProviderCalled || e.ExecStarted) && e.Result == "" {
			return true
		}
	}
	return false
}

// safeFormView wraps huh.Form.View() to avoid panics from internal selector when options are empty
func safeFormView(f *huh.Form) string {
	defer func() {
		_ = recover()
	}()
	return f.View()
}

// safeFormUpdate wraps huh.Form.Update() to recover from internal panics
// (e.g., selector with empty options). It returns the original form and
// a nil command if a panic occurs, allowing the UI loop to continue.
func safeFormUpdate(f *huh.Form, msg tea.Msg) (tea.Model, tea.Cmd) {
	var model tea.Model
	var cmd tea.Cmd
	// Run update inside a closure so we can recover without named returns
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Warn().Interface("recover", r).Msg("huh.Form.Update panic recovered")
				model = f
				cmd = nil
			}
		}()
		model, cmd = f.Update(msg)
	}()
	return model, cmd
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
