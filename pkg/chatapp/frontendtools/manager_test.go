package frontendtools

import (
	"context"
	"testing"
	"time"

	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestManagerManifestDescriptorAndAvailability(t *testing.T) {
	ctx := context.Background()
	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 4)}
	sid := sessionstream.SessionId("manifest-session")
	schema, err := structpb.NewStruct(map[string]any{"type": "object"})
	if err != nil {
		t.Fatalf("schema struct: %v", err)
	}
	if err := manager.HandleManifest(ctx, sessionstream.Command{SessionId: sid, Name: CommandManifest, Payload: &toolv1.FrontendToolManifestCommand{Revision: 2, Tools: []*toolv1.FrontendToolDescriptor{{Name: "cart.add", InputSchema: schema, Available: true}}}}, nil, publisher); err != nil {
		t.Fatalf("HandleManifest: %v", err)
	}
	desc, ok := manager.Descriptor(sid, "cart.add")
	if !ok || desc.GetName() != "cart.add" || !desc.GetAvailable() {
		t.Fatalf("unexpected descriptor: %#v ok=%v", desc, ok)
	}
	if !manager.HasAvailableTool(sid, "cart.add") {
		t.Fatalf("expected cart.add to be available")
	}
}

func TestManagerRequestReceivesDeniedResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 4)}
	sid := sessionstream.SessionId("denied-session")

	resultCh := make(chan *toolv1.FrontendToolResultCommand, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := manager.Request(ctx, sid, publisher, Request{MessageID: "msg-1", ToolCallID: "call-1", ToolName: "checkout.confirm", Input: map[string]any{"orderId": "ord-1"}, Mode: toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_HUMAN})
		resultCh <- result
		errCh <- err
	}()

	select {
	case ev := <-publisher.events:
		if ev.Name != EventCallRequested {
			t.Fatalf("unexpected event %s", ev.Name)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for request event")
	}

	if err := manager.HandleResult(ctx, sessionstream.Command{SessionId: sid, Name: CommandResult, Payload: &toolv1.FrontendToolResultCommand{ToolCallId: "call-1", ToolName: "checkout.confirm", Status: "denied", Error: "user declined"}}, nil, publisher); err != nil {
		t.Fatalf("HandleResult: %v", err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Request error: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for request error")
	}
	select {
	case result := <-resultCh:
		if result.GetStatus() != "denied" || result.GetError() != "user declined" {
			t.Fatalf("unexpected result: %#v", result)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for result")
	}
}
