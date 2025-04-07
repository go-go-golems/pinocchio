package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/steps/ai"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed_cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type WebUICommand struct {
	*glazed_cmds.CommandDescription
	stepSettings *settings.StepSettings
}

var _ glazed_cmds.BareCommand = (*WebUICommand)(nil)

func NewWebUICommand() (*WebUICommand, error) {
	stepSettings, err := settings.NewStepSettings()
	if err != nil {
		return nil, err
	}
	geppettoLayers, err := cmds.CreateGeppettoLayers(stepSettings)
	if err != nil {
		return nil, err
	}

	return &WebUICommand{
		CommandDescription: glazed_cmds.NewCommandDescription(
			"web-ui",
			glazed_cmds.WithShort("Web UI for chat interactions"),
			glazed_cmds.WithFlags(
				parameters.NewParameterDefinition(
					"port",
					parameters.ParameterTypeString,
					parameters.WithDefault("8080"),
					parameters.WithHelp("Port to listen on"),
				),
				parameters.NewParameterDefinition(
					"verbose",
					parameters.ParameterTypeBool,
					parameters.WithDefault(false),
					parameters.WithHelp("Enable verbose logging"),
				),
			),
			glazed_cmds.WithLayersList(geppettoLayers...),
		),
		stepSettings: stepSettings,
	}, nil
}

type WebUISettings struct {
	Port    string `glazed.parameter:"port"`
	Verbose bool   `glazed.parameter:"verbose"`
}

func (c *WebUICommand) Run(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
) error {
	settings := &WebUISettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, settings)
	if err != nil {
		return err
	}

	// Update step settings from parsed layers
	err = c.stepSettings.UpdateFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	c.stepSettings.Chat.Stream = true

	// Create step factory
	stepFactory := &ai.StandardStepFactory{
		Settings: c.stepSettings,
	}

	// Create server with step factory
	server := NewServer(stepFactory)
	defer func() {
		if err := server.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing server")
		}
	}()

	// Register handlers
	server.Register()

	// Create HTTP server
	httpServer := &http.Server{
		Addr:              ":" + settings.Port,
		Handler:           nil, // Use default mux
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info().Msgf("Server starting on http://localhost:%s", settings.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Wait for interrupt signal
	<-done
	log.Info().Msg("Server is shutting down...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown the server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited properly")
	return nil
}

func main() {
	// Create help system
	helpSystem := help.NewHelpSystem()

	webUICommand, err := NewWebUICommand()
	cobra.CheckErr(err)

	webUICobraCommand, err := cmds.BuildCobraCommandWithGeppettoMiddlewares(webUICommand, cli.WithProfileSettingsLayer())
	cobra.CheckErr(err)

	// Setup help system with root command
	helpSystem.SetupCobraRootCommand(webUICobraCommand)

	err = clay.InitViper("pinocchio", webUICobraCommand)
	cobra.CheckErr(err)

	err = webUICobraCommand.Execute()
	cobra.CheckErr(err)
}
