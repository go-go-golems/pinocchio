package frontendtools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/pkg/errors"
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

	defaultTerminalMaxEntries     = 4096
	defaultTerminalTTL            = 15 * time.Minute
	defaultTerminalPublishTimeout = 5 * time.Second
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
	InvocationErrorTerminalConflict InvocationErrorCode = "terminal_conflict"
	InvocationErrorLateResult       InvocationErrorCode = "late_result"
	InvocationErrorKeyReuse         InvocationErrorCode = "key_reuse"
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

// ManagerConfig controls bounded frontend-tool terminal retention.
type ManagerConfig struct {
	TerminalMaxEntries int
	TerminalTTL        time.Duration
}

// DefaultManagerConfig returns production-safe bounded terminal retention.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{TerminalMaxEntries: defaultTerminalMaxEntries, TerminalTTL: defaultTerminalTTL}
}

type Manager struct {
	mu                     sync.Mutex
	manifests              map[sessionstream.SessionId]*toolv1.FrontendToolManifestUpdated
	pending                map[pendingKey]*pendingCall
	terminal               *boundedTerminalStore
	terminalPublishTimeout time.Duration
	now                    func() time.Time
}

func NewManager() *Manager {
	return newManager(DefaultManagerConfig())
}

// NewManagerWithConfig creates a manager with explicit terminal count and age
// limits. Both limits must be positive.
func NewManagerWithConfig(config ManagerConfig) (*Manager, error) {
	if config.TerminalMaxEntries <= 0 {
		return nil, fmt.Errorf("frontend tool terminal max entries must be positive")
	}
	if config.TerminalTTL <= 0 {
		return nil, fmt.Errorf("frontend tool terminal TTL must be positive")
	}
	return newManager(config), nil
}

