package main

import (
	"context"
	"embed"
	"html/template"
	"net/http"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

//go:embed templates/*
var templatesFS embed.FS

func main() {
	// Setup zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := zerolog.New(zerolog.NewConsoleWriter()).
		With().
		Timestamp().
		Caller().
		Logger()
	logger.Level(zerolog.TraceLevel)

	log.Logger = logger

	// Load templates
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to parse templates")
	}

	// Create event router with verbose logging
	router, err := events.NewEventRouter(events.WithVerbose(true))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create event router")
	}
	defer router.Close()

	// Start router in background
	go func() {
		logger.Info().Msg("Starting router")
		if err := router.Run(context.Background()); err != nil {
			logger.Fatal().Err(err).Msg("Router failed")
		}
		defer func() {
			router.Close()
			logger.Info().Msg("Router stopped")
		}()
	}()

	// Create server
	server := NewServer(tmpl, router)

	// Register handlers
	RegisterHandlers(server, tmpl)

	logger.Info().Msg("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}
