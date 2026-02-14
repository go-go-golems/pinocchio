package webchat

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/go-go-golems/geppetto/pkg/turns/serde"
)

var errUnsafePath = errors.New("unsafe path")

type offlineRunSummary struct {
	RunID      string         `json:"run_id"`
	Kind       string         `json:"kind"`
	Display    string         `json:"display"`
	SourcePath string         `json:"source_path"`
	Timestamp  int64          `json:"timestamp_ms,omitempty"`
	ConvID     string         `json:"conv_id,omitempty"`
	SessionID  string         `json:"session_id,omitempty"`
	Counts     map[string]any `json:"counts,omitempty"`
}

func encodeOfflineRunID(kind string, parts ...string) string {
	encoded := make([]string, 0, len(parts)+1)
	encoded = append(encoded, kind)
	for _, p := range parts {
		encoded = append(encoded, url.PathEscape(strings.TrimSpace(p)))
	}
	return strings.Join(encoded, "|")
}

func decodeOfflineRunID(raw string) (string, []string, error) {
	parts := strings.Split(raw, "|")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", nil, errors.New("invalid run id")
	}
	kind := parts[0]
	decoded := make([]string, 0, len(parts)-1)
	for _, p := range parts[1:] {
		v, err := url.PathUnescape(p)
		if err != nil {
			return "", nil, err
		}
		decoded = append(decoded, v)
	}
	return kind, decoded, nil
}

func parsePositiveInt(q string, defaultValue int) int {
	s := strings.TrimSpace(q)
	if s == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultValue
	}
	return v
}

