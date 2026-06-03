package cmds

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	gp "github.com/go-go-golems/geppetto/pkg/js/modules/geppetto"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/pkg/errors"
)

type pinocchioJSTurnStore struct {
	store chatstore.TurnStore
}

var _ gp.TurnStore = (*pinocchioJSTurnStore)(nil)

func newPinocchioJSTurnStore(store chatstore.TurnStore) *pinocchioJSTurnStore {
	if store == nil {
		return nil
	}
	return &pinocchioJSTurnStore{store: store}
}

func (s *pinocchioJSTurnStore) PersistTurn(ctx context.Context, t *turns.Turn) error {
	if s == nil || s.store == nil || t == nil {
		return nil
	}
	sessionID := turnSessionID(t)
	if sessionID == "" {
		return errors.New("pinocchio js turn store: sessionID is empty")
	}
	turnID := strings.TrimSpace(t.ID)
	if turnID == "" {
		turnID = "turn"
	}
	payload, err := serde.ToYAML(t, serde.Options{})
	if err != nil {
		return errors.Wrap(err, "pinocchio js turn store: serialize turn")
	}
	return s.store.Save(ctx, sessionID, sessionID, turnID, "final", time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
		RuntimeKey:  turnRuntimeKey(t),
		InferenceID: turnInferenceID(t),
	})
}

func (s *pinocchioJSTurnStore) ListTurns(ctx context.Context, q gp.TurnStoreQuery) ([]gp.TurnStoreSnapshot, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("pinocchio js turn store: store is nil")
	}
	items, err := s.store.List(ctx, chatstore.TurnQuery{
		ConvID:    strings.TrimSpace(q.ConvID),
		SessionID: strings.TrimSpace(q.SessionID),
		Phase:     strings.TrimSpace(q.Phase),
		SinceMs:   q.SinceMs,
		Limit:     q.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]gp.TurnStoreSnapshot, 0, len(items))
	for _, item := range items {
		snap, err := convertPinocchioTurnSnapshot(item)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, nil
}

func (s *pinocchioJSTurnStore) LoadLatestTurn(ctx context.Context, q gp.TurnStoreQuery) (*gp.TurnStoreSnapshot, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("pinocchio js turn store: store is nil")
	}
	convID := strings.TrimSpace(q.ConvID)
	if convID == "" {
		convID = strings.TrimSpace(q.SessionID)
	}
	if convID == "" {
		return nil, errors.New("pinocchio js turn store: convId or sessionId required")
	}
	phase := strings.TrimSpace(q.Phase)
	if phase == "" {
		phase = "final"
	}
	item, err := s.store.LoadLatestTurn(ctx, convID, phase)
	if err != nil || item == nil {
		return nil, err
	}
	snap, err := convertPinocchioTurnSnapshot(*item)
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *pinocchioJSTurnStore) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

func openPinocchioJSTurnStore(turnsDSN, turnsDB string) (*pinocchioJSTurnStore, func(), error) {
	noop := func() {}
	turnsDSN = strings.TrimSpace(turnsDSN)
	turnsDB = strings.TrimSpace(turnsDB)
	if turnsDSN == "" && turnsDB == "" {
		return nil, noop, nil
	}
	dsn := turnsDSN
	if dsn == "" {
		if dir := filepath.Dir(turnsDB); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, noop, errors.Wrap(err, "create turns db dir")
			}
		}
		var err error
		dsn, err = chatstore.SQLiteTurnDSNForFile(turnsDB)
		if err != nil {
			return nil, noop, err
		}
	}
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	if err != nil {
		return nil, noop, err
	}
	wrapped := newPinocchioJSTurnStore(store)
	return wrapped, func() { _ = wrapped.Close() }, nil
}

func convertPinocchioTurnSnapshot(item chatstore.TurnSnapshot) (gp.TurnStoreSnapshot, error) {
	var t *turns.Turn
	if strings.TrimSpace(item.Payload) != "" {
		decoded, err := serde.FromYAML([]byte(item.Payload))
		if err != nil {
			return gp.TurnStoreSnapshot{}, errors.Wrap(err, "pinocchio js turn store: decode turn payload")
		}
		t = decoded
	}
	return gp.TurnStoreSnapshot{
		ConvID:      item.ConvID,
		SessionID:   item.SessionID,
		TurnID:      item.TurnID,
		Phase:       item.Phase,
		RuntimeKey:  item.RuntimeKey,
		InferenceID: item.InferenceID,
		CreatedAtMs: item.CreatedAtMs,
		Turn:        t,
	}, nil
}

func turnSessionID(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	if v, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func turnRuntimeKey(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	if v, ok, err := turns.KeyTurnMetaRuntime.Get(t.Metadata); err == nil && ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func turnInferenceID(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	if v, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		return strings.TrimSpace(v)
	}
	return ""
}
