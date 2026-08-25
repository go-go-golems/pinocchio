package frontendtools

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestAcceptManifestCreatesRetainsAndReplacesAssignment(t *testing.T) {
	manager := NewManager()
	ids := []string{"assignment-1", "assignment-2"}
	manager.newAssignmentID = func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 8)}
	sid := sessionstream.SessionId("session-1")

	first, err := manager.AcceptManifest(context.Background(), sid, publisher, executorManifest("client-a", "connection-a", 1, "lookup"))
	require.NoError(t, err)
	require.Equal(t, "assignment-1", first.GetExecutor().GetAssignmentId())
	require.Equal(t, EventManifestUpdated, (<-publisher.events).Name)

	duplicate, err := manager.AcceptManifest(context.Background(), sid, publisher, executorManifest("client-a", "connection-a", 1, "lookup"))
	require.NoError(t, err)
	require.True(t, proto.Equal(first.GetExecutor(), duplicate.GetExecutor()))
	requireNoEvent(t, publisher)

	updated, err := manager.AcceptManifest(context.Background(), sid, publisher, executorManifest("client-a", "connection-a", 2, "lookup", "write"))
	require.NoError(t, err)
	require.Equal(t, "assignment-1", updated.GetExecutor().GetAssignmentId())
	<-publisher.events

	replacement, err := manager.AcceptManifest(context.Background(), sid, publisher, executorManifest("client-b", "connection-b", 1, "lookup"))
	require.NoError(t, err)
	require.Equal(t, "assignment-2", replacement.GetExecutor().GetAssignmentId())
	require.NotEqual(t, updated.GetExecutor().GetConnectionId(), replacement.GetExecutor().GetConnectionId())
}

func TestAcceptManifestRejectsMissingIdentityAndRevisionConflicts(t *testing.T) {
	manager := NewManager()
	manager.newAssignmentID = func() string { return "assignment-1" }
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 8)}
	sid := sessionstream.SessionId("session-1")

	_, err := manager.AcceptManifest(context.Background(), sid, publisher, executorManifest("", "connection-a", 1, "lookup"))
	requireManifestErrorCode(t, err, ManifestErrorIdentityMissing)

	_, err = manager.AcceptManifest(context.Background(), sid, publisher, executorManifest("client-a", "connection-a", 2, "lookup"))
	require.NoError(t, err)
	<-publisher.events

	_, err = manager.AcceptManifest(context.Background(), sid, publisher, executorManifest("client-a", "connection-a", 1, "lookup"))
	requireManifestErrorCode(t, err, ManifestErrorRevisionRegression)

	_, err = manager.AcceptManifest(context.Background(), sid, publisher, executorManifest("client-a", "connection-a", 2, "different"))
	requireManifestErrorCode(t, err, ManifestErrorRevisionConflict)
}

func TestAcceptManifestPublicationFailureRestoresPreviousAssignment(t *testing.T) {
	manager := NewManager()
	ids := []string{"assignment-1", "assignment-2"}
	manager.newAssignmentID = func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}
	sid := sessionstream.SessionId("session-1")
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 2)}
	first, err := manager.AcceptManifest(context.Background(), sid, publisher, executorManifest("client-a", "connection-a", 1, "lookup"))
	require.NoError(t, err)
	<-publisher.events

	publishErr := errors.New("timeline unavailable")
	_, err = manager.AcceptManifest(context.Background(), sid, failManifestPublisher{err: publishErr}, executorManifest("client-b", "connection-b", 1, "lookup"))
	require.ErrorIs(t, err, publishErr)

	manager.mu.Lock()
	current := cloneManifestUpdated(manager.manifests[sid].updated)
	manager.mu.Unlock()
	require.True(t, proto.Equal(first.GetExecutor(), current.GetExecutor()))
}

