package evtstream

import (
	"context"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/require"
)

func TestOrdinalAssignerUsesStreamIDWhenAvailable(t *testing.T) {
	assigner := NewOrdinalAssigner(func(context.Context, SessionId) (uint64, error) { return 0, nil })
	metadata := message.Metadata{}
	metadata.Set(MetadataKeyStreamID, "1700000000000-2")

	ord, err := assigner.Next(context.Background(), "s-1", metadata)
	require.NoError(t, err)
	require.Equal(t, uint64(1700000000000000002), ord)
}

func TestOrdinalAssignerFallsBackWhenStreamIDMissingOrInvalid(t *testing.T) {
	assigner := NewOrdinalAssigner(func(context.Context, SessionId) (uint64, error) { return 0, nil })

	ord1, err := assigner.Next(context.Background(), "s-1", message.Metadata{})
	require.NoError(t, err)
	require.Equal(t, uint64(1), ord1)

	metadata := message.Metadata{}
	metadata.Set(MetadataKeyStreamID, "bad")
	ord2, err := assigner.Next(context.Background(), "s-1", metadata)
	require.NoError(t, err)
	require.Equal(t, uint64(2), ord2)
}

func TestOrdinalAssignerResumesFromCursorAndStaysMonotonic(t *testing.T) {
	assigner := NewOrdinalAssigner(func(context.Context, SessionId) (uint64, error) { return 5, nil })
	metadata := message.Metadata{}
	metadata.Set(MetadataKeyStreamID, "1-2")

	ord, err := assigner.Next(context.Background(), "s-1", metadata)
	require.NoError(t, err)
	require.Equal(t, uint64(1_000_002), ord)
}

func TestDeriveOrdinalFromStreamID(t *testing.T) {
	ord, ok := DeriveOrdinalFromStreamID("1700000000000-7")
	require.True(t, ok)
	require.Equal(t, uint64(1700000000000000007), ord)

	_, ok = DeriveOrdinalFromStreamID("not-a-stream-id")
	require.False(t, ok)
}
