package export

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

const (
	minitraceSchemaVersion    = "minitrace-v0.2.0"
	minitraceSourceFormat     = "pinocchio-turns-sqlite-v1"
	minitraceConverterVersion = "pinocchio-chatapp-export-dev"
	minitraceTruncateLimit    = 10 * 1024

	minitracePayloadKeyArgs   = "args"
	minitracePayloadKeyError  = "error"
	minitracePayloadKeyID     = "id"
	minitracePayloadKeyName   = "name"
	minitracePayloadKeyResult = "result"
	minitracePayloadKeyText   = "text"
)

var minitracePhasePreference = []string{"final", "post_tools", "post_inference", "pre_inference"}

type minitraceSnapshotSummary struct {
	ConvID              string
	SessionID           string
	TurnID              string
	TurnCreatedAtMS     int64
	RuntimeKey          string
	InferenceID         string
	Phase               string
	SnapshotCreatedAtMS int64
}

type minitraceBlock struct {
	ID       string
	Kind     string
	Role     string
	Payload  map[string]any
	Metadata map[string]any
}

type minitraceCanonicalSnapshot struct {
	minitraceSnapshotSummary
	Blocks []minitraceBlock
}

func (s *Service) ExportTurnsMinitrace(ctx context.Context, sessionID string, _ Options) (any, error) {
	if s == nil || strings.TrimSpace(s.turnsDBPath) == "" {
		return nil, ErrTurnsDBPathRequired
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.Wrap(ErrNotFound, "session id is empty")
	}
	session, err := convertTurnsDBToMinitrace(ctx, s.turnsDBPath, sessionID, s.formatNow())
	if err != nil {
		return nil, err
	}
	return session, nil
}

func convertTurnsDBToMinitrace(ctx context.Context, dbPath string, convID string, exportedAt string) (map[string]any, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, errors.Wrap(ErrNotFound, "stat turns db")
	}
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return nil, errors.Wrap(err, "open turns db")
	}
	defer func() { _ = db.Close() }()

	snapshots, err := loadMinitraceCanonicalSnapshots(ctx, db, convID)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, ErrNotFound
	}
	return buildMinitraceSession(convID, dbPath, exportedAt, snapshots), nil
}

