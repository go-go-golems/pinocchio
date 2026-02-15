package webchat

import (
	"context"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

func (r *Router) registerTimelineAPIHandlers(mux *http.ServeMux) {
	logger := log.With().Str("component", "webchat").Logger()
	handler := r.timelineSnapshotHandler(logger)

	mux.HandleFunc("/api/timeline", handler)
	mux.HandleFunc("/api/timeline/", handler)
}

func (r *Router) timelineSnapshotHandler(logger zerolog.Logger) func(http.ResponseWriter, *http.Request) {
	if r == nil || r.timelineStore == nil {
		return NewTimelineHTTPHandler(nil, logger)
	}
	return NewTimelineHTTPHandler(TimelineServiceFunc(func(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV1, error) {
		return r.timelineStore.GetSnapshot(ctx, convID, sinceVersion, limit)
	}), logger)
}
