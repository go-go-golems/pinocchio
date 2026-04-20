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
	mux.HandleFunc("/api/phase3/run", s.handlePhase3Run)
	mux.HandleFunc("/api/phase3/state", s.handlePhase3State)
	mux.HandleFunc("/api/phase3/ws", s.handlePhase3WS)
	mux.HandleFunc("/api/phase4/run", s.handlePhase4Run)
	mux.HandleFunc("/api/phase4/state", s.handlePhase4State)
	mux.HandleFunc("/api/phase4/ws", s.handlePhase4WS)
	mux.HandleFunc("/api/phase5/run", s.handlePhase5Run)
	mux.HandleFunc("/api/phase5/state", s.handlePhase5State)
	mux.HandleFunc("/api/phase5/ws", s.handlePhase5WS)
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
			{"id": "phase3", "title": "Hydration and Reconnect", "implemented": true, "chapter": true},
			{"id": "phase4", "title": "Chat Example", "implemented": true, "chapter": true},
			{"id": "phase5", "title": "Persistence and Restart", "implemented": true, "chapter": true},
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

func (s *systemlabServer) handlePhase3Run(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var in phase3RunRequest
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("decode request: %v", err)})
		return
	}
	resp, err := s.env.RunPhase3(req.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *systemlabServer) handlePhase3State(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	resp, err := s.env.RunPhase3(req.Context(), phase3RunRequest{
		Action:    "state",
		SessionID: req.URL.Query().Get("sessionId"),
		Prompt:    req.URL.Query().Get("prompt"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *systemlabServer) handlePhase3WS(w http.ResponseWriter, req *http.Request) {
	s.env.mu.Lock()
	state := s.env.phase3
	s.env.mu.Unlock()
	if state == nil || state.ws == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "phase 3 websocket transport is not initialized"})
		return
	}
	state.ws.ServeHTTP(w, req)
}

func (s *systemlabServer) handlePhase4Run(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var in phase4RunRequest
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("decode request: %v", err)})
		return
	}
	resp, err := s.env.RunPhase4(req.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *systemlabServer) handlePhase4State(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	resp, err := s.env.RunPhase4(req.Context(), phase4RunRequest{
		Action:    "state",
		SessionID: req.URL.Query().Get("sessionId"),
		Prompt:    req.URL.Query().Get("prompt"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *systemlabServer) handlePhase4WS(w http.ResponseWriter, req *http.Request) {
	s.env.mu.Lock()
	state := s.env.phase4
	s.env.mu.Unlock()
	if state == nil || state.ws == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "phase 4 websocket transport is not initialized"})
		return
	}
	state.ws.ServeHTTP(w, req)
}

func (s *systemlabServer) handlePhase5Run(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var in phase5RunRequest
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("decode request: %v", err)})
		return
	}
	resp, err := s.env.RunPhase5(req.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *systemlabServer) handlePhase5State(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	resp, err := s.env.RunPhase5(req.Context(), phase5RunRequest{
		Action:    "state",
		Mode:      req.URL.Query().Get("mode"),
		SessionID: req.URL.Query().Get("sessionId"),
		Text:      req.URL.Query().Get("text"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *systemlabServer) handlePhase5WS(w http.ResponseWriter, req *http.Request) {
	s.env.mu.Lock()
	state := s.env.phase5
	s.env.mu.Unlock()
	if state == nil || state.runtime == nil || state.runtime.ws == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "phase 5 websocket transport is not initialized"})
		return
	}
	state.runtime.ws.ServeHTTP(w, req)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
