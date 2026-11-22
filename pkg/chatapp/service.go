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
	Prompt string
	// MessageID, when set, is the run's message id (Service.NewMessageID):
	// the caller knows the run before any event names it. Empty: generated.
	MessageID string
	// Attachments are user-provided attachments (images) referenced by URL. They
	// are echoed to clients in ChatUserMessageAccepted / ChatMessageEntity and,
	// for image kinds, appended to the user turn block as geppetto image entries.
	// A request may have attachments and an empty Prompt.
	Attachments    []Attachment
	IdempotencyKey string
	Runtime        *infruntime.ComposedRuntime
	// Blocks are appended to the turn after the person's message, in order, for
	// this turn only. They stay in the history afterwards (chatapp never edits
	// blocks it has sent). Use them for caller-owned content such as a block
	// describing the application's current state.
	Blocks []turns.Block
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

// NewMessageID returns a fresh run message id from the engine's generator,
// for PromptRequest.MessageID.
func (s *Service) NewMessageID() (string, error) {
	if s == nil || s.engine == nil {
		return "", fmt.Errorf("chat service is not initialized")
	}
	return s.engine.newMessageID()
}

func (s *Service) SubmitPromptRequest(ctx context.Context, sid sessionstream.SessionId, req PromptRequest) error {
	if s == nil || s.hub == nil {
		return fmt.Errorf("chat service is not initialized")
	}
	if sid == "" {
		return fmt.Errorf("session id is empty")
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" && len(req.Attachments) == 0 {
		return fmt.Errorf("prompt is empty and no attachments were provided")
	}
	requestID := uuid.NewString()
	if s.engine != nil {
		s.engine.setPendingRequest(requestID, req)
	}
	payload := &chatappv1.StartInferenceCommand{
		Prompt:         req.Prompt,
		RequestId:      requestID,
		IdempotencyKey: req.IdempotencyKey,
		Attachments:    AttachmentsToProto(req.Attachments),
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

// Running reports whether the session has an active run. Unlike a timeline
// snapshot, it is exact: a run stopped before any output leaves no assistant
// entity in the timeline.
func (s *Service) Running(sid sessionstream.SessionId) bool {
	return s != nil && s.engine != nil && s.engine.Running(sid)
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
