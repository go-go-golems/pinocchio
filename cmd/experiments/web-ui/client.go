package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
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
}

// NewSSEClient creates a new SSE client with the given ID
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

	c.step = step
	return nil
}

// StartStep starts the current step
func (c *SSEClient) StartStep(ctx context.Context) error {
	if c.step == nil {
		return fmt.Errorf("no step created for client %s", c.ID)
	}

	// Create a simple conversation
	msgs := []*conversation.Message{
		conversation.NewChatMessage(conversation.RoleSystem, "You are a helpful assistant."),
		conversation.NewChatMessage(conversation.RoleUser, "Hello! Please tell me a short story about a robot."),
	}

	result, err := c.step.Start(ctx, msgs)
	if err != nil {
		return fmt.Errorf("error starting step: %w", err)
	}

	c.stepResult = result

	// Process results in background
	go func() {
		resultCount := 0
		for result := range result.GetChannel() {
			resultCount++
			if result.Error() != nil {
				continue
			}
		}
	}()

	return nil
}
