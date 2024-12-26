package main

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
)

// TemplateData holds data for rendering templates
type TemplateData struct {
	ClientID string
}

// StepInstance represents a running chat step
type StepInstance struct {
	Step   *chat.EchoStep
	Topic  string
	Cancel context.CancelFunc
}
