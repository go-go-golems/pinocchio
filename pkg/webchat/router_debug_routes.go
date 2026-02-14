package webchat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"
)

func (r *Router) debugRoutesEnabled() bool {
	return !r.disableDebugRoutes
}

func (r *Router) registerDebugAPIHandlers(mux *http.ServeMux) {
	logger := log.With().Str("component", "webchat").Logger()

	r.registerOfflineDebugHandlers(mux)

	mux.HandleFunc("/api/debug/conversations", func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.cm == nil {
			http.Error(w, "conversation manager not initialized", http.StatusServiceUnavailable)
			return
		}

		type convSummary struct {
			ConvID            string `json:"conv_id"`
			SessionID         string `json:"session_id"`
			RuntimeKey        string `json:"runtime_key"`
			ActiveSockets     int    `json:"active_sockets"`
			StreamRunning     bool   `json:"stream_running"`
			QueueDepth        int    `json:"queue_depth"`
			BufferedEvents    int    `json:"buffered_events"`
			LastActivityMs    int64  `json:"last_activity_ms"`
			HasTimelineSource bool   `json:"has_timeline_source"`
		}

		r.cm.mu.Lock()
		convs := make([]*Conversation, 0, len(r.cm.conns))
		for _, conv := range r.cm.conns {
			convs = append(convs, conv)
		}
		r.cm.mu.Unlock()

		items := make([]convSummary, 0, len(convs))
		for _, conv := range convs {
			if conv == nil {
				continue
			}
			conv.mu.Lock()
			sessionID := conv.SessionID
			runtimeKey := conv.RuntimeKey
			queueDepth := len(conv.queue)
			streamRunning := conv.stream != nil && conv.stream.IsRunning()
			lastActivityMs := int64(0)
			if !conv.lastActivity.IsZero() {
				lastActivityMs = conv.lastActivity.UnixMilli()
			}
			bufferedEvents := 0
			if conv.semBuf != nil {
				bufferedEvents = len(conv.semBuf.Snapshot())
			}
			hasTimelineSource := conv.timelineProj != nil
			pool := conv.pool
			conv.mu.Unlock()

			activeSockets := 0
			if pool != nil {
				activeSockets = pool.Count()
			}

			items = append(items, convSummary{
				ConvID:            conv.ID,
				SessionID:         sessionID,
				RuntimeKey:        runtimeKey,
				ActiveSockets:     activeSockets,
				StreamRunning:     streamRunning,
				QueueDepth:        queueDepth,
				BufferedEvents:    bufferedEvents,
				LastActivityMs:    lastActivityMs,
				HasTimelineSource: hasTimelineSource,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].LastActivityMs == items[j].LastActivityMs {
				return items[i].ConvID < items[j].ConvID
			}
			return items[i].LastActivityMs > items[j].LastActivityMs
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": items,
		})
	})

	mux.HandleFunc("/api/debug/conversations/", func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.cm == nil {
			http.Error(w, "conversation manager not initialized", http.StatusServiceUnavailable)
			return
		}

		rawConvID := strings.Trim(strings.TrimPrefix(r0.URL.Path, "/api/debug/conversations/"), "/")
		if rawConvID == "" {
			http.Error(w, "missing conv_id", http.StatusBadRequest)
			return
		}
		convID, err := url.PathUnescape(rawConvID)
		if err != nil || strings.TrimSpace(convID) == "" {
			http.Error(w, "invalid conv_id", http.StatusBadRequest)
			return
		}

		conv, ok := r.cm.GetConversation(convID)
		if !ok || conv == nil {
			http.Error(w, "conversation not found", http.StatusNotFound)
			return
		}

		conv.mu.Lock()
		sessionID := conv.SessionID
		runtimeKey := conv.RuntimeKey
		queueDepth := len(conv.queue)
		streamRunning := conv.stream != nil && conv.stream.IsRunning()
		lastActivityMs := int64(0)
		if !conv.lastActivity.IsZero() {
			lastActivityMs = conv.lastActivity.UnixMilli()
		}
		bufferedEvents := 0
		if conv.semBuf != nil {
			bufferedEvents = len(conv.semBuf.Snapshot())
		}
		activeRequestKey := conv.activeRequestKey
		hasTimelineSource := conv.timelineProj != nil
		pool := conv.pool
		conv.mu.Unlock()

		activeSockets := 0
		if pool != nil {
			activeSockets = pool.Count()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"conv_id":             convID,
			"session_id":          sessionID,
			"runtime_key":         runtimeKey,
			"active_sockets":      activeSockets,
			"stream_running":      streamRunning,
			"queue_depth":         queueDepth,
			"buffered_events":     bufferedEvents,
			"last_activity_ms":    lastActivityMs,
			"active_request_key":  activeRequestKey,
			"has_timeline_source": hasTimelineSource,
		})
	})

	mux.HandleFunc("/api/debug/events/", func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.cm == nil {
			http.Error(w, "conversation manager not initialized", http.StatusServiceUnavailable)
			return
		}

		rawConvID := strings.Trim(strings.TrimPrefix(r0.URL.Path, "/api/debug/events/"), "/")
		if rawConvID == "" {
			http.Error(w, "missing conv_id", http.StatusBadRequest)
			return
		}
		convID, err := url.PathUnescape(rawConvID)
		if err != nil || strings.TrimSpace(convID) == "" {
			http.Error(w, "invalid conv_id", http.StatusBadRequest)
			return
		}

		conv, ok := r.cm.GetConversation(convID)
		if !ok || conv == nil {
			http.Error(w, "conversation not found", http.StatusNotFound)
			return
		}

		sinceSeq := int64(0)
		if s := strings.TrimSpace(r0.URL.Query().Get("since_seq")); s != "" {
			var parsed int64
			_, _ = fmt.Sscanf(s, "%d", &parsed)
			if parsed > 0 {
				sinceSeq = parsed
			}
		}
		typeFilter := strings.TrimSpace(r0.URL.Query().Get("type"))
		limit := 200
		if s := strings.TrimSpace(r0.URL.Query().Get("limit")); s != "" {
			var parsed int
			_, _ = fmt.Sscanf(s, "%d", &parsed)
			if parsed > 0 {
				limit = parsed
			}
		}

		conv.mu.Lock()
		buf := conv.semBuf
		conv.mu.Unlock()

		frames := [][]byte(nil)
		if buf != nil {
			frames = buf.Snapshot()
		}

		items := make([]map[string]any, 0, limit)
		for i, frame := range frames {
			seq := int64(i + 1)
			if seq <= sinceSeq {
				continue
			}

			item := map[string]any{
				"seq": seq,
			}
			var decoded map[string]any
			if err := json.Unmarshal(frame, &decoded); err != nil {
				item["decode_error"] = err.Error()
				item["raw"] = string(frame)
				if typeFilter != "" {
					continue
				}
				items = append(items, item)
				if len(items) >= limit {
					break
				}
				continue
			}

			eventType := ""
			eventID := ""
			if eventRaw, ok := decoded["event"].(map[string]any); ok {
				if t, ok := eventRaw["type"].(string); ok {
					eventType = strings.TrimSpace(t)
				}
				if id, ok := eventRaw["id"].(string); ok {
					eventID = strings.TrimSpace(id)
				}
			}
			if typeFilter != "" && eventType != typeFilter {
				continue
			}

			item["type"] = eventType
			item["id"] = eventID
			item["frame"] = decoded
			items = append(items, item)
			if len(items) >= limit {
				break
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"conv_id":   convID,
			"since_seq": sinceSeq,
			"type":      typeFilter,
			"limit":     limit,
			"items":     items,
		})
	})

	// debug endpoints (dev-gated via PINOCCHIO_WEBCHAT_DEBUG=1)
	stepEnableHandler := func(w http.ResponseWriter, r0 *http.Request) {
		if os.Getenv("PINOCCHIO_WEBCHAT_DEBUG") != "1" {
			http.NotFound(w, r0)
			return
		}
		if r0.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ConvID    string `json:"conv_id"`
			SessionID string `json:"session_id"`
			Owner     string `json:"owner"`
		}
		if err := json.NewDecoder(r0.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		sessionID := strings.TrimSpace(body.SessionID)
		convID := strings.TrimSpace(body.ConvID)
		if sessionID == "" && convID != "" {
			if c, ok := r.cm.GetConversation(convID); ok && c != nil {
				sessionID = c.SessionID
			}
		}
		if sessionID == "" {
			http.Error(w, "missing session_id (or unknown conv_id)", http.StatusBadRequest)
			return
		}
		if r.stepCtrl == nil {
			http.Error(w, "step controller not initialized", http.StatusInternalServerError)
			return
		}
		r.stepCtrl.Enable(toolloop.StepScope{SessionID: sessionID, ConversationID: convID, Owner: strings.TrimSpace(body.Owner)})
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "session_id": sessionID, "conv_id": convID})
	}
	mux.HandleFunc("/debug/step/enable", stepEnableHandler)
	mux.HandleFunc("/api/debug/step/enable", stepEnableHandler)

	stepDisableHandler := func(w http.ResponseWriter, r0 *http.Request) {
		if os.Getenv("PINOCCHIO_WEBCHAT_DEBUG") != "1" {
			http.NotFound(w, r0)
			return
		}
		if r0.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			SessionID string `json:"session_id"`
			ConvID    string `json:"conv_id"`
		}
		if err := json.NewDecoder(r0.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		sessionID := strings.TrimSpace(body.SessionID)
		convID := strings.TrimSpace(body.ConvID)
		if sessionID == "" && convID != "" {
			if c, ok := r.cm.GetConversation(convID); ok && c != nil {
				sessionID = c.SessionID
			}
		}
		if sessionID == "" {
			http.Error(w, "missing session_id (or unknown conv_id)", http.StatusBadRequest)
			return
		}
		if r.stepCtrl != nil {
			r.stepCtrl.DisableSession(sessionID)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "session_id": sessionID})
	}
	mux.HandleFunc("/debug/step/disable", stepDisableHandler)
	mux.HandleFunc("/api/debug/step/disable", stepDisableHandler)

	continueHandler := func(w http.ResponseWriter, r0 *http.Request) {
		if os.Getenv("PINOCCHIO_WEBCHAT_DEBUG") != "1" {
			http.NotFound(w, r0)
			return
		}
		if r0.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			PauseID string `json:"pause_id"`
			ConvID  string `json:"conv_id,omitempty"`
		}
		if err := json.NewDecoder(r0.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		pauseID := strings.TrimSpace(body.PauseID)
		if pauseID == "" {
			http.Error(w, "missing pause_id", http.StatusBadRequest)
			return
		}
		if r.stepCtrl == nil {
			http.Error(w, "step controller not initialized", http.StatusInternalServerError)
			return
		}
		if convID := strings.TrimSpace(body.ConvID); convID != "" {
			if meta, ok := r.stepCtrl.Lookup(pauseID); ok {
				if meta.Scope.ConversationID != "" && meta.Scope.ConversationID != convID {
					http.Error(w, "pause does not belong to this conversation", http.StatusForbidden)
					return
				}
			}
		}
		meta, ok := r.stepCtrl.Continue(pauseID)
		if !ok {
			http.Error(w, "unknown pause_id", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "pause": meta})
	}
	mux.HandleFunc("/debug/continue", continueHandler)
	mux.HandleFunc("/api/debug/continue", continueHandler)

	timelineHandler := func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.timelineStore == nil {
			http.Error(w, "timeline store not enabled", http.StatusNotFound)
			return
		}

		convID := strings.TrimSpace(r0.URL.Query().Get("conv_id"))
		if convID == "" {
			http.Error(w, "missing conv_id", http.StatusBadRequest)
			return
		}

		var sinceVersion uint64
		if s := strings.TrimSpace(r0.URL.Query().Get("since_version")); s != "" {
			_, _ = fmt.Sscanf(s, "%d", &sinceVersion)
		}
		limit := 0
		if s := strings.TrimSpace(r0.URL.Query().Get("limit")); s != "" {
			var v int
			_, _ = fmt.Sscanf(s, "%d", &v)
			if v > 0 {
				limit = v
			}
		}

		snap, err := r.timelineStore.GetSnapshot(r0.Context(), convID, sinceVersion, limit)
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
	mux.HandleFunc("/timeline", timelineHandler)
	mux.HandleFunc("/timeline/", timelineHandler)
	mux.HandleFunc("/api/debug/timeline", timelineHandler)
	mux.HandleFunc("/api/debug/timeline/", timelineHandler)

	turnsHandler := func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.turnStore == nil {
			http.Error(w, "turn store not enabled", http.StatusNotFound)
			return
		}

		convID := strings.TrimSpace(r0.URL.Query().Get("conv_id"))
		sessionID := strings.TrimSpace(r0.URL.Query().Get("session_id"))
		if convID == "" && sessionID == "" {
			http.Error(w, "missing conv_id or session_id", http.StatusBadRequest)
			return
		}
		phase := strings.TrimSpace(r0.URL.Query().Get("phase"))

		var sinceMs int64
		if s := strings.TrimSpace(r0.URL.Query().Get("since_ms")); s != "" {
			var v int64
			_, _ = fmt.Sscanf(s, "%d", &v)
			if v > 0 {
				sinceMs = v
			}
		}
		limit := 0
		if s := strings.TrimSpace(r0.URL.Query().Get("limit")); s != "" {
			var v int
			_, _ = fmt.Sscanf(s, "%d", &v)
			if v > 0 {
				limit = v
			}
		}

		items, err := r.turnStore.List(r0.Context(), chatstore.TurnQuery{
			ConvID:    convID,
			SessionID: sessionID,
			Phase:     phase,
			SinceMs:   sinceMs,
			Limit:     limit,
		})
		if err != nil {
			logger.Error().Err(err).Str("conv_id", convID).Str("session_id", sessionID).Msg("turns query failed")
			http.Error(w, "turns query failed", http.StatusInternalServerError)
			return
		}

		resp := map[string]any{
			"conv_id":    convID,
			"session_id": sessionID,
			"phase":      phase,
			"since_ms":   sinceMs,
			"items":      items,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
	mux.HandleFunc("/turns", turnsHandler)
	mux.HandleFunc("/turns/", turnsHandler)
	mux.HandleFunc("/api/debug/turns", turnsHandler)
	mux.HandleFunc("/api/debug/turns/", turnsHandler)

	mux.HandleFunc("/api/debug/turn/", func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.turnStore == nil {
			http.Error(w, "turn store not enabled", http.StatusNotFound)
			return
		}

		raw := strings.Trim(strings.TrimPrefix(r0.URL.Path, "/api/debug/turn/"), "/")
		parts := strings.Split(raw, "/")
		if len(parts) != 3 {
			http.Error(w, "expected /api/debug/turn/:convId/:sessionId/:turnId", http.StatusBadRequest)
			return
		}

		convID, err := url.PathUnescape(parts[0])
		if err != nil || strings.TrimSpace(convID) == "" {
			http.Error(w, "invalid conv_id", http.StatusBadRequest)
			return
		}
		sessionID, err := url.PathUnescape(parts[1])
		if err != nil || strings.TrimSpace(sessionID) == "" {
			http.Error(w, "invalid session_id", http.StatusBadRequest)
			return
		}
		turnID, err := url.PathUnescape(parts[2])
		if err != nil || strings.TrimSpace(turnID) == "" {
			http.Error(w, "invalid turn_id", http.StatusBadRequest)
			return
		}

		items, err := r.turnStore.List(r0.Context(), chatstore.TurnQuery{
			ConvID:    convID,
			SessionID: sessionID,
			Limit:     500,
		})
		if err != nil {
			logger.Error().Err(err).Str("conv_id", convID).Str("session_id", sessionID).Str("turn_id", turnID).Msg("turn detail query failed")
			http.Error(w, "turn detail query failed", http.StatusInternalServerError)
			return
		}

		type phaseSnapshot struct {
			Phase       string         `json:"phase"`
			CreatedAtMs int64          `json:"created_at_ms"`
			Payload     string         `json:"payload"`
			Parsed      map[string]any `json:"parsed,omitempty"`
			ParseError  string         `json:"parse_error,omitempty"`
		}
		phases := make([]phaseSnapshot, 0, 4)
		for _, item := range items {
			if item.TurnID != turnID {
				continue
			}
			s := phaseSnapshot{
				Phase:       item.Phase,
				CreatedAtMs: item.CreatedAtMs,
				Payload:     item.Payload,
			}
			t, err := serde.FromYAML([]byte(item.Payload))
			if err != nil {
				s.ParseError = err.Error()
			} else if t != nil {
				b, err := json.Marshal(t)
				if err != nil {
					s.ParseError = err.Error()
				} else {
					var parsed map[string]any
					if err := json.Unmarshal(b, &parsed); err != nil {
						s.ParseError = err.Error()
					} else {
						s.Parsed = parsed
					}
				}
			}
			phases = append(phases, s)
		}
		if len(phases) == 0 {
			http.Error(w, "turn not found", http.StatusNotFound)
			return
		}
		sort.Slice(phases, func(i, j int) bool {
			return phases[i].CreatedAtMs > phases[j].CreatedAtMs
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"conv_id":    convID,
			"session_id": sessionID,
			"turn_id":    turnID,
			"items":      phases,
		})
	})
}
