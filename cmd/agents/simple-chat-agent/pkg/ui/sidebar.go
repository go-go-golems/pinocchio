package ui

import (
    "encoding/json"
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/go-go-golems/geppetto/pkg/events"
    toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
)

// SetSidebarSizeMsg informs the sidebar of its width
type SetSidebarSizeMsg struct {
    Width int
}

type ComputationRecord struct {
    ID     string
    A      float64
    B      float64
    Op     string
    Result *float64
}

type SidebarModel struct {
    width         int
    computations  []ComputationRecord
    compIndexByID map[string]int
}

func NewSidebarModel() SidebarModel {
    return SidebarModel{
        width:         24,
        computations:  make([]ComputationRecord, 0, 32),
        compIndexByID: make(map[string]int),
    }
}

func (m SidebarModel) Init() tea.Cmd { return nil }

func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
    switch ev := msg.(type) {
    case SetSidebarSizeMsg:
        if ev.Width > 0 {
            m.width = ev.Width
        }
        return m, nil
    case *events.EventToolCall:
        if ev.ToolCall.Name == "calc" {
            var req toolspkg.CalcRequest
            _ = json.Unmarshal([]byte(ev.ToolCall.Input), &req)
            rec := ComputationRecord{ID: ev.ToolCall.ID, A: req.A, B: req.B, Op: req.Op}
            m.compIndexByID[ev.ToolCall.ID] = len(m.computations)
            m.computations = append(m.computations, rec)
        }
        return m, nil
    case *events.EventToolResult:
        if idx, ok := m.compIndexByID[ev.ToolResult.ID]; ok {
            var r struct{ Result float64 `json:"result"` }
            if err := json.Unmarshal([]byte(ev.ToolResult.Result), &r); err == nil {
                m.computations[idx].Result = &r.Result
            }
        }
        return m, nil
    case *events.EventToolCallExecutionResult:
        if idx, ok := m.compIndexByID[ev.ToolResult.ID]; ok {
            var r struct{ Result float64 `json:"result"` }
            if err := json.Unmarshal([]byte(ev.ToolResult.Result), &r); err == nil {
                m.computations[idx].Result = &r.Result
            }
        }
        return m, nil
    }
    return m, nil
}

func (m SidebarModel) View() string {
    title := subHeaderStyle.Render("Computations (Ctrl+G)")
    if len(m.computations) == 0 {
        return lipgloss.NewStyle().Width(m.width).Render(title + "\nNo computations yet")
    }
    var out string
    out += title + "\n"
    for _, c := range m.computations {
        line := fmt.Sprintf("%s: %.2f %s %.2f", c.ID, c.A, c.Op, c.B)
        if c.Result != nil {
            line += fmt.Sprintf(" = %.4f", *c.Result)
        } else {
            line += " (running)"
        }
        out += line + "\n"
    }
    return lipgloss.NewStyle().Width(m.width).Render(out)
}


