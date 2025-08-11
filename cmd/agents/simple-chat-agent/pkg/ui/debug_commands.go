package ui

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "sort"
    "strings"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    _ "github.com/mattn/go-sqlite3"

    "github.com/go-go-golems/bobatea/pkg/repl"
    store "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/store"
)

// RegisterDebugCommands wires the /dbg command into the REPL and routes subcommands.
func RegisterDebugCommands(m *repl.Model, s *store.SQLiteStore) {
    m.AddCustomCommand("dbg", func(args []string) tea.Cmd {
        return func() tea.Msg {
            output, err := runDebugCommand(args)
            // Echo full input for history clarity
            input := "/dbg"
            if len(args) > 0 {
                input += " " + strings.Join(args, " ")
            }
            return repl.EvaluationCompleteMsg{Input: input, Output: output, Error: err}
        }
    })
}

// runDebugCommand dispatches to the appropriate inspector.
func runDebugCommand(args []string) (string, error) {
    if len(args) == 0 || args[0] == "help" {
        return helpText(), nil
    }
    switch args[0] {
    case "runs":
        n := parseOptionalInt(args, 1, 10)
        return listRuns(n)
    case "turns":
        runID := argOrEmpty(args, 1)
        n := parseOptionalInt(args, 2, 10)
        return listTurns(runID, n)
    case "last-turn":
        return showLastTurn()
    case "blocks":
        var turnID string
        var phase string
        var head, tail int
        verbose := false
        // parse flags and positional turnID
        for i := 1; i < len(args); i++ {
            a := args[i]
            switch {
            case a == "-v" || a == "--verbose":
                verbose = true
            case strings.HasPrefix(a, "--phase="):
                phase = strings.TrimPrefix(a, "--phase=")
            case a == "--phase" && i+1 < len(args):
                phase = args[i+1]
                i++
            case strings.HasPrefix(a, "--head="):
                fmt.Sscanf(strings.TrimPrefix(a, "--head="), "%d", &head)
            case a == "--head" && i+1 < len(args):
                fmt.Sscanf(args[i+1], "%d", &head)
                i++
            case strings.HasPrefix(a, "--tail="):
                fmt.Sscanf(strings.TrimPrefix(a, "--tail="), "%d", &tail)
            case a == "--tail" && i+1 < len(args):
                fmt.Sscanf(args[i+1], "%d", &tail)
                i++
            default:
                if turnID == "" {
                    turnID = a
                }
            }
        }
        return showBlocks(turnID, phase, verbose, head, tail)
    case "toolcalls":
        turnID := mustArg(args, 1)
        return showToolCalls(turnID)
    case "tools":
        var turnID string
        if len(args) > 1 {
            turnID = args[1]
        }
        return showToolDefinitions(turnID)
    case "events":
        n := parseOptionalInt(args, 1, 20)
        return listEvents(n)
    case "mode":
        turnID := mustArg(args, 1)
        return showMode(turnID)
    case "injected-mode-prompts":
        n := parseOptionalInt(args, 1, 5)
        return listInjectedPrompts(n)
    case "schema":
        return printAppSchema()
    case "prompts":
        return printAppPrompts()
    default:
        return fmt.Sprintf("unknown: /dbg %s", strings.Join(args, " ")), nil
    }
}

// Database helpers (read-only)

func openAppDBRO() (*sql.DB, error) {
    // Read-only connection to the agent snapshot DB in module working dir
    return sql.Open("sqlite3", "file:simple-agent.db?mode=ro&_busy_timeout=5000")
}

func openTxnDBRO() (*sql.DB, error) {
    // Read-only connection to the anonymized transaction DB
    return sql.Open("sqlite3", "file:anonymized-data.db?mode=ro&_busy_timeout=5000")
}

// Output helpers

func helpText() string {
    return strings.TrimSpace(`Debug commands:
  /dbg help                                Show this help
  /dbg runs [N]                            List last N runs (default 10)
  /dbg turns [run_id] [N]                  List last N turns for run (default latest run, N=10)
  /dbg last-turn                           Show last turn id and quick facts
  /dbg blocks [turn_id] [--phase PHASE] [-v] [--head N|--tail N]
                                           List blocks for a turn (defaults to last).
                                           PHASE: pre_middleware, pre_inference, post_inference, post_middleware, post_tools.
                                           -v shows all block metadata; --head/--tail limit which blocks are shown.
  /dbg tools [turn_id]                     Show tool definitions captured for turn (defaults to last turn)
  /dbg toolcalls <turn_id>                 Show tool calls/results for a turn
  /dbg events [N]                          Show last N chat events (default 20)
  /dbg mode <turn_id>                      Show mode alignment for a turn
  /dbg injected-mode-prompts [N]           Show last N injected mode prompts
  /dbg schema                              Print app DB schema (without _prompts)
  /dbg prompts                             Print sample prompts from transaction DB`)
}

