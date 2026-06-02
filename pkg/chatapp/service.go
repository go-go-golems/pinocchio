package chatapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/turns"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// PromptRequest is the app-facing prompt submission input.
type PromptRequest struct {
	Prompt         string
	IdempotencyKey string
	Runtime        *infruntime.ComposedRuntime
	// InitialTurn optionally seeds the Geppetto runtime with a fully rendered
	// turn instead of appending Prompt as a new user-only turn. This is used by
	// Pinocchio verbs whose inputs can include system prompts, pre-seeded blocks,
	// images, and templated content.
	InitialTurn *turns.Turn
	// OnFinalTurn is called with the final Geppetto turn after successful runtime
	// inference. Callers that maintain an in-memory conversation accumulator can
	// clone this turn directly instead of reconstructing assistant output from
	// projected timeline entities.
	OnFinalTurn func(*turns.Turn)
	// RuntimeContext decorates the Geppetto run context with app-owned values such
	// as browser-tool bridge handles. It runs inside chatapp when the session id,
	// message id, and event publisher are known.
	RuntimeContext func(ctx context.Context, sid sessionstream.SessionId, messageID string, pub sessionstream.EventPublisher) context.Context
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

func (s *Service) SubmitCommand(ctx context.Context, sid sessionstream.SessionId, name string, payload proto.Message) error {
	if s == nil || s.hub == nil {
		return fmt.Errorf("chat service is not initialized")
	}
	if sid == "" {
		return fmt.Errorf("session id is empty")
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("command name is empty")
	}
	if payload == nil {
		return fmt.Errorf("command %q payload is nil", name)
	}
	return s.hub.Submit(ctx, sid, name, payload)
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
