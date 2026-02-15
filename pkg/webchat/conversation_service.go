package webchat

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

type ConversationServiceConfig struct {
	BaseCtx            context.Context
	ConvManager        *ConvManager
	StepController     *toolloop.StepController
	TimelineStore      chatstore.TimelineStore
	TurnStore          chatstore.TurnStore
	SEMPublisher       message.Publisher
	TimelineUpsertHook func(*Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64)
	ToolFactories      map[string]ToolFactory
}

type ConversationService struct {
	baseCtx context.Context
	cm      *ConvManager
	streams *StreamHub

	stepCtrl       *toolloop.StepController
	timelineStore  chatstore.TimelineStore
	turnStore      chatstore.TurnStore
	semPublisher   message.Publisher
	timelineUpsert func(*Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64)
	toolFactories  map[string]ToolFactory
}

type AppConversationRequest struct {
	ConvID     string
	RuntimeKey string
	Overrides  map[string]any
}

type ConversationHandle struct {
	ConvID             string
	SessionID          string
	RuntimeKey         string
	RuntimeFingerprint string
	SeedSystemPrompt   string
	AllowedTools       []string
}

type SubmitPromptInput struct {
	ConvID         string
	RuntimeKey     string
	Overrides      map[string]any
	Prompt         string
	IdempotencyKey string
}

type SubmitPromptResult struct {
	HTTPStatus int
	Response   map[string]any
}

type WebSocketAttachOptions struct {
	SendHello      bool
	HandlePingPong bool
}

func NewConversationService(cfg ConversationServiceConfig) (*ConversationService, error) {
	if cfg.BaseCtx == nil {
		return nil, errors.New("conversation service base context is nil")
	}
	if cfg.ConvManager == nil {
		return nil, errors.New("conversation service conv manager is nil")
	}
	streams, err := NewStreamHub(StreamHubConfig{
		BaseCtx:     cfg.BaseCtx,
		ConvManager: cfg.ConvManager,
	})
	if err != nil {
		return nil, err
	}
	toolFactories := cfg.ToolFactories
	if toolFactories == nil {
		toolFactories = map[string]ToolFactory{}
	}
	return &ConversationService{
		baseCtx:        cfg.BaseCtx,
		cm:             cfg.ConvManager,
		streams:        streams,
		stepCtrl:       cfg.StepController,
		timelineStore:  cfg.TimelineStore,
		turnStore:      cfg.TurnStore,
		semPublisher:   cfg.SEMPublisher,
		timelineUpsert: cfg.TimelineUpsertHook,
		toolFactories:  toolFactories,
	}, nil
}

func (s *ConversationService) StreamHub() *StreamHub {
	if s == nil {
		return nil
	}
	return s.streams
}

func (s *ConversationService) SetTimelineStore(store chatstore.TimelineStore) {
	if s == nil {
		return
	}
	s.timelineStore = store
}

func (s *ConversationService) SetTurnStore(store chatstore.TurnStore) {
	if s == nil {
		return
	}
	s.turnStore = store
}

func (s *ConversationService) SetStepController(sc *toolloop.StepController) {
	if s == nil {
		return
	}
	s.stepCtrl = sc
}

func (s *ConversationService) RegisterTool(name string, f ToolFactory) {
	if s == nil || strings.TrimSpace(name) == "" || f == nil {
		return
	}
	if s.toolFactories == nil {
		s.toolFactories = map[string]ToolFactory{}
	}
	s.toolFactories[strings.TrimSpace(name)] = f
}

func (s *ConversationService) ResolveAndEnsureConversation(ctx context.Context, req AppConversationRequest) (*ConversationHandle, error) {
	if s == nil || s.streams == nil {
		return nil, errors.New("conversation service is not initialized")
	}
	return s.streams.ResolveAndEnsureConversation(ctx, req)
}

