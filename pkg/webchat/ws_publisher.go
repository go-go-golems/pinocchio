package webchat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrConnectionPoolAbsent = errors.New("connection pool not available")
)

type WSPublisher interface {
	PublishJSON(ctx context.Context, convID string, envelope map[string]any) error
}

type conversationWSPublisher struct {
	cm *ConvManager
}

func NewWSPublisher(cm *ConvManager) WSPublisher {
	return &conversationWSPublisher{cm: cm}
}

func (p *conversationWSPublisher) PublishJSON(_ context.Context, convID string, envelope map[string]any) error {
	if p == nil || p.cm == nil {
		return ErrConversationNotFound
	}
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return ErrConversationNotFound
	}
	conv, ok := p.cm.GetConversation(convID)
	if !ok || conv == nil {
		return ErrConversationNotFound
	}
	if conv.pool == nil {
		return ErrConnectionPoolAbsent
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	conv.pool.Broadcast(b)
	return nil
}
