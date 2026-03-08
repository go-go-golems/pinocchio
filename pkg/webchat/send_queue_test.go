package webchat

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreparePromptSubmission_Idempotent(t *testing.T) {
	conv := &Conversation{
		ID:        "c1",
		SessionID: "s1",
		requests: map[string]*chatRequestRecord{
			"k1": {
				IdempotencyKey: "k1",
				Status:         "queued",
				Response: map[string]any{
					"status":          "queued",
					"idempotency_key": "k1",
				},
			},
		},
	}

	prep, err := preparePromptSubmission(conv, "k1", nil, nil)
	require.NoError(t, err)
	require.False(t, prep.Start)
	require.Equal(t, http.StatusAccepted, prep.HTTPStatus)
	require.Equal(t, "queued", prep.Response["status"])
}

func TestPreparePromptSubmission_QueuesWhenBusy(t *testing.T) {
	conv := &Conversation{ID: "c1", SessionID: "s1", activeRequestKey: "busy"}

	prep, err := preparePromptSubmission(conv, "k2", map[string]any{"prompt": "hi"}, map[string]any{"x": "y"})
	require.NoError(t, err)
	require.False(t, prep.Start)
	require.Equal(t, http.StatusAccepted, prep.HTTPStatus)
	require.Equal(t, "queued", prep.Response["status"])
	require.Equal(t, 1, prep.Response["queue_position"])
	require.Len(t, conv.queue, 1)
}

func TestPreparePromptSubmission_StartsWhenIdle(t *testing.T) {
	conv := &Conversation{ID: "c1", SessionID: "s1"}

	prep, err := preparePromptSubmission(conv, "k3", nil, nil)
	require.NoError(t, err)
	require.True(t, prep.Start)
	require.Equal(t, "k3", conv.activeRequestKey)
	require.NotNil(t, conv.requests["k3"])
	require.Equal(t, "running", conv.requests["k3"].Status)
}

func TestClaimNextQueuedPrompt(t *testing.T) {
	conv := &Conversation{
		ID:        "c1",
		SessionID: "s1",
		queue:     []queuedChat{{IdempotencyKey: "k1"}},
		requests: map[string]*chatRequestRecord{
			"k1": {IdempotencyKey: "k1", Status: "queued"},
		},
	}

	q, ok := claimNextQueuedPrompt(conv)
	require.True(t, ok)
	require.Equal(t, "k1", q.IdempotencyKey)
	require.Equal(t, "k1", conv.activeRequestKey)
	require.Equal(t, "running", conv.requests["k1"].Status)
	require.False(t, conv.requests["k1"].StartedAt.IsZero())
}
