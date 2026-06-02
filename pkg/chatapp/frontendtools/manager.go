package frontendtools

import (
	"context"
	"fmt"
	"sync"

	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	CommandManifest = "ChatFrontendToolManifest"
	CommandResult   = "ChatFrontendToolResult"

	EventManifestUpdated = "ChatFrontendToolManifestUpdated"
	EventCallRequested   = "ChatFrontendToolCallRequested"
	EventResultReceived  = "ChatFrontendToolResultReceived"

	TimelineEntityFrontendToolCall = "ChatFrontendToolCall"
)

type pendingCall struct {
	messageID string
	toolName  string
	ch        chan *toolv1.FrontendToolResultCommand
}

type Manager struct {
	mu        sync.Mutex
	manifests map[sessionstream.SessionId]*toolv1.FrontendToolManifestUpdated
	pending   map[string]*pendingCall
}

func NewManager() *Manager {
	return &Manager{
		manifests: map[sessionstream.SessionId]*toolv1.FrontendToolManifestUpdated{},
		pending:   map[string]*pendingCall{},
	}
}

func RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterCommand(CommandManifest, &toolv1.FrontendToolManifestCommand{}),
		reg.RegisterCommand(CommandResult, &toolv1.FrontendToolResultCommand{}),
		reg.RegisterEvent(EventManifestUpdated, &toolv1.FrontendToolManifestUpdated{}),
		reg.RegisterEvent(EventCallRequested, &toolv1.FrontendToolCallRequested{}),
		reg.RegisterEvent(EventResultReceived, &toolv1.FrontendToolResultReceived{}),
		reg.RegisterUIEvent(EventCallRequested, &toolv1.FrontendToolCallRequested{}),
		reg.RegisterUIEvent(EventResultReceived, &toolv1.FrontendToolResultReceived{}),
		reg.RegisterTimelineEntity(TimelineEntityFrontendToolCall, &toolv1.FrontendToolCallEntity{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Install(hub *sessionstream.Hub) error {
	if hub == nil {
		return fmt.Errorf("hub is nil")
	}
	if err := hub.RegisterCommand(CommandManifest, m.HandleManifest); err != nil {
		return err
	}
	if err := hub.RegisterCommand(CommandResult, m.HandleResult); err != nil {
		return err
	}
	return nil
}

func (m *Manager) HandleManifest(ctx context.Context, cmd sessionstream.Command, _ *sessionstream.Session, pub sessionstream.EventPublisher) error {
	payload, ok := cmd.Payload.(*toolv1.FrontendToolManifestCommand)
	if !ok || payload == nil {
		return fmt.Errorf("frontend tool manifest payload must be %T, got %T", &toolv1.FrontendToolManifestCommand{}, cmd.Payload)
	}
	updated := &toolv1.FrontendToolManifestUpdated{
		Tools:    cloneDescriptors(payload.Tools),
		Revision: payload.Revision,
	}

	m.mu.Lock()
	m.manifests[cmd.SessionId] = proto.Clone(updated).(*toolv1.FrontendToolManifestUpdated)
	m.mu.Unlock()

	return pub.Publish(ctx, sessionstream.Event{Name: EventManifestUpdated, SessionId: cmd.SessionId, Payload: updated})
}

func (m *Manager) HandleResult(ctx context.Context, cmd sessionstream.Command, _ *sessionstream.Session, pub sessionstream.EventPublisher) error {
	payload, ok := cmd.Payload.(*toolv1.FrontendToolResultCommand)
	if !ok || payload == nil {
		return fmt.Errorf("frontend tool result payload must be %T, got %T", &toolv1.FrontendToolResultCommand{}, cmd.Payload)
	}
	if payload.ToolCallId == "" {
		return fmt.Errorf("frontend tool result is missing tool_call_id")
	}

	m.mu.Lock()
	pending := m.pending[payload.ToolCallId]
	m.mu.Unlock()
	messageID := ""
	if pending != nil {
		if payload.ToolName == "" {
			payload.ToolName = pending.toolName
		}
		messageID = pending.messageID
	}
	if payload.Status == "" {
		payload.Status = "success"
	}

	if err := pub.Publish(ctx, sessionstream.Event{Name: EventResultReceived, SessionId: cmd.SessionId, Payload: &toolv1.FrontendToolResultReceived{
		MessageId:  messageID,
		ToolCallId: payload.ToolCallId,
		ToolName:   payload.ToolName,
		Result:     payload.Result,
		Status:     payload.Status,
		Error:      payload.Error,
	}}); err != nil {
		return err
	}

	if pending != nil {
		select {
		case pending.ch <- proto.Clone(payload).(*toolv1.FrontendToolResultCommand):
		default:
		}
	}
	return nil
}

type Request struct {
	MessageID  string
	ToolCallID string
	ToolName   string
	Input      map[string]any
	Mode       toolv1.ToolExecutionMode
}

func (m *Manager) Request(ctx context.Context, sid sessionstream.SessionId, pub sessionstream.EventPublisher, req Request) (*toolv1.FrontendToolResultCommand, error) {
	if m == nil {
		return nil, fmt.Errorf("frontend tools manager is nil")
	}
	if req.ToolCallID == "" || req.ToolName == "" {
		return nil, fmt.Errorf("frontend tool request requires tool call id and tool name")
	}
	input, err := structpb.NewStruct(req.Input)
	if err != nil {
		return nil, err
	}
	if req.Mode == toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_UNSPECIFIED {
		req.Mode = toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_AUTO
	}
	ch := make(chan *toolv1.FrontendToolResultCommand, 1)

	m.mu.Lock()
	m.pending[req.ToolCallID] = &pendingCall{messageID: req.MessageID, toolName: req.ToolName, ch: ch}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.pending, req.ToolCallID)
		m.mu.Unlock()
	}()

	if err := pub.Publish(context.WithoutCancel(ctx), sessionstream.Event{Name: EventCallRequested, SessionId: sid, Payload: &toolv1.FrontendToolCallRequested{
		MessageId:  req.MessageID,
		ToolCallId: req.ToolCallID,
		ToolName:   req.ToolName,
		Input:      input,
		Mode:       req.Mode,
		Status:     "requested",
	}}); err != nil {
		return nil, err
	}

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *Manager) Descriptor(sid sessionstream.SessionId, name string) (*toolv1.FrontendToolDescriptor, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	manifest := m.manifests[sid]
	if manifest == nil {
		return nil, false
	}
	for _, tool := range manifest.Tools {
		if tool.GetName() == name {
			return proto.Clone(tool).(*toolv1.FrontendToolDescriptor), true
		}
	}
	return nil, false
}

func (m *Manager) HasAvailableTool(sid sessionstream.SessionId, name string) bool {
	descriptor, ok := m.Descriptor(sid, name)
	return ok && descriptor.GetAvailable()
}

func cloneDescriptors(in []*toolv1.FrontendToolDescriptor) []*toolv1.FrontendToolDescriptor {
	out := make([]*toolv1.FrontendToolDescriptor, 0, len(in))
	for _, descriptor := range in {
		if descriptor == nil {
			continue
		}
		out = append(out, proto.Clone(descriptor).(*toolv1.FrontendToolDescriptor))
	}
	return out
}
