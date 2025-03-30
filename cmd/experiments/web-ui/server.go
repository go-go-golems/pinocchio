package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/client"
	webconv "github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/conversation"
	"github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/templates"
	"github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/templates/components"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Server handles the web UI and SSE connections
type Server struct {
	clients    map[string]*client.ChatClient
	clientsMux sync.RWMutex
	logger     zerolog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewServer() *Server {
	logger := zerolog.New(zerolog.NewConsoleWriter()).
		With().
		Timestamp().
		Caller().
		Str("component", "web-ui").
		Logger()

	logger.Level(zerolog.TraceLevel)

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		clients: make(map[string]*client.ChatClient),
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (s *Server) RegisterClient(client *client.ChatClient) {
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()
	s.clients[client.ID] = client
	s.logger.Info().Str("client_id", client.ID).Msg("Registered new client")
}

func (s *Server) UnregisterClient(clientID string) {
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()
	if client, ok := s.clients[clientID]; ok {
		client.Close()
		delete(s.clients, clientID)
		s.logger.Info().Str("client_id", clientID).Msg("Unregistered client")
	}
}

// Register sets up all HTTP handlers for the server
func (s *Server) Register() {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/events", s.handleEvents)
	http.HandleFunc("/chat", s.handleChat)
}

// handleIndex serves the index page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Check if there's a client ID in the URL
	clientID := r.URL.Query().Get("client_id")

	// Verify client exists if ID provided
	var messages *webconv.WebConversation
	if clientID != "" {
		s.clientsMux.RLock()
		client, exists := s.clients[clientID]
		s.clientsMux.RUnlock()

		if !exists {
			// Client doesn't exist, redirect to root
			s.logger.Info().Str("client_id", clientID).Msg("Client not found, redirecting to root")
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		// Convert conversation to web format
		conv, err := webconv.ConvertConversation(client.GetConversation())
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to convert conversation")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		messages = conv
	}

	component := templates.Index(clientID, messages)
	if err := component.Render(context.Background(), w); err != nil {
		s.logger.Error().Err(err).Msg("Failed to render index page")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleEvents handles SSE events endpoint
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	// Get client ID from query params
	clientID := r.URL.Query().Get("client_id")
	s.logger.Info().Str("client_id", clientID).Msg("Handling SSE events")
	if clientID == "" {
		s.logger.Error().Msg("No client ID provided for events")
		http.Error(w, "No client ID provided", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.logger.Error().Msg("Streaming unsupported")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	client_, isNew, err := s.getOrCreateClient(clientID)
	if err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to get or create client")
		http.Error(w, fmt.Sprintf("Error getting or creating client: %v", err), http.StatusInternalServerError)
		return
	}

	if isNew {
		s.logger.Info().Str("client_id", clientID).Msg("Created new client")
	}

	s.logger.Info().Str("client_id", client_.ID).Msg("SSE connection established")

	// Write events to client with timeout to prevent stuck connections
	for {
		select {
		case <-r.Context().Done():
			s.logger.Info().Str("client_id", client_.ID).Msg("SSE connection closed by client")
			return
		case msg, ok := <-client_.MessageChan:
			if !ok {
				s.logger.Info().Str("client_id", client_.ID).Msg("SSE message channel closed")
				return
			}
			// Handle multiline messages by prefacing each line with data:
			lines := strings.Split(msg, "\n")
			s.logger.Info().Str("client_id", client_.ID).Str("message", msg).Msg("Sending SSE message")
			fmt.Fprintf(w, "event: message\n")
			for _, line := range lines {
				fmt.Fprintf(w, "data: %s\n", line)
			}
			fmt.Fprintf(w, "\n")
			flusher.Flush()
		case <-time.After(30 * time.Second):
			// Send heartbeat to keep connection alive
			fmt.Fprintf(w, "event: heartbeat\ndata: ping\n\n")
			flusher.Flush()
			s.logger.Debug().Str("client_id", client_.ID).Msg("Sent heartbeat")
		}
	}
}

// handleChat processes chat messages
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientID := r.FormValue("client_id")
	message := r.FormValue("message")

	// For new chats, generate a client ID
	if clientID == "" {
		clientID = uuid.New().String()
		s.logger.Info().Str("client_id", clientID).Msg("Starting new chat")
	}

	// Get or create client
	client_, exists, err := s.getOrCreateClient(clientID)
	if err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to get or create client")
		http.Error(w, fmt.Sprintf("Error getting or creating client: %v", err), http.StatusInternalServerError)
		return
	}

	// Send user message
	if err := client_.SendUserMessage(context.Background(), message); err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to send message")
		http.Error(w, fmt.Sprintf("Error sending message: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert conversation for rendering
	conv, err := webconv.ConvertConversation(client_.GetConversation())
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to convert conversation")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// For new chats, update the URL and set up SSE
	if !exists {
		w.Header().Set("HX-Push-Url", fmt.Sprintf("/?client_id=%s", clientID))
		err = components.EventContainer(clientID, true).Render(context.Background(), w)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to render event container")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Render conversation history
	err = components.ConversationHistory(conv, true).Render(context.Background(), w)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to render conversation history")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = components.ChatInput(clientID).Render(context.Background(), w)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to render chat input")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// getOrCreateClient retrieves an existing client or creates a new one
func (s *Server) getOrCreateClient(clientID string) (*client.ChatClient, bool, error) {
	s.clientsMux.RLock()
	client_, exists := s.clients[clientID]
	s.clientsMux.RUnlock()

	if exists {
		return client_, false, nil
	}

	s.logger.Info().Str("client_id", clientID).Msg("Creating new client")
	client_, err := client.NewChatClient(clientID)
	if err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to create client")
		return nil, false, fmt.Errorf("failed to create client: %w", err)
	}

	s.RegisterClient(client_)

	// Start the client using the server's context
	if err := client_.Start(s.ctx); err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to start client")
		return nil, false, fmt.Errorf("failed to start client: %w", err)
	}

	return client_, true, nil
}

// Close properly cleans up the server and all its clients
func (s *Server) Close() error {
	s.logger.Info().Msg("Closing server")

	// Cancel the server context
	s.cancel()

	// Close all clients
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()

	var errs []error
	for id, client := range s.clients {
		if err := client.Close(); err != nil {
			s.logger.Error().Err(err).Str("client_id", id).Msg("Failed to close client")
			errs = append(errs, fmt.Errorf("failed to close client %s: %w", id, err))
		}
	}

	// Clear the clients map
	s.clients = make(map[string]*client.ChatClient)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing clients: %v", errs)
	}
	return nil
}
