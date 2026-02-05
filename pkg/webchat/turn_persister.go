package webchat

import (
	"context"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/pkg/errors"
)

type turnStorePersister struct {
	store  TurnStore
	convID string
	runID  string
	phase  string
}

func newTurnStorePersister(store TurnStore, conv *Conversation, phase string) *turnStorePersister {
	if store == nil || conv == nil {
		return nil
	}
	return &turnStorePersister{
		store:  store,
		convID: conv.ID,
		runID:  conv.RunID,
		phase:  phase,
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
	runID := strings.TrimSpace(p.runID)
	if runID == "" {
		if v, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok {
			runID = v
		}
	}
	if runID == "" {
		return errors.New("turn persister: runID is empty")
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
	return p.store.Save(ctx, convID, runID, turnID, phase, time.Now().UnixMilli(), string(payload))
}