func loadMinitraceCanonicalSnapshots(ctx context.Context, db *sql.DB, convID string) ([]minitraceCanonicalSnapshot, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
		  t.conv_id,
		  t.session_id,
		  t.turn_id,
		  t.turn_created_at_ms,
		  COALESCE(t.runtime_key, '') AS runtime_key,
		  COALESCE(t.inference_id, '') AS inference_id,
		  m.phase,
		  m.snapshot_created_at_ms
		FROM turns t
		JOIN turn_block_membership m
		  ON m.conv_id = t.conv_id
		 AND m.session_id = t.session_id
		 AND m.turn_id = t.turn_id
		WHERE t.conv_id = ?
		GROUP BY
		  t.conv_id,
		  t.session_id,
		  t.turn_id,
		  t.turn_created_at_ms,
		  t.runtime_key,
		  t.inference_id,
		  m.phase,
		  m.snapshot_created_at_ms
		ORDER BY t.turn_created_at_ms ASC, t.session_id ASC, t.turn_id ASC, m.snapshot_created_at_ms ASC
	`, convID)
	if err != nil {
		return nil, errors.Wrap(err, "query minitrace snapshot summaries")
	}
	defer func() { _ = rows.Close() }()

	grouped := map[string][]minitraceSnapshotSummary{}
	for rows.Next() {
		var s minitraceSnapshotSummary
		if err := rows.Scan(&s.ConvID, &s.SessionID, &s.TurnID, &s.TurnCreatedAtMS, &s.RuntimeKey, &s.InferenceID, &s.Phase, &s.SnapshotCreatedAtMS); err != nil {
			return nil, errors.Wrap(err, "scan minitrace snapshot summary")
		}
		key := s.ConvID + "\x00" + s.SessionID + "\x00" + s.TurnID
		grouped[key] = append(grouped[key], s)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate minitrace snapshot summaries")
	}

	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := grouped[keys[i]][0]
		right := grouped[keys[j]][0]
		if left.TurnCreatedAtMS != right.TurnCreatedAtMS {
			return left.TurnCreatedAtMS < right.TurnCreatedAtMS
		}
		if left.SessionID != right.SessionID {
			return left.SessionID < right.SessionID
		}
		return left.TurnID < right.TurnID
	})

	out := make([]minitraceCanonicalSnapshot, 0, len(keys))
	for _, key := range keys {
		summary := chooseMinitraceSummary(grouped[key])
		blocks, err := loadMinitraceBlocks(ctx, db, summary)
		if err != nil {
			return nil, err
		}
		out = append(out, minitraceCanonicalSnapshot{minitraceSnapshotSummary: summary, Blocks: blocks})
	}
	return out, nil
}

func chooseMinitraceSummary(summaries []minitraceSnapshotSummary) minitraceSnapshotSummary {
	for _, phase := range minitracePhasePreference {
		var selected *minitraceSnapshotSummary
		for i := range summaries {
			if summaries[i].Phase != phase {
				continue
			}
			if selected == nil || summaries[i].SnapshotCreatedAtMS > selected.SnapshotCreatedAtMS {
				selected = &summaries[i]
			}
		}
		if selected != nil {
			return *selected
		}
	}
	selected := summaries[0]
	for _, summary := range summaries[1:] {
		if summary.SnapshotCreatedAtMS > selected.SnapshotCreatedAtMS {
			selected = summary
		}
	}
	return selected
}

func loadMinitraceBlocks(ctx context.Context, db *sql.DB, summary minitraceSnapshotSummary) ([]minitraceBlock, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
		  b.block_id,
		  b.kind,
		  b.role,
		  COALESCE(b.payload_json, '{}') AS payload_json,
		  COALESCE(b.block_metadata_json, '{}') AS block_metadata_json
		FROM turn_block_membership m
		JOIN blocks b
		  ON b.block_id = m.block_id
		 AND b.content_hash = m.content_hash
		WHERE m.conv_id = ?
		  AND m.session_id = ?
		  AND m.turn_id = ?
		  AND m.phase = ?
		  AND m.snapshot_created_at_ms = ?
		ORDER BY m.ordinal ASC
	`, summary.ConvID, summary.SessionID, summary.TurnID, summary.Phase, summary.SnapshotCreatedAtMS)
	if err != nil {
		return nil, errors.Wrap(err, "query minitrace blocks")
	}
	defer func() { _ = rows.Close() }()

	blocks := []minitraceBlock{}
	for rows.Next() {
		var block minitraceBlock
		var payloadJSON, metadataJSON string
		if err := rows.Scan(&block.ID, &block.Kind, &block.Role, &payloadJSON, &metadataJSON); err != nil {
			return nil, errors.Wrap(err, "scan minitrace block")
		}
		block.Payload = parseJSONMap(payloadJSON)
		block.Metadata = parseJSONMap(metadataJSON)
		blocks = append(blocks, block)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate minitrace blocks")
	}
	return blocks, nil
}

