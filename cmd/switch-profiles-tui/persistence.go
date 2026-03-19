package main

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
	store  chatstore.TurnStore
	convID string
}

func newTurnStorePersister(store chatstore.TurnStore, convID string) *turnStorePersister {
	if store == nil || strings.TrimSpace(convID) == "" {
		return nil
	}
	return &turnStorePersister{
		store:  store,
		convID: strings.TrimSpace(convID),
	}
}

func (p *turnStorePersister) PersistTurn(ctx context.Context, t *turns.Turn) error {
	if p == nil || p.store == nil || t == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if p.convID == "" {
		return errors.New("turn persister: convID is empty")
	}

	sessionID, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata)
	if err != nil || !ok || strings.TrimSpace(sessionID) == "" {
		return errors.New("turn persister: sessionID is empty")
	}

	turnID := strings.TrimSpace(t.ID)
	if turnID == "" {
		turnID = "turn"
	}

	payload, err := serde.ToYAML(t, serde.Options{})
	if err != nil {
		return errors.Wrap(err, "turn persister: serialize")
	}

	runtimeKey := ""
	if v, ok, err := turns.KeyTurnMetaRuntime.Get(t.Metadata); err == nil && ok {
		runtimeKey = strings.TrimSpace(runtimeKeyFromMetaValue(v))
	}
	inferenceID := ""
	if v, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		inferenceID = strings.TrimSpace(v)
	}

	return p.store.Save(ctx, p.convID, strings.TrimSpace(sessionID), turnID, "final", time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
		RuntimeKey:  runtimeKey,
		InferenceID: inferenceID,
	})
}

func runtimeKeyFromMetaValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		for _, key := range []string{"runtime_key", "key", "slug", "profile", "profile_key"} {
			if raw, ok := t[key]; ok {
				if s, ok := raw.(string); ok {
					return s
				}
			}
		}
		return ""
	default:
		return ""
	}
}
