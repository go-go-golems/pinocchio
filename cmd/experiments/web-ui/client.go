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
	stepResult   steps.StepResult[string]
	logger       zerolog.Logger
	tmpl         *template.Template
}

// NewSSEClient creates a new SSE client with the given ID
func NewSSEClient(id string, tmpl *template.Template) *SSEClient {
	logger := zerolog.New(zerolog.NewConsoleWriter()).
		With().
		Timestamp().
		Caller().
		Str("client_id", id).
		Logger()

	return &SSEClient{
		ID:           id,
		MessageChan:  make(chan string, messageBufferSize),
		DisconnectCh: make(chan struct{}),
		logger:       logger,
		tmpl:         tmpl,
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

// CreateStep creates a new step for this client
func (c *SSEClient) CreateStep(router *events.EventRouter) error {
	// Cancel existing step if any
	if c.stepResult != nil {
		c.stepResult.Cancel()
		c.stepResult = nil
	}

	// Create new step
	step := chat.NewEchoStep()
	step.TimePerCharacter = 50 * time.Millisecond

	// Setup topic and event routing
	c.topic = fmt.Sprintf("chat-%s", c.ID)
	if err := step.AddPublishedTopic(router.Publisher, c.topic); err != nil {
		return fmt.Errorf("error setting up event publishing: %w", err)
	}

	// Add handler for this client's events
	c.logger.Info().Str("topic", c.topic).Msg("Adding handler")
	router.AddHandler(
		c.topic,
		c.topic,
		func(msg *message.Message) error {
			baseLogger := c.logger.With().Str("message_id", msg.UUID).Logger()
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
			html, err := EventToHTML(c.tmpl, e)
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
			if !c.TrySend(html) {
				baseLogger.Warn().
					Str("event_type", string(e.Type())).
					Int64("dropped", atomic.LoadInt64(&c.DroppedMsgs)).
					Msg("Dropped message for client")
			} else {
				baseLogger.Debug().
					Str("event_type", string(e.Type())).
					Msg("Sent message to client")
			}

			return nil
		},
	)

	c.step = step
	return nil
}

// StartStep starts the current step with the given conversation messages
func (c *SSEClient) StartStep(ctx context.Context, msgs []*conversation.Message) error {
	if c.step == nil {
		return fmt.Errorf("no step created for client %s", c.ID)
	}

	result, err := c.step.Start(ctx, msgs)
	if err != nil {
		return fmt.Errorf("error starting step: %w", err)
	}

	c.stepResult = result

	// Process results in background
	go func() {
		c.logger.Info().Msg("Starting to process step results")
		resultCount := 0
		for result := range result.GetChannel() {
			resultCount++
			if result.Error() != nil {
				c.logger.Error().
					Err(result.Error()).
					Int("result_count", resultCount).
					Msg("Error in step result")
				continue
			}
			c.logger.Debug().
				Int("result_count", resultCount).
				Str("result", result.Unwrap()).
				Msg("Received step result")
		}
		c.logger.Info().
			Int("total_results", resultCount).
			Msg("Step completed")
	}()

	return nil
}
