package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed static chapters
var appFS embed.FS

type systemlabServer struct {
	env *labEnvironment
}

func newSystemlabServer() (*systemlabServer, error) {
	env, err := newLabEnvironment()
	if err != nil {
		return nil, err
	}
	return &systemlabServer{env: env}, nil
}

func (s *systemlabServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/reset", s.handleReset)
	mux.HandleFunc("/api/chapters/", s.handleChapterHTML)
	mux.HandleFunc("/api/phase1/run", s.handlePhase1Run)
	mux.HandleFunc("/api/phase1/export", s.handlePhase1Export)
	mux.HandleFunc("/api/phase2/run", s.handlePhase2Run)
	mux.HandleFunc("/api/phase2/export", s.handlePhase2Export)
	chaptersSub, _ := fs.Sub(appFS, "chapters")
	mux.Handle("/chapters/", http.StripPrefix("/chapters/", http.FileServer(http.FS(chaptersSub))))
	staticSub, _ := fs.Sub(appFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticSub)))
	return mux
}

func (s *systemlabServer) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"app":    "evtstream-systemlab",
		"phases": []string{"phase0", "phase1", "phase2", "phase3", "phase4", "phase5", "phase6"},
		"labs": []map[string]any{
			{"id": "phase0", "title": "Foundations", "implemented": true, "chapter": true},
			{"id": "phase1", "title": "Command → Event → Projection", "implemented": true, "chapter": true},
			{"id": "phase2", "title": "Ordering and Ordinals", "implemented": true, "chapter": true},
			{"id": "phase3", "title": "Hydration and Reconnect", "implemented": false, "chapter": true},
			{"id": "phase4", "title": "Chat Example", "implemented": false, "chapter": true},
			{"id": "phase5", "title": "Persistence and Restart", "implemented": false, "chapter": true},
		},
		"boundary": map[string]any{
			"systemlabCalls":  []string{"public evtstream package APIs", "its own HTTP endpoints"},
			"systemlabAvoids": []string{"legacy webchat internals", "package-global registries", "SEM-specific substrate types"},
		},
	})
}

func (s *systemlabServer) handleReset(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if err := s.env.Reset(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset"})
}

func (s *systemlabServer) handlePhase1Run(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var in phase1RunRequest
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("decode request: %v", err)})
		return
	}
	resp, err := s.env.RunPhase1(req.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *systemlabServer) handlePhase1Export(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	sessionID := req.URL.Query().Get("sessionId")
	format := req.URL.Query().Get("format")
	filename, contentType, body, err := s.env.ExportPhase1(sessionID, format)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *systemlabServer) handlePhase2Run(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var in phase2RunRequest
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("decode request: %v", err)})
		return
	}
	resp, err := s.env.RunPhase2(req.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *systemlabServer) handlePhase2Export(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	format := req.URL.Query().Get("format")
	filename, contentType, body, err := s.env.ExportPhase2(format)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
