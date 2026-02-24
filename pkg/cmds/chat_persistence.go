package cmds

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/pkg/errors"
)

type cliTurnStorePersister struct {
	store             chatstore.TurnStore
	convID            string
	fallbackSessionID string
	phase             string
}

func newCLITurnStorePersister(store chatstore.TurnStore, convID string, sessionID string, phase string) *cliTurnStorePersister {
	if store == nil {
		return nil
	}
	return &cliTurnStorePersister{
		store:             store,
		convID:            strings.TrimSpace(convID),
		fallbackSessionID: strings.TrimSpace(sessionID),
		phase:             strings.TrimSpace(phase),
	}
}

func (p *cliTurnStorePersister) PersistTurn(ctx context.Context, t *turns.Turn) error {
	if p == nil || p.store == nil || t == nil {
		return nil
	}
	if p.convID == "" {
		return errors.New("cli turn persister: convID is empty")
	}
	sessionID := p.fallbackSessionID
	if sessionID == "" {
		if v, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok {
			sessionID = strings.TrimSpace(v)
		}
	}
	if sessionID == "" {
		return errors.New("cli turn persister: sessionID is empty")
	}
	turnID := strings.TrimSpace(t.ID)
	if turnID == "" {
		turnID = "turn"
	}
	phase := p.phase
	if phase == "" {
		phase = "final"
	}
	payload, err := serde.ToYAML(t, serde.Options{})
	if err != nil {
		return errors.Wrap(err, "cli turn persister: serialize")
	}
	runtimeKey := ""
	if v, ok, err := turns.KeyTurnMetaRuntime.Get(t.Metadata); err == nil && ok {
		runtimeKey = strings.TrimSpace(toString(v))
	}
	inferenceID := ""
	if v, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		inferenceID = strings.TrimSpace(v)
	}
	return p.store.Save(ctx, p.convID, sessionID, turnID, phase, time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
		RuntimeKey:  runtimeKey,
		InferenceID: inferenceID,
	})
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return ""
}

func openChatPersistenceStores(settings run.PersistenceSettings) (chatstore.TimelineStore, chatstore.TurnStore, func(), error) {
	var timelineStore chatstore.TimelineStore
	var turnStore chatstore.TurnStore

	cleanup := func() {
		if turnStore != nil {
			_ = turnStore.Close()
		}
		if timelineStore != nil {
			_ = timelineStore.Close()
		}
	}

	openTimeline := strings.TrimSpace(settings.TimelineDSN) != "" || strings.TrimSpace(settings.TimelineDB) != ""
	openTurns := strings.TrimSpace(settings.TurnsDSN) != "" || strings.TrimSpace(settings.TurnsDB) != ""
	if !openTimeline && !openTurns {
		return nil, nil, cleanup, nil
	}

	if openTimeline {
		dsn := strings.TrimSpace(settings.TimelineDSN)
		if dsn == "" {
			timelineDB := strings.TrimSpace(settings.TimelineDB)
			if dir := filepath.Dir(timelineDB); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return nil, nil, cleanup, errors.Wrap(err, "create timeline db dir")
				}
			}
			var err error
			dsn, err = chatstore.SQLiteTimelineDSNForFile(timelineDB)
			if err != nil {
				return nil, nil, cleanup, err
			}
		}
		s, err := chatstore.NewSQLiteTimelineStore(dsn)
		if err != nil {
			return nil, nil, cleanup, err
		}
		timelineStore = s
	}

	if openTurns {
		dsn := strings.TrimSpace(settings.TurnsDSN)
		if dsn == "" {
			turnsDB := strings.TrimSpace(settings.TurnsDB)
			if dir := filepath.Dir(turnsDB); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					cleanup()
					return nil, nil, cleanup, errors.Wrap(err, "create turns db dir")
				}
			}
			var err error
			dsn, err = chatstore.SQLiteTurnDSNForFile(turnsDB)
			if err != nil {
				cleanup()
				return nil, nil, cleanup, err
			}
		}
		s, err := chatstore.NewSQLiteTurnStore(dsn)
		if err != nil {
			cleanup()
			return nil, nil, cleanup, err
		}
		turnStore = s
	}

	return timelineStore, turnStore, cleanup, nil
}
