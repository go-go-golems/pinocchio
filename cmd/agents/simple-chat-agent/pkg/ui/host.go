package ui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/go-go-golems/geppetto/pkg/events"
)

// HostModel composes an inner model (timeline+input) with a toggleable sidebar.
// Toggle the sidebar with Ctrl+T. The sidebar shows agent mode and calculator history.
type HostModel struct {
    inner      tea.Model
    uiEvents   <-chan interface{}
    showSidebar bool
    sidebar     SidebarModel

    totalWidth  int
    totalHeight int
    leftWidth   int
    rightWidth  int
}

func NewHostModel(inner tea.Model, uiCh <-chan interface{}) HostModel {
    return HostModel{
        inner:      inner,
        uiEvents:   uiCh,
        sidebar:    NewSidebarModel(),
        showSidebar: false,
    }
}

func (m HostModel) Init() tea.Cmd {
    return tea.Batch(m.inner.Init(), m.waitForUIEvent())
}

func (m HostModel) waitForUIEvent() tea.Cmd {
    return func() tea.Msg {
        if m.uiEvents == nil {
            return nil
        }
        e, ok := <-m.uiEvents
        if !ok {
            return nil
        }
        return e
    }
}

func (m HostModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Optionally transform messages before forwarding to inner
    forwardMsg := msg
    // Update sidebar state based on incoming events and sizing/keys
    switch ev := msg.(type) {
    case tea.WindowSizeMsg:
        m.totalWidth = ev.Width
        m.totalHeight = ev.Height
        if m.showSidebar {
            desiredRightTotal := int(float64(ev.Width) * 0.25)
            if desiredRightTotal < 24 {
                desiredRightTotal = 24
            }
            if desiredRightTotal > ev.Width/2 {
                desiredRightTotal = ev.Width / 2
            }
            m.rightWidth = maxInt(0, desiredRightTotal-4)
        } else {
            m.rightWidth = 0
        }
        m.leftWidth = ev.Width - (m.rightWidth + 4)
        if m.leftWidth < 0 {
            m.leftWidth = 0
        }
        if m.rightWidth > 0 {
            m.sidebar, _ = m.sidebar.Update(SetSidebarSizeMsg{Width: m.rightWidth})
        }
        // Forward a resized WindowSizeMsg to the inner model so it renders within the left pane.
        // Reserve a couple lines for the bottom help widget from the inner chat UI.
        reservedBottom := 2
        innerHeight := ev.Height - reservedBottom
        if innerHeight < 1 {
            innerHeight = 1
        }
        forwardMsg = tea.WindowSizeMsg{Width: m.leftWidth, Height: innerHeight}
    case tea.KeyMsg:
        if ev.String() == "ctrl+t" {
            m.showSidebar = !m.showSidebar
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
            if m.rightWidth > 0 {
                m.sidebar, _ = m.sidebar.Update(SetSidebarSizeMsg{Width: m.rightWidth})
            }
            // Force inner to recompute layout under new left width/height
            reservedBottom := 2
            innerHeight := m.totalHeight - reservedBottom
            if innerHeight < 1 { innerHeight = 1 }
            forwardMsg = tea.WindowSizeMsg{Width: m.leftWidth, Height: innerHeight}
        }
    case *events.EventLog, *events.EventInfo, *events.EventToolCall, *events.EventToolResult, *events.EventToolCallExecutionResult:
        m.sidebar, _ = m.sidebar.Update(ev)
        // keep forwarding to inner too
    }

    // Always forward message to inner model
    innerModel, cmd := m.inner.Update(forwardMsg)
    m.inner = innerModel
    return m, tea.Batch(cmd, m.waitForUIEvent())
}

func (m HostModel) View() string {
    // Compose the inner view with an optional sidebar
    leftView := lipgloss.NewStyle().Width(m.leftWidth).Render(m.inner.View())
    if !m.showSidebar || m.rightWidth <= 0 {
        return leftView
    }
    rightView := m.sidebar.View()
    return lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
}


