package main

import (
	"context"
	"html/template"
	"sync"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/rs/zerolog"
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

// Server is the main server struct that handles all web UI functionality
type Server struct {
	tmpl       *template.Template
	router     *events.EventRouter
	clients    map[string]*SSEClient
	steps      map[string]*StepInstance
	clientsMux sync.RWMutex
	stepsMux   sync.RWMutex
	logger     zerolog.Logger
	metrics    struct {
		TotalDroppedMsgs int64
	}
}
