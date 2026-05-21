package cmds

import (
	"fmt"
	"io"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	"gopkg.in/yaml.v3"
)

func pinocchioStepPrinterFunc(name string, w io.Writer) func(msg *message.Message) error {
	isFirst := true

	return func(msg *message.Message) error {
		defer msg.Ack()

		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			return err
		}

		switch p := e.(type) {
		case *events.EventError:
			_, writeErr := fmt.Fprintf(w, "\n[error] %s\n", p.ErrorString)
			return writeErr
		case *events.EventTextDelta:
			if isFirst && name != "" {
				isFirst = false
				if _, err := fmt.Fprintf(w, "\n%s: \n", name); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(w, "%s", p.Delta)
			return err
		case *events.EventReasoningDelta:
			_, err = fmt.Fprintf(w, "%s", p.Delta)
			return err
		case *events.EventTextSegmentFinished:
			if !strings.HasSuffix(p.Text, "\n") {
				_, err = fmt.Fprintln(w)
				return err
			}
		case *events.EventToolCallRequested:
			v, err := yaml.Marshal(map[string]any{"id": p.ToolCallID, "name": p.ToolName, "input": p.Input})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(w, "%s\n", v)
			return err
		case *events.EventToolResultReady:
			v, err := yaml.Marshal(map[string]any{"id": p.ToolCallID, "name": p.ToolName, "result": p.Result, "status": p.Status})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(w, "%s\n", v)
			return err
		case *events.EventLog:
			level := p.Level
			if level == "" {
				level = "info"
			}
			if _, err := fmt.Fprintf(w, "\n[%s] %s\n", level, p.Message); err != nil {
				return err
			}
			if len(p.Fields) > 0 {
				v, err := yaml.Marshal(p.Fields)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(w, "%s\n", v)
				return err
			}
		case *events.EventInfo:
			return printInfoEvent(w, p)
		case *events.EventWebSearchStarted:
			if p.Query != "" {
				_, err = fmt.Fprintf(w, "\n🔎 Searching: %s\n", p.Query)
			} else {
				_, err = fmt.Fprintln(w, "\n🔎 Searching...")
			}
			return err
		case *events.EventWebSearchSearching:
			_, err = fmt.Fprintln(w, "… searching")
			return err
		case *events.EventWebSearchOpenPage:
			if p.URL != "" {
				_, err = fmt.Fprintf(w, "🌐 Open: %s\n", p.URL)
				return err
			}
		case *events.EventWebSearchDone:
			_, err = fmt.Fprintln(w, "✅ Search done")
			return err
		case *events.EventCitation:
			if p.Title != "" || p.URL != "" {
				_, err = fmt.Fprintf(w, "📎 %s - %s\n", p.Title, p.URL)
				return err
			}
		case *events.EventProviderCallStarted, *events.EventInterrupt:
		}

		return nil
	}
}

func printInfoEvent(w io.Writer, p *events.EventInfo) error {
	switch p.Message {
	case "thinking-started", "reasoning-summary-started":
		_, err := fmt.Fprintln(w, "\n--- Thinking started ---")
		return err
	case "thinking-ended", "reasoning-summary-ended":
		_, err := fmt.Fprintln(w, "\n--- Thinking ended ---")
		return err
	case "output-started":
		_, err := fmt.Fprintln(w, "\n--- Output started ---")
		return err
	case "output-ended":
		_, err := fmt.Fprintln(w, "\n--- Output ended ---")
		return err
	case "reasoning-summary-delta", "reasoning-summary":
		return nil
	default:
		_, err := fmt.Fprintf(w, "\n[i] %s\n", p.Message)
		return err
	}
}