func parseOptionalInt(args []string, idx int, def int) int {
    if len(args) <= idx {
        return def
    }
    // very small parser to avoid extra imports
    var n int
    for _, ch := range args[idx] {
        if ch < '0' || ch > '9' {
            return def
        }
    }
    _, _ = fmt.Sscanf(args[idx], "%d", &n)
    if n <= 0 {
        return def
    }
    return n
}

func argOrEmpty(args []string, idx int) string {
    if len(args) <= idx {
        return ""
    }
    return args[idx]
}

func mustArg(args []string, idx int) string {
    if len(args) <= idx {
        return ""
    }
    return args[idx]
}

func truncateString(s string, max int) string {
    if len(s) <= max {
        return s
    }
    if max <= 3 {
        return s[:max]
    }
    return s[:max-3] + "..."
}

func oneLineJSON(s string) string {
    if s == "" {
        return ""
    }
    var v any
    if err := json.Unmarshal([]byte(s), &v); err != nil {
        return truncateString(s, 160)
    }
    b, err := json.Marshal(v)
    if err != nil {
        return truncateString(s, 160)
    }
    return truncateString(string(b), 160)
}

// Command impls

func listRuns(n int) (string, error) {
    db, err := openAppDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    rows, err := db.Query("SELECT id, created_at FROM runs ORDER BY created_at DESC LIMIT ?", n)
    if err != nil {
        return "", err
    }
    defer rows.Close()
    var sb strings.Builder
    count := 0
    for rows.Next() {
        var id, createdAt string
        if err := rows.Scan(&id, &createdAt); err != nil {
            return "", err
        }
        count++
        sb.WriteString(fmt.Sprintf("%s  %s\n", id, createdAt))
    }
    if count == 0 {
        return "No records found", nil
    }
    return strings.TrimRight(sb.String(), "\n"), nil
}

func getLatestRunID(ctx context.Context, db *sql.DB) (string, error) {
    var id string
    err := db.QueryRowContext(ctx, "SELECT id FROM runs ORDER BY created_at DESC LIMIT 1").Scan(&id)
    if err != nil {
        return "", err
    }
    return id, nil
}

func listTurns(runID string, n int) (string, error) {
    db, err := openAppDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    if runID == "" {
        rid, err := getLatestRunID(ctx, db)
        if err != nil {
            return "", err
        }
        runID = rid
    }
    rows, err := db.QueryContext(ctx, "SELECT id, created_at FROM turns WHERE run_id=? ORDER BY created_at DESC LIMIT ?", runID, n)
    if err != nil {
        return "", err
    }
    defer rows.Close()
    var sb strings.Builder
    count := 0
    for rows.Next() {
        var id, createdAt string
        if err := rows.Scan(&id, &createdAt); err != nil {
            return "", err
        }
        count++
        sb.WriteString(fmt.Sprintf("%s  %s\n", id, createdAt))
    }
    if count == 0 {
        return "No records found", nil
    }
    return strings.TrimRight(sb.String(), "\n"), nil
}

func showLastTurn() (string, error) {
    db, err := openAppDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    var turnID string
    err = db.QueryRow("SELECT turn_id FROM block_payload_kv ORDER BY rowid DESC LIMIT 1").Scan(&turnID)
    if err != nil {
        if err == sql.ErrNoRows {
            return "No records found", nil
        }
        return "", err
    }
    // quick facts
    var numBlocks int
    _ = db.QueryRow("SELECT COUNT(*) FROM blocks WHERE turn_id=?", turnID).Scan(&numBlocks)
    return fmt.Sprintf("turn_id=%s  blocks=%d", turnID, numBlocks), nil
}

