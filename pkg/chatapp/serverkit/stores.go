package serverkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	storemysql "github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/mysql"
	storesqlite "github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/sqlite"
	"github.com/go-sql-driver/mysql"
)

// StoreBackend identifies the persistence implementation selected by a
// StoreSpec. Backend selection is explicit; a DSN is never classified by its
// punctuation.
type StoreBackend string

const (
	StoreBackendDisabled StoreBackend = "disabled"
	StoreBackendMemory   StoreBackend = "memory"
	StoreBackendSQLite   StoreBackend = "sqlite"
	StoreBackendMySQL    StoreBackend = "mysql"
)

// StoreSpec describes one persistence component. Path is the SQLite file-path
// convenience; DSN is used by SQLite or MySQL according to Backend.
type StoreSpec struct {
	Backend StoreBackend
	DSN     string
	Path    string
}

type StoreOptions struct {
	Timeline StoreSpec
	Turns    StoreSpec
}

// StoreFactory owns persistence construction so every composition root can use
// the same backend policy and tests can inject constructor spies.
type StoreFactory struct {
	OpenSQLiteTurn      func(string) (chatstore.TurnStore, error)
	OpenMySQLTurn       func(context.Context, string) (chatstore.TurnStore, error)
	OpenSQLiteHydration func(string, *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, error)
	OpenMySQLHydration  func(context.Context, string, *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, error)
}

type Stores struct {
	Timeline sessionstream.HydrationStore
	Turns    chatstore.TurnStore
	Close    func() error
}

func hydrationClose(store sessionstream.HydrationStore) func() error {
	if closer, ok := store.(interface{ Close() error }); ok {
		return closer.Close
	}
	return func() error { return nil }
}

func turnClose(store chatstore.TurnStore) func() error {
	if closer, ok := store.(interface{ Close() error }); ok {
		return closer.Close
	}
	return func() error { return nil }
}

func (f StoreFactory) OpenHydration(ctx context.Context, spec StoreSpec, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error) {
	if reg == nil {
		return nil, nil, fmt.Errorf("schema registry is nil")
	}
	if ctx == nil {
		return nil, nil, fmt.Errorf("context is nil")
	}
	backend, err := resolveBackend(spec)
	if err != nil {
		return nil, nil, err
	}
	spec.DSN = strings.TrimSpace(spec.DSN)
	spec.Path = strings.TrimSpace(spec.Path)
	switch backend {
	case StoreBackendDisabled:
		return nil, func() error { return nil }, nil
	case StoreBackendMemory:
		open := f.OpenSQLiteHydration
		if open == nil {
			open = func(dsn string, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, error) {
				return storesqlite.NewInMemory(reg)
			}
		}
		store, err := open("", reg)
		if err != nil {
			return nil, nil, fmt.Errorf("open in-memory hydration store: %w", err)
		}
		return store, hydrationClose(store), nil
	case StoreBackendSQLite:
		dsn, err := sqliteDSN(spec)
		if err != nil {
			return nil, nil, err
		}
		open := f.OpenSQLiteHydration
		if open == nil {
			open = func(dsn string, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, error) {
				return storesqlite.New(dsn, reg)
			}
		}
		store, err := open(dsn, reg)
		if err != nil {
			return nil, nil, fmt.Errorf("open sqlite hydration store: %w", err)
		}
		return store, hydrationClose(store), nil
	case StoreBackendMySQL:
		if err := validateMySQLDSN(spec.DSN); err != nil {
			return nil, nil, err
		}
		open := f.OpenMySQLHydration
		if open == nil {
			open = func(ctx context.Context, dsn string, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, error) {
				return storemysql.Open(ctx, dsn, reg)
			}
		}
		store, err := open(ctx, spec.DSN, reg)
		if err != nil {
			return nil, nil, fmt.Errorf("open mysql hydration store: %w", err)
		}
		return store, hydrationClose(store), nil
	default:
		return nil, nil, fmt.Errorf("unsupported persistence backend %q", backend)
	}
}

