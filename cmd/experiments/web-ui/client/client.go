package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/templates/components"
)

// ChatClient represents a connected SSE client
type ChatClient struct {
	ID           string
	MessageChan  chan string
	DisconnectCh chan struct{}
	router       *events.EventRouter
	manager      conversation.Manager
	mu           sync.RWMutex
}

// NewChatClient creates a new SSE client
func NewChatClient(id string, router *events.EventRouter) *ChatClient {
	return &ChatClient{
		ID:           id,
		MessageChan:  make(chan string),
		DisconnectCh: make(chan struct{}),
		router:       router,
		manager:      conversation.NewManager(),
	}
}

// SendUserMessage sends a user message to the conversation
func (c *ChatClient) SendUserMessage(ctx context.Context, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add user message to conversation
	userMsg := conversation.NewChatMessage(conversation.RoleUser, message)
	c.manager.AppendMessages(userMsg)

	// Create event for user message
	event := components.EventData{
		Message: message,
	}

	// Send event to client
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	c.MessageChan <- string(eventJSON)

	return nil
}

// GetConversation returns the current conversation
func (c *ChatClient) GetConversation() []*conversation.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.manager.GetConversation()
}
