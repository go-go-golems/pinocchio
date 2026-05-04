package chatapp

import (
	"context"
	"fmt"
	"strings"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/google/uuid"
)

// PromptRequest is the app-facing prompt submission input.
type PromptRequest struct {
	Prompt         string
	IdempotencyKey string
	Runtime        *infruntime.ComposedRuntime
}

// Service is an app-facing chat service surface suitable for consumer apps such as cmd/web-chat.
// It wraps command submission and snapshot access behind domain methods rather than exposing raw
// command names to callers.
type Service struct {
	hub    *sessionstream.Hub
	engine *Engine
}

func NewService(hub *sessionstream.Hub, engine *Engine) (*Service, error) {
	if hub == nil {
		return nil, fmt.Errorf("hub is nil")
	}
	if engine == nil {
		engine = NewEngine()
	}
	return &Service{hub: hub, engine: engine}, nil
}

func (s *Service) SubmitPrompt(ctx context.Context, sid sessionstream.SessionId, prompt string) error {
	return s.SubmitPromptRequest(ctx, sid, PromptRequest{Prompt: prompt})
}

func (s *Service) SubmitPromptRequest(ctx context.Context, sid sessionstream.SessionId, req PromptRequest) error {
	if s == nil || s.hub == nil {
		return fmt.Errorf("chat service is not initialized")
	}
	if sid == "" {
		return fmt.Errorf("session id is empty")
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		return fmt.Errorf("prompt is empty")
	}
	requestID := uuid.NewString()
	if s.engine != nil {
		s.engine.setPendingRequest(requestID, req)
	}
	payload := &chatappv1.StartInferenceCommand{
		Prompt:         req.Prompt,
		RequestId:      requestID,
		IdempotencyKey: req.IdempotencyKey,
	}
	if err := s.hub.Submit(ctx, sid, CommandStartInference, payload); err != nil {
		s.engine.clearPendingRequest(requestID)
		return err
	}
	return nil
}

func (s *Service) Stop(ctx context.Context, sid sessionstream.SessionId) error {
	if s == nil || s.hub == nil {
		return fmt.Errorf("chat service is not initialized")
	}
	if sid == "" {
		return fmt.Errorf("session id is empty")
	}
	return s.hub.Submit(ctx, sid, CommandStopInference, &chatappv1.StopInferenceCommand{})
}

func (s *Service) WaitIdle(ctx context.Context, sid sessionstream.SessionId) error {
	if s == nil || s.engine == nil {
		return fmt.Errorf("chat engine is not initialized")
	}
	if sid == "" {
		return fmt.Errorf("session id is empty")
	}
	return s.engine.WaitIdle(ctx, sid)
}

func (s *Service) Snapshot(ctx context.Context, sid sessionstream.SessionId) (sessionstream.Snapshot, error) {
	if s == nil || s.hub == nil {
		return sessionstream.Snapshot{}, fmt.Errorf("chat service is not initialized")
	}
	if sid == "" {
		return sessionstream.Snapshot{}, fmt.Errorf("session id is empty")
	}
	return s.hub.Snapshot(ctx, sid)
}
