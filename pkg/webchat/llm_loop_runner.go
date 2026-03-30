package webchat

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

type LLMLoopStartPayload struct {
	Prompt         string
	IdempotencyKey string
}

type LLMLoopRunnerConfig struct {
	BaseCtx        context.Context
	ConvManager    *ConvManager
	StepController *toolloop.StepController
	TurnStore      chatstore.TurnStore
	SEMPublisher   message.Publisher
	ToolFactories  map[string]infruntime.ToolRegistrar
}

type LLMLoopRunner struct {
	baseCtx       context.Context
	cm            *ConvManager
	stepCtrl      *toolloop.StepController
	turnStore     chatstore.TurnStore
	semPublisher  message.Publisher
	toolFactories map[string]infruntime.ToolRegistrar
}

func NewLLMLoopRunner(cfg LLMLoopRunnerConfig) *LLMLoopRunner {
	toolFactories := cfg.ToolFactories
	if toolFactories == nil {
		toolFactories = map[string]infruntime.ToolRegistrar{}
	}
	return &LLMLoopRunner{
		baseCtx:       cfg.BaseCtx,
		cm:            cfg.ConvManager,
		stepCtrl:      cfg.StepController,
		turnStore:     cfg.TurnStore,
		semPublisher:  cfg.SEMPublisher,
		toolFactories: toolFactories,
	}
}

type sessionExecutionRunHandle struct {
	waitFn func() error
}

func (h sessionExecutionRunHandle) Wait() error {
	if h.waitFn == nil {
		return nil
	}
	return h.waitFn()
}

func (r *LLMLoopRunner) Start(ctx context.Context, req StartRequest) (StartResult, error) {
	payload, ok := req.Payload.(LLMLoopStartPayload)
	if !ok {
		return StartResult{}, errors.New("llm loop runner payload has unexpected type")
	}
	if r == nil || r.cm == nil {
		return StartResult{}, errors.New("llm loop runner is not initialized")
	}
	conv, ok := r.cm.GetConversation(req.ConvID)
	if !ok || conv == nil {
		return StartResult{}, errors.New("conversation not found")
	}
	state, err := r.cm.ensureLLMState(conv)
	if err != nil {
		return StartResult{}, err
	}

	sessionLog := log.With().Str("component", "webchat").Str("conv_id", req.ConvID).Str("session_id", req.SessionID).Logger()

	conv.mu.Lock()
	stream := conv.stream
	baseCtx := conv.baseCtx
	conv.mu.Unlock()
	if baseCtx == nil {
		baseCtx = r.baseCtx
	}
	if baseCtx == nil {
		baseCtx = ctx
	}
	if baseCtx == nil {
		return StartResult{}, errors.New("llm loop runner base context is nil")
	}
	if stream != nil && !stream.IsRunning() {
		_ = stream.Start(baseCtx)
	}

	tmpReg := geptools.NewInMemoryToolRegistry()
	for _, tf := range r.toolFactories {
		_ = tf(tmpReg)
	}
	registry := geptools.NewInMemoryToolRegistry()
	if len(state.toolNames) == 0 {
		for _, td := range tmpReg.ListTools() {
			_ = registry.RegisterTool(td.Name, td)
		}
	} else {
		allowed := map[string]struct{}{}
		for _, n := range state.toolNames {
			if t := strings.TrimSpace(n); t != "" {
				allowed[t] = struct{}{}
			}
		}
		for _, td := range tmpReg.ListTools() {
			if _, ok := allowed[td.Name]; ok {
				_ = registry.RegisterTool(td.Name, td)
			}
		}
	}

	hook := snapshotHookForConv(conv, r.turnStore)

	seed, err := state.session.AppendNewTurnFromUserPrompt(payload.Prompt)
	if err != nil {
		return StartResult{}, err
	}
	turnID := ""
	if seed != nil && seed.ID != "" {
		turnID = seed.ID
	}
	if turnID != "" && strings.TrimSpace(payload.Prompt) != "" {
		if err := r.publishUserChatMessageEvent(baseCtx, req.ConvID, "user-"+turnID, payload.Prompt); err != nil {
			return StartResult{}, errors.Wrap(err, "publish user chat.message event")
		}
		if err := r.projectUserChatMessageTimeline(baseCtx, conv, req.Timeline, "user-"+turnID, payload.Prompt); err != nil {
			return StartResult{}, errors.Wrap(err, "project user chat.message timeline entity")
		}
	}

	loopCfg := toolloop.NewLoopConfig().WithMaxIterations(5)
	toolCfg := geptools.DefaultToolConfig().WithExecutionTimeout(60 * time.Second)
	state.session.Builder = &enginebuilder.Builder{
		Base:             state.engine,
		Registry:         registry,
		LoopConfig:       &loopCfg,
		ToolConfig:       &toolCfg,
		EventSinks:       []events.EventSink{req.Sink},
		SnapshotHook:     hook,
		StepController:   r.stepCtrl,
		StepPauseTimeout: 30 * time.Second,
		Persister:        newTurnStorePersister(r.turnStore, conv, "final"),
	}

	sessionLog.Info().Str("idempotency_key", payload.IdempotencyKey).Msg("starting inference loop")
	handle, err := state.session.StartInference(baseCtx)
	if err != nil {
		return StartResult{}, err
	}
	if handle == nil {
		return StartResult{}, errors.New("start inference returned nil handle")
	}

	resp := map[string]any{
		"status":          "started",
		"idempotency_key": payload.IdempotencyKey,
		"conv_id":         req.ConvID,
		"session_id":      req.SessionID,
	}
	if turnID != "" {
		resp["turn_id"] = turnID
	}
	if handle.InferenceID != "" {
		resp["inference_id"] = handle.InferenceID
	}
	if handle.Input != nil && handle.Input.ID != "" {
		turnID = handle.Input.ID
		resp["turn_id"] = handle.Input.ID
	}

	return StartResult{
		Response: resp,
		Handle: sessionExecutionRunHandle{
			waitFn: func() error {
				_, waitErr := handle.Wait()
				return waitErr
			},
		},
		RunID:  handle.InferenceID,
		TurnID: turnID,
	}, nil
}

