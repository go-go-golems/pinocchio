package frontendtools

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
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
	if err := manager.HandleManifest(ctx, sessionstream.Command{SessionId: sid, Name: CommandManifest, Payload: &toolv1.FrontendToolManifestCommand{Revision: 2, ClientInstanceId: "client-test", ConnectionId: "connection-test", Tools: []*toolv1.FrontendToolDescriptor{{Name: "cart.add", InputSchema: schema, Available: true}}}}, nil, publisher); err != nil {
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

	ensureTestManifest(t, manager, sid, "checkout.confirm")
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

	if err := manager.HandleResult(ctx, sessionstream.Command{SessionId: sid, Name: CommandResult, Payload: &toolv1.FrontendToolResultCommand{ToolCallId: "call-1", ToolName: "checkout.confirm", Status: "denied", Error: "user declined", Executor: testExecutor()}}, nil, publisher); err != nil {
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

func TestManagerSameToolCallIDInDifferentSessionsIsIndependent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 16)}
	sessionA := sessionstream.SessionId("session-a")
	sessionB := sessionstream.SessionId("session-b")
	outcomeA := startManagerRequest(ctx, manager, publisher, sessionA, Request{MessageID: "msg-a", ToolCallID: "shared-call", ToolName: "browser.read"})
	requireRequestedEvent(t, ctx, publisher, sessionA, "msg-a", "shared-call", "browser.read")
	outcomeB := startManagerRequest(ctx, manager, publisher, sessionB, Request{MessageID: "msg-b", ToolCallID: "shared-call", ToolName: "browser.read"})
	requireRequestedEvent(t, ctx, publisher, sessionB, "msg-b", "shared-call", "browser.read")

	resultA, err := structpb.NewStruct(map[string]any{"source": "a"})
	require.NoError(t, err)
	require.NoError(t, manager.HandleResult(ctx, resultCommand(sessionA, "shared-call", "browser.read", "success", resultA), nil, publisher))
	requireResultEvent(t, ctx, publisher, sessionA, "msg-a", "shared-call", "browser.read")

	resultB, err := structpb.NewStruct(map[string]any{"source": "b"})
	require.NoError(t, err)
	require.NoError(t, manager.HandleResult(ctx, resultCommand(sessionB, "shared-call", "browser.read", "success", resultB), nil, publisher))
	requireResultEvent(t, ctx, publisher, sessionB, "msg-b", "shared-call", "browser.read")

	require.Equal(t, "a", requireRequestOutcome(t, ctx, outcomeA).GetResult().AsMap()["source"])
	require.Equal(t, "b", requireRequestOutcome(t, ctx, outcomeB).GetResult().AsMap()["source"])
}

func TestManagerRejectsCrossSessionResultAndKeepsOwnerPending(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 16)}
	victimSession := sessionstream.SessionId("victim-session")
	outcome := startManagerRequest(ctx, manager, publisher, victimSession, Request{MessageID: "msg-victim", ToolCallID: "call-7", ToolName: "dangerous_browser_tool"})
	requireRequestedEvent(t, ctx, publisher, victimSession, "msg-victim", "call-7", "dangerous_browser_tool")

	err := manager.HandleResult(ctx, resultCommand("attacker-session", "call-7", "different_name", "success", nil), nil, publisher)
	requireInvocationErrorCode(t, err, InvocationErrorSessionMismatch)
	requireNoEvent(t, publisher)

	require.NoError(t, manager.HandleResult(ctx, resultCommand(victimSession, "call-7", "dangerous_browser_tool", "success", nil), nil, publisher))
	requireResultEvent(t, ctx, publisher, victimSession, "msg-victim", "call-7", "dangerous_browser_tool")
	require.Equal(t, "success", requireRequestOutcome(t, ctx, outcome).GetStatus())
}

