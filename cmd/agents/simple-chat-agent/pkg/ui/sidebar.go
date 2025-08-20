package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/geppetto/pkg/events"
	toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
)

var sidebarTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
var sidebarBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("240")).
	Padding(0, 1)

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
	width int
	// Computation tool mini-log
	computations  []ComputationRecord
	compIndexByID map[string]int

	// Agent mode state and history
	currentMode        string
	currentExplanation string
	modeHistory        []ModeSwitch
}

type ModeSwitch struct {
	From     string
	To       string
	Analysis string
	At       time.Time
}

func NewSidebarModel() SidebarModel {
	return SidebarModel{
		width:         28,
		computations:  make([]ComputationRecord, 0, 32),
		compIndexByID: make(map[string]int),
		currentMode:   "",
		modeHistory:   make([]ModeSwitch, 0, 16),
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
			var r struct {
				Result float64 `json:"result"`
			}
			if err := json.Unmarshal([]byte(ev.ToolResult.Result), &r); err == nil {
				m.computations[idx].Result = &r.Result
			}
		}
		return m, nil
	case *events.EventToolCallExecutionResult:
		if idx, ok := m.compIndexByID[ev.ToolResult.ID]; ok {
			var r struct {
				Result float64 `json:"result"`
			}
			if err := json.Unmarshal([]byte(ev.ToolResult.Result), &r); err == nil {
				m.computations[idx].Result = &r.Result
			}
		}
		return m, nil
	case *events.EventLog:
		// Capture initial mode insertion log
		if strings.HasPrefix(ev.Message, "agentmode:") {
			if ev.Message == "agentmode: user prompt inserted" {
				if mode, ok := ev.Fields["mode"].(string); ok && mode != "" {
					m.currentMode = mode
				}
				if prompt, ok := ev.Fields["prompt"].(string); ok && prompt != "" {
					m.currentExplanation = prompt
				}
			}
		}
		return m, nil
	case *events.EventInfo:
		// Capture switch events with from/to/analysis
		if ev.Message == "agentmode: mode switched" {
			from, _ := ev.Data["from"].(string)
			to, _ := ev.Data["to"].(string)
			analysis, _ := ev.Data["analysis"].(string)
			if to != "" {
				m.modeHistory = append(m.modeHistory, ModeSwitch{From: from, To: to, Analysis: analysis, At: time.Now()})
				m.currentMode = to
				if analysis != "" {
					m.currentExplanation = analysis
				}
			}
		}
		return m, nil
	}
	return m, nil
}

func (m SidebarModel) View() string {
	// Always render a titled, bordered sidebar even if empty
	var out string
	// Agent mode section
	titleMode := sidebarTitleStyle.Render("Agent Mode (Ctrl+T)")
	curr := m.currentMode
	if curr == "" {
		curr = "(unknown)"
	}
	out += titleMode + "\nCurrent: " + curr + "\n"
	if m.currentExplanation != "" {
		expl := m.currentExplanation
		if len(expl) > 96 {
			expl = expl[:96] + "…"
		}
		out += "Why: " + expl + "\n"
	}
	// History block (may be empty)
	out += "History:\n"
	if len(m.modeHistory) > 0 {
		start := 0
		if len(m.modeHistory) > 8 {
			start = len(m.modeHistory) - 8
		}
		for _, h := range m.modeHistory[start:] {
			line := fmt.Sprintf("%s → %s", h.From, h.To)
			if h.Analysis != "" {
				s := h.Analysis
				if len(s) > 48 {
					s = s[:48] + "…"
				}
				line += " — " + s
			}
			out += line + "\n"
		}
	} else {
		out += "(none)\n"
	}
	out += "\n"

	// Computations block (may be empty)
	out += sidebarTitleStyle.Render("Computations") + "\n"
	if len(m.computations) == 0 {
		out += "No computations yet\n"
	} else {
		for _, c := range m.computations {
			line := fmt.Sprintf("%s: %.2f %s %.2f", c.ID, c.A, c.Op, c.B)
			if c.Result != nil {
				line += fmt.Sprintf(" = %.4f", *c.Result)
			} else {
				line += " (running)"
			}
			out += line + "\n"
		}
	}
	return sidebarBoxStyle.Width(m.width).Render(out)
}
