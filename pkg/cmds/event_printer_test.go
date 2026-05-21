package cmds

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	"github.com/stretchr/testify/require"
)

func TestPinocchioStepPrinterFormatsReasoningSummaryAsThinking(t *testing.T) {
	var out bytes.Buffer
	printer := pinocchioStepPrinterFunc("", &out)
	meta := gepevents.EventMetadata{SessionID: "sid"}

	require.NoError(t, printer(eventMessage(t, gepevents.NewInfoEvent(meta, "reasoning-summary-started", map[string]any{"item_id": "item"}))))
	require.NoError(t, printer(eventMessage(t, gepevents.NewReasoningDeltaEvent(meta, gepevents.Correlation{SessionID: "sid", SegmentID: "reasoning"}, "thinking text", "thinking text", 1))))
	require.NoError(t, printer(eventMessage(t, gepevents.NewInfoEvent(meta, "reasoning-summary-ended", map[string]any{"item_id": "item"}))))
	require.NoError(t, printer(eventMessage(t, gepevents.NewInfoEvent(meta, "reasoning-summary", map[string]any{"text": "duplicate aggregate"}))))

	got := out.String()
	require.Contains(t, got, "--- Thinking started ---")
	require.Contains(t, got, "thinking text")
	require.Contains(t, got, "--- Thinking ended ---")
	require.NotContains(t, got, "item_id")
	require.NotContains(t, got, "duplicate aggregate")
	require.NotContains(t, got, "[i] reasoning-summary")
}

func TestShouldUsePrettyTextPrinterForDefaultText(t *testing.T) {
	require.True(t, shouldUsePrettyTextPrinter(nil))
	require.True(t, shouldUsePrettyTextPrinter(&run.UISettings{Output: "text"}))
	require.False(t, shouldUsePrettyTextPrinter(&run.UISettings{Output: "text", WithMetadata: true}))
}

func eventMessage(t *testing.T, ev gepevents.Event) *message.Message {
	t.Helper()
	payload, err := json.Marshal(ev)
	require.NoError(t, err)
	return message.NewMessage("test", payload)
}
