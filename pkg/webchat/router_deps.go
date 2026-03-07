package webchat

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
)

// ParsedRouterInputs is the parsed-values input surface preserved for compatibility wrappers.
type ParsedRouterInputs = *values.Values

// RouterDeps contains the already-resolved infrastructure used to build a Router.
type RouterDeps struct {
	StaticFS      fs.FS
	Settings      RouterSettings
	StreamBackend StreamBackend
	TimelineStore chatstore.TimelineStore
	TurnStore     chatstore.TurnStore
}

// BuildRouterDepsFromValues adapts parsed Glazed values into explicit router dependencies.
func BuildRouterDepsFromValues(ctx context.Context, parsed ParsedRouterInputs, staticFS fs.FS) (RouterDeps, error) {
	if parsed == nil {
		return RouterDeps{}, errors.New("parsed values are nil")
	}

	settings := RouterSettings{}
	if err := parsed.DecodeSectionInto(values.DefaultSlug, &settings); err != nil {
		return RouterDeps{}, errors.Wrap(err, "parse router settings")
	}

	streamBackend, err := NewStreamBackendFromValues(ctx, parsed)
	if err != nil {
		return RouterDeps{}, err
	}

	timelineStore, err := NewTimelineStoreFromSettings(settings)
	if err != nil {
		return RouterDeps{}, err
	}
	turnStore, err := NewTurnStoreFromSettings(settings)
	if err != nil {
		return RouterDeps{}, err
	}

	return RouterDeps{
		StaticFS:      staticFS,
		Settings:      settings,
		StreamBackend: streamBackend,
		TimelineStore: timelineStore,
		TurnStore:     turnStore,
	}, nil
}

// NewDefaultTimelineStore returns the in-memory fallback used when no durable timeline store is configured.
func NewDefaultTimelineStore(settings RouterSettings) chatstore.TimelineStore {
	return chatstore.NewInMemoryTimelineStore(settings.TimelineInMemoryMaxEntities)
}

// NewTimelineStoreFromSettings builds the configured timeline store, falling back to the default in-memory store.
func NewTimelineStoreFromSettings(settings RouterSettings) (chatstore.TimelineStore, error) {
	if dsn := strings.TrimSpace(settings.TimelineDSN); dsn != "" {
		store, err := chatstore.NewSQLiteTimelineStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open timeline store (dsn)")
		}
		return store, nil
	}
	if p := strings.TrimSpace(settings.TimelineDB); p != "" {
		if dir := filepath.Dir(p); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, errors.Wrap(err, "create timeline db dir")
			}
		}
		dsn, err := chatstore.SQLiteTimelineDSNForFile(p)
		if err != nil {
			return nil, errors.Wrap(err, "build timeline DSN")
		}
		store, err := chatstore.NewSQLiteTimelineStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open timeline store (file)")
		}
		return store, nil
	}
	return NewDefaultTimelineStore(settings), nil
}

// NewTurnStoreFromSettings builds the configured durable turn store, if any.
func NewTurnStoreFromSettings(settings RouterSettings) (chatstore.TurnStore, error) {
	if dsn := strings.TrimSpace(settings.TurnsDSN); dsn != "" {
		store, err := chatstore.NewSQLiteTurnStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open turn store (dsn)")
		}
		return store, nil
	}
	if p := strings.TrimSpace(settings.TurnsDB); p != "" {
		if dir := filepath.Dir(p); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, errors.Wrap(err, "create turns db dir")
			}
		}
		dsn, err := chatstore.SQLiteTurnDSNForFile(p)
		if err != nil {
			return nil, errors.Wrap(err, "build turn DSN")
		}
		store, err := chatstore.NewSQLiteTurnStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open turn store (file)")
		}
		return store, nil
	}
	return nil, nil
}