func showBlocks(turnID string, phase string, verbose bool, head, tail int) (string, error) {
    db, err := openAppDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    if turnID == "" {
        var tid string
        if err := db.QueryRow("SELECT turn_id FROM block_payload_kv ORDER BY rowid DESC LIMIT 1").Scan(&tid); err != nil {
            if err == sql.ErrNoRows {
                return "No records found", nil
            }
            return "", err
        }
        turnID = tid
    }
    const q = `WITH b AS (
      SELECT * FROM blocks WHERE turn_id = ? ORDER BY ord
    )
    SELECT b.id, b.ord, b.kind, b.role,
      (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='name' AND (? = '' OR phase = ?) ORDER BY rowid DESC LIMIT 1) AS tool_name,
      (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='id' AND (? = '' OR phase = ?) ORDER BY rowid DESC LIMIT 1) AS tool_id,
      (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='args' AND (? = '' OR phase = ?) ORDER BY rowid DESC LIMIT 1) AS args,
      (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='result' AND (? = '' OR phase = ?) ORDER BY rowid DESC LIMIT 1) AS result,
      (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='text' AND (? = '' OR phase = ?) ORDER BY rowid DESC LIMIT 1) AS text
    FROM b;`
    rows, err := db.Query(q, turnID, phase, phase, phase, phase, phase, phase, phase, phase, phase, phase)
    if err != nil {
        return "", err
    }
    defer rows.Close()
    type blockRow struct {
        id       string
        ord      int
        kind     int
        role     sql.NullString
        toolName sql.NullString
        toolID   sql.NullString
        args     sql.NullString
        result   sql.NullString
        text     sql.NullString
    }
    var all []blockRow
    for rows.Next() {
        var id string
        var ord int
        var kind int
        var role sql.NullString
        var toolName, toolID, args, result, text sql.NullString
        if err := rows.Scan(&id, &ord, &kind, &role, &toolName, &toolID, &args, &result, &text); err != nil {
            return "", err
        }
        all = append(all, blockRow{id: id, ord: ord, kind: kind, role: role, toolName: toolName, toolID: toolID, args: args, result: result, text: text})
    }
    // Apply head/tail slicing
    start, end := 0, len(all)
    if head > 0 {
        if head < end {
            end = head
        }
    } else if tail > 0 {
        if tail < end {
            start = end - tail
        }
    }
    var sb strings.Builder
    count := 0
    for i := start; i < end; i++ {
        r := all[i]
        count++
        // Header with explicit order, kind name, role
        header := fmt.Sprintf("ord=%02d  block=%s  kind=%s  role=%s", r.ord, r.id, blockKindName(r.kind), nullToString(r.role))
        var parts []string
        if r.toolName.Valid || r.toolID.Valid {
            parts = append(parts, "Tool: "+truncateString(r.toolName.String, 60))
            if r.args.Valid && r.args.String != "" {
                parts = append(parts, "Args:\n"+indent(multilineJSON(r.args.String), 2, 10))
            }
            if r.result.Valid && r.result.String != "" {
                parts = append(parts, "Result:\n"+indent(multilineJSON(r.result.String), 2, 10))
            }
        }
        if r.text.Valid && r.text.String != "" {
            parts = append(parts, "Text:\n"+indent(limitLines(r.text.String, 10), 2, 10))
        }
        if verbose {
            // Load metadata for this block
            metaMap := map[string]string{}
            if phase == "" {
                // latest per key
                qmeta := `SELECT key, COALESCE(value_text, value_json) FROM block_metadata_kv 
                          WHERE block_id=? AND turn_id=? AND rowid IN (
                            SELECT MAX(rowid) FROM block_metadata_kv WHERE block_id=? AND turn_id=? GROUP BY key
                          )`
                mrows, err := db.Query(qmeta, r.id, turnID, r.id, turnID)
                if err == nil {
                    for mrows.Next() {
                        var k, v sql.NullString
                        _ = mrows.Scan(&k, &v)
                        if k.Valid {
                            metaMap[k.String] = v.String
                        }
                    }
                    mrows.Close()
                }
            } else {
                qmeta := `SELECT key, COALESCE(value_text, value_json) FROM block_metadata_kv 
                          WHERE block_id=? AND turn_id=? AND phase=?`
                mrows, err := db.Query(qmeta, r.id, turnID, phase)
                if err == nil {
                    for mrows.Next() {
                        var k, v sql.NullString
                        _ = mrows.Scan(&k, &v)
                        if k.Valid {
                            metaMap[k.String] = v.String
                        }
                    }
                    mrows.Close()
                }
            }
            if len(metaMap) > 0 {
                parts = append(parts, "Metadata:")
                keys := make([]string, 0, len(metaMap))
                for k := range metaMap { keys = append(keys, k) }
                sort.Strings(keys)
                for _, k := range keys {
                    parts = append(parts, indent(fmt.Sprintf("%s: %s", k, multilineJSON(metaMap[k])), 2, 0))
                }
            }
        }
        body := strings.Join(parts, "\n")
        if body == "" {
            body = "(no content)"
        }
        sb.WriteString(header)
        sb.WriteString("\n")
        sb.WriteString(body)
        sb.WriteString("\n")
        sb.WriteString("---\n")
    }
    if count == 0 {
        return "No records found", nil
    }
    out := sb.String()
    // Trim trailing separator
    if strings.HasSuffix(out, "---\n") {
        out = strings.TrimSuffix(out, "---\n")
    }
    out = strings.TrimRight(out, "\n")
    return out, nil
}

