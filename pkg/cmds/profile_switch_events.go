package cmds

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/google/uuid"
)

func systemNoticeEntityCmd(text string) tea.Cmd {
	id := uuid.NewString()
	now := time.Now()
	created := func() tea.Msg {
		return timeline.UIEntityCreated{
			ID:        timeline.EntityID{LocalID: id, Kind: "llm_text"},
			Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
			Props:     map[string]any{"role": "assistant", "text": strings.TrimSpace(text), "streaming": false},
			StartedAt: now,
		}
	}
	completed := func() tea.Msg {
		return timeline.UIEntityCompleted{
			ID:     timeline.EntityID{LocalID: id, Kind: "llm_text"},
			Result: map[string]any{"text": strings.TrimSpace(text)},
		}
	}
	return tea.Batch(created, completed)
}

func publishProfileSwitchedInfo(sink events.EventSink, convID, from, to string) error {
	if sink == nil {
		return nil
	}
	md := events.EventMetadata{
		ID: uuid.New(),
		Extra: map[string]any{
			"conversation_id": strings.TrimSpace(convID),
			"runtime_key":     strings.TrimSpace(to),
			"profile.slug":    strings.TrimSpace(to),
		},
	}
	return sink.PublishEvent(events.NewInfoEvent(md, "profile-switched", map[string]any{
		"from": strings.TrimSpace(from),
		"to":   strings.TrimSpace(to),
	}))
}
