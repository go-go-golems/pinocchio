package chatapp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

// eventRecorder collects backend events by name through the engine hooks.
type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	name    string
	payload map[string]any
}

func (r *eventRecorder) hooks() Option {
	return WithHooks(Hooks{OnBackendEvent: func(_, name string, payload map[string]any) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.events = append(r.events, recordedEvent{name: name, payload: payload})
	}})
}

func (r *eventRecorder) named(name string) []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []map[string]any
	for _, ev := range r.events {
		if ev.name == name {
			out = append(out, ev.payload)
		}
	}
	return out
}

// memTurnStore keeps the latest saved turn per conversation and phase, so
// history really carries over between runs.
type memTurnStore struct {
	mu     sync.Mutex
	latest map[string]chatstore.TurnSnapshot
	saves  int
}

func newMemTurnStore() *memTurnStore {
	return &memTurnStore{latest: map[string]chatstore.TurnSnapshot{}}
}

func (s *memTurnStore) Save(_ context.Context, convID, sessionID, turnID, phase string, createdAtMs int64, payload string, _ chatstore.TurnSaveOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves++
	s.latest[convID+"|"+phase] = chatstore.TurnSnapshot{ConvID: convID, SessionID: sessionID, TurnID: turnID, Phase: phase, CreatedAtMs: createdAtMs, Payload: payload}
	return nil
}

func (s *memTurnStore) List(context.Context, chatstore.TurnQuery) ([]chatstore.TurnSnapshot, error) {
	return nil, nil
}

func (s *memTurnStore) LoadLatestTurn(_ context.Context, convID, phase string) (*chatstore.TurnSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.latest[convID+"|"+phase]
	if !ok {
		return nil, nil
	}
	return &snap, nil
}

func (s *memTurnStore) Close() error { return nil }

func (s *memTurnStore) final(t *testing.T, sid string) *turns.Turn {
	t.Helper()
	snap, err := s.LoadLatestTurn(context.Background(), sid, "final")
	require.NoError(t, err)
	require.NotNil(t, snap, "no final turn saved for %s", sid)
	turn, err := serde.FromYAML([]byte(snap.Payload))
	require.NoError(t, err)
	return turn
}

func submit(t *testing.T, e *Engine, hub *sessionstream.Hub, sid string, req PromptRequest) error {
	t.Helper()
	id := fmt.Sprintf("req-%s-%d", sid, time.Now().UnixNano())
	e.setPendingRequest(id, req)
	return hub.Submit(context.Background(), sessionstream.SessionId(sid), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: id})
}

func waitIdle(t *testing.T, e *Engine, sid string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, e.WaitIdle(ctx, sessionstream.SessionId(sid)))
}

// toolCallingEngine asks for the "noop" tool on every call, unless the turn's
// tool configuration forbids tools (geppetto's final answer), then it answers.
type toolCallingEngine struct {
	mu    sync.Mutex
	calls int
}

func (e *toolCallingEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	e.mu.Lock()
	e.calls++
	n := e.calls
	e.mu.Unlock()
	if cfg, ok, _ := engine.KeyToolConfig.Get(t.Data); ok && cfg.ToolChoice == engine.ToolChoiceNone {
		turns.AppendBlock(t, turns.NewAssistantTextBlock("final answer"))
		return t, nil
	}
	turns.AppendBlock(t, turns.NewToolCallBlock(fmt.Sprintf("call-%d", n), "noop", map[string]any{}))
	return t, nil
}

func noopRegistry(t *testing.T) geptools.ToolRegistry {
	t.Helper()
	reg := geptools.NewInMemoryToolRegistry()
	def, err := geptools.NewToolFromFunc("noop", "does nothing", func(struct{}) (string, error) { return "ok", nil })
	require.NoError(t, err)
	require.NoError(t, reg.RegisterTool("noop", *def))
	return reg
}

func TestBusyRejectRefusesASecondPromptAndPublishesNothing(t *testing.T) {
	rec := &eventRecorder{}
	e := newTestEngine(rec.hooks())
	hub := newTestHub(t, e)

	require.NoError(t, submit(t, e, hub, "busy-reject", PromptRequest{Prompt: "first", Runtime: slowAnswerRuntime()}))
	err := submit(t, e, hub, "busy-reject", PromptRequest{Prompt: "second", Runtime: answerRuntime()})
	require.ErrorIs(t, err, ErrRunActive)
	require.Len(t, rec.named(EventUserMessageAccepted), 1, "the refused prompt must not be published")

	require.NoError(t, hub.Submit(context.Background(), "busy-reject", CommandStopInference, &chatappv1.StopInferenceCommand{}))
	waitIdle(t, e, "busy-reject")
}

