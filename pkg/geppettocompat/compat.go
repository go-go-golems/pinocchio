package geppettocompat

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/google/uuid"
)

type inferenceRunnerFunc func(ctx context.Context, t *turns.Turn) (*turns.Turn, error)

func (f inferenceRunnerFunc) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	return f(ctx, t)
}

// WrapEngineWithMiddlewares applies middlewares around an engine using the
// current middleware.Chain API.
func WrapEngineWithMiddlewares(eng engine.Engine, mws ...middleware.Middleware) engine.Engine {
	if len(mws) == 0 {
		return eng
	}
	handler := middleware.Chain(func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
		return eng.RunInference(ctx, t)
	}, mws...)
	return inferenceRunnerFunc(handler)
}

// EnsureTurnID makes sure the turn has an ID and returns it.
func EnsureTurnID(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return t.ID
}

// TurnSessionID returns the canonical session identifier from turn metadata.
func TurnSessionID(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	sid, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata)
	if err != nil || !ok {
		return ""
	}
	return sid
}

// EnsureTurnSessionID ensures the turn session ID metadata is present.
func EnsureTurnSessionID(t *turns.Turn, fallback string) string {
	if t == nil {
		return ""
	}
	if sid := TurnSessionID(t); sid != "" {
		return sid
	}
	if fallback == "" {
		fallback = uuid.NewString()
	}
	_ = turns.KeyTurnMetaSessionID.Set(&t.Metadata, fallback)
	return fallback
}

// TurnInferenceID returns the canonical inference identifier from turn metadata.
func TurnInferenceID(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	iid, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata)
	if err != nil || !ok {
		return ""
	}
	return iid
}

// EnsureTurnInferenceID ensures the turn inference ID metadata is present.
func EnsureTurnInferenceID(t *turns.Turn, fallback string) string {
	if t == nil {
		return ""
	}
	if iid := TurnInferenceID(t); iid != "" {
		return iid
	}
	if fallback == "" {
		fallback = uuid.NewString()
	}
	_ = turns.KeyTurnMetaInferenceID.Set(&t.Metadata, fallback)
	return fallback
}

// EventSessionID returns session_id with fallback to inference_id for legacy "run id" uses.
func EventSessionID(md events.EventMetadata) string {
	if md.SessionID != "" {
		return md.SessionID
	}
	return md.InferenceID
}