func buildMinitraceSession(convID string, sourcePath string, exportedAt string, snapshots []minitraceCanonicalSnapshot) map[string]any {
	turns := []map[string]any{}
	toolCalls := []map[string]any{}
	annotations := []map[string]any{}
	timestamps := []time.Time{}
	modelsSeen := []string{}
	previous := []minitraceBlock{}

	for _, snapshot := range snapshots {
		timestamp := formatMillis(snapshot.TurnCreatedAtMS)
		if timestamp != "" {
			if ts, err := time.Parse(time.RFC3339, timestamp); err == nil {
				timestamps = append(timestamps, ts)
			}
		}
		if snapshot.RuntimeKey != "" {
			modelsSeen = append(modelsSeen, snapshot.RuntimeKey)
		}
		delta := minitraceDelta(previous, snapshot.Blocks)
		previous = snapshot.Blocks

		reasoning := []string{}
		startTurnIndex := len(turns)
		for _, block := range delta {
			text := stringifyMinitracePayload(block.Payload)
			if block.Kind == "reasoning" {
				if strings.TrimSpace(text) != "" {
					reasoning = append(reasoning, text)
				}
				continue
			}
			role, source, inputChannel := classifyMinitraceBlock(block)
			if role == "" {
				continue
			}
			turn := map[string]any{
				"index":              len(turns),
				"timestamp":          nilIfEmpty(timestamp),
				"role":               role,
				"source":             nilIfEmpty(source),
				"model":              nilIfEmpty(snapshot.RuntimeKey),
				"content_type":       nil,
				"input_channel":      nilIfEmpty(inputChannel),
				"content":            text,
				"framework_metadata": map[string]any{"pinocchio_kind": block.Kind, "original_role": block.Role, "turn_id": snapshot.TurnID, "phase": snapshot.Phase, "session_id": snapshot.SessionID},
				"tool_calls_in_turn": []string{},
				"thinking":           nil,
				"intent_markers":     nil,
				"streaming":          map[string]any{"was_streamed": false, "stream_log": nil},
				"usage":              nil,
			}
			if len(reasoning) > 0 && role == "assistant" {
				turn["thinking"] = strings.Join(reasoning, "\n\n")
				reasoning = nil
			}
			turns = append(turns, turn)
		}
		emittingTurn := -1
		if len(turns) > 0 && len(turns)-1 >= startTurnIndex {
			emittingTurn = len(turns) - 1
		}
		deltaTools, deltaAnnotations := buildMinitraceToolCalls(delta, timestamp, emittingTurn)
		toolCalls = append(toolCalls, deltaTools...)
		annotations = append(annotations, deltaAnnotations...)
	}

	startedAt, endedAt, duration := minitraceTiming(timestamps)
	model := ""
	if len(modelsSeen) > 0 {
		model = modelsSeen[len(modelsSeen)-1]
	}
	quality := "C"
	if len(turns) > 0 {
		quality = "B"
	}
	if len(turns) > 5 && len(toolCalls) > 10 {
		quality = "A"
	}

	return map[string]any{
		"id":                  convID,
		"schema_version":      minitraceSchemaVersion,
		"profile":             "organic",
		"scenario_id":         nil,
		"quality":             quality,
		"title":               minitraceTitle(turns),
		"summary":             nil,
		"classification":      "internal",
		"provenance":          map[string]any{"source_format": minitraceSourceFormat, "source_path": sourcePath, "converted_at": exportedAt, "converter_version": minitraceConverterVersion, "original_session_id": convID},
		"flags":               map[string]any{"for_research": false, "needs_cleaning": true, "contains_error": false, "contains_pii": strings.Contains(sourcePath, "/home/") || strings.Contains(sourcePath, "/Users/"), "category": []string{}},
		"environment":         map[string]any{"model": nilIfEmpty(model), "model_version": nil, "temperature": nil, "tools_enabled": minitraceToolNames(toolCalls), "system_prompt": nil, "agent_framework": "pinocchio", "agent_version": nil, "platform_type": "agent", "provider_hint": providerHint(model)},
		"operational_context": map[string]any{"working_directory": nil, "git_branch": nil, "git_ref": nil, "autonomy_level": nil, "sandbox": nil, "framework_config": nil},
		"timing":              map[string]any{"privacy_level": "full", "duration_seconds": duration, "active_duration_seconds": duration, "started_at": startedAt, "ended_at": endedAt, "hour_of_day": hourOfDay(startedAt), "day_of_week": dayOfWeek(startedAt)},
		"condition":           nil,
		"coordination":        map[string]any{"project_id": nil, "predecessor_session": nil, "concurrent_sessions": nil, "human_attention": "unknown"},
		"handover":            map[string]any{"received": nil, "produced": nil},
		"turns":               turns,
		"tool_calls":          toolCalls,
		"outcome":             nil,
		"annotations":         annotations,
		"metrics":             minitraceMetrics(turns, toolCalls, duration),
	}
}

