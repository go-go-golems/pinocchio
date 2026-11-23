package scripted

import (
	"context"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/stretchr/testify/require"
)

func lastText(t *turns.Turn) string {
	b := t.Blocks[len(t.Blocks)-1]
	s, _ := b.Payload[turns.PayloadKeyText].(string)
	return s
}

func TestThePersonsMessageIsTheScriptEvenWithCallerBlocksAfterIt(t *testing.T) {
	turn := &turns.Turn{}
	turns.AppendBlock(turn, turns.NewUserTextBlock("earlier"))
	turns.AppendBlock(turn, turns.NewAssistantTextBlock("echo: earlier"))
	turns.AppendBlock(turn, turns.NewUserTextBlock("/tool eval {\"code\":\"1\"}"))
	turns.AppendBlock(turn, turns.NewUserTextBlock("[state] the page is open"))
	out, err := (&Engine{}).RunInference(context.Background(), turn)
	require.NoError(t, err)
	last := out.Blocks[len(out.Blocks)-1]
	require.Equal(t, turns.BlockKindToolCall, last.Kind)
	require.Equal(t, "eval", last.Payload[turns.PayloadKeyName])

	turns.AppendBlock(out, turns.NewToolUseBlockWithError(last.Payload[turns.PayloadKeyID].(string), `"done"`, ""))
	out, err = (&Engine{}).RunInference(context.Background(), out)
	require.NoError(t, err)
	require.Equal(t, `eval returned: "done"`, lastText(out))
}

func TestEchoAndRepeat(t *testing.T) {
	turn := &turns.Turn{}
	turns.AppendBlock(turn, turns.NewUserTextBlock("hello"))
	out, err := (&Engine{}).RunInference(context.Background(), turn)
	require.NoError(t, err)
	require.Equal(t, "echo: hello", lastText(out))

	turn = &turns.Turn{}
	turns.AppendBlock(turn, turns.NewUserTextBlock("/tools 2 noop {}"))
	for i := 0; i < 2; i++ {
		turn, err = (&Engine{}).RunInference(context.Background(), turn)
		require.NoError(t, err)
		call := turn.Blocks[len(turn.Blocks)-1]
		require.Equal(t, turns.BlockKindToolCall, call.Kind, "round %d", i)
		turns.AppendBlock(turn, turns.NewToolUseBlockWithError(call.Payload[turns.PayloadKeyID].(string), `"ok"`, ""))
	}
	turn, err = (&Engine{}).RunInference(context.Background(), turn)
	require.NoError(t, err)
	require.Equal(t, `noop returned: "ok"`, lastText(turn))
}
