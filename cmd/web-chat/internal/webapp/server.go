package webapp

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	zlog "github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

func RunHTTPServer(ctx context.Context, srv *http.Server, closeFn func() error) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}
	if srv == nil {
		return errors.New("http server is not initialized")
	}
	srvCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	eg, egCtx := errgroup.WithContext(srvCtx)
	eg.Go(func() error {
		<-egCtx.Done()
		shutdownBase := context.WithoutCancel(ctx)
		shutdownCtx, cancel := context.WithTimeout(shutdownBase, 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		if closeFn != nil {
			return closeFn()
		}
		return nil
	})
	eg.Go(func() error {
		zlog.Info().Str("addr", srv.Addr).Msg("starting web-chat server")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	return eg.Wait()
}
