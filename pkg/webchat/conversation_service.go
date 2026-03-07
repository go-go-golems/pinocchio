package webchat

import (
	"context"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
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
	TimelineUpsertHook func(*Conversation) func(entity *timelinepb.TimelineEntityV2, version uint64)
	ToolFactories      map[string]infruntime.ToolRegistrar
}

type ConversationService struct {
	baseCtx context.Context
	cm      *ConvManager
	streams *StreamHub

	stepCtrl       *toolloop.StepController
	timelineStore  chatstore.TimelineStore
	turnStore      chatstore.TurnStore
	semPublisher   message.Publisher
	timelineUpsert func(*Conversation) func(entity *timelinepb.TimelineEntityV2, version uint64)
	toolFactories  map[string]infruntime.ToolRegistrar
}

type ConversationRuntimeRequest struct {
	ConvID                  string
	RuntimeKey              string
	RuntimeFingerprint      string
	ProfileVersion          uint64
	ResolvedRuntime         *gepprofiles.RuntimeSpec
	ResolvedProfileMetadata map[string]any
	Overrides               map[string]any
}

type ConversationHandle struct {
	ConvID                  string
	SessionID               string
	RuntimeKey              string
	RuntimeFingerprint      string
	ResolvedProfileMetadata map[string]any
	SeedSystemPrompt        string
	AllowedTools            []string
}

