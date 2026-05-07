package app

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	chatexport "github.com/go-go-golems/pinocchio/pkg/chatapp/export"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

type fullExportResponse struct {
	SessionID string                     `json:"session_id" yaml:"session_id"`
	Timeline  *chatexport.TimelineExport `json:"timeline,omitempty" yaml:"timeline,omitempty"`
	Turns     *chatexport.TurnsExport    `json:"turns,omitempty" yaml:"turns,omitempty"`
}

func (s *Server) handleTimelineExport(w http.ResponseWriter, r *http.Request, sid sessionstream.SessionId) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	opts, err := parseExportOptions(r, chatexport.TimelineViewEntities)
	if err != nil {
		writeExportError(w, err)
		return
	}
	if opts.Format == chatexport.FormatMinitrace {
		writeExportError(w, chatexport.ErrInvalidFormat)
		return
	}
	exported, err := s.exportService.ExportTimeline(r.Context(), string(sid), opts)
	if err != nil {
		writeExportError(w, err)
		return
	}
	writeRenderedExport(w, r, exported, opts.Format, fmt.Sprintf("pinocchio-%s-timeline", sid))
}

func (s *Server) handleTurnsExport(w http.ResponseWriter, r *http.Request, sid sessionstream.SessionId) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	opts, err := parseExportOptions(r, chatexport.TimelineViewEntities)
	if err != nil {
		writeExportError(w, err)
		return
	}
	var exported any
	if opts.Format == chatexport.FormatMinitrace {
		exported, err = s.exportService.ExportTurnsMinitrace(r.Context(), string(sid), opts)
	} else {
		exported, err = s.exportService.ExportTurns(r.Context(), string(sid), opts)
	}
	if err != nil {
		writeExportError(w, err)
		return
	}
	writeRenderedExport(w, r, exported, opts.Format, fmt.Sprintf("pinocchio-%s-turns", sid))
}

func (s *Server) handleFullExport(w http.ResponseWriter, r *http.Request, sid sessionstream.SessionId) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	opts, err := parseExportOptions(r, chatexport.TimelineViewEntities)
	if err != nil {
		writeExportError(w, err)
		return
	}
	if opts.Format == chatexport.FormatMinitrace {
		writeExportError(w, chatexport.ErrInvalidFormat)
		return
	}
	timeline, err := s.exportService.ExportTimeline(r.Context(), string(sid), opts)
	if err != nil {
		writeExportError(w, err)
		return
	}
	turns, err := s.exportService.ExportTurns(r.Context(), string(sid), opts)
	if err != nil && !errors.Is(err, chatexport.ErrTurnStoreUnavailable) {
		writeExportError(w, err)
		return
	}
	payload := fullExportResponse{SessionID: string(sid), Timeline: timeline, Turns: turns}
	writeRenderedExport(w, r, payload, opts.Format, fmt.Sprintf("pinocchio-%s-export", sid))
}

func parseExportOptions(r *http.Request, defaultView chatexport.TimelineView) (chatexport.Options, error) {
	query := r.URL.Query()
	opts := chatexport.Options{
		Format:     chatexport.Format(query.Get("format")),
		View:       chatexport.TimelineView(query.Get("view")),
		Download:   query.Get("download") == "true" || query.Get("download") == "1",
		TurnPhase:  query.Get("phase"),
		LatestOnly: true,
	}
	if opts.View == "" {
		opts.View = defaultView
	}
	return opts.Normalized()
}

func writeRenderedExport(w http.ResponseWriter, r *http.Request, payload any, format chatexport.Format, filenameBase string) {
	rendered, err := chatexport.Render(payload, format)
	if err != nil {
		writeExportError(w, err)
		return
	}
	w.Header().Set("Content-Type", rendered.ContentType)
	if shouldDownload(r) {
		filename := sanitizeDownloadFilename(filenameBase) + rendered.Extension
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rendered.Body)
}

func writeExportError(w http.ResponseWriter, err error) {
	status := exportHTTPStatus(err)
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

func exportHTTPStatus(err error) int {
	switch {
	case errors.Is(err, chatexport.ErrInvalidFormat), errors.Is(err, chatexport.ErrInvalidView):
		return http.StatusBadRequest
	case errors.Is(err, chatexport.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, chatexport.ErrTurnsDBPathRequired):
		return http.StatusConflict
	case errors.Is(err, chatexport.ErrSnapshotUnavailable), errors.Is(err, chatexport.ErrTurnStoreUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func shouldDownload(r *http.Request) bool {
	value := r.URL.Query().Get("download")
	return value == "true" || value == "1"
}

func sanitizeDownloadFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "export"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}