func minitraceDelta(previous, current []minitraceBlock) []minitraceBlock {
	seen := map[string]int{}
	for _, block := range previous {
		seen[minitraceBlockFingerprint(block)]++
	}
	out := []minitraceBlock{}
	for _, block := range current {
		fp := minitraceBlockFingerprint(block)
		if seen[fp] > 0 {
			seen[fp]--
			continue
		}
		out = append(out, block)
	}
	return out
}

func minitraceBlockFingerprint(block minitraceBlock) string {
	payload, _ := json.Marshal(block.Payload)
	metadata, _ := json.Marshal(block.Metadata)
	return fmt.Sprintf("%s|%s|%s|%s|%s", block.Kind, block.Role, block.ID, payload, metadata)
}

func classifyMinitraceBlock(block minitraceBlock) (string, string, string) {
	switch block.Kind {
	case "llm_text":
		return "assistant", "model", ""
	case "system":
		return "system", "system", "system_prompt"
	case "user":
		return "user", "human", "user_input"
	default:
		return "", "", ""
	}
}

func buildMinitraceToolCalls(blocks []minitraceBlock, timestamp string, emittingTurn int) ([]map[string]any, []map[string]any) {
	toolCalls := []map[string]any{}
	annotations := []map[string]any{}
	pending := map[string]int{}
	for _, block := range blocks {
		switch block.Kind {
		case "tool_call":
			id := firstNonEmpty(stringValue(block.Payload[minitracePayloadKeyID]), fmt.Sprintf("tool-call-%d", len(toolCalls)))
			turnIndex := any(nil)
			if emittingTurn >= 0 {
				turnIndex = emittingTurn
			}
			call := map[string]any{
				"id":                  id,
				"emitting_turn_index": turnIndex,
				"timestamp":           nilIfEmpty(timestamp),
				"tool_name":           firstNonEmpty(stringValue(block.Payload[minitracePayloadKeyName]), "unknown"),
				"operation_type":      "EXECUTE",
				"input":               map[string]any{"file_path": nil, "command": nil, "justification": nil, "arguments": block.Payload[minitracePayloadKeyArgs]},
				"output":              map[string]any{"success": false, "result": nil, "error": nil, "exit_code": nil, "duration_ms": nil, "truncated": false, "full_bytes": nil, "full_hash": nil, "full_reference": nil, "redacted": nil, "content_origin": "local_exec"},
				"context":             map[string]any{"position_in_session": nil, "tools_before": []string{}, "time_since_last_user": nil},
				"framework_metadata":  map[string]any{"pinocchio_kind": block.Kind, "payload": block.Payload},
				"spawned_agent":       nil,
			}
			pending[id] = len(toolCalls)
			toolCalls = append(toolCalls, call)
		case "tool_use":
			id := stringValue(block.Payload[minitracePayloadKeyID])
			idx, ok := pending[id]
			if !ok {
				annotations = append(annotations, minitraceAnnotation(fmt.Sprintf("ann-tool-use-orphan-%d", len(annotations)), "session", "session", "observation", "Orphan tool_use block", stringifyAny(block.Payload), []string{"pinocchio", "tool_use", "orphan"}))
				continue
			}
			output := toolCalls[idx]["output"].(map[string]any)
			errorText := stringValue(block.Payload[minitracePayloadKeyError])
			result, fullBytes, fullHash := truncateMinitraceContent(stringifyAny(block.Payload[minitracePayloadKeyResult]))
			output["success"] = strings.TrimSpace(errorText) == ""
			output["result"] = result
			output["truncated"] = fullBytes != nil
			output["full_bytes"] = fullBytes
			output["full_hash"] = fullHash
			if strings.TrimSpace(errorText) != "" {
				output["error"] = errorText
			}
			delete(pending, id)
		}
	}
	for id, idx := range pending {
		toolCalls[idx]["output"].(map[string]any)["error"] = "no tool result received"
		annotations = append(annotations, minitraceAnnotation(fmt.Sprintf("ann-tool-call-pending-%d", len(annotations)), "tool_call", id, "observation", "Tool call never received result", id, []string{"pinocchio", "tool_call", "orphan"}))
	}
	return toolCalls, annotations
}