func blockKindName(kind int) string {
    switch kind {
    case 0:
        return "user"
    case 1:
        return "llm_text"
    case 2:
        return "tool_call"
    case 3:
        return "tool_use"
    case 4:
        return "system"
    default:
        return fmt.Sprintf("other(%d)", kind)
    }
}

func showToolCalls(turnID string) (string, error) {
    db, err := openAppDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    if turnID == "" {
        var tid string
        if err := db.QueryRow("SELECT turn_id FROM block_payload_kv ORDER BY rowid DESC LIMIT 1").Scan(&tid); err != nil {
            if err == sql.ErrNoRows {
                return "No records found", nil
            }
            return "", err
        }
        turnID = tid
    }
    const q = `WITH b AS (
      SELECT * FROM blocks WHERE turn_id = ? ORDER BY ord
    )
    SELECT b.id,
      (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='name' LIMIT 1) AS name,
      (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='id' LIMIT 1) AS id,
      (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='args' LIMIT 1) AS args,
      (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='result' LIMIT 1) AS result
    FROM b
    WHERE (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='name' LIMIT 1) IS NOT NULL
       OR (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='id' LIMIT 1) IS NOT NULL;`
    rows, err := db.Query(q, turnID)
    if err != nil {
        return "", err
    }
    defer rows.Close()
    var sb strings.Builder
    count := 0
    for rows.Next() {
        var blockID string
        var name, id, args, result sql.NullString
        if err := rows.Scan(&blockID, &name, &id, &args, &result); err != nil {
            return "", err
        }
        count++
        sb.WriteString(fmt.Sprintf("%02d  %s  name=%s id=%s\n", count, blockID, truncateString(name.String, 40), truncateString(id.String, 24)))
        if args.Valid {
            sb.WriteString("    args:   ")
            sb.WriteString(oneLineJSON(args.String))
            sb.WriteString("\n")
        }
        if result.Valid {
            sb.WriteString("    result: ")
            sb.WriteString(oneLineJSON(result.String))
            sb.WriteString("\n")
        }
    }
    if count == 0 {
        return "No records found", nil
    }
    return strings.TrimRight(sb.String(), "\n"), nil
}

func listEvents(n int) (string, error) {
    db, err := openAppDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    rows, err := db.Query("SELECT id, created_at, type, message, tool_name, tool_id FROM chat_events ORDER BY id DESC LIMIT ?", n)
    if err != nil {
        return "", err
    }
    defer rows.Close()
    var sb strings.Builder
    count := 0
    for rows.Next() {
        var id int64
        var createdAt, typ, message, toolName, toolID sql.NullString
        if err := rows.Scan(&id, &createdAt, &typ, &message, &toolName, &toolID); err != nil {
            return "", err
        }
        count++
        msg := truncateString(message.String, 120)
        if toolID.Valid || toolName.Valid {
            sb.WriteString(fmt.Sprintf("%d  %s  %-24s  %s#%s  %s\n", id, createdAt.String, typ.String, truncateString(toolName.String, 24), truncateString(toolID.String, 16), msg))
        } else {
            sb.WriteString(fmt.Sprintf("%d  %s  %-24s  %s\n", id, createdAt.String, typ.String, msg))
        }
    }
    if count == 0 {
        return "No records found", nil
    }
    return strings.TrimRight(sb.String(), "\n"), nil
}