func newManager(config ManagerConfig) *Manager {
	return &Manager{
		manifests:              map[sessionstream.SessionId]*toolv1.FrontendToolManifestUpdated{},
		pending:                map[pendingKey]*pendingCall{},
		terminal:               newBoundedTerminalStore(config.TerminalMaxEntries, config.TerminalTTL),
		terminalPublishTimeout: defaultTerminalPublishTimeout,
		now:                    time.Now,
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
	resultBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload.Result)
	if err != nil {
		return errors.Wrap(err, "encode frontend tool result for idempotency")
	}

	key := pendingKey{sessionID: cmd.SessionId, toolCallID: payload.ToolCallId}
	m.mu.Lock()
	now := m.now()
	if terminal, exists := m.terminal.get(key, now); exists {
		if payload.ToolName == "" {
			payload.ToolName = terminal.toolName
		} else if payload.ToolName != terminal.toolName {
			m.mu.Unlock()
			return newInvocationError(InvocationErrorToolMismatch, cmd.SessionId, payload.ToolCallId, payload.ToolName)
		}
		digest := frontendToolResultDigest(payload, resultBytes)
		m.mu.Unlock()
		if terminal.origin == terminalOriginContext && digest != terminal.digest {
			return newInvocationError(InvocationErrorLateResult, cmd.SessionId, payload.ToolCallId, payload.ToolName)
		}
		if digest == terminal.digest {
			return nil
		}
		return newInvocationError(InvocationErrorTerminalConflict, cmd.SessionId, payload.ToolCallId, payload.ToolName)
	}

	pending := m.pending[key]
	if pending == nil {
		wrongSession := m.hasToolCallInOtherSession(key, now)
		m.mu.Unlock()
		code := InvocationErrorUnknownResult
		if wrongSession {
			code = InvocationErrorSessionMismatch
		}
		return newInvocationError(code, cmd.SessionId, payload.ToolCallId, payload.ToolName)
	}
	if payload.ToolName == "" {
		payload.ToolName = pending.toolName
	} else if payload.ToolName != pending.toolName {
		m.mu.Unlock()
		return newInvocationError(InvocationErrorToolMismatch, cmd.SessionId, payload.ToolCallId, payload.ToolName)
	}
	digest := frontendToolResultDigest(payload, resultBytes)
	delete(m.pending, key)
	m.terminal.add(&terminalCall{
		key:         key,
		toolName:    pending.toolName,
		digest:      digest,
		origin:      terminalOriginResult,
		completedAt: now,
	}, now)
	m.mu.Unlock()

	if err := pub.Publish(ctx, frontendToolResultEvent(pending, payload)); err != nil {
		failure := proto.Clone(payload).(*toolv1.FrontendToolResultCommand)
		failure.Status = "failed"
		failure.Error = fmt.Sprintf("frontend tool result timeline publication failed: %v", err)
		deliverCompletion(pending, failure)
		return errors.Wrap(err, "publish frontend tool result")
	}
	deliverCompletion(pending, payload)
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
	if _, exists := m.terminal.get(key, m.now()); exists {
		m.mu.Unlock()
		return nil, newInvocationError(InvocationErrorKeyReuse, sid, req.ToolCallID, req.ToolName)
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
		if err := m.terminalizeContext(ctx, pub, pending); err != nil {
			return nil, errors.Wrapf(ctx.Err(), "publish frontend tool cancellation: %v", err)
		}
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

func (m *Manager) terminalizeContext(ctx context.Context, pub sessionstream.EventPublisher, pending *pendingCall) error {
	if m == nil || pending == nil {
		return nil
	}
	status := "cancelled"
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		status = "timeout"
	}
	payload := &toolv1.FrontendToolResultCommand{
		ToolCallId: pending.key.toolCallID,
		ToolName:   pending.toolName,
		Status:     status,
		Error:      ctx.Err().Error(),
	}
	resultBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload.Result)
	if err != nil {
		return errors.Wrap(err, "encode frontend tool cancellation for idempotency")
	}

	m.mu.Lock()
	if current := m.pending[pending.key]; current != pending {
		m.mu.Unlock()
		return nil
	}
	now := m.now()
	delete(m.pending, pending.key)
	m.terminal.add(&terminalCall{
		key:         pending.key,
		toolName:    pending.toolName,
		digest:      frontendToolResultDigest(payload, resultBytes),
		origin:      terminalOriginContext,
		completedAt: now,
	}, now)
	m.mu.Unlock()

	publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.terminalPublishTimeout)
	defer cancel()
	return pub.Publish(publishCtx, frontendToolResultEvent(pending, payload))
}

func (m *Manager) hasToolCallInOtherSession(key pendingKey, now time.Time) bool {
	for candidate := range m.pending {
		if candidate.sessionID != key.sessionID && candidate.toolCallID == key.toolCallID {
			return true
		}
	}
	return m.terminal.hasToolCallInOtherSession(key, now)
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

func frontendToolResultEvent(pending *pendingCall, payload *toolv1.FrontendToolResultCommand) sessionstream.Event {
	return sessionstream.Event{Name: EventResultReceived, SessionId: pending.key.sessionID, Payload: &toolv1.FrontendToolResultReceived{
		MessageId:  pending.messageID,
		ToolCallId: pending.key.toolCallID,
		ToolName:   pending.toolName,
		Result:     payload.Result,
		Status:     payload.Status,
		Error:      payload.Error,
	}}
}

func frontendToolResultDigest(payload *toolv1.FrontendToolResultCommand, resultBytes []byte) [32]byte {
	hash := sha256.New()
	for _, field := range []string{payload.GetToolCallId(), payload.GetToolName(), payload.GetStatus(), payload.GetError()} {
		_, _ = fmt.Fprintf(hash, "%d:", len(field))
		_, _ = hash.Write([]byte(field))
	}
	_, _ = fmt.Fprintf(hash, "%d:", len(resultBytes))
	_, _ = hash.Write(resultBytes)
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func deliverCompletion(pending *pendingCall, payload *toolv1.FrontendToolResultCommand) {
	select {
	case pending.ch <- payload:
	default:
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
