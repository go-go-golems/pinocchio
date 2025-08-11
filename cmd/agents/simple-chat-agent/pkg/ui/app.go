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
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	jsonStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
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

    // Tool call/result short log shown above the REPL (cleared on final)
    toolEvents string

	// Sidebar (toggle with Ctrl+G)
	showSidebar bool
	sidebar     SidebarModel

	// Layout
	totalWidth  int
	totalHeight int
	leftWidth   int
	rightWidth  int
}

func NewAppModel(uiCh <-chan interface{}, replModel repl.Model, toolReqCh <-chan toolspkg.ToolUIRequest) AppModel {
	sp := bspinner.New()
	sp.Spinner = bspinner.Line
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	vp := viewport.New(0, 0)
	vp.Style = replContainerStyle
	return AppModel{
		spinner:   sp,
		uiEvents:  uiCh,
		repl:      replModel,
		viewport:  vp,
		toolReqCh: toolReqCh,
		sidebar:   NewSidebarModel(),
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
		m.repl.SetWidth(maxInt(0, m.leftWidth-2))
		// Leave room for header line(s)
		m.viewport.Width = maxInt(0, m.leftWidth)
		m.viewport.Height = maxInt(0, ev.Height-3)
		m.viewport.SetContent(m.repl.View())
		if m.rightWidth > 0 {
			m.sidebar, _ = m.sidebar.Update(SetSidebarSizeMsg{Width: m.rightWidth})
		}
		return m, nil
	case tea.KeyMsg:
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
			m.repl.SetWidth(maxInt(0, m.leftWidth-2))
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
        // Append compact line to toolEvents
        line := toolCallStyle.Render("→ " + ev.ToolCall.Name)
        if ev.ToolCall.Input != "" {
            line += " " + jsonStyle.Render(ev.ToolCall.Input)
        }
        if m.toolEvents != "" {
            m.toolEvents += "\n"
        }
        m.toolEvents += line
        // Also interleave into REPL history
        rm := m.repl
        rm.GetHistory().Add("[tool] "+ev.ToolCall.Name, ev.ToolCall.Input, false)
        m.sidebar, _ = m.sidebar.Update(ev)
        return m, waitForUIEvent(m.uiEvents)
	case *events.EventToolCallExecute:
        m.status = "Executing: " + ev.ToolCall.Name
        line := toolCallStyle.Render("↳ exec " + ev.ToolCall.Name)
        if ev.ToolCall.Input != "" {
            line += " " + jsonStyle.Render(ev.ToolCall.Input)
        }
        if m.toolEvents != "" { m.toolEvents += "\n" }
        m.toolEvents += line
        // Interleave into REPL history
        rm := m.repl
        rm.GetHistory().Add("[tool-exec] "+ev.ToolCall.Name, ev.ToolCall.Input, false)
        return m, waitForUIEvent(m.uiEvents)
	case *events.EventToolResult:
        m.status = ""
        res := ev.ToolResult.Result
        line := toolResultStyle.Render("← result ") + jsonStyle.Render(res)
        if m.toolEvents != "" { m.toolEvents += "\n" }
        m.toolEvents += line
        // Interleave into REPL history
        rm := m.repl
        rm.GetHistory().Add("[tool-result] "+ev.ToolResult.ID, res, false)
        m.sidebar, _ = m.sidebar.Update(ev)
        return m, waitForUIEvent(m.uiEvents)
	case *events.EventToolCallExecutionResult:
        m.status = ""
        res := ev.ToolResult.Result
        line := toolResultStyle.Render("↳ exec result ") + jsonStyle.Render(res)
        if m.toolEvents != "" { m.toolEvents += "\n" }
        m.toolEvents += line
        // Interleave into REPL history
        rm := m.repl
        rm.GetHistory().Add("[tool-exec-result] "+ev.ToolResult.ID, res, false)
        m.sidebar, _ = m.sidebar.Update(ev)
        return m, waitForUIEvent(m.uiEvents)
	case *events.EventFinal:
		m.isStreaming = false
        m.live = ""
        m.toolEvents = ""
		m.status = ""
		return m, nil
	case *events.EventError:
		m.isStreaming = false
        m.live = ""
        m.toolEvents = ""
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
	// Update viewport with latest content and pass through events for scrolling
	m.viewport.SetContent(m.repl.View())
	var vpcmd tea.Cmd
	m.viewport, vpcmd = m.viewport.Update(msg)
	cmds = append(cmds, vpcmd)
	return m, tea.Batch(append(cmds, waitForUIEvent(m.uiEvents), waitForToolReq(m.toolReqCh))...)
}

func (m AppModel) View() string {
	if m.activeForm != nil {
		return safeFormView(m.activeForm)
	}
	var header string
    if m.status != "" || m.isStreaming {
        header = headerStyle.Render(m.status)
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
        top := m.toolEvents
        if m.live != "" {
            if top != "" { top += "\n" }
            top += jsonStyle.Render(m.live)
        }
        return m.renderLayout(top, m.viewport.View())
    }
    return m.renderLayout("", m.viewport.View())
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

// safeFormView wraps huh.Form.View() to avoid panics from internal selector when options are empty
func safeFormView(f *huh.Form) string {
	defer func() {
		_ = recover()
	}()
	return f.View()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
