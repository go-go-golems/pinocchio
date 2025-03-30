package client

import (
	"context"
	"fmt"
	steps2 "github.com/go-go-golems/geppetto/pkg/steps/ai/chat/steps"
	"strings"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
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

// इंप्लीमेंट्स ChatEventHandler Interface Check - DO NOT REMOVE
var _ ChatEventHandler = (*ChatClient)(nil)

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
	defaultStep := steps2.NewEchoStep()
	options = append([]ChatClientOption{WithStep(defaultStep)}, options...)

	// Apply options
	for _, opt := range options {
		if err := opt(client); err != nil {
			client.logger.Error().Err(err).Msg("Failed to apply client option")
		}
	}

	// NOTE: Registration and running handlers is now done in the Start method.

	return client
}

// Start registers the chat handler for the client and starts the event router handlers.
// This should be called after the client is created.
func (c *ChatClient) Start(ctx context.Context) error {
	// Register the client as the handler for its own chat events
	err := c.registerChatHandler(ctx)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to register chat handler")
		return fmt.Errorf("failed to register chat handler: %w", err)
	}

	// Start the router handlers after registering ours, RunHandlers is idempotent and can be called multiple times
	err = c.router.RunHandlers(ctx)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to run event router handlers")
		return fmt.Errorf("failed to run event router handlers: %w", err)
	}

	c.logger.Info().Msg("ChatClient started and handlers running")
	return nil
}

// createChatDispatchHandler creates a Watermill handler function that parses chat events
// and dispatches them to the appropriate method of the provided ChatEventHandler.
func createChatDispatchHandler(handler ChatEventHandler, logger zerolog.Logger) message.NoPublishHandlerFunc {
	return func(msg *message.Message) error {
		msgLogger := logger.With().Str("message_id", msg.UUID).Logger()
		msgLogger.Debug().
			Str("metadata", fmt.Sprintf("%v", msg.Metadata)).
			Msg("Received message for chat handler")

		// Parse the generic chat event
		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			msgLogger.Error().Err(err).
				Str("payload", string(msg.Payload)).
				Msg("Failed to parse chat event from message payload")
			// Don't kill the handler for one bad message, just log and continue
			return nil // Or return err depending on desired resilience
		}

		msgLogger.Debug().
			Str("event_type", string(e.Type())).
			Msg("Parsed chat event")

		// Dispatch to the specific handler method based on event type
		// Pass the message context down to the handler
		msgCtx := msg.Context()
		var handlerErr error
		switch ev := e.(type) {
		case *events.EventPartialCompletion:
			handlerErr = handler.HandlePartialCompletion(msgCtx, ev)
		case *events.EventText:
			handlerErr = handler.HandleText(msgCtx, ev)
		case *events.EventFinal:
			handlerErr = handler.HandleFinal(msgCtx, ev)
		case *events.EventError:
			handlerErr = handler.HandleError(msgCtx, ev)
		case *events.EventInterrupt:
			handlerErr = handler.HandleInterrupt(msgCtx, ev)
		default:
			msgLogger.Warn().Str("event_type", string(e.Type())).Msg("Unhandled chat event type")
			// Decide if unknown types should be an error or ignored
		}

		if handlerErr != nil {
			msgLogger.Error().Err(handlerErr).
				Str("event_type", string(e.Type())).
				Msg("Error processing chat event")
			// Return the error to potentially signal Watermill handler failure
			return handlerErr
		}

		return nil
	}
}

// registerChatHandler sets up event publishing for the client's step and registers
// the client itself as the handler for events on its topic.
func (c *ChatClient) registerChatHandler(ctx context.Context) error {
	topic := fmt.Sprintf("chat-%s", c.ID)
	// Use client's logger instead of creating a new one or relying on context
	logger := c.logger

	logger.Info().Str("topic", topic).Msg("Setting up chat handler")

	// Configure step to publish events to this client's topic
	if err := c.step.AddPublishedTopic(c.router.Publisher, topic); err != nil {
		logger.Error().Err(err).Msg("Failed to add published topic to step")
		return fmt.Errorf("failed to setup event publishing for step: %w", err)
	}

	// Create the dispatch handler using the reusable function
	dispatchHandler := createChatDispatchHandler(c, logger)

	// Add the created handler to the router
	c.router.AddHandler(
		topic, // Handler name (using topic for uniqueness per client)
		topic, // Topic to subscribe to
		dispatchHandler,
	)

	logger.Info().Str("topic", topic).Msg("Chat handler added successfully")
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
