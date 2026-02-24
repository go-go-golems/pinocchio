package webchat

import (
	"context"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/pkg/errors"
)

type turnStorePersister struct {
	store     chatstore.TurnStore
	convID    string
	sessionID string
	runtime   string
	phase     string
}

func newTurnStorePersister(store chatstore.TurnStore, conv *Conversation, phase string) *turnStorePersister {
	if store == nil || conv == nil {
		return nil
	}
	return &turnStorePersister{
		store:     store,
		convID:    conv.ID,
		sessionID: conv.SessionID,
		runtime:   strings.TrimSpace(conv.RuntimeKey),
		phase:     phase,
	}
}

func (p *turnStorePersister) PersistTurn(ctx context.Context, t *turns.Turn) error {
	if p == nil || p.store == nil || t == nil {
		return nil
	}
	convID := strings.TrimSpace(p.convID)
	if convID == "" {
		return errors.New("turn persister: convID is empty")
	}
	sessionID := strings.TrimSpace(p.sessionID)
	if sessionID == "" {
		if v, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok {
			sessionID = v
		}
	}
	if sessionID == "" {
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
	return p.store.Save(ctx, convID, sessionID, turnID, phase, time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
		RuntimeKey:  p.runtime,
		InferenceID: inferenceID,
	})
}
