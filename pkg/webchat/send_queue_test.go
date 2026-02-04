package webchat

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConversationPrepareRun_Idempotent(t *testing.T) {
	conv := &Conversation{
		ID:    "c1",
		RunID: "r1",
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

	prep, err := conv.PrepareRun("k1", "default", nil, "hi")
	require.NoError(t, err)
	require.False(t, prep.Start)
	require.Equal(t, http.StatusAccepted, prep.HTTPStatus)
	require.Equal(t, "queued", prep.Response["status"])
}

func TestConversationPrepareRun_QueuesWhenBusy(t *testing.T) {
	conv := &Conversation{ID: "c1", RunID: "r1", runningKey: "busy"}

	prep, err := conv.PrepareRun("k2", "default", map[string]any{"x": "y"}, "hi")
	require.NoError(t, err)
	require.False(t, prep.Start)
	require.Equal(t, http.StatusAccepted, prep.HTTPStatus)
	require.Equal(t, "queued", prep.Response["status"])
	require.Equal(t, 1, prep.Response["queue_position"])
	require.Len(t, conv.queue, 1)
}

func TestConversationPrepareRun_StartsWhenIdle(t *testing.T) {
	conv := &Conversation{ID: "c1", RunID: "r1"}

	prep, err := conv.PrepareRun("k3", "default", nil, "hi")
	require.NoError(t, err)
	require.True(t, prep.Start)
	require.Equal(t, "k3", conv.runningKey)
	require.NotNil(t, conv.requests["k3"])
	require.Equal(t, "running", conv.requests["k3"].Status)
}

func TestConversationClaimNextQueued(t *testing.T) {
	conv := &Conversation{
		ID:    "c1",
		RunID: "r1",
		queue: []queuedChat{{IdempotencyKey: "k1"}},
		requests: map[string]*chatRequestRecord{
			"k1": {IdempotencyKey: "k1", Status: "queued"},
		},
	}

	q, ok := conv.ClaimNextQueued()
	require.True(t, ok)
	require.Equal(t, "k1", q.IdempotencyKey)
	require.Equal(t, "k1", conv.runningKey)
	require.Equal(t, "running", conv.requests["k1"].Status)
	require.False(t, conv.requests["k1"].StartedAt.IsZero())
}