func minitraceAnnotation(id, scopeType, targetID, category, title, detail string, tags []string) map[string]any {
	return map[string]any{"id": id, "timestamp": time.Now().UTC().Format(time.RFC3339), "annotator": "adapter", "scope": map[string]any{"type": scopeType, "target_id": targetID}, "content": map[string]any{"category": category, "tags": tags, "title": title, "detail": detail}, "taxonomy_mappings": map[string]any{"minitrace": []string{}, "mast": []string{}, "toolemu": []string{}}, "classification": nil}
}

func minitraceTiming(timestamps []time.Time) (any, any, any) {
	if len(timestamps) == 0 {
		return nil, nil, nil
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i].Before(timestamps[j]) })
	start := timestamps[0].UTC().Format(time.RFC3339)
	end := timestamps[len(timestamps)-1].UTC().Format(time.RFC3339)
	duration := timestamps[len(timestamps)-1].Sub(timestamps[0]).Seconds()
	return start, end, duration
}

func minitraceMetrics(turns []map[string]any, toolCalls []map[string]any, duration any) map[string]any {
	return map[string]any{"turn_count": len(turns), "tool_call_count": len(toolCalls), "read_count": 0, "modify_count": 0, "create_count": 0, "execute_count": len(toolCalls), "delegate_count": 0, "read_ratio": nil, "time_to_first_action": nil, "idle_ratio": nil, "total_input_tokens": nil, "total_output_tokens": nil, "total_cache_read_tokens": nil, "total_cache_creation_tokens": nil, "total_reasoning_tokens": nil, "total_tool_tokens": nil, "session_cost": nil, "subagent_count": 0, "subagent_tool_calls": 0, "duration_seconds": duration}
}

func minitraceToolNames(toolCalls []map[string]any) []string {
	seen := map[string]struct{}{}
	for _, call := range toolCalls {
		if name := stringValue(call["tool_name"]); name != "" {
			seen[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func minitraceTitle(turns []map[string]any) any {
	for _, turn := range turns {
		if turn["role"] != "user" {
			continue
		}
		text := strings.TrimSpace(stringValue(turn["content"]))
		if text == "" {
			continue
		}
		if len(text) > 80 {
			return text[:77] + "..."
		}
		return text
	}
	return nil
}

func providerHint(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(model, "gpt-"):
		return "openai"
	case strings.HasPrefix(model, "cerebras-"):
		return "cerebras"
	default:
		return "unknown"
	}
}

func hourOfDay(value any) any {
	text, ok := value.(string)
	if !ok || text == "" {
		return nil
	}
	ts, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return nil
	}
	return ts.Hour()
}

func dayOfWeek(value any) any {
	text, ok := value.(string)
	if !ok || text == "" {
		return nil
	}
	ts, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return nil
	}
	weekday := int(ts.Weekday()) - 1
	if weekday < 0 {
		weekday = 6
	}
	return weekday
}

func truncateMinitraceContent(value string) (any, *int, *string) {
	if len(value) <= minitraceTruncateLimit {
		return value, nil, nil
	}
	fullBytes := len(value)
	sum := sha256.Sum256([]byte(value))
	hash := hex.EncodeToString(sum[:])
	truncated := value[:minitraceTruncateLimit]
	return truncated, &fullBytes, &hash
}

func parseJSONMap(raw string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func stringifyMinitracePayload(payload map[string]any) string {
	if text := stringValue(payload[minitracePayloadKeyText]); strings.TrimSpace(text) != "" {
		return text
	}
	return stringifyAny(payload)
}

func stringifyAny(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(body)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nilIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