func TestManagerRejectsWrongToolAndInvalidStatusWithoutCompleting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 16)}
	sid := sessionstream.SessionId("strict-session")
	outcome := startManagerRequest(ctx, manager, publisher, sid, Request{MessageID: "msg-1", ToolCallID: "call-1", ToolName: "expected_tool"})
	requireRequestedEvent(t, ctx, publisher, sid, "msg-1", "call-1", "expected_tool")

	err := manager.HandleResult(ctx, resultCommand(sid, "call-1", "other_tool", "success", nil), nil, publisher)
	requireInvocationErrorCode(t, err, InvocationErrorToolMismatch)
	requireNoEvent(t, publisher)

	err = manager.HandleResult(ctx, resultCommand(sid, "call-1", "expected_tool", "mystery", nil), nil, publisher)
	requireInvocationErrorCode(t, err, InvocationErrorInvalidStatus)
	requireNoEvent(t, publisher)

	require.NoError(t, manager.HandleResult(ctx, resultCommand(sid, "call-1", "", "", nil), nil, publisher))
	requireResultEvent(t, ctx, publisher, sid, "msg-1", "call-1", "expected_tool")
	require.Equal(t, "success", requireRequestOutcome(t, ctx, outcome).GetStatus())
}

func TestManagerRejectsDuplicatePendingWithoutOverwritingFirst(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 16)}
	sid := sessionstream.SessionId("duplicate-session")
	first := startManagerRequest(ctx, manager, publisher, sid, Request{MessageID: "msg-first", ToolCallID: "call-1", ToolName: "first_tool"})
	requireRequestedEvent(t, ctx, publisher, sid, "msg-first", "call-1", "first_tool")

	result, err := manager.Request(ctx, sid, publisher, Request{MessageID: "msg-second", ToolCallID: "call-1", ToolName: "second_tool"})
	require.Nil(t, result)
	requireInvocationErrorCode(t, err, InvocationErrorDuplicatePending)
	requireNoEvent(t, publisher)

	require.NoError(t, manager.HandleResult(ctx, resultCommand(sid, "call-1", "first_tool", "success", nil), nil, publisher))
	requireResultEvent(t, ctx, publisher, sid, "msg-first", "call-1", "first_tool")
	require.Equal(t, "first_tool", requireRequestOutcome(t, ctx, first).GetToolName())
}

func TestManagerRejectsUnknownResultWithoutPublishing(t *testing.T) {
	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 1)}
	err := manager.HandleResult(context.Background(), resultCommand("session", "missing", "tool", "success", nil), nil, publisher)
	requireInvocationErrorCode(t, err, InvocationErrorUnknownResult)
	requireNoEvent(t, publisher)
}

func TestManagerRemovePendingDoesNotDeleteReplacement(t *testing.T) {
	manager := NewManager()
	key := pendingKey{sessionID: "session", toolCallID: "call"}
	oldPending := &pendingCall{key: key}
	replacement := &pendingCall{key: key}
	manager.pending[key] = replacement

	manager.removePending(oldPending)

	require.Same(t, replacement, manager.pending[key])
}

func TestManagerTerminalResultRetriesAreIdempotentAndConflictsAreRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 16)}
	sid := sessionstream.SessionId("terminal-session")
	outcome := startManagerRequest(ctx, manager, publisher, sid, Request{MessageID: "msg-1", ToolCallID: "call-1", ToolName: "browser.write"})
	requireRequestedEvent(t, ctx, publisher, sid, "msg-1", "call-1", "browser.write")

	result, err := structpb.NewStruct(map[string]any{"revision": 7})
	require.NoError(t, err)
	accepted := resultCommand(sid, "call-1", "browser.write", "success", result)
	require.NoError(t, manager.HandleResult(ctx, accepted, nil, publisher))
	requireResultEvent(t, ctx, publisher, sid, "msg-1", "call-1", "browser.write")
	require.Equal(t, float64(7), requireRequestOutcome(t, ctx, outcome).GetResult().AsMap()["revision"])

	require.NoError(t, manager.HandleResult(ctx, accepted, nil, publisher), "identical terminal retry must be acknowledged")
	requireNoEvent(t, publisher)

	conflictingResult, err := structpb.NewStruct(map[string]any{"revision": 8})
	require.NoError(t, err)
	err = manager.HandleResult(ctx, resultCommand(sid, "call-1", "browser.write", "success", conflictingResult), nil, publisher)
	requireInvocationErrorCode(t, err, InvocationErrorTerminalConflict)
	requireNoEvent(t, publisher)

	reused, err := manager.Request(ctx, sid, publisher, Request{MessageID: "msg-2", ToolCallID: "call-1", ToolName: "browser.write"})
	require.Nil(t, reused)
	requireInvocationErrorCode(t, err, InvocationErrorKeyReuse)
}

