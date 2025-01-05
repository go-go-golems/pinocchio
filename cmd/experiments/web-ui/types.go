package main

import (
	webconv "github.com/go-go-golems/pinocchio/cmd/experiments/web-ui/conversation"
)

// TemplateData represents the data passed to the main template
type TemplateData struct {
	ClientID string
	Messages *webconv.WebConversation
}

// MessageData represents the data passed to the message template
type MessageData struct {
	Message *webconv.WebMessage
}
