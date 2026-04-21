package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"google.golang.org/protobuf/types/known/structpb"
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
	hub    *evtstream.Hub
	engine *Engine
}

func NewService(hub *evtstream.Hub, engine *Engine) (*Service, error) {
	if hub == nil {
		return nil, fmt.Errorf("hub is nil")
	}
	if engine == nil {
		engine = NewEngine()
	}
	return &Service{hub: hub, engine: engine}, nil
}

func (s *Service) SubmitPrompt(ctx context.Context, sid evtstream.SessionId, prompt string) error {
	return s.SubmitPromptRequest(ctx, sid, PromptRequest{Prompt: prompt})
}

func (s *Service) SubmitPromptRequest(ctx context.Context, sid evtstream.SessionId, req PromptRequest) error {
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
	if s.engine != nil {
		s.engine.setPendingRequest(sid, req)
	}
	payload, err := structpb.NewStruct(map[string]any{"prompt": req.Prompt})
	if err != nil {
		return err
	}
	return s.hub.Submit(ctx, sid, CommandStartInference, payload)
}

func (s *Service) Stop(ctx context.Context, sid evtstream.SessionId) error {
	if s == nil || s.hub == nil {
		return fmt.Errorf("chat service is not initialized")
	}
	if sid == "" {
		return fmt.Errorf("session id is empty")
	}
	payload, err := structpb.NewStruct(map[string]any{})
	if err != nil {
		return err
	}
	return s.hub.Submit(ctx, sid, CommandStopInference, payload)
}

func (s *Service) WaitIdle(ctx context.Context, sid evtstream.SessionId) error {
	if s == nil || s.engine == nil {
		return fmt.Errorf("chat engine is not initialized")
	}
	if sid == "" {
		return fmt.Errorf("session id is empty")
	}
	return s.engine.WaitIdle(ctx, sid)
}

func (s *Service) Snapshot(ctx context.Context, sid evtstream.SessionId) (evtstream.Snapshot, error) {
	if s == nil || s.hub == nil {
		return evtstream.Snapshot{}, fmt.Errorf("chat service is not initialized")
	}
	if sid == "" {
		return evtstream.Snapshot{}, fmt.Errorf("session id is empty")
	}
	return s.hub.Snapshot(ctx, sid)
}
