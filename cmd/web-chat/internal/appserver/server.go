package appserver

import (
	"time"

	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatexport "github.com/go-go-golems/pinocchio/pkg/chatapp/export"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	wstransport "github.com/go-go-golems/sessionstream/pkg/sessionstream/transport/ws"
)

type Server struct {
	service             *chatapp.Service
	ws                  *wstransport.Server
	defaultProfile      string
	chunkDelay          time.Duration
	sqliteDSN           string
	sqliteDBPath        string
	runtimeResolver     RuntimeResolver
	turnStore           chatstore.TurnStore
	turnsDBPath         string
	exportService       *chatexport.Service
	chatPlugins         []chatapp.ChatPlugin
	frontendToolManager *frontendtools.Manager
	closeFn             func() error
}

func NewServer(opts ...Option) (*Server, error) {
	s := &Server{chunkDelay: 20 * time.Millisecond}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	reg := sessionstream.NewSchemaRegistry()
	if err := chatapp.RegisterSchemas(reg, s.chatPlugins...); err != nil {
		return nil, err
	}
	store, cleanup, err := newHydrationStore(s, reg)
	if err != nil {
		return nil, err
	}
	provider := &hydrationSnapshotProvider{server: s}
	ws, err := wstransport.NewServer(provider)
	if err != nil {
		return nil, err
	}
	engine := chatapp.NewEngine(chatapp.WithChunkDelay(s.chunkDelay), chatapp.WithPlugins(s.chatPlugins...), chatapp.WithTurnStore(s.turnStore))
	hubOptions := []sessionstream.HubOption{
		sessionstream.WithSchemaRegistry(reg),
		sessionstream.WithHydrationStore(store),
		sessionstream.WithUIFanout(ws),
	}
	hub, err := sessionstream.NewHub(hubOptions...)
	if err != nil {
		return nil, err
	}
	if err := chatapp.Install(hub, engine); err != nil {
		return nil, err
	}
	if s.frontendToolManager != nil {
		if err := s.frontendToolManager.Install(hub); err != nil {
			return nil, err
		}
	}
	service, err := chatapp.NewService(hub, engine)
	if err != nil {
		return nil, err
	}

	s.service = service
	s.exportService = chatexport.NewService(service, chatexport.WithTurnStore(s.turnStore), chatexport.WithTurnsDBPath(s.turnsDBPath))
	s.ws = ws
	s.closeFn = cleanup
	return s, nil
}

func (s *Server) Close() error {
	if s == nil || s.closeFn == nil {
		return nil
	}
	return s.closeFn()
}
