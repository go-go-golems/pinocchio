package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/templates/components"
	"github.com/rs/zerolog"
)

// ChatClient represents a connected SSE client
type ChatClient struct {
	ID           string
	MessageChan  chan string
	DisconnectCh chan struct{}
	router       *events.EventRouter
	manager      conversation.Manager
	step         chat.Step
	stepResult   steps.StepResult[*conversation.Message]
	mu           sync.RWMutex
	logger       zerolog.Logger
}

type ChatClientOption func(*ChatClient) error

func WithStep(step chat.Step) ChatClientOption {
	return func(c *ChatClient) error {
		c.step = step
		return nil
	}
}

// NewChatClient creates a new SSE client
func NewChatClient(id string, router *events.EventRouter, options ...ChatClientOption) *ChatClient {
	logger := zerolog.New(zerolog.NewConsoleWriter()).
		With().
		Timestamp().
		Caller().
		Str("client_id", id).
		Logger()

	client := &ChatClient{
		ID:           id,
		MessageChan:  make(chan string, 100),
		DisconnectCh: make(chan struct{}),
		router:       router,
		manager:      conversation.NewManager(),
		logger:       logger,
	}

	// Set default step if none provided
	defaultStep := chat.NewEchoStep()
	options = append([]ChatClientOption{WithStep(defaultStep)}, options...)

	// Apply options
	for _, opt := range options {
		if err := opt(client); err != nil {
			client.logger.Error().Err(err).Msg("Failed to apply client option")
		}
	}

	// Setup topic and event routing
	topic := fmt.Sprintf("chat-%s", id)
	if err := client.step.AddPublishedTopic(router.Publisher, topic); err != nil {
		client.logger.Error().Err(err).Msg("Failed to setup event publishing")
		return client
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
			html, err := EventToHTML(e)
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
			select {
			case client.MessageChan <- html:
				baseLogger.Debug().
					Str("event_type", string(e.Type())).
					Msg("Sent message to client")
			default:
				baseLogger.Warn().
					Str("event_type", string(e.Type())).
					Msg("Failed to send message to client (channel full)")
			}

			return nil
		},
	)
	err := router.RunHandlers(context.Background())
	if err != nil {
		client.logger.Error().Err(err).Msg("Failed to run event router")
	}

	return client
}

// SendUserMessage sends a user message to the conversation and starts a chat step
func (c *ChatClient) SendUserMessage(ctx context.Context, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add user message to conversation
	userMsg := conversation.NewChatMessage(conversation.RoleUser, message)
	c.manager.AppendMessages(userMsg)

	go func() {
		c.MessageChan <- "<div class=\"event\"><div class=\"timestamp\">" + time.Now().Format("15:04:05") + "</div>Starting...</div>"
	}()

	// Cancel existing step result if any
	if c.stepResult != nil {
		c.stepResult.Cancel()
		c.stepResult = nil
	}

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
				msg := result.Unwrap()
				c.manager.AppendMessages(msg)
				c.logger.Debug().
					Int("result_count", resultCount).
					Msg("Received step result")
			}
		}
	}()

	return nil
}

// GetConversation returns the current conversation
func (c *ChatClient) GetConversation() []*conversation.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.manager.GetConversation()
}

// EventToHTML converts a chat event to HTML
func EventToHTML(e chat.Event) (string, error) {
	var buf strings.Builder
	now := time.Now()

	switch e_ := e.(type) {
	case *chat.EventPartialCompletion:
		// Partial completions are sent directly as text
		return e_.Delta, nil
	case *chat.EventText:
		err := components.EventFinal(now, e_.Text).Render(context.Background(), &buf)
		if err != nil {
			return "", fmt.Errorf("failed to render text event: %w", err)
		}
	case *chat.EventFinal:
		err := components.EventFinal(now, e_.Text).Render(context.Background(), &buf)
		if err != nil {
			return "", fmt.Errorf("failed to render final event: %w", err)
		}
	case *chat.EventError:
		errStr := ""
		if err := e_.Error(); err != nil {
			errStr = err.Error()
		}
		err := components.EventError(now, errStr).Render(context.Background(), &buf)
		if err != nil {
			return "", fmt.Errorf("failed to render error event: %w", err)
		}
	default:
		return "", fmt.Errorf("unknown event type: %T", e)
	}

	return buf.String(), nil
}
