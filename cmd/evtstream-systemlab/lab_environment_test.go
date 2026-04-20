package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLabEnvironmentRunAndExportPhase1(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	resp, err := env.RunPhase1(context.Background(), phase1RunRequest{
		SessionID:   "lab-session-test",
		CommandName: phase1CommandName,
		Prompt:      "hello from systemlab",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["sessionExists"])
	require.True(t, resp.Checks["cursorAdvanced"])
	require.Len(t, resp.UIEvents, 4)

	filename, contentType, body, err := env.ExportPhase1("lab-session-test", "markdown")
	require.NoError(t, err)
	require.Equal(t, "phase1-transcript-lab-session-test.md", filename)
	require.Equal(t, "text/markdown; charset=utf-8", contentType)
	require.Contains(t, string(body), "# Phase 1 Transcript")
	require.Contains(t, string(body), "hello from systemlab")
}
