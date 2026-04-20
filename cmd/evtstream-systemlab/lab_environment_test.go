package main

import (
	"context"
	"testing"
	"time"

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

func TestLabEnvironmentRunAndExportPhase2(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	resp, err := env.RunPhase2(context.Background(), phase2RunRequest{
		Action:     "publish-a",
		SessionA:   "s-a",
		SessionB:   "s-b",
		BurstCount: 3,
		StreamMode: "derived",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["publishOrdinalZero"])
	require.True(t, resp.Checks["monotonicPerSession"])
	require.Len(t, resp.MessageHistory, 1)
	require.NotEmpty(t, resp.PerSessionOrdinals["s-a"])

	resp, err = env.RunPhase2(context.Background(), phase2RunRequest{
		Action:     "burst-a",
		SessionA:   "s-a",
		SessionB:   "s-b",
		BurstCount: 3,
		StreamMode: "missing",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["monotonicPerSession"])
	require.Len(t, resp.PerSessionOrdinals["s-a"], 4)

	filename, contentType, body, err := env.ExportPhase2("markdown")
	require.NoError(t, err)
	require.Equal(t, "phase2-transcript.md", filename)
	require.Equal(t, "text/markdown; charset=utf-8", contentType)
	require.Contains(t, string(body), "# Phase 2 Transcript")
	require.Contains(t, string(body), "burst-a")
}

func TestLabEnvironmentRunPhase3(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	resp, err := env.RunPhase3(context.Background(), phase3RunRequest{
		Action:    "seed-session",
		SessionID: "reconnect-demo",
		Prompt:    "watch reconnect preserve a coherent snapshot",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["snapshotBeforeLive"])
	require.True(t, resp.Checks["sessionHydrated"])
	require.Equal(t, "4", resp.Snapshot["ordinal"])
}

func TestLabEnvironmentRunPhase4(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	_, err = env.RunPhase4(context.Background(), phase4RunRequest{
		Action:    "send",
		SessionID: "chat-demo",
		Prompt:    "Explain ordinals in plain language",
	})
	require.NoError(t, err)

	time.Sleep(80 * time.Millisecond)
	resp, err := env.RunPhase4(context.Background(), phase4RunRequest{
		Action:    "await-idle",
		SessionID: "chat-demo",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["hasChatEntity"])
	require.True(t, resp.Checks["timelineMatchesUI"])
}

func TestLabEnvironmentRunPhase5SQLRestart(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	_, err = env.RunPhase5(context.Background(), phase5RunRequest{
		Action:    "seed-session",
		Mode:      "sql",
		SessionID: "persist-demo",
		Text:      "persist this record",
	})
	require.NoError(t, err)

	resp, err := env.RunPhase5(context.Background(), phase5RunRequest{
		Action:    "restart-backend",
		Mode:      "sql",
		SessionID: "persist-demo",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["cursorPreserved"])
	require.True(t, resp.Checks["entitiesPreserved"])

	resp, err = env.RunPhase5(context.Background(), phase5RunRequest{
		Action:    "seed-session",
		Mode:      "sql",
		SessionID: "persist-demo",
		Text:      "persist this record again",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["resumeWithoutGaps"])
}
