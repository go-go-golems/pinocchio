package webchat

import (
	"context"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

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

func (s *ChatService) RegisterTool(name string, f infruntime.ToolRegistrar) {
	if s == nil || s.svc == nil {
		return
	}
	s.svc.RegisterTool(name, f)
}

func (s *ChatService) ResolveAndEnsureConversation(ctx context.Context, req ConversationRuntimeRequest) (*ConversationHandle, error) {
	if s == nil || s.svc == nil {
		return nil, errors.New("chat service is not initialized")
	}
	return s.svc.ResolveAndEnsureConversation(ctx, req)
}

func (s *ChatService) PrepareRunnerStart(ctx context.Context, in PrepareRunnerStartInput) (*ConversationHandle, StartRequest, error) {
	if s == nil || s.svc == nil {
		return nil, StartRequest{}, errors.New("chat service is not initialized")
	}
	return s.svc.PrepareRunnerStart(ctx, in)
}

func (s *ChatService) NewLLMLoopRunner() *LLMLoopRunner {
	if s == nil || s.svc == nil {
		return nil
	}
	return NewLLMLoopRunner(LLMLoopRunnerConfig{
		BaseCtx:        s.svc.baseCtx,
		ConvManager:    s.svc.cm,
		StepController: s.svc.stepCtrl,
		TurnStore:      s.svc.turnStore,
		SEMPublisher:   s.svc.semPublisher,
		ToolFactories:  s.svc.toolFactories,
	})
}

type StartPromptWithRunnerInput struct {
	Runtime        ConversationRuntimeRequest
	IdempotencyKey string
	Payload        any
	Metadata       map[string]any
}

func (s *ChatService) StartPromptWithRunner(ctx context.Context, runner Runner, in StartPromptWithRunnerInput) (SubmitPromptResult, error) {
	if s == nil || s.svc == nil {
		return SubmitPromptResult{}, errors.New("chat service is not initialized")
	}
	if runner == nil {
		return SubmitPromptResult{}, errors.New("runner is nil")
	}
	if ctx == nil {
		ctx = s.svc.baseCtx
	}
	handle, startReq, err := s.PrepareRunnerStart(ctx, PrepareRunnerStartInput{
		Runtime:  in.Runtime,
		Payload:  in.Payload,
		Metadata: in.Metadata,
	})
	if err != nil {
		return SubmitPromptResult{}, err
	}
	conv, ok := s.svc.cm.GetConversation(handle.ConvID)
	if !ok || conv == nil {
		return SubmitPromptResult{}, errors.New("conversation not found after resolve")
	}
	idempotencyKey := strings.TrimSpace(in.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
	}
	payload := in.Payload
	if llmPayload, ok := payload.(LLMLoopStartPayload); ok {
		llmPayload.IdempotencyKey = idempotencyKey
		payload = llmPayload
		startReq.Payload = llmPayload
	}
	prep, err := preparePromptSubmission(conv, idempotencyKey, payload, in.Metadata)
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
	startResult, err := runner.Start(ctx, startReq)
	if err != nil {
		s.finishPromptSubmission(conv, idempotencyKey, "", "", err)
		return SubmitPromptResult{}, err
	}
	persistPromptStartResult(conv, idempotencyKey, startResult.Response, startResult.RunID, startResult.TurnID)
	resp := appendProfileMetadata(cloneResponse(startResult.Response), handle)
	if startResult.RunID != "" {
		resp["inference_id"] = startResult.RunID
	}
	if startResult.TurnID != "" {
		resp["turn_id"] = startResult.TurnID
	}
	go s.waitForPromptCompletion(conv, runner, idempotencyKey, startResult)
	return SubmitPromptResult{HTTPStatus: 200, Response: resp}, nil
}

func (s *ChatService) SubmitPrompt(ctx context.Context, in SubmitPromptInput) (SubmitPromptResult, error) {
	if s == nil || s.svc == nil {
		return SubmitPromptResult{}, errors.New("chat service is not initialized")
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return SubmitPromptResult{HTTPStatus: 400, Response: map[string]any{"status": "error", "error": "missing prompt"}}, nil
	}
	runner := s.NewLLMLoopRunner()
	return s.StartPromptWithRunner(ctx, runner, StartPromptWithRunnerInput{
		Runtime: ConversationRuntimeRequest{
			ConvID:                    in.ConvID,
			RuntimeKey:                in.RuntimeKey,
			RuntimeFingerprint:        in.RuntimeFingerprint,
			ProfileVersion:            in.ProfileVersion,
			ResolvedInferenceSettings: cloneInferenceSettings(in.ResolvedInferenceSettings),
			ResolvedRuntime:           in.ResolvedRuntime,
			ResolvedProfileMetadata:   in.ResolvedProfileMetadata,
		},
		IdempotencyKey: in.IdempotencyKey,
		Payload: LLMLoopStartPayload{
			Prompt:         prompt,
			IdempotencyKey: strings.TrimSpace(in.IdempotencyKey),
		},
	})
}

func (s *ChatService) waitForPromptCompletion(conv *Conversation, runner Runner, idempotencyKey string, result StartResult) {
	if s == nil || conv == nil {
		return
	}
	var waitErr error
	if result.Handle != nil {
		waitErr = result.Handle.Wait()
	}
	s.finishPromptSubmission(conv, idempotencyKey, result.RunID, result.TurnID, waitErr)
	if waitErr != nil {
		log.Error().Err(waitErr).Str("component", "webchat").Str("conv_id", conv.ID).Str("run_id", result.RunID).Msg("runner completed with error")
	}
	s.tryDrainQueue(conv, runner)
}

func (s *ChatService) finishPromptSubmission(conv *Conversation, idempotencyKey string, runID string, turnID string, err error) {
	if conv == nil {
		return
	}
	conv.mu.Lock()
	defer conv.mu.Unlock()

	if conv.activeRequestKey == idempotencyKey {
		conv.activeRequestKey = ""
	}
	conv.touchLocked(time.Now())
	ensurePromptQueueInitLocked(conv)
	if rec, ok := getPromptRecordLocked(conv, idempotencyKey); ok && rec != nil {
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
		if runID != "" {
			rec.Response["inference_id"] = runID
		}
		if turnID != "" {
			rec.Response["turn_id"] = turnID
		}
		rec.Response["status"] = rec.Status
	}
}

func (s *ChatService) tryDrainQueue(conv *Conversation, runner Runner) {
	if s == nil || conv == nil || runner == nil {
		return
	}
	for {
		q, ok := claimNextQueuedPrompt(conv)
		if !ok {
			return
		}
		req, err := s.svc.PrepareRunnerStartForConvID(conv.ID, q.Payload, q.Metadata)
		if err != nil {
			s.finishPromptSubmission(conv, q.IdempotencyKey, "", "", err)
			continue
		}
		result, err := runner.Start(s.svc.baseCtx, req)
		if err != nil {
			s.finishPromptSubmission(conv, q.IdempotencyKey, "", "", err)
			continue
		}
		persistPromptStartResult(conv, q.IdempotencyKey, result.Response, result.RunID, result.TurnID)
		go s.waitForPromptCompletion(conv, runner, q.IdempotencyKey, result)
		return
	}
}
