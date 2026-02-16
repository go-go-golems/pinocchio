package webchat

import (
	"context"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/rs/zerolog/log"
)

func snapshotHookForConv(conv *Conversation, store chatstore.TurnStore) toolloop.SnapshotHook {
	if conv == nil || store == nil {
		return nil
	}
	snapLog := log.With().
		Str("component", "webchat").
		Str("conv_id", conv.ID).
		Str("session_id", conv.SessionID).
		Logger()
	return func(ctx context.Context, t *turns.Turn, phase string) {
		if t == nil {
			return
		}
		turnID := t.ID
		if turnID == "" {
			turnID = "turn"
		}
		sessionID := conv.SessionID
		if sessionID == "" {
			if v, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok {
				sessionID = v
			}
		}
		if sessionID == "" {
			return
		}
		payload, err := serde.ToYAML(t, serde.Options{})
		if err != nil {
			snapLog.Warn().Err(err).Str("phase", phase).Msg("webchat snapshot: serialize failed (store)")
			return
		}
		if err := store.Save(ctx, conv.ID, sessionID, turnID, phase, time.Now().UnixMilli(), string(payload)); err != nil {
			snapLog.Warn().Err(err).Str("phase", phase).Msg("webchat snapshot: store save failed")
		}
	}
}