func (r *Router) registerOfflineDebugHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/debug/runs", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		artifactsRoot := strings.TrimSpace(req.URL.Query().Get("artifacts_root"))
		turnsDB := strings.TrimSpace(req.URL.Query().Get("turns_db"))
		timelineDB := strings.TrimSpace(req.URL.Query().Get("timeline_db"))
		limit := parsePositiveInt(req.URL.Query().Get("limit"), 200)

		if artifactsRoot == "" && turnsDB == "" && timelineDB == "" {
			http.Error(w, "at least one source is required (artifacts_root, turns_db, timeline_db)", http.StatusBadRequest)
			return
		}

		items := make([]offlineRunSummary, 0, limit)
		if artifactsRoot != "" {
			runs, err := scanArtifactRuns(artifactsRoot)
			if err != nil {
				http.Error(w, fmt.Sprintf("scan artifact runs: %v", err), http.StatusInternalServerError)
				return
			}
			items = append(items, runs...)
		}
		if turnsDB != "" {
			runs, err := scanTurnsSQLiteRuns(turnsDB, limit)
			if err != nil {
				http.Error(w, fmt.Sprintf("scan turns sqlite runs: %v", err), http.StatusInternalServerError)
				return
			}
			items = append(items, runs...)
		}
		if timelineDB != "" {
			runs, err := scanTimelineSQLiteRuns(timelineDB, limit)
			if err != nil {
				http.Error(w, fmt.Sprintf("scan timeline sqlite runs: %v", err), http.StatusInternalServerError)
				return
			}
			items = append(items, runs...)
		}

		sort.Slice(items, func(i, j int) bool {
			if items[i].Timestamp == items[j].Timestamp {
				return items[i].RunID < items[j].RunID
			}
			return items[i].Timestamp > items[j].Timestamp
		})
		if len(items) > limit {
			items = items[:limit]
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"artifacts_root": artifactsRoot,
			"turns_db":       turnsDB,
			"timeline_db":    timelineDB,
			"limit":          limit,
			"items":          items,
		})
	})

	mux.HandleFunc("/api/debug/runs/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rawRunID := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/debug/runs/"), "/")
		if rawRunID == "" {
			http.Error(w, "missing run id", http.StatusBadRequest)
			return
		}
		runID, err := url.PathUnescape(rawRunID)
		if err != nil {
			http.Error(w, "invalid run id", http.StatusBadRequest)
			return
		}
		kind, parts, err := decodeOfflineRunID(runID)
		if err != nil {
			http.Error(w, "invalid run id", http.StatusBadRequest)
			return
		}

		artifactsRoot := strings.TrimSpace(req.URL.Query().Get("artifacts_root"))
		turnsDB := strings.TrimSpace(req.URL.Query().Get("turns_db"))
		timelineDB := strings.TrimSpace(req.URL.Query().Get("timeline_db"))
		limit := parsePositiveInt(req.URL.Query().Get("limit"), 500)

		switch kind {
		case "artifact":
			if artifactsRoot == "" {
				http.Error(w, "artifacts_root is required for artifact runs", http.StatusBadRequest)
				return
			}
			if len(parts) != 1 {
				http.Error(w, "invalid artifact run id", http.StatusBadRequest)
				return
			}
			detail, err := loadArtifactRunDetail(artifactsRoot, parts[0], limit)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					http.Error(w, "run not found", http.StatusNotFound)
					return
				}
				if errors.Is(err, errUnsafePath) {
					http.Error(w, "invalid run path", http.StatusBadRequest)
					return
				}
				http.Error(w, fmt.Sprintf("load artifact run: %v", err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"run_id": runID,
				"kind":   kind,
				"detail": detail,
			})
			return

		case "turns":
			if turnsDB == "" {
				http.Error(w, "turns_db is required for turns runs", http.StatusBadRequest)
				return
			}
			if len(parts) != 2 {
				http.Error(w, "invalid turns run id", http.StatusBadRequest)
				return
			}
			convID := parts[0]
			sessionID := parts[1]
			detail, err := loadTurnsSQLiteRunDetail(req.Context(), turnsDB, convID, sessionID, limit)
			if err != nil {
				http.Error(w, fmt.Sprintf("load turns run: %v", err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"run_id": runID,
				"kind":   kind,
				"detail": detail,
			})
			return

		case "timeline":
			if timelineDB == "" {
				http.Error(w, "timeline_db is required for timeline runs", http.StatusBadRequest)
				return
			}
			if len(parts) != 1 {
				http.Error(w, "invalid timeline run id", http.StatusBadRequest)
				return
			}
			convID := parts[0]
			sinceVersion := uint64(parsePositiveInt(req.URL.Query().Get("since_version"), 0))
			detail, err := loadTimelineSQLiteRunDetail(req.Context(), timelineDB, convID, sinceVersion, limit)
			if err != nil {
				http.Error(w, fmt.Sprintf("load timeline run: %v", err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"run_id": runID,
				"kind":   kind,
				"detail": detail,
			})
			return

		default:
			http.Error(w, "unknown run kind", http.StatusBadRequest)
			return
		}
	})
}

func scanArtifactRuns(root string) ([]offlineRunSummary, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("empty artifact root")
	}
	stat, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, errors.New("artifact root is not a directory")
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	items := make([]offlineRunSummary, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		runPath := filepath.Join(root, e.Name())
		files, err := os.ReadDir(runPath)
		if err != nil {
			continue
		}

		hasTurns := false
		hasEvents := false
		hasLogs := false
		turnCount := 0
		eventFiles := 0

		for _, f := range files {
			name := strings.ToLower(strings.TrimSpace(f.Name()))
			switch {
			case name == "input_turn.yaml":
				hasTurns = true
			case name == "final_turn.yaml":
				hasTurns = true
				turnCount++
			case strings.HasPrefix(name, "final_turn_") && strings.HasSuffix(name, ".yaml"):
				hasTurns = true
				turnCount++
			case name == "events.ndjson":
				hasEvents = true
				eventFiles++
			case strings.HasPrefix(name, "events-") && strings.HasSuffix(name, ".ndjson"):
				hasEvents = true
				eventFiles++
			case name == "logs.jsonl":
				hasLogs = true
			}
		}

		if !hasTurns && !hasEvents && !hasLogs {
			continue
		}
		info, _ := e.Info()
		ts := int64(0)
		if info != nil {
			ts = info.ModTime().UnixMilli()
		}
		items = append(items, offlineRunSummary{
			RunID:      encodeOfflineRunID("artifact", e.Name()),
			Kind:       "artifact",
			Display:    e.Name(),
			SourcePath: runPath,
			Timestamp:  ts,
			Counts: map[string]any{
				"turns":       turnCount,
				"event_files": eventFiles,
				"has_logs":    hasLogs,
			},
		})
	}
	return items, nil
}

