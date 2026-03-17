package webchat

import (
	"context"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
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
	ResolvedStepSettings    *aisettings.StepSettings
	ResolvedRuntime         *gepprofiles.RuntimeSpec
	ResolvedProfileMetadata map[string]any
}

type ConversationHandle struct {
	ConvID                  string
	SessionID               string
	RuntimeKey              string
	RuntimeFingerprint      string
	ResolvedProfileMetadata map[string]any
}

type SubmitPromptInput struct {
	ConvID                  string
	RuntimeKey              string
	RuntimeFingerprint      string
	ProfileVersion          uint64
	ResolvedStepSettings    *aisettings.StepSettings
	ResolvedRuntime         *gepprofiles.RuntimeSpec
	ResolvedProfileMetadata map[string]any
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
	return NewChatServiceFromConversation(s).SubmitPrompt(ctx, in)
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

func cloneStepSettings(in *aisettings.StepSettings) *aisettings.StepSettings {
	if in == nil {
		return nil
	}
	return in.Clone()
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
	req, err := s.PrepareRunnerStartForConvID(handle.ConvID, in.Payload, in.Metadata)
	if err != nil {
		return nil, StartRequest{}, err
	}
	return handle, req, nil
}

func (s *ConversationService) PrepareRunnerStartForConvID(convID string, payload any, metadata map[string]any) (StartRequest, error) {
	if s == nil || s.cm == nil {
		return StartRequest{}, errors.New("conversation service is not initialized")
	}
	conv, ok := s.cm.GetConversation(convID)
	if !ok || conv == nil {
		return StartRequest{}, errors.New("conversation not found")
	}
	return s.startRequestForConversation(conv, payload, metadata), nil
}

func (s *ConversationService) startRequestForConversation(conv *Conversation, payload any, metadata map[string]any) StartRequest {
	var timeline TimelineEmitter
	if hook := s.TimelineUpsertHook(conv); hook != nil || s.timelineStore != nil {
		timeline = TimelineEmitterFunc(func(ctx context.Context, entity *timelinepb.TimelineEntityV2, version uint64) error {
			if ctx == nil {
				ctx = s.baseCtx
			}
			if s.timelineStore != nil {
				if err := s.timelineStore.Upsert(ctx, conv.ID, version, entity); err != nil {
					return err
				}
			}
			if hook != nil {
				hook(entity, version)
			}
			return nil
		})
	}
	return StartRequest{
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
