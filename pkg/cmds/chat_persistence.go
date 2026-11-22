package cmds

import (
	"context"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/pkg/errors"
)

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