type SubmitPromptInput struct {
	ConvID                  string
	RuntimeKey              string
	RuntimeFingerprint      string
	ProfileVersion          uint64
	ResolvedRuntime         *gepprofiles.RuntimeSpec
	ResolvedProfileMetadata map[string]any
	Overrides               map[string]any
	Prompt                  string
	IdempotencyKey          string
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
		toolFactories = map[string]infruntime.ToolRegistrar{}
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

func (s *ConversationService) RegisterTool(name string, f infruntime.ToolRegistrar) {
	if s == nil || strings.TrimSpace(name) == "" || f == nil {
		return
	}
	if s.toolFactories == nil {
		s.toolFactories = map[string]infruntime.ToolRegistrar{}
	}
	s.toolFactories[strings.TrimSpace(name)] = f
}

func (s *ConversationService) ResolveAndEnsureConversation(ctx context.Context, req ConversationRuntimeRequest) (*ConversationHandle, error) {
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
	handle, err := s.ResolveAndEnsureConversation(ctx, ConversationRuntimeRequest{
		ConvID:                  in.ConvID,
		RuntimeKey:              in.RuntimeKey,
		RuntimeFingerprint:      in.RuntimeFingerprint,
		ProfileVersion:          in.ProfileVersion,
		ResolvedRuntime:         in.ResolvedRuntime,
		ResolvedProfileMetadata: in.ResolvedProfileMetadata,
		Overrides:               in.Overrides,
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
			Response:   appendProfileMetadata(prep.Response, handle),
		}, nil
	}

	resp, err := s.startInferenceForPrompt(conv, in.Overrides, prompt, idempotencyKey)
	if err != nil {
		return SubmitPromptResult{}, err
	}
	return SubmitPromptResult{HTTPStatus: 200, Response: appendProfileMetadata(resp, handle)}, nil
}

func appendProfileMetadata(resp map[string]any, handle *ConversationHandle) map[string]any {
	if handle == nil {
		return resp
	}
	if resp == nil {
		resp = map[string]any{}
	}
	if runtimeFingerprint := strings.TrimSpace(handle.RuntimeFingerprint); runtimeFingerprint != "" {
		resp["runtime_fingerprint"] = runtimeFingerprint
	}
	if len(handle.ResolvedProfileMetadata) > 0 {
		resp["profile_metadata"] = copyStringAnyMap(handle.ResolvedProfileMetadata)
	}
	return resp
}

func copyStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (s *ConversationService) PrepareRunnerStart(ctx context.Context, in PrepareRunnerStartInput) (*ConversationHandle, StartRequest, error) {
	if s == nil || s.cm == nil {
		return nil, StartRequest{}, errors.New("conversation service is not initialized")
	}
	if ctx == nil {
		ctx = s.baseCtx
	}
	handle, err := s.ResolveAndEnsureConversation(ctx, in.Runtime)
	if err != nil {
		return nil, StartRequest{}, err
	}
	conv, ok := s.cm.GetConversation(handle.ConvID)
	if !ok || conv == nil {
		return nil, StartRequest{}, errors.New("conversation not found after resolve")
	}
	return handle, s.startRequestForConversation(conv, in.Payload, in.Metadata), nil
}

func (s *ConversationService) startRequestForConversation(conv *Conversation, payload any, metadata map[string]any) StartRequest {
	var timeline TimelineEmitter
	if hook := s.TimelineUpsertHook(conv); hook != nil {
		timeline = TimelineEmitterFunc(func(_ context.Context, entity *timelinepb.TimelineEntityV2, version uint64) error {
			hook(entity, version)
			return nil
		})
	}
	return StartRequest{
		Conversation:       conv,
		ConvID:             conv.ID,
		SessionID:          conv.SessionID,
		RuntimeKey:         conv.RuntimeKey,
		RuntimeFingerprint: conv.RuntimeFingerprint,
		Sink:               conv.Sink,
		Timeline:           timeline,
		Payload:            payload,
		Metadata:           copyStringAnyMap(metadata),
	}
}

func (s *ConversationService) AttachWebSocket(ctx context.Context, convID string, conn *websocket.Conn, opts WebSocketAttachOptions) error {
	if s == nil || s.streams == nil {
		return errors.New("conversation service is not initialized")
	}
	return s.streams.AttachWebSocket(ctx, convID, conn, opts)
}

func (s *ConversationService) TimelineUpsertHook(conv *Conversation) func(entity *timelinepb.TimelineEntityV2, version uint64) {
	if s != nil && s.timelineUpsert != nil {
		return s.timelineUpsert(conv)
	}
	return s.timelineUpsertHookDefault(conv)
}

func (s *ConversationService) timelineUpsertHookDefault(conv *Conversation) func(entity *timelinepb.TimelineEntityV2, version uint64) {
	if s == nil || conv == nil {
		return nil
	}
	return func(entity *timelinepb.TimelineEntityV2, version uint64) {
		s.emitTimelineUpsert(conv, entity, version)
	}
}

func (s *ConversationService) emitTimelineUpsert(conv *Conversation, entity *timelinepb.TimelineEntityV2, version uint64) {
	if s == nil || conv == nil || entity == nil {
		return
	}
	payload, err := protoToRaw(&timelinepb.TimelineUpsertV2{
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
	runner := NewLLMLoopRunner(LLMLoopRunnerConfig{
		BaseCtx:        s.baseCtx,
		StepController: s.stepCtrl,
		TurnStore:      s.turnStore,
		SEMPublisher:   s.semPublisher,
		ToolFactories:  s.toolFactories,
	})
	result, err := runner.Start(s.baseCtx, s.startRequestForConversation(conv, LLMLoopStartPayload{
		Prompt:         prompt,
		Overrides:      overrides,
		IdempotencyKey: idempotencyKey,
	}, nil))
	if err != nil {
		s.finishSessionInference(conv, idempotencyKey, "", "", err)
		return nil, err
	}
	resp := result.Response
	if resp == nil {
		resp = map[string]any{}
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
		handle := result.Handle
		if handle == nil {
			err := errors.New("inference handle missing after runner start")
			s.finishSessionInference(conv, idempotencyKey, "", result.TurnID, err)
			return
		}
		_, waitErr := handle.Wait()
		s.finishSessionInference(conv, idempotencyKey, handle.InferenceID, result.TurnID, waitErr)
		if waitErr != nil {
			log.Error().Err(waitErr).Str("component", "webchat").Str("conv_id", conv.ID).Str("session_id", conv.SessionID).Str("inference_id", handle.InferenceID).Msg("inference loop error")
		}
		log.Info().Str("component", "webchat").Str("conv_id", conv.ID).Str("session_id", conv.SessionID).Str("inference_id", handle.InferenceID).Msg("inference loop finished")
		s.tryDrainQueue(conv)
	}()
	return resp, nil
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
