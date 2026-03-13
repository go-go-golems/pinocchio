package webchat

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

type toolSnapshotEngine struct{}

func (e toolSnapshotEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	out := t.Clone()

	for _, b := range out.Blocks {
		if b.Kind != turns.BlockKindToolUse {
			continue
		}
		if id, _ := b.Payload[turns.PayloadKeyID].(string); id == "call-go-double" {
			turns.AppendBlock(out, turns.NewAssistantTextBlock("go tools done"))
			return out, nil
		}
	}

	turns.AppendBlock(out, turns.NewToolCallBlock("call-go-double", "go_double", map[string]any{"n": 5}))
	return out, nil
}

func TestSnapshotHookForConv_PersistsToolLoopTurnDataAndBlocksToSQLite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "turns.db")
	dsn, err := chatstore.SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)

	store, err := chatstore.NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	rawDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rawDB.Close() })

	reg := geptools.NewInMemoryToolRegistry()
	type goDoubleInput struct {
		N int `json:"n" jsonschema:"required,description=Number to double"`
	}
	goDouble, err := geptools.NewToolFromFunc(
		"go_double",
		"Double a number and return {value}",
		func(in goDoubleInput) (map[string]any, error) {
			return map[string]any{"value": in.N * 2}, nil
		},
	)
	require.NoError(t, err)
	require.NoError(t, reg.RegisterTool("go_double", *goDouble))

	conv := &Conversation{
		ID:         "conv-tool-data-persist",
		SessionID:  "session-tool-data-persist",
		RuntimeKey: "tools@v1",
	}

	initial := &turns.Turn{ID: "turn-tool-data-persist"}
	turns.AppendBlock(initial, turns.NewUserTextBlock("double 5"))
	require.NoError(t, turns.KeyBlockMetaMiddleware.Set(&initial.Blocks[0].Metadata, "snapshot-persist-test"))
	require.NoError(t, turns.KeyTurnMetaInferenceID.Set(&initial.Metadata, "inf-tool-data-persist"))

	loop := toolloop.New(
		toolloop.WithEngine(toolSnapshotEngine{}),
		toolloop.WithRegistry(reg),
		toolloop.WithLoopConfig(toolloop.NewLoopConfig().WithMaxIterations(3)),
		toolloop.WithToolConfig(geptools.DefaultToolConfig().WithMaxIterations(3)),
		toolloop.WithSnapshotHook(snapshotHookForConv(conv, store)),
	)

	out, err := loop.RunLoop(context.Background(), initial)
	require.NoError(t, err)
	require.Equal(t, initial.ID, out.ID)

	var turnDataJSON string
	var runtimeKey string
	var inferenceID string
	require.NoError(t, rawDB.QueryRow(
		`SELECT turn_data_json, runtime_key, inference_id FROM turns WHERE conv_id = ? AND session_id = ? AND turn_id = ?`,
		conv.ID,
		conv.SessionID,
		out.ID,
	).Scan(&turnDataJSON, &runtimeKey, &inferenceID))
	require.Equal(t, conv.RuntimeKey, runtimeKey)
	require.Equal(t, "inf-tool-data-persist", inferenceID)

	var turnData map[string]any
	require.NoError(t, json.Unmarshal([]byte(turnDataJSON), &turnData))

	cfgRaw, ok := turnData[engine.KeyToolConfig.String()]
	require.True(t, ok, "expected persisted %s in turn_data_json", engine.KeyToolConfig.String())
	cfg, ok := cfgRaw.(map[string]any)
	require.True(t, ok, "expected tool config map, got %T", cfgRaw)
	require.Equal(t, true, cfg["enabled"])
	if maxIterations, ok := cfg["max_iterations"]; ok {
		require.Equal(t, float64(3), maxIterations)
	}
	if toolChoice, ok := cfg["tool_choice"]; ok {
		require.Equal(t, string(engine.ToolChoiceAuto), toolChoice)
	}

	defsRaw, ok := turnData[engine.KeyToolDefinitions.String()]
	require.True(t, ok, "expected persisted %s in turn_data_json", engine.KeyToolDefinitions.String())
	defs, ok := defsRaw.([]any)
	require.True(t, ok, "expected tool definitions slice, got %T", defsRaw)
	require.Len(t, defs, 1)

	def0, ok := defs[0].(map[string]any)
	require.True(t, ok, "expected first tool definition map, got %T", defs[0])
	require.Equal(t, "go_double", def0["name"])
	require.Equal(t, "Double a number and return {value}", def0["description"])

	paramsRaw, ok := def0["parameters"]
	require.True(t, ok, "expected persisted tool parameters")
	params, ok := paramsRaw.(map[string]any)
	require.True(t, ok, "expected parameters map, got %T", paramsRaw)
	require.Equal(t, "object", params["type"])
	propsRaw, ok := params["properties"]
	require.True(t, ok, "expected tool schema properties")
	props, ok := propsRaw.(map[string]any)
	require.True(t, ok, "expected schema properties map, got %T", propsRaw)
	_, ok = props["n"]
	require.True(t, ok, "expected schema property for input field n")

	rows, err := rawDB.Query(`
		SELECT b.kind, b.payload_json, b.block_metadata_json
		FROM turn_block_membership m
		JOIN blocks b
		  ON b.block_id = m.block_id
		 AND b.content_hash = m.content_hash
		WHERE m.conv_id = ? AND m.session_id = ? AND m.turn_id = ? AND m.phase = ?
		ORDER BY m.ordinal
	`, conv.ID, conv.SessionID, out.ID, "post_tools")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	foundUserMetadata := false
	foundToolUsePayload := false
	for rows.Next() {
		var kind string
		var payloadJSON string
		var metadataJSON string
		require.NoError(t, rows.Scan(&kind, &payloadJSON, &metadataJSON))

		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))

		var metadata map[string]any
		require.NoError(t, json.Unmarshal([]byte(metadataJSON), &metadata))

		switch kind {
		case turns.BlockKindUser.String():
			if metadata[turns.KeyBlockMetaMiddleware.String()] == "snapshot-persist-test" {
				foundUserMetadata = true
			}
		case turns.BlockKindToolUse.String():
			if payload[turns.PayloadKeyID] != "call-go-double" {
				continue
			}
			resultText, _ := payload[turns.PayloadKeyResult].(string)
			if strings.Contains(resultText, "10") {
				foundToolUsePayload = true
			}
		}
	}
	require.NoError(t, rows.Err())
	require.True(t, foundUserMetadata, "expected user block metadata to persist into block_metadata_json")
	require.True(t, foundToolUsePayload, "expected tool_use payload to persist into payload_json")
}
