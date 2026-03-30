package agentmode

import (
	"context"
	"testing"

	rootmw "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/stretchr/testify/require"
)

func TestBuildModeSwitchInstructions_UsesStructuredTag(t *testing.T) {
	instructions := BuildModeSwitchInstructions("analyst", []string{"analyst", "reviewer"})

	require.Contains(t, instructions, modeSwitchOpenTag)
	require.Contains(t, instructions, modeSwitchCloseTag)
	require.Contains(t, instructions, "```yaml")
	require.Contains(t, instructions, "Current mode: analyst")
	require.Contains(t, instructions, "Available modes: analyst, reviewer")
}

func TestParseModeSwitchPayload_SanitizesByDefault(t *testing.T) {
	raw := []byte("```yaml\nmode_switch:\n  analysis:test switch\n  new_mode:reviewer\n```")

	parsed, err := ParseModeSwitchPayload(raw, ParseOptions{})
	require.NoError(t, err)
	require.Equal(t, "test switch", parsed.Analysis)
	require.Equal(t, "reviewer", parsed.NewMode)
	require.NotEqual(t, parsed.RawYAML, parsed.ParsedYAML)
	require.True(t, parsed.Sanitized)
}

func TestParseModeSwitchPayload_CanDisableSanitize(t *testing.T) {
	raw := []byte("```yaml\nmode_switch:\n  analysis:test switch\n  new_mode:reviewer\n```")

	parsed, err := ParseModeSwitchPayload(raw, DefaultParseOptions().WithSanitizeYAML(false))
	require.Error(t, err)
	require.Nil(t, parsed)
}

func TestDetectModeSwitchInBlocks_UsesStructuredPayload(t *testing.T) {
	blocks := []turns.Block{
		turns.NewAssistantTextBlock(
			"before\n" +
				modeSwitchOpenTag + "\n```yaml\nmode_switch:\n  analysis:test switch\n  new_mode:reviewer\n```\n" +
				modeSwitchCloseTag +
				"\nafter",
		),
	}

	parsed, ok := DetectModeSwitchInBlocks(blocks, ParseOptions{})
	require.True(t, ok)
	require.Equal(t, "test switch", parsed.Analysis)
	require.Equal(t, "reviewer", parsed.NewMode)
}

func TestNewMiddleware_AppliesStructuredModeSwitch(t *testing.T) {
	svc := NewStaticService([]*AgentMode{
		{Name: "analyst", Prompt: "Analyze things"},
		{Name: "reviewer", Prompt: "Review things"},
	})
	cfg := DefaultConfig()
	mw := NewMiddleware(svc, cfg)

	turn := &turns.Turn{ID: "turn-1"}
	require.NoError(t, turns.KeyTurnMetaSessionID.Set(&turn.Metadata, "sess-1"))
	require.NoError(t, turns.KeyAgentMode.Set(&turn.Data, "analyst"))
	turns.AppendBlock(turn, turns.NewUserTextBlock("please review"))

	handler := mw(func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
		res := t.Clone()
		turns.AppendBlock(res, turns.NewAssistantTextBlock(
			"I should switch.\n"+
				modeSwitchOpenTag+"\n```yaml\nmode_switch:\n  analysis:test switch\n  new_mode:reviewer\n```\n"+
				modeSwitchCloseTag,
		))
		return res, nil
	})

	res, err := handler(context.Background(), turn)
	require.NoError(t, err)

	mode, ok, err := turns.KeyAgentMode.Get(res.Data)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "reviewer", mode)
	require.Equal(t, "reviewer", svc.current["sess-1"])

	last := res.Blocks[len(res.Blocks)-1]
	text, _ := last.Payload[turns.PayloadKeyText].(string)
	require.Equal(t, "[agent-mode] switched to reviewer", text)
}

func TestNewMiddleware_RespectsSanitizeDisable(t *testing.T) {
	svc := NewStaticService([]*AgentMode{
		{Name: "analyst", Prompt: "Analyze things"},
		{Name: "reviewer", Prompt: "Review things"},
	})
	cfg := DefaultConfig()
	cfg.ParseOptions = cfg.ParseOptions.WithSanitizeYAML(false)
	mw := NewMiddleware(svc, cfg)

	turn := &turns.Turn{ID: "turn-1"}
	require.NoError(t, turns.KeyTurnMetaSessionID.Set(&turn.Metadata, "sess-1"))
	require.NoError(t, turns.KeyAgentMode.Set(&turn.Data, "analyst"))
	turns.AppendBlock(turn, turns.NewUserTextBlock("please review"))

	handler := mw(func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
		res := t.Clone()
		turns.AppendBlock(res, turns.NewAssistantTextBlock(
			modeSwitchOpenTag+"\n```yaml\nmode_switch:\n  analysis:test switch\n  new_mode:reviewer\n```\n"+modeSwitchCloseTag,
		))
		return res, nil
	})

	res, err := handler(context.Background(), turn)
	require.NoError(t, err)

	mode, ok, err := turns.KeyAgentMode.Get(res.Data)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "analyst", mode)
	_, switched := svc.current["sess-1"]
	require.False(t, switched)

	last := res.Blocks[len(res.Blocks)-1]
	text, _ := last.Payload[turns.PayloadKeyText].(string)
	require.NotEqual(t, "[agent-mode] switched to reviewer", text)
}

func TestDeprecatedDetectYamlModeSwitch_WrapsStructuredDetection(t *testing.T) {
	turn := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn, turns.NewAssistantTextBlock(
		modeSwitchOpenTag+"\n```yaml\nmode_switch:\n  analysis:test switch\n  new_mode:reviewer\n```\n"+modeSwitchCloseTag,
	))

	newMode, analysis := DetectYamlModeSwitch(turn)
	require.Equal(t, "reviewer", newMode)
	require.Equal(t, "test switch", analysis)
}

func TestMiddlewareDiffUsesNewBlockIDs(t *testing.T) {
	turn := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn, turns.NewUserTextBlock("hello"))

	baseline := rootmw.SnapshotBlockIDs(turn)
	turns.AppendBlock(turn, turns.NewAssistantTextBlock("world"))

	added := rootmw.NewBlocksNotIn(turn, baseline)
	require.Len(t, added, 1)
	text, _ := added[0].Payload[turns.PayloadKeyText].(string)
	require.Equal(t, "world", text)
}
