package chatapp_test

import (
	"context"
	"sync"
	"testing"
	"time"

	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/plugins"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

type recordingFanout struct {
	mu      sync.Mutex
	batches []recordedBatch
}

type recordedBatch struct {
	sid    sessionstream.SessionId
	ord    uint64
	events []sessionstream.UIEvent
}

func (f *recordingFanout) PublishUI(_ context.Context, sid sessionstream.SessionId, ord uint64, events []sessionstream.UIEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]sessionstream.UIEvent, len(events))
	copy(copied, events)
	f.batches = append(f.batches, recordedBatch{sid: sid, ord: ord, events: copied})
	return nil
}

func (f *recordingFanout) eventNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var names []string
	for _, batch := range f.batches {
		for _, ev := range batch.events {
			names = append(names, ev.Name)
		}
	}
	return names
}

func TestRunnerSubmitPromptWaitIdleAndSnapshot(t *testing.T) {
	fanout := &recordingFanout{}
	runner, err := chatapp.NewRunner(chatapp.RunnerOptions{UIFanout: fanout, ChunkDelay: time.Nanosecond})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	defer func() { _ = runner.Close() }()

	ctx := context.Background()
	const sid sessionstream.SessionId = "session-1"
	if err := runner.Service.SubmitPrompt(ctx, sid, "hello runner"); err != nil {
		t.Fatalf("SubmitPrompt: %v", err)
	}
	if err := runner.Service.WaitIdle(ctx, sid); err != nil {
		t.Fatalf("WaitIdle: %v", err)
	}

	names := fanout.eventNames()
	for _, want := range []string{chatapp.EventUserMessageAccepted, chatapp.EventChatTextPatch, chatapp.EventChatTextSegmentFinished, chatapp.EventChatRunFinished} {
		if !contains(names, want) {
			t.Fatalf("expected fanout event %s in %v", want, names)
		}
	}

	snap, err := runner.Service.Snapshot(ctx, sid)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.SessionId != sid {
		t.Fatalf("unexpected snapshot session: %q", snap.SessionId)
	}
	if len(snap.Entities) == 0 {
		t.Fatal("expected snapshot entities")
	}
}

func TestRunnerRegistersPluginSchemas(t *testing.T) {
	runner, err := chatapp.NewRunner(chatapp.RunnerOptions{Plugins: []chatapp.ChatPlugin{plugins.NewReasoningPlugin(), plugins.NewToolCallPlugin()}})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	defer func() { _ = runner.Close() }()

	if _, ok := runner.Registry.EventSchema(chatapp.EventChatReasoningPatch); !ok {
		t.Fatalf("expected reasoning event schema %q", chatapp.EventChatReasoningPatch)
	}
	if _, ok := runner.Registry.EventSchema(chatapp.EventChatToolResultReady); !ok {
		t.Fatalf("expected tool result event schema %q", chatapp.EventChatToolResultReady)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
