package runtime

import (
	"context"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
)

func newTurnSnapshotHook(sessionID string, runtimeKey string, store chatstore.TurnStore) toolloop.SnapshotHook {
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
		_ = store.Save(ctx, sessionID, sessionID, turnID, phase, time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
			RuntimeKey:  runtimeKey,
			InferenceID: inferenceID,
		})
	}
}
