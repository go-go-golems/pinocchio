package xevents

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/geppetto/pkg/events"
)

var (
	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	subHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	toolNameStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	jsonStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	deltaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	finalStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("118"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

// AddPrettyHandlers prints a nice stream of events to a writer (optional utility).
func AddPrettyHandlers(router *events.EventRouter, w io.Writer) {
	router.AddHandler("pretty", "chat", func(msg *message.Message) error {
		defer msg.Ack()
		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			return err
		}
		switch ev := e.(type) {
		case *events.EventTextSegmentStarted:
			fmt.Fprintln(w, headerStyle.Render("— Text segment started —"))
		case *events.EventTextDelta:
			if ev.Delta != "" {
				_, _ = fmt.Fprint(w, deltaStyle.Render(ev.Delta))
			}
		case *events.EventTextSegmentFinished:
			if ev.Text != "" {
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, finalStyle.Render("— Text segment finished —"))
			}
		case *events.EventToolCallRequested:
			inputJSON := ev.Input
			if s := strings.TrimSpace(inputJSON); strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
				var tmp interface{}
				if err := json.Unmarshal([]byte(inputJSON), &tmp); err == nil {
					if b, err := json.MarshalIndent(tmp, "", "  "); err == nil {
						inputJSON = string(b)
					}
				}
			}
			block := []string{
				subHeaderStyle.Render("Tool Call Requested:"),
				toolNameStyle.Render(fmt.Sprintf("Name: %s", ev.ToolName)),
				jsonStyle.Render(fmt.Sprintf("ID: %s", ev.ToolCallID)),
				jsonStyle.Render(inputJSON),
			}
			fmt.Fprintln(w, strings.Join(block, "\n"))
		case *events.EventToolExecutionStarted:
			inputJSON := ev.Input
			if s := strings.TrimSpace(inputJSON); strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
				var tmp interface{}
				if err := json.Unmarshal([]byte(inputJSON), &tmp); err == nil {
					if b, err := json.MarshalIndent(tmp, "", "  "); err == nil {
						inputJSON = string(b)
					}
				}
			}
			block := []string{
				subHeaderStyle.Render("Tool Execute:"),
				toolNameStyle.Render(fmt.Sprintf("Name: %s", ev.ToolName)),
				jsonStyle.Render(fmt.Sprintf("ID: %s", ev.ToolCallID)),
				jsonStyle.Render(inputJSON),
			}
			fmt.Fprintln(w, strings.Join(block, "\n"))
		case *events.EventToolResultReady:
			resultJSON := ev.Result
			if s := strings.TrimSpace(resultJSON); strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
				var tmp interface{}
				if err := json.Unmarshal([]byte(resultJSON), &tmp); err == nil {
					if b, err := json.MarshalIndent(tmp, "", "  "); err == nil {
						resultJSON = string(b)
					}
				}
			}
			block := []string{
				subHeaderStyle.Render("Tool Result:"),
				toolNameStyle.Render(fmt.Sprintf("ID: %s", ev.ToolCallID)),
				jsonStyle.Render(resultJSON),
			}
			fmt.Fprintln(w, strings.Join(block, "\n"))
		case *events.EventError:
			fmt.Fprintln(w, errorStyle.Render("Error: ")+ev.ErrorString)
		case *events.EventInterrupt:
			fmt.Fprintln(w, errorStyle.Render("Interrupted"))
		case *events.EventLog:
			lvl := ev.Level
			if lvl == "" {
				lvl = "info"
			}
			fmt.Fprintln(w, subHeaderStyle.Render(fmt.Sprintf("[%s] %s", strings.ToUpper(lvl), ev.Message)))
			if len(ev.Fields) > 0 {
				b, _ := json.MarshalIndent(ev.Fields, "", "  ")
				fmt.Fprintln(w, jsonStyle.Render(string(b)))
			}
		case *events.EventInfo:
			fmt.Fprintln(w, subHeaderStyle.Render(fmt.Sprintf("[i] %s", ev.Message)))
			if len(ev.Data) > 0 {
				b, _ := json.MarshalIndent(ev.Data, "", "  ")
				fmt.Fprintln(w, jsonStyle.Render(string(b)))
			}
		}
		return nil
	})
}

// AddUIForwarder forwards all chat events into a channel consumed by the Bubble Tea model.
func AddUIForwarder(router *events.EventRouter, ch chan<- interface{}) {
	router.AddHandler("ui-forwarder", "chat", func(msg *message.Message) error {
		defer msg.Ack()
		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			return err
		}
		select {
		case ch <- e:
		default:
			// drop if channel is full to avoid blocking
		}
		return nil
	})
}
