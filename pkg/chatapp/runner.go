package chatapp

import (
	"fmt"
	"time"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	storesqlite "github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/sqlite"
)

// RunnerOptions configures a non-web chatapp/sessionstream runtime.
type RunnerOptions struct {
	Registry       *sessionstream.SchemaRegistry
	HydrationStore sessionstream.HydrationStore
	UIFanout       sessionstream.UIFanout
	TurnStore      chatstore.TurnStore
	Plugins        []ChatPlugin
	ChunkDelay     time.Duration
}

// Runner is the reusable non-web wiring of chatapp on top of sessionstream.
//
// It mirrors the core setup used by cmd/web-chat/internal/appserver.Server without HTTP or
// websocket assumptions, so CLI, RPC, and TUI adapters can share the same
// command/event/projection/hydration pipeline.
type Runner struct {
	Registry *sessionstream.SchemaRegistry
	Store    sessionstream.HydrationStore
	Hub      *sessionstream.Hub
	Engine   *Engine
	Service  *Service

	closeFn func() error
}

// NewRunner builds a chatapp runner with schema registration, hydration store,
// hub, engine, projections, and service already installed.
func NewRunner(opts RunnerOptions) (*Runner, error) {
	reg := opts.Registry
	if reg == nil {
		reg = sessionstream.NewSchemaRegistry()
	}
	if err := RegisterSchemas(reg, opts.Plugins...); err != nil {
		return nil, err
	}

	store := opts.HydrationStore
	closeFn := func() error { return nil }
	if store == nil {
		memoryStore, err := storesqlite.NewInMemory(reg)
		if err != nil {
			return nil, err
		}
		store = memoryStore
		closeFn = memoryStore.Close
	}

	engineOptions := []Option{
		WithPlugins(opts.Plugins...),
		WithTurnStore(opts.TurnStore),
	}
	if opts.ChunkDelay > 0 {
		engineOptions = append(engineOptions, WithChunkDelay(opts.ChunkDelay))
	}
	engine := NewEngine(engineOptions...)

	hubOptions := []sessionstream.HubOption{
		sessionstream.WithSchemaRegistry(reg),
		sessionstream.WithHydrationStore(store),
	}
	if opts.UIFanout != nil {
		hubOptions = append(hubOptions, sessionstream.WithUIFanout(opts.UIFanout))
	}
	hub, err := sessionstream.NewHub(hubOptions...)
	if err != nil {
		_ = closeFn()
		return nil, err
	}
	if err := Install(hub, engine); err != nil {
		_ = closeFn()
		return nil, err
	}
	service, err := NewService(hub, engine)
	if err != nil {
		_ = closeFn()
		return nil, err
	}
	return &Runner{
		Registry: reg,
		Store:    store,
		Hub:      hub,
		Engine:   engine,
		Service:  service,
		closeFn:  closeFn,
	}, nil
}

// Close releases resources owned by the runner, such as the default in-memory
// SQLite hydration store.
func (r *Runner) Close() error {
	if r == nil || r.closeFn == nil {
		return nil
	}
	if err := r.closeFn(); err != nil {
		return fmt.Errorf("close chatapp runner: %w", err)
	}
	return nil
}