func (f StoreFactory) OpenTurn(ctx context.Context, spec StoreSpec) (chatstore.TurnStore, func() error, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("context is nil")
	}
	backend, err := resolveBackend(spec)
	if err != nil {
		return nil, nil, err
	}
	spec.DSN = strings.TrimSpace(spec.DSN)
	spec.Path = strings.TrimSpace(spec.Path)
	switch backend {
	case StoreBackendDisabled:
		return nil, func() error { return nil }, nil
	case StoreBackendMemory:
		store := NewMemoryTurnStore()
		return store, store.Close, nil
	case StoreBackendSQLite:
		dsn, err := sqliteTurnDSN(spec)
		if err != nil {
			return nil, nil, err
		}
		open := f.OpenSQLiteTurn
		if open == nil {
			open = func(dsn string) (chatstore.TurnStore, error) {
				return chatstore.NewSQLiteTurnStore(dsn)
			}
		}
		store, err := open(dsn)
		if err != nil {
			return nil, nil, fmt.Errorf("open sqlite turn store: %w", err)
		}
		return store, turnClose(store), nil
	case StoreBackendMySQL:
		if err := validateMySQLDSN(spec.DSN); err != nil {
			return nil, nil, err
		}
		open := f.OpenMySQLTurn
		if open == nil {
			open = func(ctx context.Context, dsn string) (chatstore.TurnStore, error) {
				return chatstore.NewMySQLTurnStore(ctx, dsn)
			}
		}
		store, err := open(ctx, spec.DSN)
		if err != nil {
			return nil, nil, fmt.Errorf("open mysql turn store: %w", err)
		}
		return store, turnClose(store), nil
	default:
		return nil, nil, fmt.Errorf("unsupported persistence backend %q", backend)
	}
}

func OpenStores(ctx context.Context, opts StoreOptions, reg *sessionstream.SchemaRegistry) (*Stores, error) {
	factory := StoreFactory{}
	timeline, closeTimeline, err := factory.OpenHydration(ctx, opts.Timeline, reg)
	if err != nil {
		return nil, err
	}
	turns, closeTurns, err := factory.OpenTurn(ctx, opts.Turns)
	if err != nil {
		_ = closeTimeline()
		return nil, err
	}
	return &Stores{
		Timeline: timeline,
		Turns:    turns,
		Close:    func() error { return CloseAll(closeTurns, closeTimeline) },
	}, nil
}

func OpenHydrationStore(ctx context.Context, spec StoreSpec, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error) {
	return (StoreFactory{}).OpenHydration(ctx, spec, reg)
}

func OpenTurnStore(ctx context.Context, opts StoreOptions) (chatstore.TurnStore, func() error, error) {
	return (StoreFactory{}).OpenTurn(ctx, opts.Turns)
}

func resolveBackend(spec StoreSpec) (StoreBackend, error) {
	backend := StoreBackend(strings.ToLower(strings.TrimSpace(string(spec.Backend))))
	dsn := strings.TrimSpace(spec.DSN)
	path := strings.TrimSpace(spec.Path)
	if backend == "" {
		if dsn != "" {
			return "", fmt.Errorf("persistence DSN requires an explicit backend")
		}
		if path != "" {
			backend = StoreBackendSQLite
		} else {
			backend = StoreBackendDisabled
		}
	}
	switch backend {
	case StoreBackendDisabled, StoreBackendMemory:
		if dsn != "" || path != "" {
			return "", fmt.Errorf("persistence backend %q does not accept DSN or path", backend)
		}
	case StoreBackendSQLite:
		if dsn != "" && path != "" {
			return "", fmt.Errorf("sqlite persistence accepts exactly one of DSN or path")
		}
		if dsn == "" && path == "" {
			return "", fmt.Errorf("sqlite persistence requires DSN or path")
		}
	case StoreBackendMySQL:
		if dsn == "" {
			return "", fmt.Errorf("mysql persistence requires DSN")
		}
		if path != "" {
			return "", fmt.Errorf("mysql persistence does not accept a path")
		}
	default:
		return "", fmt.Errorf("unsupported persistence backend %q", backend)
	}
	return backend, nil
}

