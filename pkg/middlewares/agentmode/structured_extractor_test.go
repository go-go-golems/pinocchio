package agentmode

import (
	"context"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/stretchr/testify/require"
)

func TestModeSwitchExtractorOnRaw_EmitsAnalysisPreview(t *testing.T) {
	extractor := NewModeSwitchExtractor(DefaultExtractorConfig())
	session := extractor.NewSession(context.Background(), gepevents.EventMetadata{SessionID: "sess-1"}, "item-1")

	events := session.OnRaw(context.Background(), []byte("```yaml\nmode_switch:\n  analysis: hello\n"))
	require.Len(t, events, 1)

	preview, ok := events[0].(*EventModeSwitchPreview)
	require.True(t, ok)
	require.Equal(t, "item-1", preview.ItemID)
	require.Equal(t, "", preview.CandidateMode)
	require.Equal(t, "hello", preview.Analysis)
	require.Equal(t, "analysis-only", preview.ParseState)
}

func TestModeSwitchExtractorOnRaw_EmitsCandidatePreviewWhenModeAppears(t *testing.T) {
	extractor := NewModeSwitchExtractor(DefaultExtractorConfig())
	session := extractor.NewSession(context.Background(), gepevents.EventMetadata{SessionID: "sess-2"}, "item-2")

	analysisOnly := session.OnRaw(context.Background(), []byte("```yaml\nmode_switch:\n  analysis: hello\n"))
	require.Len(t, analysisOnly, 1)

	candidate := session.OnRaw(context.Background(), []byte("  new_mode: reviewer\n"))
	require.Len(t, candidate, 1)

	preview, ok := candidate[0].(*EventModeSwitchPreview)
	require.True(t, ok)
	require.Equal(t, "reviewer", preview.CandidateMode)
	require.Equal(t, "hello", preview.Analysis)
	require.Equal(t, "candidate", preview.ParseState)
}

func TestModeSwitchExtractorOnRaw_DeduplicatesEquivalentPreview(t *testing.T) {
	extractor := NewModeSwitchExtractor(DefaultExtractorConfig())
	session := extractor.NewSession(context.Background(), gepevents.EventMetadata{SessionID: "sess-3"}, "item-3")

	first := session.OnRaw(context.Background(), []byte("```yaml\nmode_switch:\n  analysis: hello\n"))
	require.Len(t, first, 1)

	second := session.OnRaw(context.Background(), []byte("\n"))
	require.Nil(t, second)
}
