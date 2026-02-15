package webchat

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (r *Router) registerTimelineAPIHandlers(mux *http.ServeMux) {
	logger := log.With().Str("component", "webchat").Logger()
	handler := r.timelineSnapshotHandler(logger)

	mux.HandleFunc("/api/timeline", handler)
	mux.HandleFunc("/api/timeline/", handler)
}

func (r *Router) timelineSnapshotHandler(logger zerolog.Logger) func(http.ResponseWriter, *http.Request) {
	if r == nil {
		return NewTimelineHTTPHandler(nil, logger)
	}
	if r.timelineService == nil && r.timelineStore != nil {
		r.timelineService = NewTimelineService(r.timelineStore)
	}
	if r.timelineService == nil {
		return NewTimelineHTTPHandler(nil, logger)
	}
	return NewTimelineHTTPHandler(r.timelineService, logger)
}
