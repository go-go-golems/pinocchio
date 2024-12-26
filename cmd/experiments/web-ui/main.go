package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

//go:embed templates/*
var templatesFS embed.FS

// EventTemplateData holds the data for event templates
type EventTemplateData struct {
	Timestamp  string
	Completion string
	Text       string
	Name       string
	Input      string
	Result     string
	Error      string
}

// EventToHTML converts different event types to HTML snippets using templates
func EventToHTML(tmpl *template.Template, e chat.Event) (string, error) {
	data := EventTemplateData{
		Timestamp: time.Now().Format("15:04:05"),
	}

	var templateName string

	switch e_ := e.(type) {
	case *chat.EventPartialCompletionStart:
		templateName = "event-start"

	case *chat.EventPartialCompletion:
		templateName = "event-partial"
		data.Completion = e_.Completion

	case *chat.EventFinal:
		templateName = "event-final"
		data.Text = e_.Text

	case *chat.EventToolCall:
		templateName = "event-tool-call"
		data.Name = e_.ToolCall.Name
		data.Input = e_.ToolCall.Input

	case *chat.EventToolResult:
		templateName = "event-tool-result"
		data.Result = e_.ToolResult.Result

	case *chat.EventError:
		templateName = "event-error"
		data.Error = e_.Error().Error()

	case *chat.EventInterrupt:
		templateName = "event-interrupt"
		data.Text = e_.Text

	default:
		return "", fmt.Errorf("unknown event type: %T", e)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}

type SSEClient struct {
	ID           string
	MessageChan  chan string
	DisconnectCh chan struct{}
	DroppedMsgs  int64 // Counter for monitoring purposes
}

const (
	// Buffer sizes
	messageBufferSize = 100  // Number of messages to buffer per client
	clientBufferSize  = 1000 // Maximum number of clients
)

func NewSSEClient(id string) *SSEClient {
	return &SSEClient{
		ID:           id,
		MessageChan:  make(chan string, messageBufferSize),
		DisconnectCh: make(chan struct{}),
	}
}

// TrySend attempts to send a message to the client without blocking
// Returns true if the message was sent, false if it was dropped
func (c *SSEClient) TrySend(msg string) bool {
	select {
	case c.MessageChan <- msg:
		return true
	default:
		atomic.AddInt64(&c.DroppedMsgs, 1)
		return false
	}
}

type StepInstance struct {
	Step   *chat.EchoStep
	Topic  string
	Cancel context.CancelFunc
}

type Server struct {
	tmpl       *template.Template
	router     *events.EventRouter
	clients    map[string]*SSEClient
	steps      map[string]*StepInstance
	clientsMux sync.RWMutex
	stepsMux   sync.RWMutex
	logger     zerolog.Logger
	metrics    struct {
		TotalDroppedMsgs int64
	}
}

func NewServer(tmpl *template.Template, router *events.EventRouter) *Server {
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Str("component", "web-ui").Logger()
	return &Server{
		tmpl:    tmpl,
		router:  router,
		clients: make(map[string]*SSEClient, clientBufferSize),
		steps:   make(map[string]*StepInstance),
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

func (s *Server) CreateStep(clientID string) error {
	s.stepsMux.Lock()
	defer s.stepsMux.Unlock()

	// Cancel existing step if any
	if instance, ok := s.steps[clientID]; ok {
		instance.Cancel()
		delete(s.steps, clientID)
		s.logger.Info().Str("client_id", clientID).Msg("Cancelled existing step")
	}

	// Create new step
	step := chat.NewEchoStep()
	step.TimePerCharacter = 150 * time.Millisecond

	// Setup topic and event routing
	topic := fmt.Sprintf("chat-%s", clientID)
	if err := step.AddPublishedTopic(s.router.Publisher, topic); err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to setup event publishing")
		return fmt.Errorf("error setting up event publishing: %w", err)
	}

	// Add handler for this client's events
	s.router.AddHandler(
		fmt.Sprintf("sse-%s", clientID),
		topic,
		func(msg *message.Message) error {
			s.logger.Debug().
				Str("client_id", clientID).
				Str("message_id", msg.UUID).
				Str("metadata", fmt.Sprintf("%v", msg.Metadata)).
				Msg("Received message from router")

			// Parse event
			e, err := chat.NewEventFromJson(msg.Payload)
			if err != nil {
				s.logger.Error().Err(err).
					Str("client_id", clientID).
					Str("message_id", msg.UUID).
					Str("payload", string(msg.Payload)).
					Msg("Failed to parse event")
				return err
			}

			s.logger.Debug().
				Str("client_id", clientID).
				Str("message_id", msg.UUID).
				Str("event_type", string(e.Type())).
				Msg("Parsed event")

			// Convert to HTML
			html, err := EventToHTML(s.tmpl, e)
			if err != nil {
				s.logger.Error().Err(err).
					Str("client_id", clientID).
					Str("message_id", msg.UUID).
					Str("event_type", string(e.Type())).
					Msg("Failed to convert event to HTML")
				return err
			}

			s.logger.Debug().
				Str("client_id", clientID).
				Str("message_id", msg.UUID).
				Str("event_type", string(e.Type())).
				Int("html_length", len(html)).
				Msg("Converted event to HTML")

			// Send to client's message channel
			s.clientsMux.RLock()
			client, ok := s.clients[clientID]
			s.clientsMux.RUnlock()
			if !ok {
				s.logger.Warn().
					Str("client_id", clientID).
					Str("message_id", msg.UUID).
					Str("event_type", string(e.Type())).
					Msg("Client not found for event")
				return nil
			}

			// Try to send without blocking
			if !client.TrySend(html) {
				atomic.AddInt64(&s.metrics.TotalDroppedMsgs, 1)
				s.logger.Warn().
					Str("client_id", clientID).
					Str("message_id", msg.UUID).
					Str("event_type", string(e.Type())).
					Int64("total_dropped", atomic.LoadInt64(&s.metrics.TotalDroppedMsgs)).
					Int64("client_dropped", atomic.LoadInt64(&client.DroppedMsgs)).
					Msg("Dropped message for client")
			} else {
				s.logger.Debug().
					Str("client_id", clientID).
					Str("message_id", msg.UUID).
					Str("event_type", string(e.Type())).
					Msg("Sent message to client")
			}

			return nil
		},
	)

	// Store step with cancel function (to be set when started)
	s.steps[clientID] = &StepInstance{
		Step:  step,
		Topic: topic,
	}
	s.logger.Info().Str("client_id", clientID).Msg("Created new step")

	return nil
}

func (s *Server) StartStep(clientID string) error {
	s.stepsMux.Lock()
	instance, ok := s.steps[clientID]
	s.stepsMux.Unlock()

	if !ok {
		s.logger.Error().Str("client_id", clientID).Msg("No step found for client")
		return fmt.Errorf("no step found for client %s", clientID)
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	instance.Cancel = cancel

	// Create a simple conversation
	msgs := []*conversation.Message{
		conversation.NewChatMessage(conversation.RoleSystem, "You are a helpful assistant."),
		conversation.NewChatMessage(conversation.RoleUser, "Hello! Please tell me a short story about a robot."),
	}

	// Start step
	s.logger.Info().
		Str("client_id", clientID).
		Int("message_count", len(msgs)).
		Str("first_message", msgs[0].Content.(*conversation.ChatMessageContent).Text).
		Msg("Starting step with conversation")

	result, err := instance.Step.Start(ctx, msgs)
	if err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to start step")
		return fmt.Errorf("error starting step: %w", err)
	}

	s.logger.Info().Str("client_id", clientID).Msg("Started step")

	// Process results in background
	go func() {
		s.logger.Info().Str("client_id", clientID).Msg("Starting to process step results")
		resultCount := 0
		for result := range result.GetChannel() {
			resultCount++
			if result.Error() != nil {
				s.logger.Error().
					Err(result.Error()).
					Str("client_id", clientID).
					Int("result_count", resultCount).
					Msg("Error in step result")
				continue
			}
			s.logger.Debug().
				Str("client_id", clientID).
				Int("result_count", resultCount).
				Str("result", result.Unwrap()).
				Msg("Received step result")
		}
		s.logger.Info().
			Str("client_id", clientID).
			Int("total_results", resultCount).
			Msg("Step completed")
	}()

	return nil
}

// TemplateData holds data for rendering templates
type TemplateData struct {
	ClientID string
}

func main() {
	// Setup zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

	// Load templates
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to parse templates")
	}

	// Create event router with verbose logging
	router, err := events.NewEventRouter(events.WithVerbose(true))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create event router")
	}
	defer router.Close()

	// Start router in background
	go func() {
		if err := router.Run(context.Background()); err != nil {
			logger.Fatal().Err(err).Msg("Router failed")
		}
	}()

	// Create server
	server := NewServer(tmpl, router)

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
			client = NewSSEClient(clientID)
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
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
				flusher.Flush()
			case <-time.After(30 * time.Second):
				// Send heartbeat to keep connection alive
				fmt.Fprintf(w, "event: heartbeat\ndata: ping\n\n")
				flusher.Flush()
				server.logger.Debug().Str("client_id", client.ID).Msg("Sent heartbeat")
			}
		}
	})

	// Handle start endpoint - creates and starts a new step
	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		// Create new client ID
		clientID := uuid.New().String()
		server.logger.Info().Str("client_id", clientID).Msg("Starting new chat")

		// Create new client
		client := NewSSEClient(clientID)
		server.RegisterClient(client)

		if err := server.CreateStep(clientID); err != nil {
			server.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to create step")
			http.Error(w, fmt.Sprintf("Error creating step: %v", err), http.StatusInternalServerError)
			return
		}

		if err := server.StartStep(clientID); err != nil {
			server.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to start step")
			http.Error(w, fmt.Sprintf("Error starting step: %v", err), http.StatusInternalServerError)
			return
		}

		// Return the chat container template with the new client ID
		data := TemplateData{
			ClientID: clientID,
		}
		if err := tmpl.ExecuteTemplate(w, "chat-container", data); err != nil {
			server.logger.Error().Err(err).Msg("Failed to render chat container")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	logger.Info().Msg("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}
