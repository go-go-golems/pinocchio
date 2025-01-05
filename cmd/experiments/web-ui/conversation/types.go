package conversation

import (
	"encoding/json"
	"time"
)

// WebMessage represents a simplified message structure for web rendering
type WebMessage struct {
	ID         string                 `json:"id"`
	ParentID   string                 `json:"parentId"`
	Time       time.Time              `json:"time"`
	LastUpdate time.Time              `json:"lastUpdate"`
	Type       string                 `json:"type"`
	Content    WebMessageContent      `json:"content"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// WebMessageContent is the base interface for different types of web message content
type WebMessageContent interface {
	Type() string
	ToJSON() ([]byte, error)
}

// WebChatMessage represents a chat message for web rendering
type WebChatMessage struct {
	Role   string   `json:"role"`
	Text   string   `json:"text"`
	Images []string `json:"images,omitempty"` // URLs only for web rendering
}

func (w *WebChatMessage) Type() string {
	return "chat"
}

func (w *WebChatMessage) ToJSON() ([]byte, error) {
	return json.Marshal(w)
}

// WebToolUseMessage represents a tool use message for web rendering
type WebToolUseMessage struct {
	ToolID string          `json:"toolId"`
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
}

func (w *WebToolUseMessage) Type() string {
	return "tool-use"
}

func (w *WebToolUseMessage) ToJSON() ([]byte, error) {
	return json.Marshal(w)
}

// WebToolResultMessage represents a tool result message for web rendering
type WebToolResultMessage struct {
	ToolID string `json:"toolId"`
	Result string `json:"result"`
}

func (w *WebToolResultMessage) Type() string {
	return "tool-result"
}

func (w *WebToolResultMessage) ToJSON() ([]byte, error) {
	return json.Marshal(w)
}

// WebConversation represents a conversation for web rendering
type WebConversation struct {
	Messages []*WebMessage `json:"messages"`
}
