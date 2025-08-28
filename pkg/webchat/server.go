package webchat

import (
	"context"
	"embed"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/go-go-golems/glazed/pkg/cmds/layers"
)

// Server drives the event router and HTTP server lifecycle.
type Server struct {
	baseCtx context.Context
	router  *Router
	httpSrv *http.Server
}

func NewServer(ctx context.Context, parsed *layers.ParsedLayers, staticFS embed.FS) (*Server, error) {
	r, err := NewRouter(ctx, parsed, staticFS)
	if err != nil {
		return nil, err
	}
	httpSrv, err := r.BuildHTTPServer()
	if err != nil {
		return nil, errors.Wrap(err, "build http server")
	}
	return &Server{baseCtx: ctx, router: r, httpSrv: httpSrv}, nil
}

func (s *Server) Router() *Router { return s.router }

// NewFromRouter constructs a server from an existing Router and http.Server.
func NewFromRouter(ctx context.Context, r *Router, httpSrv *http.Server) *Server {
	return &Server{baseCtx: ctx, router: r, httpSrv: httpSrv}
}

func (s *Server) Run(ctx context.Context) error {
	eg := errgroup.Group{}
	srvCtx, srvCancel := context.WithCancel(ctx)
	defer srvCancel()

	eg.Go(func() error { return s.router.router.Run(srvCtx) })

	eg.Go(func() error {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Info().Msg("received interrupt signal, shutting down gracefully...")
		srvCancel()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("server shutdown error")
			return err
		}
		if err := s.router.router.Close(); err != nil {
			log.Error().Err(err).Msg("router close error")
		} else {
			log.Info().Msg("router closed")
		}
		log.Info().Msg("server shutdown complete")
		return nil
	})

	eg.Go(func() error {
		log.Info().Str("addr", s.httpSrv.Addr).Msg("starting web-chat server")
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("server listen error")
			return err
		}
		return nil
	})

	return eg.Wait()
}