// showToolDefinitions prints tool definitions that were available for a given turn.
// We infer definitions by reading provider-advertised blocks or by capturing registry snapshots in KV.
// For now, we derive from block_payload_kv: when provider advertises tools, we usually see tool_call blocks later.
// As a proxy, we list unique tool names used in that turn. If none, we also try to read any kv under turn_kv data section with key 'tool_registry'.
func showToolDefinitions(turnID string) (string, error) {
    db, err := openAppDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    if turnID == "" {
        var tid string
        if err := db.QueryRow("SELECT turn_id FROM block_payload_kv ORDER BY rowid DESC LIMIT 1").Scan(&tid); err != nil {
            if err == sql.ErrNoRows {
                return "No records found", nil
            }
            return "", err
        }
        turnID = tid
    }

    // Prefer system blocks with role='tool_registry' captured by middleware
    const regQ = `WITH b AS (
        SELECT * FROM blocks WHERE turn_id = ? AND role = 'tool_registry' ORDER BY ord
    )
    SELECT 
      COALESCE((SELECT value_text FROM block_metadata_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='toolsnap' LIMIT 1), '') AS phase,
      (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='tools' LIMIT 1) AS tools_json
    FROM b`
    rrows, err := db.Query(regQ, turnID)
    if err != nil {
        return "", err
    }
    defer rrows.Close()
    type snap struct{ Phase string; ToolsJSON string }
    var snaps []snap
    for rrows.Next() {
        var phase, tj sql.NullString
        if err := rrows.Scan(&phase, &tj); err != nil {
            return "", err
        }
        if tj.Valid && tj.String != "" {
            snaps = append(snaps, snap{Phase: phase.String, ToolsJSON: tj.String})
        }
    }
    if len(snaps) > 0 {
        title := lipgloss.NewStyle().Bold(true).Render("Tool registry snapshots for turn " + turnID)
        lab := lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
        box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
        var out []string
        for i, s := range snaps {
            // Render compact list of tools (name + optional version/tags); fall back to pretty JSON
            var tools []map[string]any
            var summary string
            if err := json.Unmarshal([]byte(s.ToolsJSON), &tools); err == nil && len(tools) > 0 {
                var lines []string
                for j, tdef := range tools {
                    name, _ := tdef["name"].(string)
                    version, _ := tdef["version"].(string)
                    tagsAny := tdef["tags"]
                    var tagsStr string
                    if arr, ok := tagsAny.([]any); ok && len(arr) > 0 {
                        var ts []string
                        for _, v := range arr { if s, ok := v.(string); ok && s != "" { ts = append(ts, s) } }
                        if len(ts) > 0 { tagsStr = " ["+strings.Join(ts, ", ")+"]" }
                    }
                    line := fmt.Sprintf("%2d. %s", j+1, name)
                    if version != "" { line += " (" + version + ")" }
                    if tagsStr != "" { line += tagsStr }
                    lines = append(lines, line)
                }
                summary = strings.Join(lines, "\n")
            } else {
                summary = multilineJSON(s.ToolsJSON)
            }
            ph := s.Phase
            if ph == "" { ph = fmt.Sprintf("snap#%d", i+1) }
            out = append(out, box.Render(lab.Render("phase: "+ph)+"\n"+summary))
        }
        return strings.TrimRight(title+"\n"+strings.Join(out, "\n"), "\n"), nil
    }

    // Fallback: list distinct tool names used in tool_call blocks
    const toolsQ = `WITH b AS (
        SELECT * FROM blocks WHERE turn_id = ?
    )
    SELECT DISTINCT (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='name' LIMIT 1) AS name
    FROM b
    WHERE (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='name' LIMIT 1) IS NOT NULL`
    trows, err := db.Query(toolsQ, turnID)
    if err != nil {
        return "", err
    }
    defer trows.Close()
    var names []string
    for trows.Next() {
        var n sql.NullString
        if err := trows.Scan(&n); err != nil {
            return "", err
        }
        if n.Valid && n.String != "" {
            names = append(names, n.String)
        }
    }
    if len(names) == 0 {
        var regJSON sql.NullString
        _ = db.QueryRow("SELECT value_json FROM turn_kv WHERE turn_id=? AND section='data' AND key='tool_registry' LIMIT 1", turnID).Scan(&regJSON)
        if regJSON.Valid && regJSON.String != "" {
            return "tool_registry: \n" + indent(multilineJSON(regJSON.String), 2, 10), nil
        }
        return "No tool definitions observed for this turn", nil
    }
    title := lipgloss.NewStyle().Bold(true).Render("Tools for turn " + turnID)
    list := ""
    for i, n := range names {
        list += fmt.Sprintf("%2d. %s\n", i+1, n)
    }
    return strings.TrimRight(title+"\n"+list, "\n"), nil
}

