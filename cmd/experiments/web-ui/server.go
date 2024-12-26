package main

import (
	"context"
	"fmt"
	"html/template"
	"sync/atomic"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/rs/zerolog"
)

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
	step.TimePerCharacter = 50 * time.Millisecond

	// Setup topic and event routing
	topic := fmt.Sprintf("chat-%s", clientID)
	if err := step.AddPublishedTopic(s.router.Publisher, topic); err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to setup event publishing")
		return fmt.Errorf("error setting up event publishing: %w", err)
	}

	// Add handler for this client's events
	s.logger.Info().Str("client_id", clientID).Str("topic", topic).Msg("Adding handler")
	s.router.AddHandler(
		topic,
		topic,
		func(msg *message.Message) error {
			baseLogger := s.logger.With().Str("client_id", clientID).Str("message_id", msg.UUID).Logger()
			baseLogger.Debug().
				Str("metadata", fmt.Sprintf("%v", msg.Metadata)).
				Msg("Received message from router")

			// Parse event
			e, err := chat.NewEventFromJson(msg.Payload)
			if err != nil {
				baseLogger.Error().Err(err).
					Str("payload", string(msg.Payload)).
					Msg("Failed to parse event")
				return err
			}

			baseLogger.Debug().
				Str("event_type", string(e.Type())).
				Msg("Parsed event")

			// Convert to HTML
			html, err := EventToHTML(s.tmpl, e)
			if err != nil {
				baseLogger.Error().Err(err).
					Str("event_type", string(e.Type())).
					Msg("Failed to convert event to HTML")
				return err
			}

			baseLogger.Debug().
				Str("event_type", string(e.Type())).
				Int("html_length", len(html)).
				Msg("Converted event to HTML")

			// Send to client's message channel
			s.clientsMux.RLock()
			client, ok := s.clients[clientID]
			s.clientsMux.RUnlock()
			if !ok {
				baseLogger.Warn().
					Str("event_type", string(e.Type())).
					Msg("Client not found for event")
				return nil
			}

			// Try to send without blocking
			if !client.TrySend(html) {
				atomic.AddInt64(&s.metrics.TotalDroppedMsgs, 1)
				baseLogger.Warn().
					Str("event_type", string(e.Type())).
					Int64("total_dropped", atomic.LoadInt64(&s.metrics.TotalDroppedMsgs)).
					Int64("client_dropped", atomic.LoadInt64(&client.DroppedMsgs)).
					Msg("Dropped message for client")
			} else {
				baseLogger.Debug().
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
