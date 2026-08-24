package frontendtools

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// InvocationErrorCode identifies a stable frontend-tool request or result
// rejection reason without exposing result payload data.
type InvocationErrorCode string

const (
	InvocationErrorDuplicatePending InvocationErrorCode = "duplicate_pending"
	InvocationErrorUnknownResult    InvocationErrorCode = "unknown_result"
	InvocationErrorSessionMismatch  InvocationErrorCode = "session_mismatch"
	InvocationErrorToolMismatch     InvocationErrorCode = "tool_mismatch"
	InvocationErrorInvalidStatus    InvocationErrorCode = "invalid_status"
)

// InvocationError describes a rejected frontend-tool request or result.
type InvocationError struct {
	Code       InvocationErrorCode
	SessionID  sessionstream.SessionId
	ToolCallID string
	ToolName   string
}

func (e *InvocationError) Error() string {
	if e == nil {
		return "frontend tool invocation rejected"
	}
	return fmt.Sprintf("frontend tool invocation rejected: code=%s session_id=%q tool_call_id=%q tool_name=%q", e.Code, e.SessionID, e.ToolCallID, e.ToolName)
}

// InvocationErrorCodeOf returns the stable code carried by an InvocationError.
func InvocationErrorCodeOf(err error) (InvocationErrorCode, bool) {
	var invocationErr *InvocationError
	if !errors.As(err, &invocationErr) {
		return "", false
	}
	return invocationErr.Code, true
}

type pendingKey struct {
	sessionID  sessionstream.SessionId
	toolCallID string
}

type pendingCall struct {
	key       pendingKey
	messageID string
	toolName  string
	ch        chan *toolv1.FrontendToolResultCommand
}

type Manager struct {
	mu        sync.Mutex
	manifests map[sessionstream.SessionId]*toolv1.FrontendToolManifestUpdated
	pending   map[pendingKey]*pendingCall
}

func NewManager() *Manager {
	return &Manager{
		manifests: map[sessionstream.SessionId]*toolv1.FrontendToolManifestUpdated{},
		pending:   map[pendingKey]*pendingCall{},
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
	if m == nil {
		return fmt.Errorf("frontend tools manager is nil")
	}
	if pub == nil {
		return fmt.Errorf("frontend tool result publisher is nil")
	}
	payload, ok := cmd.Payload.(*toolv1.FrontendToolResultCommand)
	if !ok || payload == nil {
		return fmt.Errorf("frontend tool result payload must be %T, got %T", &toolv1.FrontendToolResultCommand{}, cmd.Payload)
	}
	payload = proto.Clone(payload).(*toolv1.FrontendToolResultCommand)
	if payload.ToolCallId == "" {
		return fmt.Errorf("frontend tool result is missing tool_call_id")
	}
	status, ok := canonicalResultStatus(payload.Status)
	if !ok {
		return newInvocationError(InvocationErrorInvalidStatus, cmd.SessionId, payload.ToolCallId, payload.ToolName)
	}
	payload.Status = status

	key := pendingKey{sessionID: cmd.SessionId, toolCallID: payload.ToolCallId}
	m.mu.Lock()
	pending := m.pending[key]
	wrongSession := false
	if pending == nil {
		for candidate := range m.pending {
			if candidate.toolCallID == payload.ToolCallId {
				wrongSession = true
				break
			}
		}
	}
	m.mu.Unlock()
	if pending == nil {
		code := InvocationErrorUnknownResult
		if wrongSession {
			code = InvocationErrorSessionMismatch
		}
		return newInvocationError(code, cmd.SessionId, payload.ToolCallId, payload.ToolName)
	}
	if payload.ToolName == "" {
		payload.ToolName = pending.toolName
	} else if payload.ToolName != pending.toolName {
		return newInvocationError(InvocationErrorToolMismatch, cmd.SessionId, payload.ToolCallId, payload.ToolName)
	}

	if err := pub.Publish(ctx, sessionstream.Event{Name: EventResultReceived, SessionId: pending.key.sessionID, Payload: &toolv1.FrontendToolResultReceived{
		MessageId:  pending.messageID,
		ToolCallId: pending.key.toolCallID,
		ToolName:   pending.toolName,
		Result:     payload.Result,
		Status:     payload.Status,
		Error:      payload.Error,
	}}); err != nil {
		return err
	}

	select {
	case pending.ch <- payload:
	default:
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
	if sid == "" {
		return nil, fmt.Errorf("frontend tool request requires session id")
	}
	if pub == nil {
		return nil, fmt.Errorf("frontend tool request publisher is nil")
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
	key := pendingKey{sessionID: sid, toolCallID: req.ToolCallID}
	pending := &pendingCall{key: key, messageID: req.MessageID, toolName: req.ToolName, ch: make(chan *toolv1.FrontendToolResultCommand, 1)}

	m.mu.Lock()
	if _, exists := m.pending[key]; exists {
		m.mu.Unlock()
		return nil, newInvocationError(InvocationErrorDuplicatePending, sid, req.ToolCallID, req.ToolName)
	}
	m.pending[key] = pending
	m.mu.Unlock()
	defer m.removePending(pending)

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
	case result := <-pending.ch:
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

func (m *Manager) removePending(pending *pendingCall) {
	if m == nil || pending == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.pending[pending.key]; current == pending {
		delete(m.pending, pending.key)
	}
}

func canonicalResultStatus(status string) (string, bool) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return "success", true
	}
	switch status {
	case "success", "failed", "denied", "cancelled", "timeout":
		return status, true
	default:
		return "", false
	}
}

func newInvocationError(code InvocationErrorCode, sid sessionstream.SessionId, toolCallID, toolName string) error {
	return &InvocationError{Code: code, SessionID: sid, ToolCallID: toolCallID, ToolName: toolName}
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
