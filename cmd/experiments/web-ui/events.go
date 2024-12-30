package main

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
)

// EventTemplateData holds the data for event templates
type EventTemplateData struct {
	Timestamp  string
	Completion string
	Text       string
	Name       string
	Input      string
	Result     string
	Error      string
}

// EventToHTML converts different event types to HTML snippets using templates
func EventToHTML(tmpl *template.Template, e chat.Event) (string, error) {
	data := EventTemplateData{
		Timestamp: time.Now().Format("15:04:05"),
	}

	var templateName string

	switch e_ := e.(type) {
	case *chat.EventPartialCompletionStart:
		templateName = "event-start"

	case *chat.EventPartialCompletion:
		templateName = "event-partial"
		data.Completion = e_.Completion

	case *chat.EventFinal:
		templateName = "event-final"
		data.Text = e_.Text

	case *chat.EventToolCall:
		templateName = "event-tool-call"
		data.Name = e_.ToolCall.Name
		data.Input = e_.ToolCall.Input

	case *chat.EventToolResult:
		templateName = "event-tool-result"
		data.Result = e_.ToolResult.Result

	case *chat.EventError:
		templateName = "event-error"
		data.Error = e_.Error().Error()

	case *chat.EventInterrupt:
		templateName = "event-interrupt"
		data.Text = e_.Text

	default:
		return "", fmt.Errorf("unknown event type: %T", e)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}
