package agentd

import (
	"context"
	"fmt"
	"sync"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// Usage counts tokens; Rounds counts provider calls.
type Usage struct {
	Input         int      `json:"input"`
	Output        int      `json:"output"`
	Cached        int      `json:"cached,omitempty"`
	CacheCreation int      `json:"cacheCreation,omitempty"`
	CacheRead     int      `json:"cacheRead,omitempty"`
	Rounds        int      `json:"rounds"`
	USD           *float64 `json:"usd,omitempty"`
}

func (u *Usage) add(v *gepevents.Usage) {
	u.Rounds++
	if v == nil {
		return
	}
	u.Input += v.InputTokens
	u.Output += v.OutputTokens
	u.Cached += v.CachedTokens
	u.CacheCreation += v.CacheCreationInputTokens
	u.CacheRead += v.CacheReadInputTokens
}

// runSink turns geppetto inference events of one run into wire events. Text
// of all segments is joined with a blank line, so `text` is always the whole
// answer of the run so far (the page assigns it).
type runSink struct {
	mu        sync.Mutex
	s         *Session
	run       string
	text      string
	reasoning string
	segments  int
}

var _ gepevents.EventSink = (*runSink)(nil)

func (k *runSink) PublishEvent(ev gepevents.Event) error {
	switch e := ev.(type) {
	case *gepevents.EventTextSegmentStarted:
		k.mu.Lock()
		if k.text != "" {
			k.text += "\n\n"
		}
		k.segments++
		k.mu.Unlock()
	case *gepevents.EventTextDelta:
		if e.Delta == "" {
			return nil
		}
		k.mu.Lock()
		k.text += e.Delta
		text := k.text
		k.mu.Unlock()
		k.s.log.Append(EvTextDelta, k.run, map[string]any{"delta": e.Delta, "text": text})
	case *gepevents.EventReasoningDelta:
		if e.Delta == "" {
			return nil
		}
		k.mu.Lock()
		k.reasoning += e.Delta
		k.mu.Unlock()
		k.s.log.Append(EvReasoningDelta, k.run, map[string]any{"delta": e.Delta})
	case *gepevents.EventProviderCallFinished:
		round, total := k.s.addUsage(e.Usage)
		k.s.log.Append(EvUsage, k.run, map[string]any{"round": round, "total": total, "stopReason": e.StopReason})
	}
	return nil
}

func (k *runSink) answer() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.text
}

// toolPublisher receives the frontendtools.Manager's events for one session
// and records them as wire events. It implements sessionstream.EventPublisher.
type toolPublisher struct{ s *Session }

var _ sessionstream.EventPublisher = toolPublisher{}

func (p toolPublisher) Publish(_ context.Context, ev sessionstream.Event) error {
	switch m := ev.Payload.(type) {
	case *toolv1.FrontendToolCallRequested:
		mode := "auto"
		if m.GetMode() == toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_HUMAN {
			mode = "human"
		}
		input := map[string]any{}
		if m.GetInput() != nil {
			input = m.GetInput().AsMap()
		}
		p.s.log.Append(EvToolRequested, m.GetMessageId(), map[string]any{
			"callId": m.GetToolCallId(), "name": m.GetToolName(), "input": input, "mode": mode,
		})
	case *toolv1.FrontendToolResultReceived:
		p.s.log.Append(EvToolFinished, m.GetMessageId(), map[string]any{
			"callId": m.GetToolCallId(), "name": m.GetToolName(), "status": m.GetStatus(), "error": m.GetError(),
		})
	case *toolv1.FrontendToolManifestUpdated:
		names := []string{}
		for _, t := range m.GetTools() {
			names = append(names, t.GetName())
		}
		p.s.log.Append(EvToolsManifest, "", map[string]any{"revision": m.GetRevision(), "tools": names, "executor": executorJSON(m.GetExecutor())})
	default:
		if ev.Name != frontendtools.EventManifestUpdated {
			return fmt.Errorf("agentd: unexpected tool event %s (%T)", ev.Name, ev.Payload)
		}
	}
	return nil
}

func executorJSON(e *toolv1.FrontendToolExecutor) map[string]string {
	if e == nil {
		return nil
	}
	return map[string]string{"clientInstanceId": e.GetClientInstanceId(), "connectionId": e.GetConnectionId(), "assignmentId": e.GetAssignmentId()}
}