func scanTurnsSQLiteRuns(dbPath string, limit int) ([]offlineRunSummary, error) {
	dsn, err := chatstore.SQLiteTurnDSNForFile(dbPath)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`
		SELECT conv_id, session_id, COUNT(*) AS n, MAX(created_at_ms) AS max_created, MIN(created_at_ms) AS min_created
		FROM turns
		GROUP BY conv_id, session_id
		ORDER BY max_created DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]offlineRunSummary, 0, 32)
	for rows.Next() {
		var convID, sessionID string
		var n int
		var maxCreated, minCreated int64
		if err := rows.Scan(&convID, &sessionID, &n, &maxCreated, &minCreated); err != nil {
			return nil, err
		}
		items = append(items, offlineRunSummary{
			RunID:      encodeOfflineRunID("turns", convID, sessionID),
			Kind:       "turns",
			Display:    fmt.Sprintf("%s / %s", convID, sessionID),
			SourcePath: dbPath,
			Timestamp:  maxCreated,
			ConvID:     convID,
			SessionID:  sessionID,
			Counts: map[string]any{
				"turn_rows":        n,
				"first_created_ms": minCreated,
				"last_created_ms":  maxCreated,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func scanTimelineSQLiteRuns(dbPath string, limit int) ([]offlineRunSummary, error) {
	dsn, err := chatstore.SQLiteTimelineDSNForFile(dbPath)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`
		SELECT conv_id, version
		FROM timeline_versions
		ORDER BY version DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]offlineRunSummary, 0, 32)
	for rows.Next() {
		var convID string
		var version int64
		if err := rows.Scan(&convID, &version); err != nil {
			return nil, err
		}
		items = append(items, offlineRunSummary{
			RunID:      encodeOfflineRunID("timeline", convID),
			Kind:       "timeline",
			Display:    convID,
			SourcePath: dbPath,
			ConvID:     convID,
			Counts: map[string]any{
				"version": version,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func loadArtifactRunDetail(root string, runName string, limit int) (map[string]any, error) {
	runPath, err := secureJoin(root, runName)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(runPath); err != nil {
		return nil, err
	}

	detail := map[string]any{
		"run_name": runName,
		"path":     runPath,
	}

	// input turn
	if b, err := os.ReadFile(filepath.Join(runPath, "input_turn.yaml")); err == nil {
		parsed, parseErr := decodeTurnPayload(string(b))
		input := map[string]any{
			"yaml": string(b),
		}
		if parseErr != nil {
			input["parse_error"] = parseErr.Error()
		} else {
			input["parsed"] = parsed
		}
		detail["input_turn"] = input
	}

	// final turns
	turnNames := make([]string, 0, 8)
	entries, err := os.ReadDir(runPath)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(e.Name()))
		if name == "final_turn.yaml" || (strings.HasPrefix(name, "final_turn_") && strings.HasSuffix(name, ".yaml")) {
			turnNames = append(turnNames, e.Name())
		}
	}
	sort.Strings(turnNames)
	if len(turnNames) > limit {
		turnNames = turnNames[:limit]
	}
	turns := make([]map[string]any, 0, len(turnNames))
	for _, name := range turnNames {
		b, err := os.ReadFile(filepath.Join(runPath, name))
		if err != nil {
			continue
		}
		item := map[string]any{
			"name": name,
			"yaml": string(b),
		}
		parsed, parseErr := decodeTurnPayload(string(b))
		if parseErr != nil {
			item["parse_error"] = parseErr.Error()
		} else {
			item["parsed"] = parsed
		}
		turns = append(turns, item)
	}
	detail["turns"] = turns

	// events ndjson
	eventFiles := make([]string, 0, 4)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(e.Name()))
		if name == "events.ndjson" || (strings.HasPrefix(name, "events-") && strings.HasSuffix(name, ".ndjson")) {
			eventFiles = append(eventFiles, e.Name())
		}
	}
	sort.Strings(eventFiles)
	if len(eventFiles) > limit {
		eventFiles = eventFiles[:limit]
	}
	events := make([]map[string]any, 0, len(eventFiles))
	for _, name := range eventFiles {
		items, err := readNDJSON(filepath.Join(runPath, name), limit)
		if err != nil {
			events = append(events, map[string]any{
				"name":  name,
				"error": err.Error(),
			})
			continue
		}
		events = append(events, map[string]any{
			"name":  name,
			"items": items,
		})
	}
	detail["events"] = events

	// logs
	logPath := filepath.Join(runPath, "logs.jsonl")
	if _, err := os.Stat(logPath); err == nil {
		logItems, logErr := readNDJSON(logPath, limit)
		if logErr != nil {
			detail["logs_error"] = logErr.Error()
		} else {
			detail["logs"] = logItems
		}
	}

	return detail, nil
}

