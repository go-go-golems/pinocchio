package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	web_conversation "github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/conversation"
	"github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/templates/components"
	"github.com/rs/zerolog"
)

// ChatEventHandler defines an interface for handling different chat events.
type ChatEventHandler interface {
	HandlePartialCompletion(ctx context.Context, e *events.EventPartialCompletion) error
	HandleText(ctx context.Context, e *events.EventText) error // Assuming we might want separate handling
	HandleFinal(ctx context.Context, e *events.EventFinal) error
	HandleError(ctx context.Context, e *events.EventError) error
	HandleInterrupt(ctx context.Context, e *events.EventInterrupt) error
	// Add other event types as needed, e.g., HandleMetadata, HandleToolCall, etc.
}

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

// ChatEventHandler Interface Check - DO NOT REMOVE
var _ ChatEventHandler = (*ChatClient)(nil)

type ChatClientOption func(*ChatClient) error

func WithStep(step chat.Step) ChatClientOption {
	return func(c *ChatClient) error {
		c.step = step
		return nil
	}
}

// NewChatClient creates a new SSE client with its own event router
func NewChatClient(id string, options ...ChatClientOption) (*ChatClient, error) {
	logger := zerolog.New(zerolog.NewConsoleWriter()).
		With().
		Timestamp().
		Caller().
		Str("client_id", id).
		Logger()

	// Create a new router for this client
	router, err := events.NewEventRouter(events.WithVerbose(true))
	if err != nil {
		return nil, fmt.Errorf("failed to create event router: %w", err)
	}

	client := &ChatClient{
		ID:           id,
		MessageChan:  make(chan string, 100),
		DisconnectCh: make(chan struct{}),
		router:       router,
		manager:      conversation.NewManager(),
		logger:       logger,
	}

	// Apply options
	for _, opt := range options {
		if err := opt(client); err != nil {
			client.logger.Error().Err(err).Msg("Failed to apply client option")
			return nil, fmt.Errorf("failed to apply client option: %w", err)
		}
	}

	// Verify that a step was provided
	if client.step == nil {
		return nil, fmt.Errorf("no step provided to chat client")
	}

	return client, nil
}

// Start registers the chat handler for the client and starts the event router handlers
func (c *ChatClient) Start(ctx context.Context) error {
	logger := c.logger.With().Str("client_id", c.ID).Logger()

	// Register the client as the handler for its own chat events
	logger.Debug().Msg("Registering chat handler")
	err := c.router.RegisterChatEventHandler(ctx, c.step, c.ID, c)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to register chat handler")
		return fmt.Errorf("failed to register chat handler: %w", err)
	}

	// Start the router in a goroutine
	go func() {
		c.logger.Info().Msg("Starting router for client")
		if err := c.router.Run(ctx); err != nil {
			logger.Error().Err(err).Msg("Router failed")
		}
		logger.Info().Msg("Router closed")
	}()

	c.logger.Info().Msg("ChatClient started and handlers running")
	return nil
}

// Close properly cleans up the client and its router
func (c *ChatClient) Close() error {
	if err := c.router.Close(); err != nil {
		c.logger.Error().Err(err).Msg("Failed to close router")
		return err
	}
	close(c.MessageChan)
	close(c.DisconnectCh)
	return nil
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

// --- ChatEventHandler Implementation ---

func (c *ChatClient) HandlePartialCompletion(ctx context.Context, e *events.EventPartialCompletion) error {
	var buf strings.Builder
	now := time.Now()
	err := components.AssistantMessage(now, e.Completion).Render(ctx, &buf)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to render partial completion event")
		return fmt.Errorf("failed to render partial completion event: %w", err)
	}
	c.sendHTMLToClient(buf.String(), e.Type())
	return nil
}

func (c *ChatClient) HandleText(ctx context.Context, e *events.EventText) error {
	// NOTE: Currently, EventFinal seems to be used for the final text.
	// Decide if EventText needs separate handling or can be merged with EventFinal.
	// For now, rendering it similarly to a final message fragment.
	var buf strings.Builder
	now := time.Now()
	err := components.EventFinal(now, e.Text).Render(ctx, &buf) // Using EventFinal component for now
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to render text event")
		return fmt.Errorf("failed to render text event: %w", err)
	}
	c.sendHTMLToClient(buf.String(), e.Type())
	return nil
}

func (c *ChatClient) HandleFinal(ctx context.Context, e *events.EventFinal) error {
	// XXX not the best place - This logic might be better suited elsewhere,
	// perhaps after the step finishes entirely, but placing it here ensures
	// the conversation history is updated before the UI refresh.
	c.mu.Lock()
	c.manager.AppendMessages(conversation.NewChatMessage(conversation.RoleAssistant, e.Text))
	conv := c.manager.GetConversation() // Get updated conversation
	c.mu.Unlock()                       // Unlock before rendering to avoid holding lock during potentially slow operations

	var buf strings.Builder
	webConv, err := web_conversation.ConvertConversation(conv)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to convert conversation to web format")
		return fmt.Errorf("failed to convert conversation to web format: %w", err)
	}

	// Render the entire conversation history
	err = components.ConversationHistory(webConv, true).Render(ctx, &buf)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to render conversation history")
		return fmt.Errorf("failed to render conversation history: %w", err)
	}

	c.sendHTMLToClient(buf.String(), e.Type())
	return nil
}

func (c *ChatClient) HandleError(ctx context.Context, e *events.EventError) error {
	var buf strings.Builder
	now := time.Now()
	errStr := ""
	if err := e.Error(); err != nil {
		errStr = err.Error()
	}
	err := components.EventError(now, errStr).Render(ctx, &buf)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to render error event")
		return fmt.Errorf("failed to render error event: %w", err)
	}
	c.sendHTMLToClient(buf.String(), e.Type())
	return nil
}

func (c *ChatClient) HandleInterrupt(ctx context.Context, e *events.EventInterrupt) error {
	var buf strings.Builder
	now := time.Now()
	// Rendering as an error for now, consider a specific interrupt component later
	err := components.EventError(now, e.Text).Render(ctx, &buf)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to render interrupt event")
		return fmt.Errorf("failed to render interrupt event: %w", err)
	}
	c.sendHTMLToClient(buf.String(), e.Type())
	return nil
}

// sendHTMLToClient tries to send rendered HTML to the client's channel without blocking.
func (c *ChatClient) sendHTMLToClient(html string, eventType events.EventType) {
	select {
	case c.MessageChan <- html:
		c.logger.Debug().
			Str("event_type", string(eventType)).
			Int("html_length", len(html)).
			Msg("Sent HTML message to client")
	default:
		c.logger.Warn().
			Str("event_type", string(eventType)).
			Msg("Failed to send HTML message to client (channel full)")
	}
}