func sqliteDSN(spec StoreSpec) (string, error) {
	if strings.TrimSpace(spec.DSN) != "" {
		return strings.TrimSpace(spec.DSN), nil
	}
	if err := ensureParentDir(spec.Path); err != nil {
		return "", err
	}
	dsn, err := storesqlite.FileDSN(strings.TrimSpace(spec.Path))
	if err != nil {
		return "", err
	}
	return dsn, nil
}

func sqliteTurnDSN(spec StoreSpec) (string, error) {
	if strings.TrimSpace(spec.DSN) != "" {
		return strings.TrimSpace(spec.DSN), nil
	}
	if err := ensureParentDir(spec.Path); err != nil {
		return "", err
	}
	dsn, err := chatstore.SQLiteTurnDSNForFile(strings.TrimSpace(spec.Path))
	if err != nil {
		return "", err
	}
	return dsn, nil
}

func validateMySQLDSN(dsn string) error {
	if _, err := mysql.ParseDSN(strings.TrimSpace(dsn)); err != nil {
		return fmt.Errorf("invalid mysql DSN: %w", err)
	}
	return nil
}

func CloseAll(fns ...func() error) error {
	var first error
	for i := len(fns) - 1; i >= 0; i-- {
		if fns[i] == nil {
			continue
		}
		if err := fns[i](); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func ensureParentDir(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

type MemoryTurnStore struct {
	mu    sync.RWMutex
	turns []chatstore.TurnSnapshot
}

func NewMemoryTurnStore() *MemoryTurnStore { return &MemoryTurnStore{} }

func (s *MemoryTurnStore) Save(_ context.Context, convID, sessionID, turnID, phase string, createdAtMs int64, payload string, opts chatstore.TurnSaveOptions) error {
	if s == nil {
		return nil
	}
	snap := chatstore.TurnSnapshot{
		ConvID:      strings.TrimSpace(convID),
		SessionID:   strings.TrimSpace(sessionID),
		TurnID:      strings.TrimSpace(turnID),
		Phase:       strings.TrimSpace(phase),
		RuntimeKey:  strings.TrimSpace(opts.RuntimeKey),
		InferenceID: strings.TrimSpace(opts.InferenceID),
		CreatedAtMs: createdAtMs,
		Payload:     payload,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.turns {
		if s.turns[i].ConvID == snap.ConvID && s.turns[i].SessionID == snap.SessionID && s.turns[i].TurnID == snap.TurnID && s.turns[i].Phase == snap.Phase {
			s.turns[i] = snap
			return nil
		}
	}
	s.turns = append(s.turns, snap)
	return nil
}

func (s *MemoryTurnStore) List(_ context.Context, q chatstore.TurnQuery) ([]chatstore.TurnSnapshot, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]chatstore.TurnSnapshot, 0, len(s.turns))
	for _, snap := range s.turns {
		if q.ConvID != "" && snap.ConvID != q.ConvID {
			continue
		}
		if q.SessionID != "" && snap.SessionID != q.SessionID {
			continue
		}
		if q.Phase != "" && snap.Phase != q.Phase {
			continue
		}
		if q.SinceMs > 0 && snap.CreatedAtMs < q.SinceMs {
			continue
		}
		out = append(out, snap)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAtMs < out[j].CreatedAtMs })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[len(out)-q.Limit:]
	}
	return out, nil
}

func (s *MemoryTurnStore) LoadLatestTurn(_ context.Context, convID, phase string) (*chatstore.TurnSnapshot, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *chatstore.TurnSnapshot
	for i := range s.turns {
		snap := s.turns[i]
		if convID != "" && snap.ConvID != convID {
			continue
		}
		if phase != "" && snap.Phase != phase {
			continue
		}
		if latest == nil || snap.CreatedAtMs > latest.CreatedAtMs {
			snapCopy := snap
			latest = &snapCopy
		}
	}
	return latest, nil
}

func (s *MemoryTurnStore) Close() error { return nil }

var _ chatstore.TurnStore = (*MemoryTurnStore)(nil)