func (r *LLMLoopRunner) publishUserChatMessageEvent(ctx context.Context, convID string, eventID string, prompt string) error {
	if r == nil || r.semPublisher == nil {
		return errors.New("sem publisher not configured")
	}
	payload, err := protoToRaw(&timelinepb.MessageSnapshotV1{
		SchemaVersion: 1,
		Role:          "user",
		Content:       prompt,
		Streaming:     false,
	})
	if err != nil {
		return err
	}
	env := map[string]any{
		"sem": true,
		"event": map[string]any{
			"type": "chat.message",
			"id":   eventID,
			"data": payload,
		},
	}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	msg := message.NewMessage(uuid.NewString(), b)
	return r.semPublisher.Publish(topicForConv(convID), msg)
}

func (r *LLMLoopRunner) projectUserChatMessageTimeline(
	ctx context.Context,
	conv *Conversation,
	timeline TimelineEmitter,
	eventID string,
	prompt string,
) error {
	if timeline == nil || strings.TrimSpace(eventID) == "" || strings.TrimSpace(prompt) == "" {
		return nil
	}
	version := uint64(1)
	if conv != nil {
		conv.mu.Lock()
		if conv.lastSeenVersion > 0 {
			version = conv.lastSeenVersion + 1
		}
		conv.mu.Unlock()
	}
	return timeline.Upsert(ctx, timelineEntityV2FromProtoMessage(eventID, "message", &timelinepb.MessageSnapshotV1{
		SchemaVersion: 1,
		Role:          "user",
		Content:       prompt,
		Streaming:     false,
	}), version)
}