func TestRequestCapturesAssignmentAndOwnershipChangeAffectsFutureCallsOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	manager := NewManager()
	ids := []string{"assignment-a", "assignment-b"}
	manager.newAssignmentID = func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 16)}
	sid := sessionstream.SessionId("session-1")

	assignmentA, err := manager.AcceptManifest(ctx, sid, publisher, executorManifest("client-a", "connection-a", 1, "mutate"))
	require.NoError(t, err)
	<-publisher.events
	first := startManagerRequest(ctx, manager, publisher, sid, Request{MessageID: "msg-1", ToolCallID: "call-1", ToolName: "mutate"})
	requestA := requireExecutorRequestEvent(t, ctx, publisher)
	require.True(t, proto.Equal(assignmentA.GetExecutor(), requestA.GetExecutor()))

	assignmentB, err := manager.AcceptManifest(ctx, sid, publisher, executorManifest("client-b", "connection-b", 1, "mutate"))
	require.NoError(t, err)
	<-publisher.events

	require.NoError(t, manager.HandleResult(ctx, executorResultCommand(sid, "call-1", "mutate", assignmentA.GetExecutor()), nil, publisher))
	<-publisher.events
	require.Equal(t, "success", requireRequestOutcome(t, ctx, first).GetStatus())

	second := startManagerRequest(ctx, manager, publisher, sid, Request{MessageID: "msg-2", ToolCallID: "call-2", ToolName: "mutate"})
	requestB := requireExecutorRequestEvent(t, ctx, publisher)
	require.True(t, proto.Equal(assignmentB.GetExecutor(), requestB.GetExecutor()))

	err = manager.HandleResult(ctx, executorResultCommand(sid, "call-2", "mutate", assignmentA.GetExecutor()), nil, publisher)
	requireInvocationErrorCode(t, err, InvocationErrorExecutorMismatch)
	require.NoError(t, manager.HandleResult(ctx, executorResultCommand(sid, "call-2", "mutate", assignmentB.GetExecutor()), nil, publisher))
	<-publisher.events
	require.Equal(t, "success", requireRequestOutcome(t, ctx, second).GetStatus())
}

func TestConcurrentManifestAcceptanceLeavesOneCoherentAssignment(t *testing.T) {
	manager := NewManager()
	publisher := &capturePublisher{events: make(chan sessionstream.Event, 64)}
	sid := sessionstream.SessionId("session-1")
	const count = 20
	acks := make(chan *toolv1.FrontendToolManifestUpdated, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			ack, err := manager.AcceptManifest(context.Background(), sid, publisher, executorManifest(
				fmt.Sprintf("client-%d", index), fmt.Sprintf("connection-%d", index), 1, "lookup",
			))
			acks <- ack
			errs <- err
		}(i)
	}
	wg.Wait()
	close(acks)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	for ack := range acks {
		require.True(t, validExecutor(ack.GetExecutor()))
	}
	manager.mu.Lock()
	current := cloneManifestUpdated(manager.manifests[sid].updated)
	manager.mu.Unlock()
	require.True(t, validExecutor(current.GetExecutor()))
	require.Len(t, publisher.events, count)
}

func executorManifest(clientID, connectionID string, revision uint64, tools ...string) *toolv1.FrontendToolManifestCommand {
	descriptors := make([]*toolv1.FrontendToolDescriptor, 0, len(tools))
	for _, tool := range tools {
		descriptors = append(descriptors, &toolv1.FrontendToolDescriptor{Name: tool, Available: true})
	}
	return &toolv1.FrontendToolManifestCommand{
		ClientInstanceId: clientID,
		ConnectionId:     connectionID,
		Revision:         revision,
		Tools:            descriptors,
	}
}

func executorResultCommand(sid sessionstream.SessionId, callID, toolName string, executor *toolv1.FrontendToolExecutor) sessionstream.Command {
	return sessionstream.Command{SessionId: sid, Name: CommandResult, Payload: &toolv1.FrontendToolResultCommand{
		ToolCallId: callID,
		ToolName:   toolName,
		Status:     "success",
		Executor:   cloneExecutor(executor),
	}}
}

func requireExecutorRequestEvent(t *testing.T, ctx context.Context, publisher *capturePublisher) *toolv1.FrontendToolCallRequested {
	t.Helper()
	select {
	case event := <-publisher.events:
		require.Equal(t, EventCallRequested, event.Name)
		payload, ok := event.Payload.(*toolv1.FrontendToolCallRequested)
		require.True(t, ok)
		require.True(t, validExecutor(payload.GetExecutor()))
		return payload
	case <-ctx.Done():
		t.Fatalf("timed out waiting for request: %v", ctx.Err())
		return nil
	}
}

func requireManifestErrorCode(t *testing.T, err error, expected ManifestErrorCode) {
	t.Helper()
	var manifestErr *ManifestError
	require.ErrorAs(t, err, &manifestErr)
	require.Equal(t, expected, manifestErr.Code)
}

type failManifestPublisher struct{ err error }

func (p failManifestPublisher) Publish(_ context.Context, event sessionstream.Event) error {
	if event.Name == EventManifestUpdated {
		return p.err
	}
	return nil
}
