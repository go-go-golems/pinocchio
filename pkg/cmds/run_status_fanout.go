package cmds

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

type runStatusFanout struct {
	target sessionstream.UIFanout

	mu      sync.Mutex
	status  string
	errText string
}

var _ sessionstream.UIFanout = (*runStatusFanout)(nil)

func newRunStatusFanout(target sessionstream.UIFanout) *runStatusFanout {
	return &runStatusFanout{target: target}
}

func (f *runStatusFanout) PublishUI(ctx context.Context, sid sessionstream.SessionId, ord uint64, events []sessionstream.UIEvent) error {
	if f == nil {
		return fmt.Errorf("run status fanout is not initialized")
	}
	f.record(events)
	if f.target == nil {
		return nil
	}
	return f.target.PublishUI(ctx, sid, ord, events)
}

func (f *runStatusFanout) record(events []sessionstream.UIEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ev := range events {
		switch p := ev.Payload.(type) {
		case *chatappv1.ChatRunFinished:
			f.status = firstNonEmptyString(p.GetStatus(), "ok")
			f.errText = ""
		case *chatappv1.ChatRunStopped:
			f.status = firstNonEmptyString(p.GetStatus(), "stopped")
			f.errText = ""
		case *chatappv1.ChatRunFailed:
			f.status = firstNonEmptyString(p.GetStatus(), "failed")
			f.errText = firstNonEmptyString(p.GetError(), "chat run failed")
		default:
			switch ev.Name {
			case chatapp.EventChatRunFinished:
				f.status = "ok"
				f.errText = ""
			case chatapp.EventChatRunStopped:
				f.status = "stopped"
				f.errText = ""
			case chatapp.EventChatRunFailed:
				f.status = "failed"
				if strings.TrimSpace(f.errText) == "" {
					f.errText = "chat run failed"
				}
			}
		}
	}
}

func (f *runStatusFanout) Result() (string, error) {
	if f == nil {
		return "", nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	status := strings.TrimSpace(f.status)
	if status == "" || status == "finished" {
		status = "ok"
	}
	if status == "failed" {
		return status, fmt.Errorf("%s", firstNonEmptyString(f.errText, "chat run failed"))
	}
	return status, nil
}
