package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/client"
	webconv "github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/conversation"
	"github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/templates"
	"github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/templates/components"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Server handles the web UI and SSE connections
type Server struct {
	router     *events.EventRouter
	clients    map[string]*client.ChatClient
	clientsMux sync.RWMutex
	logger     zerolog.Logger
}

func NewServer(router *events.EventRouter) *Server {
	logger := zerolog.New(zerolog.NewConsoleWriter()).
		With().
		Timestamp().
		Caller().
		Str("component", "web-ui").
		Logger()

	logger.Level(zerolog.TraceLevel)

	return &Server{
		router:  router,
		clients: make(map[string]*client.ChatClient),
		logger:  logger,
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
		close(client.MessageChan)
		close(client.DisconnectCh)
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

	// Get existing client or create new one
	s.clientsMux.RLock()
	client_, exists := s.clients[clientID]
	s.clientsMux.RUnlock()

	if !exists {
		s.logger.Info().Str("client_id", clientID).Msg("Creating new client")
		client_ = client.NewChatClient(clientID, s.router)
		s.RegisterClient(client_)
	}
	defer s.UnregisterClient(client_.ID)

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
	s.clientsMux.RLock()
	client_, exists := s.clients[clientID]
	s.clientsMux.RUnlock()

	if !exists {
		client_ = client.NewChatClient(clientID, s.router)
		s.RegisterClient(client_)
	}

	// Send user message
	if err := client_.SendUserMessage(context.Background(), message); err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to send message")
		http.Error(w, fmt.Sprintf("Error sending message: %v", err), http.StatusInternalServerError)
		return
	}

	// For new chats, return the chat container template
	if !exists {
		conv, err := webconv.ConvertConversation(client_.GetConversation())
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to convert conversation")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		component := templates.Index(clientID, conv)
		w.Header().Set("HX-Push-Url", fmt.Sprintf("/?client_id=%s", clientID))
		if err := component.Render(context.Background(), w); err != nil {
			s.logger.Error().Err(err).Msg("Failed to render chat container")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	// For existing chats, get the last message and render it
	conv := client_.GetConversation()
	if len(conv) == 0 {
		s.logger.Error().Msg("No messages in conversation")
		http.Error(w, "No messages in conversation", http.StatusInternalServerError)
		return
	}

	lastMsg := conv[len(conv)-1]
	webMsg, err := webconv.ConvertMessage(lastMsg)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to convert message")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render message container with user message and streaming div
	component := components.MessageContainer(webMsg)
	if err := component.Render(context.Background(), w); err != nil {
		s.logger.Error().Err(err).Msg("Failed to render message container")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add out-of-band swap to clear input
	w.Header().Set("HX-Trigger", `{"clearInput": ""}`)
}