func TestBusyReplaceCancelsTheRunningTurn(t *testing.T) {
	rec := &eventRecorder{}
	e := newTestEngine(rec.hooks(), WithBusyPolicy(BusyReplace))
	hub := newTestHub(t, e)

	require.NoError(t, submit(t, e, hub, "busy-replace", PromptRequest{Prompt: "first", Runtime: slowAnswerRuntime()}))
	require.NoError(t, submit(t, e, hub, "busy-replace", PromptRequest{Prompt: "second", Runtime: answerRuntime()}))
	waitIdle(t, e, "busy-replace")

	require.Len(t, rec.named(EventUserMessageAccepted), 2)
	require.Len(t, rec.named(EventChatRunStopped), 1)
	require.Len(t, rec.named(EventChatRunFinished), 1)
}

func TestPromptWithoutRuntimeIsRejected(t *testing.T) {
	e := newTestEngine()
	hub := newTestHub(t, e)
	err := submit(t, e, hub, "no-runtime", PromptRequest{Prompt: "hello"})
	require.ErrorIs(t, err, ErrNoRuntime)
}

func TestRoundLimitWithFinalAnswerFinishesTruncated(t *testing.T) {
	rec := &eventRecorder{}
	store := newMemTurnStore()
	e := newTestEngine(rec.hooks(), WithTurnStore(store))
	hub := newTestHub(t, e)
	final := func(n int) turns.Block {
		return turns.NewUserTextBlock(fmt.Sprintf("you used %d rounds; answer now", n))
	}

	require.NoError(t, submit(t, e, hub, "limit-answered", PromptRequest{Prompt: "loop", Runtime: &infruntime.ComposedRuntime{
		Engine:   &toolCallingEngine{},
		Registry: noopRegistry(t),
		Loop:     &toolloop.LoopConfig{MaxIterations: 2, FinalAnswer: &toolloop.FinalAnswer{Prompt: final}},
	}}))
	waitIdle(t, e, "limit-answered")

	finished := rec.named(EventChatRunFinished)
	require.Len(t, finished, 1)
	require.Equal(t, true, finished[0]["truncated"])
	require.Equal(t, CodeMaxRounds, finished[0]["reason"])
	require.Empty(t, rec.named(EventChatRunFailed))

	turn := store.final(t, "limit-answered")
	last := turn.Blocks[len(turn.Blocks)-1]
	require.Equal(t, turns.BlockKindLLMText, last.Kind)
	require.Equal(t, "final answer", last.Payload[turns.PayloadKeyText])
}

func TestRoundLimitWithoutFinalAnswerFailsWithMaxRounds(t *testing.T) {
	rec := &eventRecorder{}
	e := newTestEngine(rec.hooks())
	hub := newTestHub(t, e)

	require.NoError(t, submit(t, e, hub, "limit-failed", PromptRequest{Prompt: "loop", Runtime: &infruntime.ComposedRuntime{
		Engine:   &toolCallingEngine{},
		Registry: noopRegistry(t),
		Loop:     &toolloop.LoopConfig{MaxIterations: 2},
	}}))
	waitIdle(t, e, "limit-failed")

	failed := rec.named(EventChatRunFailed)
	require.Len(t, failed, 1)
	require.Equal(t, CodeMaxRounds, failed[0]["code"])
}

type quotaError struct{}

func (quotaError) Error() string { return "the session's token budget is used up" }
func (quotaError) Code() string  { return "quota_exceeded" }

func TestRoundGuardErrorFailsTheRunWithItsCode(t *testing.T) {
	rec := &eventRecorder{}
	e := newTestEngine(rec.hooks())
	hub := newTestHub(t, e)
	tooling := &toolCallingEngine{}
	guard := func(_ context.Context, _ *turns.Turn, round int) error {
		if round >= 1 {
			return quotaError{}
		}
		return nil
	}

	require.NoError(t, submit(t, e, hub, "guard", PromptRequest{Prompt: "loop", Runtime: &infruntime.ComposedRuntime{
		Engine: tooling, Registry: noopRegistry(t), RoundGuard: guard,
	}}))
	waitIdle(t, e, "guard")

	failed := rec.named(EventChatRunFailed)
	require.Len(t, failed, 1)
	require.Equal(t, "quota_exceeded", failed[0]["code"])
	require.Equal(t, 1, tooling.calls, "the guard stops the run before the second model call")
}

func TestErrorClassifierAndTextOptions(t *testing.T) {
	rec := &eventRecorder{}
	e := newTestEngine(rec.hooks(),
		WithErrorClassifier(func(error) string { return "custom_code" }),
		WithErrorText(func(code string, _ error) string { return "something went wrong (" + code + ")" }))
	hub := newTestHub(t, e)

	require.NoError(t, submit(t, e, hub, "classify", PromptRequest{Prompt: "fail", Runtime: &infruntime.ComposedRuntime{Engine: immediateErrorEngine{}}}))
	waitIdle(t, e, "classify")

	failed := rec.named(EventChatRunFailed)
	require.Len(t, failed, 1)
	require.Equal(t, "custom_code", failed[0]["code"])
	require.Equal(t, "something went wrong (custom_code)", failed[0]["error"])
}