func (s *ConversationService) SubmitPrompt(ctx context.Context, in SubmitPromptInput) (SubmitPromptResult, error) {
	if s == nil || s.cm == nil {
		return SubmitPromptResult{}, errors.New("conversation service is not initialized")
	}
	if ctx == nil {
		ctx = s.baseCtx
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return SubmitPromptResult{HTTPStatus: 400, Response: map[string]any{"status": "error", "error": "missing prompt"}}, nil
	}
	handle, err := s.ResolveAndEnsureConversation(ctx, AppConversationRequest{
		ConvID:     in.ConvID,
		RuntimeKey: in.RuntimeKey,
		Overrides:  in.Overrides,
	})
	if err != nil {
		return SubmitPromptResult{}, err
	}
	conv, ok := s.cm.GetConversation(handle.ConvID)
	if !ok || conv == nil {
		return SubmitPromptResult{}, errors.New("conversation not found after resolve")
	}
	if conv.Sess == nil {
		return SubmitPromptResult{}, errors.New("conversation session not initialized")
	}
	idempotencyKey := strings.TrimSpace(in.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
	}

	prep, err := conv.PrepareSessionInference(idempotencyKey, handle.RuntimeKey, in.Overrides, prompt)
	if err != nil {
		return SubmitPromptResult{}, err
	}
	if !prep.Start {
		status := prep.HTTPStatus
		if status <= 0 {
			status = 200
		}
		return SubmitPromptResult{
			HTTPStatus: status,
			Response:   prep.Response,
		}, nil
	}

	resp, err := s.startInferenceForPrompt(conv, in.Overrides, prompt, idempotencyKey)
	if err != nil {
		return SubmitPromptResult{}, err
	}
	return SubmitPromptResult{HTTPStatus: 200, Response: resp}, nil
}

func (s *ConversationService) AttachWebSocket(ctx context.Context, convID string, conn *websocket.Conn, opts WebSocketAttachOptions) error {
	if s == nil || s.streams == nil {
		return errors.New("conversation service is not initialized")
	}
	return s.streams.AttachWebSocket(ctx, convID, conn, opts)
}

func (s *ConversationService) TimelineUpsertHook(conv *Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64) {
	if s != nil && s.timelineUpsert != nil {
		return s.timelineUpsert(conv)
	}
	return s.timelineUpsertHookDefault(conv)
}

func (s *ConversationService) timelineUpsertHookDefault(conv *Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64) {
	if s == nil || conv == nil {
		return nil
	}
	return func(entity *timelinepb.TimelineEntityV1, version uint64) {
		s.emitTimelineUpsert(conv, entity, version)
	}
}

func (s *ConversationService) emitTimelineUpsert(conv *Conversation, entity *timelinepb.TimelineEntityV1, version uint64) {
	if s == nil || conv == nil || entity == nil {
		return
	}
	payload, err := protoToRaw(&timelinepb.TimelineUpsertV1{
		ConvId:  conv.ID,
		Version: version,
		Entity:  entity,
	})
	if err != nil {
		return
	}
	env := map[string]any{
		"sem": true,
		"event": map[string]any{
			"type": "timeline.upsert",
			"id":   entity.Id,
			"seq":  version,
			"data": payload,
		},
	}
	if s.cm != nil {
		_ = NewWSPublisher(s.cm).PublishJSON(s.baseCtx, conv.ID, env)
	}
}