func loadTurnsSQLiteRunDetail(ctx context.Context, dbPath string, convID string, sessionID string, limit int) (map[string]any, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	dsn, err := chatstore.SQLiteTurnDSNForFile(dbPath)
	if err != nil {
		return nil, err
	}
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()

	rows, err := store.List(ctx, chatstore.TurnQuery{
		ConvID:    convID,
		SessionID: sessionID,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{
			"conv_id":       row.ConvID,
			"session_id":    row.SessionID,
			"turn_id":       row.TurnID,
			"phase":         row.Phase,
			"created_at_ms": row.CreatedAtMs,
			"payload":       row.Payload,
		}
		parsed, parseErr := decodeTurnPayload(row.Payload)
		if parseErr != nil {
			item["parse_error"] = parseErr.Error()
		} else {
			item["parsed"] = parsed
		}
		items = append(items, item)
	}

	return map[string]any{
		"conv_id":    convID,
		"session_id": sessionID,
		"source_db":  dbPath,
		"items":      items,
	}, nil
}

func loadTimelineSQLiteRunDetail(ctx context.Context, dbPath string, convID string, sinceVersion uint64, limit int) (map[string]any, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	dsn, err := chatstore.SQLiteTimelineDSNForFile(dbPath)
	if err != nil {
		return nil, err
	}
	store, err := chatstore.NewSQLiteTimelineStore(dsn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()

	snap, err := store.GetSnapshot(ctx, convID, sinceVersion, limit)
	if err != nil {
		return nil, err
	}
	b, err := protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   false,
	}.Marshal(snap)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}

	return map[string]any{
		"conv_id":       convID,
		"since_version": sinceVersion,
		"source_db":     dbPath,
		"snapshot":      payload,
	}, nil
}

func secureJoin(root string, rel string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("empty root")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(absRoot, rel)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if absJoined != absRoot && !strings.HasPrefix(absJoined, absRoot+string(os.PathSeparator)) {
		return "", errUnsafePath
	}
	return absJoined, nil
}

func decodeTurnPayload(yamlPayload string) (map[string]any, error) {
	t, err := serde.FromYAML([]byte(yamlPayload))
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func readNDJSON(path string, limit int) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	items := make([]map[string]any, 0, limit)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			items = append(items, map[string]any{"raw": line, "parse_error": err.Error()})
		} else {
			items = append(items, m)
		}
		if len(items) >= limit {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
