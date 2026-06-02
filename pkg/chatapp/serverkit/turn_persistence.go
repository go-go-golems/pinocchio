package serverkit

import (
	"context"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/pkg/errors"
)

// TurnStorePersister persists completed Geppetto turns into a chatstore.TurnStore.
//
// It is useful for chat applications that use enginebuilder.Builder and want the
// successful final model-context accumulator to be available for debugging,
// resume, export, or later conversation history loading.
type TurnStorePersister struct {
	store      chatstore.TurnStore
	sessionID  string
	runtimeKey string
	phase      string
}

// NewTurnStorePersister creates an enginebuilder-compatible turn persister.
// The session id is used as both conversation id and session id, matching the
// web-chat/chatapp convention for per-session conversations.
func NewTurnStorePersister(store chatstore.TurnStore, sessionID string, runtimeKey string, phase string) *TurnStorePersister {
	if store == nil {
		return nil
	}
	return &TurnStorePersister{
		store:      store,
		sessionID:  strings.TrimSpace(sessionID),
		runtimeKey: strings.TrimSpace(runtimeKey),
		phase:      strings.TrimSpace(phase),
	}
}

func (p *TurnStorePersister) PersistTurn(ctx context.Context, t *turns.Turn) error {
	if p == nil || p.store == nil || t == nil {
		return nil
	}
	if strings.TrimSpace(p.sessionID) == "" {
		return errors.New("turn persister: sessionID is empty")
	}
	turnID := strings.TrimSpace(t.ID)
	if turnID == "" {
		turnID = "turn"
	}
	phase := strings.TrimSpace(p.phase)
	if phase == "" {
		phase = "final"
	}
	payload, err := serde.ToYAML(t, serde.Options{})
	if err != nil {
		return errors.Wrap(err, "turn persister: serialize")
	}
	inferenceID := ""
	if v, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		inferenceID = strings.TrimSpace(v)
	}
	return p.store.Save(ctx, p.sessionID, p.sessionID, turnID, phase, time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
		RuntimeKey:  p.runtimeKey,
		InferenceID: inferenceID,
	})
}

// NewTurnSnapshotHook returns a toolloop snapshot hook that stores intermediate
// turn states such as pre_inference, post_inference, and post_tools.
func NewTurnSnapshotHook(sessionID string, runtimeKey string, store chatstore.TurnStore) toolloop.SnapshotHook {
	if store == nil {
		return nil
	}
	sessionID = strings.TrimSpace(sessionID)
	runtimeKey = strings.TrimSpace(runtimeKey)
	return func(ctx context.Context, t *turns.Turn, phase string) {
		if t == nil || sessionID == "" {
			return
		}
		turnID := strings.TrimSpace(t.ID)
		if turnID == "" {
			turnID = "turn"
		}
		inferenceID := ""
		if v, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
			inferenceID = strings.TrimSpace(v)
		}
		payload, err := serde.ToYAML(t, serde.Options{})
		if err != nil {
			return
		}
		_ = store.Save(ctx, sessionID, sessionID, turnID, strings.TrimSpace(phase), time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
			RuntimeKey:  runtimeKey,
			InferenceID: inferenceID,
		})
	}
}
