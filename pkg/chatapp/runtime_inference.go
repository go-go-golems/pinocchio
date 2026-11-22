package chatapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

func (e *Engine) handleStartInference(ctx context.Context, cmd sessionstream.Command, _ *sessionstream.Session, pub sessionstream.EventPublisher) error {
	payload, ok := cmd.Payload.(*chatappv1.StartInferenceCommand)
	if !ok || payload == nil {
		return fmt.Errorf("start inference payload must be %T, got %T", &chatappv1.StartInferenceCommand{}, cmd.Payload)
	}
	pending := e.takePendingRequest(strings.TrimSpace(payload.GetRequestId()))
	prompt := strings.TrimSpace(pending.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(payload.GetPrompt())
	}
	if len(pending.Attachments) == 0 && len(payload.GetAttachments()) > 0 {
		// Command submitted without an in-process pending request (e.g. replay or
		// external submitter): reconstruct attachments from the wire payload.
		pending.Attachments = AttachmentsFromProto(payload.GetAttachments())
	}
	if prompt == "" && len(pending.Attachments) == 0 {
		return fmt.Errorf("start inference: the prompt is empty and there are no attachments")
	}
	if pending.Runtime == nil || pending.Runtime.Engine == nil {
		return ErrNoRuntime
	}
	messageID, err := e.newMessageID()
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(publishContext(ctx))
	run := &activeRun{messageID: messageID, cancel: cancel, done: make(chan struct{})}
	previous, err := e.reserveRun(cmd.SessionId, run)
	if err != nil {
		cancel()
		return err
	}
	if previous != nil {
		previous.cancel()
		<-previous.done
	}
	userMessageID := messageID + userMessageIDSuffix
	if err := e.publish(ctx, cmd.SessionId, pub, EventUserMessageAccepted, &chatappv1.ChatUserMessageAccepted{MessageId: userMessageID, Role: "user", Text: prompt, Content: prompt, Status: "accepted", Attachments: clientAttachmentsToProto(pending.Attachments)}); err != nil {
		e.clearRun(cmd.SessionId, messageID)
		cancel()
		close(run.done)
		return err
	}
	go e.runPrompt(runCtx, cmd.SessionId, messageID, pending, prompt, pub, run.done)
	return nil
}

func (e *Engine) handleStopInference(_ context.Context, cmd sessionstream.Command, _ *sessionstream.Session, _ sessionstream.EventPublisher) error {
	if current := e.currentRun(cmd.SessionId); current != nil {
		current.cancel()
	}
	return nil
}

func (e *Engine) runPrompt(ctx context.Context, sid sessionstream.SessionId, messageID string, pending PromptRequest, prompt string, pub sessionstream.EventPublisher, done chan struct{}) {
	defer close(done)
	defer e.clearRun(sid, messageID)
	e.runRuntimeInference(ctx, sid, messageID, prompt, pending, pub)
}

// CodeHistoryError is the code of a run that failed because its history could
// not be loaded or decoded.
const CodeHistoryError = "history_error"

// CodeToolsError: a RunToolsFunc failed before the run started.
const CodeToolsError = "tools_error"