func TestManagerContextCancellationBecomesTerminalAndRejectsLateResult(t *testing.T) {
	tests := []struct {
		name           string
		newContext     func() (context.Context, context.CancelFunc)
		trigger        func(context.CancelFunc)
		expectedStatus string
		expectedError  error
	}{
		{
			name: "cancelled",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			trigger:        func(cancel context.CancelFunc) { cancel() },
			expectedStatus: "cancelled",
			expectedError:  context.Canceled,
		},
		{
			name: "timeout",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Unix(1, 0))
			},
			trigger:        func(context.CancelFunc) {},
			expectedStatus: "timeout",
			expectedError:  context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.newContext()
			defer cancel()
			manager := NewManager()
			publisher := &capturePublisher{events: make(chan sessionstream.Event, 8)}
			sid := sessionstream.SessionId("context-" + tt.name)
			outcome := startManagerRequest(ctx, manager, publisher, sid, Request{MessageID: "msg-1", ToolCallID: "call-1", ToolName: "browser.wait"})
			waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer waitCancel()
			requireRequestedEvent(t, waitCtx, publisher, sid, "msg-1", "call-1", "browser.wait")
			tt.trigger(cancel)

			requireRequestError(t, outcome, tt.expectedError)
			requireResultEventStatus(t, publisher, sid, "msg-1", "call-1", "browser.wait", tt.expectedStatus)

			err := manager.HandleResult(context.Background(), resultCommand(sid, "call-1", "browser.wait", "success", nil), nil, publisher)
			requireInvocationErrorCode(t, err, InvocationErrorLateResult)
			requireNoEvent(t, publisher)
		})
	}
}

func TestManagerContextCancellationPublicationIsBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	manager := NewManager()
	manager.terminalPublishTimeout = 20 * time.Millisecond
	publisher := &contextBlockingResultPublisher{events: make(chan sessionstream.Event, 1)}
	sid := sessionstream.SessionId("bounded-cancellation-session")
	outcome := startManagerRequest(ctx, manager, publisher, sid, Request{
		MessageID:  "msg-1",
		ToolCallID: "call-1",
		ToolName:   "browser.wait",
	})
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	requireRequestedEvent(t, waitCtx, &capturePublisher{events: publisher.events}, sid, "msg-1", "call-1", "browser.wait")

	cancel()
	requireRequestError(t, outcome, context.Canceled)
	require.True(t, publisher.resultPublishTimedOut(), "terminal publication must stop at its bounded context deadline")

	err := manager.HandleResult(context.Background(), resultCommand(sid, "call-1", "browser.wait", "success", nil), nil, publisher)
	requireInvocationErrorCode(t, err, InvocationErrorLateResult)
}

func TestManagerResultPublicationFailureCompletesWaiterWithFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publishErr := errors.New("timeline unavailable")
	publisher := &resultFailPublisher{events: make(chan sessionstream.Event, 4), err: publishErr}
	manager := NewManager()
	sid := sessionstream.SessionId("publish-failure-session")
	outcome := startManagerRequest(ctx, manager, publisher, sid, Request{MessageID: "msg-1", ToolCallID: "call-1", ToolName: "browser.write"})
	requireRequestedEvent(t, ctx, &capturePublisher{events: publisher.events}, sid, "msg-1", "call-1", "browser.write")

	err := manager.HandleResult(ctx, resultCommand(sid, "call-1", "browser.write", "success", nil), nil, publisher)
	require.ErrorIs(t, err, publishErr)
	completed := requireRequestOutcome(t, ctx, outcome)
	require.Equal(t, "failed", completed.GetStatus())
	require.Contains(t, completed.GetError(), "timeline unavailable")
	require.Empty(t, publisher.events)

	require.NoError(t, manager.HandleResult(ctx, resultCommand(sid, "call-1", "browser.write", "success", nil), nil, publisher))
}

