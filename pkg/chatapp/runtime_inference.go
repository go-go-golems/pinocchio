package chatapp

import (
	"context"
	"fmt"
	"strings"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
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
	if prompt == "" {
		prompt = "Explain evtstream"
	}
	messageID := e.nextMessageID()
	userMessageID := messageID + "-user"
	if err := e.publish(ctx, cmd.SessionId, pub, EventUserMessageAccepted, &chatappv1.ChatUserMessageAccepted{MessageId: userMessageID, Role: "user", Text: prompt, Content: prompt, Status: "accepted"}); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(publishContext(ctx))
	run := &activeRun{messageID: messageID, cancel: cancel, done: make(chan struct{})}
	if previous := e.swapRun(cmd.SessionId, run); previous != nil {
		previous.cancel()
		<-previous.done
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
	if pending.Runtime != nil && pending.Runtime.Engine != nil {
		e.runRuntimeInference(ctx, sid, messageID, prompt, pending.Runtime, pub)
		return
	}
	e.runDemoInference(ctx, sid, messageID, prompt, pub)
}

func (e *Engine) runRuntimeInference(ctx context.Context, sid sessionstream.SessionId, messageID, prompt string, runtime *infruntime.ComposedRuntime, pub sessionstream.EventPublisher) {
	if runtime == nil || runtime.Engine == nil {
		e.runDemoInference(ctx, sid, messageID, prompt, pub)
		return
	}
	if err := e.publish(ctx, sid, pub, EventChatRunStarted, &chatappv1.ChatRunStarted{MessageId: messageID, Prompt: prompt, Correlation: runCorrelationInfo(sid, messageID)}); err != nil {
		return
	}

	baseSink := gepevents.EventSink(&runtimeEventSink{publishCtx: publishContext(ctx), sessionID: sid, messageID: messageID, prompt: prompt, pub: pub, engine: e})
	eventSink := baseSink
	if runtime.WrapSink != nil {
		wrapped, err := runtime.WrapSink(baseSink)
		if err != nil {
			e.publishRunFailed(publishContext(ctx), sid, pub, messageID, err.Error())
			return
		}
		eventSink = wrapped
	}
	sink, ok := baseSink.(*runtimeEventSink)
	if !ok {
		e.publishRunFailed(publishContext(ctx), sid, pub, messageID, "internal runtime sink type assertion failed")
		return
	}
	sess := gepsession.NewSessionWithID(string(sid))
	sess.Builder = &enginebuilder.Builder{
		Base:       runtime.Engine,
		EventSinks: []gepevents.EventSink{eventSink},
	}

	// Load conversation history: the last persisted turn contains the full
	// conversation as an accumulator. AppendNewTurnFromUserPrompt will clone
	// it and add the new user block, giving the LLM the full context.
	if e.turnStore != nil {
		snapshot, err := e.turnStore.LoadLatestTurn(ctx, string(sid), "final")
		if err != nil {
			e.publishRunFailed(publishContext(ctx), sid, pub, messageID, fmt.Sprintf("load conversation history: %v", err))
			return
		}
		if snapshot != nil {
			turn, err := serde.FromYAML([]byte(snapshot.Payload))
			if err != nil {
				e.publishRunFailed(publishContext(ctx), sid, pub, messageID, fmt.Sprintf("decode conversation history: %v", err))
				return
			}
			if turn == nil {
				e.publishRunFailed(publishContext(ctx), sid, pub, messageID, "decode conversation history: empty turn")
				return
			}
			sess.Append(turn)
		}
	}

	_, err := sess.AppendNewTurnFromUserPrompt(prompt)
	if err != nil {
		e.publishRunFailed(publishContext(ctx), sid, pub, messageID, err.Error())
		return
	}
	handle, err := sess.StartInference(ctx)
	if err != nil {
		e.publishRunFailed(publishContext(ctx), sid, pub, messageID, err.Error())
		return
	}
	output, err := handle.Wait()
	if err != nil {
		if !sink.IsTerminal() {
			if isMaxIterationsError(err) {
				_ = e.publish(publishContext(ctx), sid, pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: runtimeWarningMessageID(messageID), Role: "warning", Prompt: prompt, Text: maxIterationsWarningText(err), Content: maxIterationsWarningText(err), Status: "finished", Streaming: false, Final: true})
			}
			e.publishRunFailed(publishContext(ctx), sid, pub, messageID, err.Error())
		}
		return
	}
	if sink.IsTerminal() {
		return
	}
	_ = output
	_ = e.publish(publishContext(ctx), sid, pub, EventChatRunFinished, &chatappv1.ChatRunFinished{MessageId: messageID, Status: "finished", Correlation: runCorrelationInfo(sid, messageID)})
}

func publishContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func runCorrelationInfo(sid sessionstream.SessionId, messageID string) *chatappv1.CorrelationInfo {
	return &chatappv1.CorrelationInfo{SessionId: string(sid), RunId: messageID, CorrelationKey: messageID}
}

func (e *Engine) publishRunFailed(ctx context.Context, sid sessionstream.SessionId, pub sessionstream.EventPublisher, messageID, errText string) {
	_ = e.publish(ctx, sid, pub, EventChatRunFailed, &chatappv1.ChatRunFailed{MessageId: messageID, Status: "failed", Error: errText, Correlation: runCorrelationInfo(sid, messageID)})
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
