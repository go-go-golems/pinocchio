package webchat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/rs/zerolog/log"
)

func snapshotHookForConv(conv *Conversation, dir string, store chatstore.TurnStore) toolloop.SnapshotHook {
	if conv == nil || (dir == "" && store == nil) {
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
		if store != nil {
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
			if sessionID != "" {
				payload, err := serde.ToYAML(t, serde.Options{})
				if err != nil {
					snapLog.Warn().Err(err).Str("phase", phase).Msg("webchat snapshot: serialize failed (store)")
				} else if err := store.Save(ctx, conv.ID, sessionID, turnID, phase, time.Now().UnixMilli(), string(payload)); err != nil {
					snapLog.Warn().Err(err).Str("phase", phase).Msg("webchat snapshot: store save failed")
				}
			}
		}
		if dir == "" {
			return
		}
		subdir := filepath.Join(dir, conv.ID, conv.SessionID)
		if err := os.MkdirAll(subdir, 0755); err != nil {
			snapLog.Warn().Err(err).Str("dir", subdir).Msg("webchat snapshot: mkdir failed")
			return
		}
		ts := time.Now().UTC().Format("20060102-150405.000000000")
		turnID := t.ID
		if turnID == "" {
			turnID = "turn"
		}
		name := fmt.Sprintf("%s-%s-%s.yaml", ts, phase, turnID)
		path := filepath.Join(subdir, name)
		data, err := serde.ToYAML(t, serde.Options{})
		if err != nil {
			snapLog.Warn().Err(err).Str("path", path).Msg("webchat snapshot: serialize failed")
			return
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			snapLog.Warn().Err(err).Str("path", path).Msg("webchat snapshot: write failed")
			return
		}
		snapLog.Debug().Str("path", path).Str("phase", phase).Msg("webchat snapshot: saved turn")
	}
}