func TestManagerConcurrentTerminalDeliveryPublishesAndCompletesOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := newBlockingResultPublisher()
	manager := NewManager()
	sid := sessionstream.SessionId("concurrent-session")
	outcome := startManagerRequest(ctx, manager, publisher, sid, Request{MessageID: "msg-1", ToolCallID: "call-1", ToolName: "browser.write"})
	requireRequestedEvent(t, ctx, &capturePublisher{events: publisher.events}, sid, "msg-1", "call-1", "browser.write")

	accepted := resultCommand(sid, "call-1", "browser.write", "success", nil)
	firstErr := make(chan error, 1)
	go func() { firstErr <- manager.HandleResult(ctx, accepted, nil, publisher) }()
	select {
	case <-publisher.resultEntered:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for first result publication")
	}

	require.NoError(t, manager.HandleResult(ctx, accepted, nil, publisher), "retry while first publish is blocked must be idempotent")
	conflicting := resultCommand(sid, "call-1", "browser.write", "failed", nil)
	requireInvocationErrorCode(t, manager.HandleResult(ctx, conflicting, nil, publisher), InvocationErrorTerminalConflict)
	close(publisher.releaseResult)
	require.NoError(t, <-firstErr)
	requireResultEvent(t, ctx, &capturePublisher{events: publisher.events}, sid, "msg-1", "call-1", "browser.write")
	require.Equal(t, "success", requireRequestOutcome(t, ctx, outcome).GetStatus())
	requireNoEvent(t, &capturePublisher{events: publisher.events})
}

type contextBlockingResultPublisher struct {
	events chan sessionstream.Event
	mu     sync.Mutex
	err    error
}

func (p *contextBlockingResultPublisher) Publish(ctx context.Context, event sessionstream.Event) error {
	if event.Name != EventResultReceived {
		p.events <- event
		return nil
	}
	<-ctx.Done()
	p.mu.Lock()
	p.err = ctx.Err()
	p.mu.Unlock()
	return ctx.Err()
}

func (p *contextBlockingResultPublisher) resultPublishTimedOut() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return errors.Is(p.err, context.DeadlineExceeded)
}

type resultFailPublisher struct {
	events chan sessionstream.Event
	err    error
}

func (p *resultFailPublisher) Publish(_ context.Context, event sessionstream.Event) error {
	if event.Name == EventResultReceived {
		return p.err
	}
	p.events <- event
	return nil
}

type blockingResultPublisher struct {
	events        chan sessionstream.Event
	resultEntered chan struct{}
	releaseResult chan struct{}
	once          sync.Once
}

func newBlockingResultPublisher() *blockingResultPublisher {
	return &blockingResultPublisher{
		events:        make(chan sessionstream.Event, 8),
		resultEntered: make(chan struct{}),
		releaseResult: make(chan struct{}),
	}
}

func (p *blockingResultPublisher) Publish(_ context.Context, event sessionstream.Event) error {
	if event.Name == EventResultReceived {
		p.once.Do(func() { close(p.resultEntered) })
		<-p.releaseResult
	}
	p.events <- event
	return nil
}

type managerRequestOutcome struct {
	result *toolv1.FrontendToolResultCommand
	err    error
}

func startManagerRequest(ctx context.Context, manager *Manager, publisher sessionstream.EventPublisher, sid sessionstream.SessionId, req Request) <-chan managerRequestOutcome {
	ensureTestManifest(nil, manager, sid, req.ToolName)
	out := make(chan managerRequestOutcome, 1)
	go func() {
		result, err := manager.Request(ctx, sid, publisher, req)
		out <- managerRequestOutcome{result: result, err: err}
	}()
	return out
}

func requireRequestOutcome(t *testing.T, ctx context.Context, outcomes <-chan managerRequestOutcome) *toolv1.FrontendToolResultCommand {
	t.Helper()
	select {
	case outcome := <-outcomes:
		require.NoError(t, outcome.err)
		require.NotNil(t, outcome.result)
		return outcome.result
	case <-ctx.Done():
		t.Fatalf("timed out waiting for request outcome: %v", ctx.Err())
		return nil
	}
}

func requireRequestError(t *testing.T, outcomes <-chan managerRequestOutcome, expected error) {
	t.Helper()
	select {
	case outcome := <-outcomes:
		require.Nil(t, outcome.result)
		require.ErrorIs(t, outcome.err, expected)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request error")
	}
}

