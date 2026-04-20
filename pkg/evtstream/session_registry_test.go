package evtstream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionRegistryGetOrCreateUsesFactoryOnce(t *testing.T) {
	calls := 0
	r := newSessionRegistry(func(_ context.Context, sid SessionId) (any, error) {
		calls++
		return map[string]any{"sessionId": string(sid)}, nil
	})

	first, err := r.GetOrCreate(context.Background(), "s-1")
	require.NoError(t, err)
	second, err := r.GetOrCreate(context.Background(), "s-1")
	require.NoError(t, err)

	require.Same(t, first, second)
	require.Equal(t, 1, calls)
	require.Equal(t, map[string]any{"sessionId": "s-1"}, first.Metadata)
}

func TestSessionRegistryRejectsEmptySessionID(t *testing.T) {
	r := newSessionRegistry(nil)
	_, err := r.GetOrCreate(context.Background(), "")
	require.Error(t, err)
}
