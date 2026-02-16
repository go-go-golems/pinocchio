package webchat

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

func (r *Router) registerTimelineAPIHandlers(mux *http.ServeMux) {
	logger := log.With().Str("component", "webchat").Logger()
	handler := r.timelineSnapshotHandler(logger)

	mux.HandleFunc("/api/timeline", handler)
	mux.HandleFunc("/api/timeline/", handler)
}

func (r *Router) timelineSnapshotHandler(logger zerolog.Logger) func(http.ResponseWriter, *http.Request) {
	if r == nil {
		return timelineSnapshotHTTPHandler(nil, logger)
	}
	if r.timelineService == nil && r.timelineStore != nil {
		r.timelineService = NewTimelineService(r.timelineStore)
	}
	if r.timelineService == nil {
		return timelineSnapshotHTTPHandler(nil, logger)
	}
	return timelineSnapshotHTTPHandler(r.timelineService, logger)
}

type timelineSnapshotReader interface {
	Snapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV1, error)
}

func timelineSnapshotHTTPHandler(svc timelineSnapshotReader, logger zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if svc == nil {
			http.Error(w, "timeline service not enabled", http.StatusNotFound)
			return
		}
		convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
		if convID == "" {
			http.Error(w, "missing conv_id", http.StatusBadRequest)
			return
		}

		var sinceVersion uint64
		if s := strings.TrimSpace(req.URL.Query().Get("since_version")); s != "" {
			_, _ = fmt.Sscanf(s, "%d", &sinceVersion)
		}
		limit := 0
		if s := strings.TrimSpace(req.URL.Query().Get("limit")); s != "" {
			var v int
			_, _ = fmt.Sscanf(s, "%d", &v)
			if v > 0 {
				limit = v
			}
		}

		snap, err := svc.Snapshot(req.Context(), convID, sinceVersion, limit)
		if err != nil {
			logger.Error().Err(err).Str("conv_id", convID).Msg("timeline snapshot failed")
			http.Error(w, "timeline snapshot failed", http.StatusInternalServerError)
			return
		}
		out, err := protojson.MarshalOptions{
			EmitUnpopulated: false,
			UseProtoNames:   false,
		}.Marshal(snap)
		if err != nil {
			logger.Error().Err(err).Str("conv_id", convID).Msg("timeline marshal failed")
			http.Error(w, "timeline marshal failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// #nosec G705 -- payload is protobuf-generated JSON served as application/json.
		if _, err := w.Write(out); err != nil {
			logger.Warn().Err(err).Str("conv_id", convID).Msg("timeline write failed")
		}
	}
}
