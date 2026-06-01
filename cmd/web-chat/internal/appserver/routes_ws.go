package appserver

import "net/http"

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.ws == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "websocket transport not initialized"})
		return
	}
	s.ws.ServeHTTP(w, r)
}
