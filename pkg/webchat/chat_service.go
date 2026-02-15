package webchat

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/pkg/errors"

	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
)

// ChatServiceConfig currently mirrors ConversationServiceConfig while we migrate
// call sites to the split chat/stream API.
type ChatServiceConfig = ConversationServiceConfig

// ChatService exposes queue/idempotency/inference operations and intentionally
// excludes websocket attachment concerns.
type ChatService struct {
	svc *ConversationService
}

func NewChatService(cfg ChatServiceConfig) (*ChatService, error) {
	svc, err := NewConversationService(cfg)
	if err != nil {
		return nil, err
	}
	return NewChatServiceFromConversation(svc), nil
}

func NewChatServiceFromConversation(svc *ConversationService) *ChatService {
	if svc == nil {
		return nil
	}
	return &ChatService{svc: svc}
}

func (s *ChatService) SetTimelineStore(store chatstore.TimelineStore) {
	if s == nil || s.svc == nil {
		return
	}
	s.svc.SetTimelineStore(store)
}

func (s *ChatService) SetTurnStore(store chatstore.TurnStore) {
	if s == nil || s.svc == nil {
		return
	}
	s.svc.SetTurnStore(store)
}

func (s *ChatService) SetStepController(sc *toolloop.StepController) {
	if s == nil || s.svc == nil {
		return
	}
	s.svc.SetStepController(sc)
}

func (s *ChatService) RegisterTool(name string, f infruntime.ToolFactory) {
	if s == nil || s.svc == nil {
		return
	}
	s.svc.RegisterTool(name, f)
}

func (s *ChatService) ResolveAndEnsureConversation(ctx context.Context, req AppConversationRequest) (*ConversationHandle, error) {
	if s == nil || s.svc == nil {
		return nil, errors.New("chat service is not initialized")
	}
	return s.svc.ResolveAndEnsureConversation(ctx, req)
}

func (s *ChatService) SubmitPrompt(ctx context.Context, in SubmitPromptInput) (SubmitPromptResult, error) {
	if s == nil || s.svc == nil {
		return SubmitPromptResult{}, errors.New("chat service is not initialized")
	}
	return s.svc.SubmitPrompt(ctx, in)
}
