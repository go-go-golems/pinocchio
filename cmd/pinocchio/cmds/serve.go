package cmds

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-go-golems/glazed/pkg/help"
	helpserver "github.com/go-go-golems/glazed/pkg/help/server"
	"github.com/go-go-golems/pinocchio/pkg/spa"
	"github.com/spf13/cobra"
)

// NewServeCommand creates a cobra command that starts the pinocchio help browser.
//
// The command serves the initialized HelpSystem from main(), rather than
// rebuilding a smaller documentation set, so repository-provided help pages
// loaded during command discovery are visible in the browser as well.
func NewServeCommand(hs *help.HelpSystem) *cobra.Command {
	var address string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve pinocchio help documentation as a web application",
		Long: `Start an HTTP server that serves pinocchio's help documentation
as a browsable web application with a React SPA frontend.

The server exposes:
  GET /api/*   — JSON API for section listing, search, and retrieval
  GET /*       — React SPA (browser UI)

Use --address to change the listen address (default :8088).

If the SPA assets are not available (binary built without -tags embed,
or 'make fetch-spa' not run), the server falls back to API-only mode.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd.Context(), address, hs)
		},
	}

	cmd.Flags().StringVar(&address, "address", ":8088", "Address to listen on")

	return cmd
}

func runServe(ctx context.Context, address string, hs *help.HelpSystem) error {
	if hs == nil {
		return fmt.Errorf("help system is not initialized")
	}

	// Assign a stable package name to embedded and repository docs that were
	// loaded without package metadata. The browser asks for package-filtered
	// sections using this package name.
	if err := hs.Store.SetDefaultPackage(ctx, "pinocchio", ""); err != nil {
		return fmt.Errorf("assigning default help package: %w", err)
	}

	// Try to create the SPA handler from embedded assets.
	// If assets are not available, fall back to API-only mode.
	spaHandler, err := spa.NewHandler()
	if err != nil {
		log.Warn().Err(err).Msg("SPA handler not available, serving API only")
		spaHandler = nil
	}

	// Create the combined handler (API + SPA).
	deps := helpserver.HandlerDeps{Store: hs.Store}
	handler := helpserver.NewServeHandler(deps, spaHandler)

	count, err := hs.Store.Count(ctx)
	if err != nil {
		return fmt.Errorf("counting help sections: %w", err)
	}
	log.Info().Int64("sections", count).Msg("Loaded help sections")

	// Start the server with graceful shutdown.
	httpSrv := &http.Server{
		Addr:         address,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Info().Str("address", address).Msg("Pinocchio help browser listening")

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigCh:
		log.Info().Str("signal", sig.String()).Msg("Shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}