func requireRequestedEvent(t *testing.T, ctx context.Context, publisher *capturePublisher, sid sessionstream.SessionId, messageID, toolCallID, toolName string) {
	t.Helper()
	select {
	case event := <-publisher.events:
		require.Equal(t, EventCallRequested, event.Name)
		require.Equal(t, sid, event.SessionId)
		payload, ok := event.Payload.(*toolv1.FrontendToolCallRequested)
		require.True(t, ok)
		require.Equal(t, messageID, payload.GetMessageId())
		require.Equal(t, toolCallID, payload.GetToolCallId())
		require.Equal(t, toolName, payload.GetToolName())
	case <-ctx.Done():
		t.Fatalf("timed out waiting for request event: %v", ctx.Err())
	}
}

func requireResultEvent(t *testing.T, ctx context.Context, publisher *capturePublisher, sid sessionstream.SessionId, messageID, toolCallID, toolName string) {
	t.Helper()
	select {
	case event := <-publisher.events:
		require.Equal(t, EventResultReceived, event.Name)
		require.Equal(t, sid, event.SessionId)
		payload, ok := event.Payload.(*toolv1.FrontendToolResultReceived)
		require.True(t, ok)
		require.Equal(t, messageID, payload.GetMessageId())
		require.Equal(t, toolCallID, payload.GetToolCallId())
		require.Equal(t, toolName, payload.GetToolName())
	case <-ctx.Done():
		t.Fatalf("timed out waiting for result event: %v", ctx.Err())
	}
}

func requireResultEventStatus(t *testing.T, publisher *capturePublisher, sid sessionstream.SessionId, messageID, toolCallID, toolName, status string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	select {
	case event := <-publisher.events:
		require.Equal(t, EventResultReceived, event.Name)
		require.Equal(t, sid, event.SessionId)
		payload, ok := event.Payload.(*toolv1.FrontendToolResultReceived)
		require.True(t, ok)
		require.Equal(t, messageID, payload.GetMessageId())
		require.Equal(t, toolCallID, payload.GetToolCallId())
		require.Equal(t, toolName, payload.GetToolName())
		require.Equal(t, status, payload.GetStatus())
	case <-ctx.Done():
		t.Fatalf("timed out waiting for result event: %v", ctx.Err())
	}
}

func requireNoEvent(t *testing.T, publisher *capturePublisher) {
	t.Helper()
	select {
	case event := <-publisher.events:
		t.Fatalf("unexpected event: %#v", event)
	default:
	}
}

func requireInvocationErrorCode(t *testing.T, err error, expected InvocationErrorCode) {
	t.Helper()
	require.Error(t, err)
	code, ok := InvocationErrorCodeOf(err)
	require.True(t, ok, "expected InvocationError, got %T: %v", err, err)
	require.Equal(t, expected, code)
}

func resultCommand(sid sessionstream.SessionId, toolCallID, toolName, status string, result *structpb.Struct) sessionstream.Command {
	return sessionstream.Command{SessionId: sid, Name: CommandResult, Payload: &toolv1.FrontendToolResultCommand{
		ToolCallId: toolCallID,
		ToolName:   toolName,
		Status:     status,
		Result:     result,
		Executor:   testExecutor(),
	}}
}

func testExecutor() *toolv1.FrontendToolExecutor {
	return &toolv1.FrontendToolExecutor{
		ClientInstanceId: "client-test",
		ConnectionId:     "connection-test",
		AssignmentId:     "assignment-test",
	}
}

func ensureTestManifest(t *testing.T, manager *Manager, sid sessionstream.SessionId, toolName string) {
	if t != nil {
		t.Helper()
	}
	manager.mu.Lock()
	existing := manager.manifests[sid]
	manager.mu.Unlock()
	if existing != nil {
		return
	}
	manager.newAssignmentID = func() string { return "assignment-test" }
	_, err := manager.AcceptManifest(context.Background(), sid, discardPublisher{}, &toolv1.FrontendToolManifestCommand{
		Revision:         1,
		ClientInstanceId: "client-test",
		ConnectionId:     "connection-test",
		Tools: []*toolv1.FrontendToolDescriptor{{
			Name:      toolName,
			Available: true,
		}},
	})
	if t != nil {
		require.NoError(t, err)
	} else if err != nil {
		panic(err)
	}
}

type discardPublisher struct{}

func (discardPublisher) Publish(context.Context, sessionstream.Event) error { return nil }
