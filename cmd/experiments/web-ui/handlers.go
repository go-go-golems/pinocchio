package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/google/uuid"
)

// RegisterHandlers sets up all HTTP handlers for the server
func RegisterHandlers(server *Server, tmpl *template.Template) {
	// Serve index page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if there's a client ID in the URL
		clientID := r.URL.Query().Get("client_id")

		// Verify client exists if ID provided
		if clientID != "" {
			server.clientsMux.RLock()
			_, exists := server.clients[clientID]
			server.clientsMux.RUnlock()
			if !exists {
				// Client doesn't exist, redirect to root
				server.logger.Info().Str("client_id", clientID).Msg("Client not found, redirecting to root")
				http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
				return
			}
		}

		data := TemplateData{
			ClientID: clientID,
		}

		if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
			server.logger.Error().Err(err).Msg("Failed to render index page")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Handle SSE events endpoint
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		// Get client ID from query params
		clientID := r.URL.Query().Get("client_id")
		server.logger.Info().Str("client_id", clientID).Msg("Handling SSE events")
		if clientID == "" {
			server.logger.Error().Msg("No client ID provided for events")
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
			server.logger.Error().Msg("Streaming unsupported")
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		// Get existing client or create new one
		server.clientsMux.RLock()
		client, exists := server.clients[clientID]
		server.clientsMux.RUnlock()

		if !exists {
			server.logger.Info().Str("client_id", clientID).Msg("Creating new client")
			client = NewSSEClient(clientID, tmpl)
			server.RegisterClient(client)
		}
		defer server.UnregisterClient(client.ID)

		server.logger.Info().Str("client_id", client.ID).Msg("SSE connection established")

		// Write events to client with timeout to prevent stuck connections
		for {
			select {
			case <-r.Context().Done():
				server.logger.Info().Str("client_id", client.ID).Msg("SSE connection closed by client")
				return
			case msg, ok := <-client.MessageChan:
				if !ok {
					server.logger.Info().Str("client_id", client.ID).Msg("SSE message channel closed")
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
				server.logger.Debug().Str("client_id", client.ID).Msg("Sent heartbeat")
			}
		}
	})

	// Handle chat endpoint - processes chat messages
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		clientID := r.FormValue("client_id")
		message := r.FormValue("message")

		// For new chats, generate a client ID
		if clientID == "" {
			clientID = uuid.New().String()
			server.logger.Info().Str("client_id", clientID).Msg("Starting new chat")
		}

		// Get or create client
		server.clientsMux.RLock()
		client, exists := server.clients[clientID]
		server.clientsMux.RUnlock()

		if !exists {
			client = NewSSEClient(clientID, tmpl)
			server.RegisterClient(client)
		}

		// Create new step for this message
		if err := client.CreateStep(server.router); err != nil {
			server.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to create step")
			http.Error(w, fmt.Sprintf("Error creating step: %v", err), http.StatusInternalServerError)
			return
		}

		// Run handlers
		err := server.router.RunHandlers(context.Background())
		if err != nil {
			server.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to run handlers")
			http.Error(w, fmt.Sprintf("Error running handlers: %v", err), http.StatusInternalServerError)
			return
		}

		// Create conversation messages
		msgs := []*conversation.Message{
			conversation.NewChatMessage(conversation.RoleSystem, "You are a helpful assistant."),
		}

		// Add user message if provided
		if message != "" {
			msgs = append(msgs, conversation.NewChatMessage(conversation.RoleUser, message))
		} else {
			msgs = append(msgs, conversation.NewChatMessage(conversation.RoleUser, "Hello! Please tell me a short story about a robot."))
		}

		// Start step with conversation
		if err := client.StartStep(context.Background(), msgs); err != nil {
			server.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to start step")
			http.Error(w, fmt.Sprintf("Error starting step: %v", err), http.StatusInternalServerError)
			return
		}

		// For new chats, return the chat container template
		if !exists {
			data := TemplateData{
				ClientID: clientID,
			}
			w.Header().Set("HX-Push-Url", fmt.Sprintf("/?client_id=%s", clientID))
			if err := tmpl.ExecuteTemplate(w, "chat-container", data); err != nil {
				server.logger.Error().Err(err).Msg("Failed to render chat container")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}

		// For existing chats, return empty response as events will be sent via SSE
		w.WriteHeader(http.StatusOK)
	})
}
