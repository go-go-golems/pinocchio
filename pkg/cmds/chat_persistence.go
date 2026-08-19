package cmds

import (
	"context"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
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

func openCLISessionstreamHydrationStore(ctx context.Context, settings run.PersistenceSettings, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func(), error) {
	if strings.TrimSpace(settings.TimelineDSN) == "" && strings.TrimSpace(settings.TimelineDB) == "" && strings.TrimSpace(settings.TimelineBackend) == "" {
		return nil, func() {}, nil
	}
	store, closeStore, err := serverkit.OpenHydrationStore(ctx, serverkit.StoreSpec{
		Backend: serverkit.StoreBackend(settings.TimelineBackend),
		DSN:     settings.TimelineDSN,
		Path:    settings.TimelineDB,
	}, reg)
	if err != nil {
		return nil, func() {}, errors.Wrap(err, "open CLI hydration store")
	}
	return store, func() { _ = closeStore() }, nil
}

func loadLatestCLIFinalTurn(ctx context.Context, store chatstore.TurnStore, sessionID string) (*turns.Turn, error) {
	if store == nil {
		return nil, errors.New("resume requires --turns-db or --turns-dsn")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("resume requires --session-id")
	}
	snap, err := store.LoadLatestTurn(ctx, sessionID, "final")
	if err != nil {
		return nil, errors.Wrap(err, "load latest final turn")
	}
	if snap == nil {
		return nil, errors.Errorf("no persisted final turn found for session %q", sessionID)
	}
	t, err := serde.FromYAML([]byte(snap.Payload))
	if err != nil {
		return nil, errors.Wrap(err, "decode latest final turn")
	}
	if t == nil {
		return nil, errors.Errorf("latest final turn for session %q decoded to nil", sessionID)
	}
	_ = turns.KeyTurnMetaSessionID.Set(&t.Metadata, sessionID)
	return t, nil
}

func openCLITurnStore(ctx context.Context, settings run.PersistenceSettings) (chatstore.TurnStore, func(), error) {
	if strings.TrimSpace(settings.TurnsDSN) == "" && strings.TrimSpace(settings.TurnsDB) == "" && strings.TrimSpace(settings.TurnsBackend) == "" {
		return nil, func() {}, nil
	}
	store, closeStore, err := serverkit.OpenTurnStore(ctx, serverkit.StoreOptions{Turns: serverkit.StoreSpec{
		Backend: serverkit.StoreBackend(settings.TurnsBackend),
		DSN:     settings.TurnsDSN,
		Path:    settings.TurnsDB,
	}})
	if err != nil {
		return nil, func() {}, errors.Wrap(err, "open CLI turn store")
	}
	return store, func() { _ = closeStore() }, nil
}