func (s *ConversationService) startInferenceForPrompt(conv *Conversation, overrides map[string]any, prompt string, idempotencyKey string) (map[string]any, error) {
	if s == nil || conv == nil || conv.Sess == nil {
		return nil, errors.New("invalid conversation")
	}

	sessionLog := log.With().Str("component", "webchat").Str("conv_id", conv.ID).Str("session_id", conv.SessionID).Logger()

	conv.mu.Lock()
	stream := conv.stream
	baseCtx := conv.baseCtx
	allowedTools := append([]string(nil), conv.AllowedTools...)
	conv.mu.Unlock()
	if baseCtx == nil {
		baseCtx = s.baseCtx
	}
	if baseCtx == nil {
		return nil, errors.New("conversation context is nil")
	}
	if stream != nil && !stream.IsRunning() {
		_ = stream.Start(baseCtx)
	}

	tmpReg := geptools.NewInMemoryToolRegistry()
	for _, tf := range s.toolFactories {
		_ = tf(tmpReg)
	}
	registry := geptools.NewInMemoryToolRegistry()
	if len(allowedTools) == 0 {
		for _, td := range tmpReg.ListTools() {
			_ = registry.RegisterTool(td.Name, td)
		}
	} else {
		allowed := map[string]struct{}{}
		for _, n := range allowedTools {
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

	hook := snapshotHookForConv(conv, os.Getenv("PINOCCHIO_WEBCHAT_TURN_SNAPSHOTS_DIR"), s.turnStore)

	seed, err := conv.Sess.AppendNewTurnFromUserPrompt(prompt)
	if err != nil {
		s.finishSessionInference(conv, idempotencyKey, "", "", err)
		return nil, err
	}
	turnID := ""
	if seed != nil && seed.ID != "" {
		turnID = seed.ID
	}
	if turnID != "" && strings.TrimSpace(prompt) != "" {
		if err := s.publishUserChatMessageEvent(baseCtx, conv.ID, "user-"+turnID, prompt); err != nil {
			return nil, errors.Wrap(err, "publish user chat.message event")
		}
	}

	if stepModeFromOverrides(overrides) && s.stepCtrl != nil {
		s.stepCtrl.Enable(toolloop.StepScope{SessionID: conv.SessionID, ConversationID: conv.ID})
	}

	loopCfg := toolloop.NewLoopConfig().WithMaxIterations(5)
	toolCfg := geptools.DefaultToolConfig().WithExecutionTimeout(60 * time.Second)
	conv.Sess.Builder = &enginebuilder.Builder{
		Base:             conv.Eng,
		Registry:         registry,
		LoopConfig:       &loopCfg,
		ToolConfig:       &toolCfg,
		EventSinks:       []events.EventSink{conv.Sink},
		SnapshotHook:     hook,
		StepController:   s.stepCtrl,
		StepPauseTimeout: 30 * time.Second,
		Persister:        newTurnStorePersister(s.turnStore, conv, "final"),
	}

	sessionLog.Info().Str("idempotency_key", idempotencyKey).Msg("starting inference loop")
	if s.baseCtx == nil {
		return nil, errors.New("service context is nil")
	}
	handle, err := conv.Sess.StartInference(s.baseCtx)
	if err != nil {
		s.finishSessionInference(conv, idempotencyKey, "", turnID, err)
		return nil, err
	}
	if handle == nil {
		err := errors.New("start inference returned nil handle")
		s.finishSessionInference(conv, idempotencyKey, "", turnID, err)
		return nil, err
	}

	resp := map[string]any{
		"status":          "started",
		"idempotency_key": idempotencyKey,
		"conv_id":         conv.ID,
		"session_id":      conv.SessionID,
	}
	if turnID != "" {
		resp["turn_id"] = turnID
	}
	if handle.InferenceID != "" {
		resp["inference_id"] = handle.InferenceID
	}
	if handle.Input != nil && handle.Input.ID != "" {
		resp["turn_id"] = handle.Input.ID
	}

	conv.mu.Lock()
	conv.ensureQueueInitLocked()
	if rec, ok := conv.getRecordLocked(idempotencyKey); ok && rec != nil {
		rec.Status = "running"
		rec.StartedAt = time.Now()
		rec.Response = resp
	} else {
		conv.upsertRecordLocked(&chatRequestRecord{IdempotencyKey: idempotencyKey, Status: "running", StartedAt: time.Now(), Response: resp})
	}
	conv.mu.Unlock()

	go func() {
		_, waitErr := handle.Wait()
		s.finishSessionInference(conv, idempotencyKey, handle.InferenceID, turnID, waitErr)
		if waitErr != nil {
			sessionLog.Error().Err(waitErr).Str("inference_id", handle.InferenceID).Msg("inference loop error")
		}
		sessionLog.Info().Str("inference_id", handle.InferenceID).Msg("inference loop finished")
		s.tryDrainQueue(conv)
	}()
	return resp, nil
}

func (s *ConversationService) publishUserChatMessageEvent(ctx context.Context, convID string, eventID string, prompt string) error {
	if s == nil || s.semPublisher == nil {
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
	return s.semPublisher.Publish(topicForConv(convID), msg)
}

func (s *ConversationService) finishSessionInference(conv *Conversation, idempotencyKey string, inferenceID string, turnID string, err error) {
	if conv == nil {
		return
	}
	conv.mu.Lock()
	defer conv.mu.Unlock()

	if conv.activeRequestKey == idempotencyKey {
		conv.activeRequestKey = ""
	}
	conv.touchLocked(time.Now())
	conv.ensureQueueInitLocked()
	if rec, ok := conv.getRecordLocked(idempotencyKey); ok && rec != nil {
		if err != nil {
			rec.Status = "error"
			rec.Error = err.Error()
		} else if rec.Status == "running" {
			rec.Status = "completed"
		}
		rec.CompletedAt = time.Now()
		if rec.Response == nil {
			rec.Response = map[string]any{}
		}
		if inferenceID != "" {
			rec.Response["inference_id"] = inferenceID
		}
		if turnID != "" {
			rec.Response["turn_id"] = turnID
		}
		rec.Response["status"] = rec.Status
	}
}

func (s *ConversationService) tryDrainQueue(conv *Conversation) {
	if s == nil || conv == nil {
		return
	}
	for {
		q, ok := conv.ClaimNextQueued()
		if !ok {
			return
		}
		_, err := s.startInferenceForPrompt(conv, q.Overrides, q.Prompt, q.IdempotencyKey)
		if err != nil {
			s.finishSessionInference(conv, q.IdempotencyKey, "", "", err)
			continue
		}
		return
	}
}