func TestDefaultClassifier(t *testing.T) {
	require.Equal(t, CodeAuthenticationError, ClassifyError(errors.New("status=401 unauthorized")))
	require.Equal(t, CodeRateLimited, ClassifyError(errors.New("429 Too Many Requests")))
	require.Equal(t, CodePromptTooLarge, ClassifyError(errors.New("maximum context length exceeded")))
	require.Equal(t, CodeUpstreamError, ClassifyError(errors.New("connection reset")))
}

// callThenWaitEngine appends a tool call and then blocks until the run is
// cancelled, leaving the call without a result.
type callThenWaitEngine struct{ started chan struct{} }

func (e callThenWaitEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	turns.AppendBlock(t, turns.NewToolCallBlock("call-stuck", "noop", map[string]any{}))
	close(e.started)
	<-ctx.Done()
	return t, ctx.Err()
}

func TestStoppedTurnIsKeptWithItsUnansweredToolCallClosed(t *testing.T) {
	store := newMemTurnStore()
	e := newTestEngine(WithTurnStore(store))
	hub := newTestHub(t, e)
	started := make(chan struct{})

	require.NoError(t, submit(t, e, hub, "stopped", PromptRequest{Prompt: "run the tests", Runtime: &infruntime.ComposedRuntime{Engine: callThenWaitEngine{started: started}}}))
	<-started
	require.NoError(t, hub.Submit(context.Background(), "stopped", CommandStopInference, &chatappv1.StopInferenceCommand{}))
	waitIdle(t, e, "stopped")

	turn := store.final(t, "stopped")
	var user, call, result *turns.Block
	for i := range turn.Blocks {
		b := &turn.Blocks[i]
		switch b.Kind {
		case turns.BlockKindUser:
			user = b
		case turns.BlockKindToolCall:
			call = b
		case turns.BlockKindToolUse:
			result = b
		case turns.BlockKindLLMText, turns.BlockKindSystem, turns.BlockKindReasoning, turns.BlockKindOther:
		}
	}
	require.NotNil(t, user, "the person's message stays in the history")
	require.Equal(t, "run the tests", user.Payload[turns.PayloadKeyText])
	require.NotNil(t, call)
	require.NotNil(t, result, "the unanswered call is closed")
	require.Equal(t, "call-stuck", result.Payload[turns.PayloadKeyID])
	require.Equal(t, DefaultUnansweredToolResult, result.Payload[turns.PayloadKeyError])
}

// promptRecorder records the blocks of every model call and answers "ok".
type promptRecorder struct {
	mu    sync.Mutex
	calls [][]turns.Block
}

func (r *promptRecorder) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	r.mu.Lock()
	r.calls = append(r.calls, append([]turns.Block(nil), t.Clone().Blocks...))
	r.mu.Unlock()
	turns.AppendBlock(t, turns.NewAssistantTextBlock("ok"))
	return t, nil
}

func blockTexts(bs []turns.Block) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		text, _ := b.Payload[turns.PayloadKeyText].(string)
		out = append(out, b.Kind.String()+":"+text)
	}
	return out
}

func TestCallerBlocksFollowTheMessageAndHistoryIsAppendOnly(t *testing.T) {
	store := newMemTurnStore()
	e := newTestEngine(WithTurnStore(store))
	hub := newTestHub(t, e)
	rec := &promptRecorder{}
	rt := &infruntime.ComposedRuntime{Engine: rec}

	require.NoError(t, submit(t, e, hub, "append-only", PromptRequest{Prompt: "first", Runtime: rt, Blocks: []turns.Block{turns.NewUserTextBlock("[state] 10:00")}}))
	waitIdle(t, e, "append-only")
	require.NoError(t, submit(t, e, hub, "append-only", PromptRequest{Prompt: "second", Runtime: rt, Blocks: []turns.Block{turns.NewUserTextBlock("[state] 10:05")}}))
	waitIdle(t, e, "append-only")

	require.Len(t, rec.calls, 2)
	require.Equal(t, []string{"user:first", "user:[state] 10:00"}, blockTexts(rec.calls[0]))
	// The second call starts with everything the first call sent, then its answer,
	// then the new message and its block: nothing earlier was edited or removed.
	require.Equal(t, append(blockTexts(rec.calls[0]), "llm_text:ok", "user:second", "user:[state] 10:05"), blockTexts(rec.calls[1]))
}
