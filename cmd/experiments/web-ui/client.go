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
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/rs/zerolog"
)

const (
	// Buffer sizes
	messageBufferSize = 100  // Number of messages to buffer per client
	clientBufferSize  = 1000 // Maximum number of clients
)

// SSEClient represents a Server-Sent Events client connection
type SSEClient struct {
	ID           string
	MessageChan  chan string
	DisconnectCh chan struct{}
	DroppedMsgs  int64 // Counter for monitoring purposes
	step         chat.Step
	topic        string
	stepResult   steps.StepResult[*conversation.Message]
	logger       zerolog.Logger
	tmpl         *template.Template
	router       *events.EventRouter
	manager      conversation.Manager
}

// NewSSEClient creates a new SSE client with the given ID
func NewSSEClient(id string, tmpl *template.Template, router *events.EventRouter) *SSEClient {
	logger := zerolog.New(zerolog.NewConsoleWriter()).
		With().
		Timestamp().
		Caller().
		Str("client_id", id).
		Logger()

	topic := fmt.Sprintf("chat-%s", id)

	client := &SSEClient{
		ID:           id,
		MessageChan:  make(chan string, messageBufferSize),
		DisconnectCh: make(chan struct{}),
		logger:       logger,
		tmpl:         tmpl,
		router:       router,
		topic:        topic,
		manager:      conversation.NewManager(),
	}

	// Add handler for this client's events
	client.logger.Info().Str("topic", topic).Msg("Adding handler")
	router.AddHandler(
		topic,
		topic,
		func(msg *message.Message) error {
			baseLogger := client.logger.With().Str("message_id", msg.UUID).Logger()
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
			html, err := EventToHTML(client.tmpl, e)
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

			// Try to send without blocking
			if !client.TrySend(html) {
				baseLogger.Warn().
					Str("event_type", string(e.Type())).
					Int64("dropped", atomic.LoadInt64(&client.DroppedMsgs)).
					Msg("Dropped message for client")
			} else {
				baseLogger.Debug().
					Str("event_type", string(e.Type())).
					Msg("Sent message to client")
			}

			return nil
		},
	)

	return client
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

// CreateStep creates a new step for this client
func (c *SSEClient) CreateStep() error {
	// Cancel existing step if any
	if c.stepResult != nil {
		c.stepResult.Cancel()
		c.stepResult = nil
	}

	// Create new step
	step := chat.NewEchoStep()
	step.TimePerCharacter = 50 * time.Millisecond

	// Setup topic and event routing
	if err := step.AddPublishedTopic(c.router.Publisher, c.topic); err != nil {
		return fmt.Errorf("error setting up event publishing: %w", err)
	}

	err := c.router.RunHandlers(context.Background())
	if err != nil {
		return fmt.Errorf("error running handlers: %w", err)
	}

	c.step = step
	return nil
}

// SendUserMessage processes a user message and starts a chat step
func (c *SSEClient) SendUserMessage(ctx context.Context, message string) error {
	// Create new step if needed
	if c.step == nil {
		if err := c.CreateStep(); err != nil {
			return fmt.Errorf("error creating step: %w", err)
		}
	}

	// Add user message to conversation
	userMsg := conversation.NewChatMessage(conversation.RoleUser, message)
	c.manager.AppendMessages(userMsg)

	// Get the full conversation history
	conv := c.manager.GetConversation()

	// Start step with full conversation history
	result, err := c.step.Start(ctx, conv)
	if err != nil {
		return fmt.Errorf("error starting step: %w", err)
	}

	c.stepResult = result

	// Process results in background
	go func() {
		c.logger.Info().Msg("Starting to process step results")
		resultCount := 0
		for {
			select {
			case <-ctx.Done():
				c.logger.Info().Msg("Context cancelled, stopping step result processing")
				return
			case result, ok := <-result.GetChannel():
				if !ok {
					c.logger.Info().
						Int("total_results", resultCount).
						Msg("Step completed")
					return
				}
				resultCount++
				if result.Error() != nil {
					c.logger.Error().
						Err(result.Error()).
						Int("result_count", resultCount).
						Msg("Error in step result")
					continue
				}
				// Add assistant's response to conversation
				c.manager.AppendMessages(result.Unwrap())
				c.logger.Debug().
					Int("result_count", resultCount).
					Msg("Received step result")
			}
		}
	}()

	return nil
}

// GetConversation returns the current conversation
func (c *SSEClient) GetConversation() conversation.Conversation {
	return c.manager.GetConversation()
}
