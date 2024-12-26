package main

import (
	"html/template"
	"sync"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/rs/zerolog"
)

// Server handles the web UI and SSE connections
type Server struct {
	tmpl       *template.Template
	router     *events.EventRouter
	clients    map[string]*SSEClient
	clientsMux sync.RWMutex
	logger     zerolog.Logger
}

func NewServer(tmpl *template.Template, router *events.EventRouter) *Server {
	logger := zerolog.New(zerolog.NewConsoleWriter()).
		With().
		Timestamp().
		Caller().
		Str("component", "web-ui").
		Logger()

	logger.Level(zerolog.TraceLevel)

	return &Server{
		tmpl:    tmpl,
		router:  router,
		clients: make(map[string]*SSEClient, clientBufferSize),
		logger:  logger,
	}
}

func (s *Server) RegisterClient(client *SSEClient) {
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()
	s.clients[client.ID] = client
	s.logger.Info().Str("client_id", client.ID).Msg("Registered new client")
}

func (s *Server) UnregisterClient(clientID string) {
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()
	if client, ok := s.clients[clientID]; ok {
		close(client.MessageChan)
		close(client.DisconnectCh)
		delete(s.clients, clientID)
		s.logger.Info().Str("client_id", clientID).Msg("Unregistered client")
	}
}
