package frontendtools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/types/known/structpb"
)

type capturePublisher struct {
	events chan sessionstream.Event
}

func (p *capturePublisher) Publish(_ context.Context, ev sessionstream.Event) error {
	p.events <- ev
	return nil
}

func TestBridgeExecutorRoutesFrontendToolAndReturnsBrowserResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 8)}
	sid := sessionstream.SessionId("bridge-session")
	schema, err := structpb.NewStruct(map[string]any{"type": "object"})
	if err != nil {
		t.Fatalf("schema struct: %v", err)
	}
	if err := manager.HandleManifest(ctx, sessionstream.Command{SessionId: sid, Name: CommandManifest, Payload: &toolv1.FrontendToolManifestCommand{Revision: 1, Tools: []*toolv1.FrontendToolDescriptor{{
		Name:        "cart.add",
		Description: "Add to browser cart",
		InputSchema: schema,
		Mode:        toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_AUTO,
		Available:   true,
	}}}}, nil, publisher); err != nil {
		t.Fatalf("HandleManifest: %v", err)
	}

	executor := NewBridgeExecutor(manager, nil)
	registry := geptools.NewInMemoryToolRegistry()
	if err := manager.RegisterManifestTools(sid, registry); err != nil {
		t.Fatalf("RegisterManifestTools: %v", err)
	}
	if _, err := registry.GetTool("cart_add"); err != nil {
		t.Fatalf("expected provider-safe cart_add tool in registry: %v", err)
	}
	if _, err := registry.GetTool("cart.add"); err == nil {
		t.Fatalf("expected raw dotted cart.add name to be provider-aliased")
	}
	args, _ := json.Marshal(map[string]any{"sku": "retro-boot", "quantity": 1})
	bridgeCtx := WithBridgeContext(ctx, BridgeContext{SessionID: sid, MessageID: "msg-1", Publisher: publisher})

	resultCh := make(chan *geptools.ToolResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := executor.ExecuteToolCall(bridgeCtx, geptools.ToolCall{ID: "call-1", Name: "cart_add", Arguments: args}, registry)
		resultCh <- result
		errCh <- err
	}()

	var requested *toolv1.FrontendToolCallRequested
	for requested == nil {
		select {
		case ev := <-publisher.events:
			if ev.Name == EventCallRequested {
				requested, _ = ev.Payload.(*toolv1.FrontendToolCallRequested)
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s", EventCallRequested)
		}
	}
	if requested.GetToolCallId() != "call-1" || requested.GetToolName() != "cart.add" {
		t.Fatalf("unexpected request: %#v", requested)
	}

	resultStruct, err := structpb.NewStruct(map[string]any{"ok": true, "cartCount": 1})
	if err != nil {
		t.Fatalf("result struct: %v", err)
	}
	if err := manager.HandleResult(ctx, sessionstream.Command{SessionId: sid, Name: CommandResult, Payload: &toolv1.FrontendToolResultCommand{
		ToolCallId: "call-1",
		ToolName:   "cart.add",
		Status:     "success",
		Result:     resultStruct,
	}}, nil, publisher); err != nil {
		t.Fatalf("HandleResult: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ExecuteToolCall error: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for executor error")
	}
	select {
	case result := <-resultCh:
		if result.Error != "" {
			t.Fatalf("unexpected tool error: %s", result.Error)
		}
		out, ok := result.Result.(map[string]any)
		if !ok || out["ok"] != true || out["cartCount"] != float64(1) {
			t.Fatalf("unexpected result: %#v", result.Result)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for executor result")
	}
}

func TestRegisterManifestToolsRejectsProviderAliasCollision(t *testing.T) {
	ctx := context.Background()
	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 8)}
	sid := sessionstream.SessionId("collision-session")
	schema, err := structpb.NewStruct(map[string]any{"type": "object"})
	if err != nil {
		t.Fatalf("schema struct: %v", err)
	}
	if err := manager.HandleManifest(ctx, sessionstream.Command{SessionId: sid, Name: CommandManifest, Payload: &toolv1.FrontendToolManifestCommand{Revision: 1, Tools: []*toolv1.FrontendToolDescriptor{
		{Name: "cart.add", InputSchema: schema, Available: true},
		{Name: "cart_add", InputSchema: schema, Available: true},
	}}}, nil, publisher); err != nil {
		t.Fatalf("HandleManifest: %v", err)
	}
	registry := geptools.NewInMemoryToolRegistry()
	if err := manager.RegisterManifestTools(sid, registry); err == nil {
		t.Fatalf("expected provider alias collision error")
	}
}
