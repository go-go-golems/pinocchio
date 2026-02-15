package webchat

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

// Server drives the event router and HTTP server lifecycle for app-composed handlers.
// It does not add /chat or /ws routes; applications mount those routes themselves.
type Server struct {
	baseCtx context.Context
	router  *Router
	httpSrv *http.Server
}

// NewServer builds a Router and http.Server pair for app-composed webchat services.
// The returned server runs event routing plus whichever HTTP handlers the caller mounted.
func NewServer(ctx context.Context, parsed *values.Values, staticFS fs.FS, opts ...RouterOption) (*Server, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	r, err := NewRouter(ctx, parsed, staticFS, opts...)
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

func (s *Server) RegisterMiddleware(name string, f MiddlewareFactory) {
	if s == nil || s.router == nil {
		return
	}
	s.router.RegisterMiddleware(name, f)
}

func (s *Server) RegisterTool(name string, f ToolFactory) {
	if s == nil || s.router == nil {
		return
	}
	s.router.RegisterTool(name, f)
}

func (s *Server) ChatService() *ChatService {
	if s == nil || s.router == nil {
		return nil
	}
	return s.router.ChatService()
}

func (s *Server) StreamHub() *StreamHub {
	if s == nil || s.router == nil {
		return nil
	}
	return s.router.StreamHub()
}

func (s *Server) APIHandler() http.Handler {
	if s == nil || s.router == nil {
		return http.NotFoundHandler()
	}
	return s.router.APIHandler()
}

func (s *Server) UIHandler() http.Handler {
	if s == nil || s.router == nil {
		return http.NotFoundHandler()
	}
	return s.router.UIHandler()
}

func (s *Server) HTTPServer() *http.Server {
	if s == nil {
		return nil
	}
	return s.httpSrv
}

// NewFromRouter constructs a server from an existing Router and http.Server.
func NewFromRouter(ctx context.Context, r *Router, httpSrv *http.Server) *Server {
	if ctx == nil {
		panic("webchat: NewFromRouter requires non-nil ctx")
	}
	if r == nil {
		panic("webchat: NewFromRouter requires non-nil router")
	}
	if httpSrv == nil {
		panic("webchat: NewFromRouter requires non-nil http server")
	}
	return &Server{baseCtx: ctx, router: r, httpSrv: httpSrv}
}

func (s *Server) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}
	if s == nil || s.router == nil || s.httpSrv == nil {
		return errors.New("server is not initialized")
	}
	eg := errgroup.Group{}
	srvCtx, srvCancel := context.WithCancel(ctx)
	defer srvCancel()

	if s.router != nil && s.router.cm != nil {
		s.router.cm.StartEvictionLoop(srvCtx)
	}

	eg.Go(func() error { return s.router.router.Run(srvCtx) })

	eg.Go(func() error {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Info().Msg("received interrupt signal, shutting down gracefully...")
		srvCancel()
		shutdownBase := context.WithoutCancel(ctx)
		shutdownCtx, cancel := context.WithTimeout(shutdownBase, 30*time.Second)
		defer cancel()
		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("server shutdown error")
			return err
		}
		if s.router != nil && s.router.timelineStore != nil {
			if err := s.router.timelineStore.Close(); err != nil {
				log.Error().Err(err).Msg("timeline store close error")
			}
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