func (e *Engine) runRuntimeInference(ctx context.Context, sid sessionstream.SessionId, messageID, prompt string, pending PromptRequest, pub sessionstream.EventPublisher) {
	runtime := pending.Runtime
	// ctx is the run's context and may already be cancelled (Stop, or a
	// replacing prompt). Publishing the start and loading history use a
	// non-cancellable context so a cancelled run still ends with ChatRunStopped
	// and its turn is kept.
	if err := e.publish(publishContext(ctx), sid, pub, EventChatRunStarted, &chatappv1.ChatRunStarted{MessageId: messageID, Prompt: prompt, Correlation: runCorrelationInfo(sid, messageID)}); err != nil {
		return
	}

	baseSink := gepevents.EventSink(&runtimeEventSink{publishCtx: publishContext(ctx), sessionID: sid, messageID: messageID, prompt: prompt, pub: pub, engine: e, batchInterval: e.streamPatchBatch.Interval})
	eventSink := baseSink
	if runtime.WrapSink != nil {
		wrapped, err := runtime.WrapSink(baseSink)
		if err != nil {
			e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeUpstreamError, err)
			return
		}
		eventSink = wrapped
	}
	sink, ok := baseSink.(*runtimeEventSink)
	if !ok {
		e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeUpstreamError, errors.New("internal runtime sink type assertion failed"))
		return
	}
	runCtx := ctx
	if pending.RuntimeContext != nil {
		decorated := pending.RuntimeContext(runCtx, sid, messageID, pub)
		if decorated != nil {
			runCtx = decorated
		}
	}

	tools := RunTools{Registry: runtime.Registry, Executor: runtime.ToolExecutor, Config: geptools.DefaultToolConfig()}
	if runtime.ToolConfig != nil {
		tools.Config = *runtime.ToolConfig
	}
	for _, fn := range e.runTools {
		nextCtx, nextTools, err := fn(runCtx, RunScope{SessionID: sid, MessageID: messageID, Publisher: pub}, tools)
		if err != nil {
			e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeToolsError, fmt.Errorf("prepare run tools: %w", err))
			return
		}
		if nextCtx != nil {
			runCtx = nextCtx
		}
		nextTools.Config = tools.Config
		tools = nextTools
	}
	if runtime.WrapToolExecutor != nil {
		tools.Executor = runtime.WrapToolExecutor(tools.ExecutorOrDefault())
	}

	sess := gepsession.NewSessionWithID(string(sid))
	sess.Builder = &enginebuilder.Builder{
		Base:         runtime.Engine,
		Registry:     tools.Registry,
		ToolExecutor: tools.Executor,
		EventSinks:   []gepevents.EventSink{eventSink},
		LoopConfig:   runtime.Loop,
		ToolConfig:   runtime.ToolConfig,
		RoundGuard:   runtime.RoundGuard,
	}

	if pending.InitialTurn != nil {
		sess.Append(pending.InitialTurn.Clone())
	} else {
		// Load conversation history: the last saved turn holds the whole
		// conversation. AppendNewTurnFromUserPrompt clones it and adds the new
		// user block.
		if e.turnStore != nil {
			snapshot, err := e.turnStore.LoadLatestTurn(publishContext(ctx), string(sid), "final")
			if err != nil {
				e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeHistoryError, fmt.Errorf("load conversation history: %w", err))
				return
			}
			if snapshot != nil {
				turn, err := serde.FromYAML([]byte(snapshot.Payload))
				if err != nil {
					e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeHistoryError, fmt.Errorf("decode conversation history: %w", err))
					return
				}
				if turn == nil {
					e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeHistoryError, errors.New("decode conversation history: empty turn"))
					return
				}
				sess.Append(turn)
			}
		}

		var err error
		if images := AttachmentsToTurnImages(pending.Attachments); len(images) > 0 {
			_, err = sess.AppendNewTurnFromUserMessage(prompt, images)
		} else {
			_, err = sess.AppendNewTurnFromUserPrompt(prompt)
		}
		if err != nil {
			e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeUpstreamError, err)
			return
		}
	}
	// The caller's extra blocks for this turn (live state, instructions):
	// appended after the person's message and never removed later.
	if latest := sess.Latest(); latest != nil {
		for _, b := range pending.Blocks {
			turns.AppendBlock(latest, b)
		}
	}
	assistantBlockOffset := countAssistantBlocks(sess.Latest())
	var output *turns.Turn
	handle, err := sess.StartInference(runCtx)
	if err == nil {
		output, err = handle.Wait()
	}
	final := output
	if final == nil {
		final = sess.Latest()
	}
	outcome := e.decideOutcome(ctx, err)
	e.endRun(ctx, sid, runtime.RuntimeKey, pending, final)

	switch outcome.kind {
	case outcomeStopped:
		if !sink.IsTerminal() {
			if drainErr := sink.drainStreamPatches(); drainErr != nil {
				e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeUpstreamError, fmt.Errorf("flush stream patches before stop: %w", drainErr))
				return
			}
			_ = sink.finishActiveTextSegment("stopped", "stopped", "")
			_ = e.publish(publishContext(ctx), sid, pub, EventChatRunStopped, &chatappv1.ChatRunStopped{MessageId: messageID, Status: "stopped", Correlation: runCorrelationInfo(sid, messageID)})
		}
		return
	case outcomeFailed:
		if !sink.IsTerminal() {
			if drainErr := sink.drainStreamPatches(); drainErr != nil {
				e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeUpstreamError, fmt.Errorf("flush stream patches before failure: %w", drainErr))
				return
			}
			_ = sink.finishActiveTextSegment("failed", "error", "")
			e.publishRunFailed(publishContext(ctx), sid, pub, messageID, outcome.code, err)
		}
		return
	case outcomeFinished:
	}
	if sink.IsTerminal() {
		return
	}
	if drainErr := sink.drainStreamPatches(); drainErr != nil {
		e.publishRunFailed(publishContext(ctx), sid, pub, messageID, CodeUpstreamError, fmt.Errorf("flush stream patches before finish: %w", drainErr))
		return
	}
	if sink.HasActiveTextSegment() {
		_ = sink.finishActiveTextSegment("finished", "stop", "")
	} else if !sink.HasTextSegment() {
		_ = e.publishFallbackAssistantText(publishContext(ctx), sid, pub, messageID, prompt, output, assistantBlockOffset)
	}
	_ = e.publish(publishContext(ctx), sid, pub, EventChatRunFinished, &chatappv1.ChatRunFinished{MessageId: messageID, Status: "finished", Correlation: runCorrelationInfo(sid, messageID), Truncated: outcome.truncated, Reason: outcome.reason})
}