// Formatting helpers
func indent(s string, spaces int, maxLines int) string {
    pad := strings.Repeat(" ", spaces)
    lines := strings.Split(s, "\n")
    if maxLines > 0 && len(lines) > maxLines {
        lines = lines[:maxLines]
    }
    for i := range lines {
        lines[i] = pad + lines[i]
    }
    return strings.Join(lines, "\n")
}

func multilineJSON(s string) string {
    if s == "" {
        return ""
    }
    var v any
    if err := json.Unmarshal([]byte(s), &v); err != nil {
        return limitLines(s, 10)
    }
    b, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return limitLines(s, 10)
    }
    return string(b)
}

func limitLines(s string, max int) string {
    lines := strings.Split(s, "\n")
    if len(lines) <= max {
        return s
    }
    return strings.Join(lines[:max], "\n")
}

func showMode(turnID string) (string, error) {
    if turnID == "" {
        return "", fmt.Errorf("missing turn_id")
    }
    db, err := openAppDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    rows, err := db.Query("SELECT turn_id, data_mode, injected_mode FROM v_turn_modes WHERE turn_id=?", turnID)
    if err != nil {
        return "", err
    }
    defer rows.Close()
    if rows.Next() {
        var tid, dataMode, injectedMode sql.NullString
        if err := rows.Scan(&tid, &dataMode, &injectedMode); err != nil {
            return "", err
        }
        return fmt.Sprintf("turn_id=%s  data_mode=%s  injected_mode=%s", tid.String, dataMode.String, injectedMode.String), nil
    }
    return "No records found", nil
}

func listInjectedPrompts(n int) (string, error) {
    db, err := openAppDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    rows, err := db.Query("SELECT turn_id, substr(prompt_text,1,160) FROM v_injected_mode_prompts ORDER BY rowid DESC LIMIT ?", n)
    if err != nil {
        return "", err
    }
    defer rows.Close()
    var sb strings.Builder
    count := 0
    for rows.Next() {
        var tid, text sql.NullString
        if err := rows.Scan(&tid, &text); err != nil {
            return "", err
        }
        count++
        sb.WriteString(fmt.Sprintf("%s  %s\n", tid.String, truncateString(text.String, 160)))
    }
    if count == 0 {
        return "No records found", nil
    }
    return strings.TrimRight(sb.String(), "\n"), nil
}

func printAppSchema() (string, error) {
    db, err := openTxnDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    rows, err := db.Query("SELECT sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name != '_prompts' ORDER BY name")
    if err != nil {
        return "", err
    }
    defer rows.Close()
    var sb strings.Builder
    count := 0
    for rows.Next() {
        var sqlText sql.NullString
        if err := rows.Scan(&sqlText); err != nil {
            return "", err
        }
        if sqlText.Valid {
            count++
            line := strings.ReplaceAll(sqlText.String, "\n", " ")
            sb.WriteString(truncateString(line, 160))
            sb.WriteString("\n")
        }
    }
    if count == 0 {
        return "No records found", nil
    }
    return strings.TrimRight(sb.String(), "\n"), nil
}

func printAppPrompts() (string, error) {
    db, err := openTxnDBRO()
    if err != nil {
        return "", err
    }
    defer db.Close()
    rows, err := db.Query("SELECT substr(prompt,1,120) FROM _prompts LIMIT 20")
    if err != nil {
        return "", err
    }
    defer rows.Close()
    var sb strings.Builder
    count := 0
    for rows.Next() {
        var p sql.NullString
        if err := rows.Scan(&p); err != nil {
            return "", err
        }
        count++
        sb.WriteString(p.String)
        sb.WriteString("\n")
    }
    if count == 0 {
        return "No records found", nil
    }
    return strings.TrimRight(sb.String(), "\n"), nil
}

func nullToString(ns sql.NullString) string {
    if ns.Valid {
        return ns.String
    }
    return ""
}


