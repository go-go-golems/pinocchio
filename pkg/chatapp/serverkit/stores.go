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
	storesqlite "github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/sqlite"
)

type EmptyTurnStoreMode string

const (
	EmptyTurnStoreDisabled EmptyTurnStoreMode = "disabled"
	EmptyTurnStoreMemory   EmptyTurnStoreMode = "memory"
)

type StoreOptions struct {
	TimelineDSN string
	TimelineDB  string
	TurnsDSN    string
	TurnsDB     string

	// EmptyTurnStore controls what OpenTurnStore returns when neither TurnsDSN
	// nor TurnsDB is set. The zero value preserves web-chat/CoinVault behavior:
	// no turn store. Chat overlay uses EmptyTurnStoreMemory so real-runtime
	// history works without an explicit SQLite file.
	EmptyTurnStore EmptyTurnStoreMode
}

type Stores struct {
	Timeline sessionstream.HydrationStore
	Turns    chatstore.TurnStore
	Close    func() error
}

func OpenStores(opts StoreOptions, reg *sessionstream.SchemaRegistry) (*Stores, error) {
	timeline, closeTimeline, err := OpenHydrationStore(opts.TimelineDSN, opts.TimelineDB, reg)
	if err != nil {
		return nil, err
	}
	turns, closeTurns, err := OpenTurnStore(opts)
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

func OpenHydrationStore(dsn, dbPath string, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error) {
	if reg == nil {
		return nil, nil, fmt.Errorf("schema registry is nil")
	}
	dsn = strings.TrimSpace(dsn)
	dbPath = strings.TrimSpace(dbPath)
	if dsn == "" && dbPath == "" {
		store, err := storesqlite.NewInMemory(reg)
		if err != nil {
			return nil, nil, err
		}
		return store, store.Close, nil
	}
	if dsn == "" {
		if err := ensureParentDir(dbPath); err != nil {
			return nil, nil, err
		}
		var err error
		dsn, err = storesqlite.FileDSN(dbPath)
		if err != nil {
			return nil, nil, err
		}
	}
	store, err := storesqlite.New(dsn, reg)
	if err != nil {
		return nil, nil, err
	}
	return store, store.Close, nil
}

func OpenTurnStore(opts StoreOptions) (chatstore.TurnStore, func() error, error) {
	dsn := strings.TrimSpace(opts.TurnsDSN)
	dbPath := strings.TrimSpace(opts.TurnsDB)
	if dsn == "" && dbPath == "" {
		if opts.EmptyTurnStore == EmptyTurnStoreMemory {
			store := NewMemoryTurnStore()
			return store, store.Close, nil
		}
		return nil, func() error { return nil }, nil
	}
	if dsn == "" {
		if err := ensureParentDir(dbPath); err != nil {
			return nil, nil, err
		}
		var err error
		dsn, err = chatstore.SQLiteTurnDSNForFile(dbPath)
		if err != nil {
			return nil, nil, err
		}
	}
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open turns store: %w", err)
	}
	return store, store.Close, nil
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
