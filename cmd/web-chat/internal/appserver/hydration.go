package appserver

import (
	"fmt"
	"os"
	"path/filepath"

	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	storesqlite "github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/sqlite"
)

func newHydrationStore(s *Server, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error) {
	if s == nil || reg == nil {
		return nil, nil, fmt.Errorf("app server or schema registry is nil")
	}
	if s.sqliteDSN == "" && s.sqliteDBPath == "" {
		store, err := storesqlite.NewInMemory(reg)
		if err != nil {
			return nil, nil, err
		}
		return store, store.Close, nil
	}
	dsn := s.sqliteDSN
	if dsn == "" {
		if dir := filepath.Dir(s.sqliteDBPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, nil, err
			}
		}
		var err error
		dsn, err = storesqlite.FileDSN(s.sqliteDBPath)
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