// endRun keeps the run's turn as the conversation, whatever the outcome:
// unanswered tool calls are closed, the turn is saved as "final" when the
// engine has a turn store, and OnFinalTurn receives it.
func (e *Engine) endRun(ctx context.Context, sid sessionstream.SessionId, runtimeKey string, pending PromptRequest, t *turns.Turn) {
	if t == nil {
		return
	}
	closeUnansweredToolCalls(t, e.unansweredToolResult)
	e.persistRuntimeTurnSnapshot(publishContext(ctx), sid, runtimeKey, "final", t)
	if pending.OnFinalTurn != nil {
		pending.OnFinalTurn(t.Clone())
	}
}

func publishContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func runCorrelationInfo(sid sessionstream.SessionId, messageID string) *chatappv1.CorrelationInfo {
	return &chatappv1.CorrelationInfo{SessionId: string(sid), RunId: messageID}
}

func fallbackTextCorrelationInfo(sid sessionstream.SessionId, messageID string, segment int32) *chatappv1.CorrelationInfo {
	segmentID := textSegmentMessageID(messageID, segment)
	return &chatappv1.CorrelationInfo{SessionId: string(sid), RunId: messageID, SegmentId: segmentID}
}

func (e *Engine) publishFallbackAssistantText(ctx context.Context, sid sessionstream.SessionId, pub sessionstream.EventPublisher, messageID, prompt string, output *turns.Turn, assistantBlockOffset int) error {
	text := assistantTextFromTurnAfter(output, assistantBlockOffset)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	const segment int32 = 1
	textMessageID := textSegmentMessageID(messageID, segment)
	corr := fallbackTextCorrelationInfo(sid, messageID, segment)
	if err := e.publish(ctx, sid, pub, EventChatTextSegmentStarted, &chatappv1.ChatTextSegmentStarted{MessageId: textMessageID, Role: "assistant", Prompt: prompt, Status: "streaming", Streaming: true, Correlation: corr}); err != nil {
		return err
	}
	return e.publish(ctx, sid, pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: textMessageID, Role: "assistant", Prompt: prompt, Text: text, Content: text, Status: "finished", Streaming: false, Final: true, FinishReason: "stop", Correlation: corr})
}

func countAssistantBlocks(turn *turns.Turn) int {
	if turn == nil {
		return 0
	}
	count := 0
	for _, block := range turn.Blocks {
		if block.Role == turns.RoleAssistant {
			count++
		}
	}
	return count
}

func assistantTextFromTurnAfter(turn *turns.Turn, skip int) string {
	if turn == nil {
		return ""
	}
	parts := make([]string, 0, len(turn.Blocks))
	seen := 0
	for _, block := range turn.Blocks {
		if block.Role != turns.RoleAssistant {
			continue
		}
		if seen < skip {
			seen++
			continue
		}
		seen++
		text, _ := block.Payload[turns.PayloadKeyText].(string)
		if strings.TrimSpace(text) == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "")
}

func (e *Engine) persistRuntimeTurnSnapshot(ctx context.Context, sid sessionstream.SessionId, runtimeKey string, phase string, t *turns.Turn) {
	if e == nil || e.turnStore == nil || t == nil || strings.TrimSpace(string(sid)) == "" {
		return
	}
	turnID := strings.TrimSpace(t.ID)
	if turnID == "" {
		turnID = "turn"
	}
	phase = strings.TrimSpace(phase)
	if phase == "" {
		phase = "snapshot"
	}
	runtimeKey = strings.TrimSpace(runtimeKey)
	if runtimeKey == "" {
		runtimeKey = "default"
	}
	payload, err := serde.ToYAML(t, serde.Options{})
	if err != nil {
		log.Warn().Err(err).Str("session", string(sid)).Msg("chatapp: encode conversation turn")
		return
	}
	inferenceID := ""
	if v, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		inferenceID = strings.TrimSpace(v)
	}
	if err := e.turnStore.Save(ctx, string(sid), string(sid), turnID, phase, time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
		RuntimeKey:  runtimeKey,
		InferenceID: inferenceID,
	}); err != nil {
		log.Warn().Err(err).Str("session", string(sid)).Str("phase", phase).Msg("chatapp: save conversation turn")
	}
}

func (e *Engine) publishRunFailed(ctx context.Context, sid sessionstream.SessionId, pub sessionstream.EventPublisher, messageID, code string, err error) {
	_ = e.publish(ctx, sid, pub, EventChatRunFailed, &chatappv1.ChatRunFailed{MessageId: messageID, Status: "failed", Code: code, Error: e.failureText(code, err), Correlation: runCorrelationInfo(sid, messageID)})
}

func (e *Engine) publish(ctx context.Context, sid sessionstream.SessionId, pub sessionstream.EventPublisher, name string, payload proto.Message) error {
	if payload == nil {
		return fmt.Errorf("event %s payload is nil", name)
	}
	if e.hooks.OnBackendEvent != nil {
		e.hooks.OnBackendEvent(string(sid), name, protoMessageAsMap(payload))
	}
	return pub.Publish(ctx, sessionstream.Event{Name: name, SessionId: sid, Payload: payload})
}
