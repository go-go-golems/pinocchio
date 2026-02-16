package webchat

import (
	"context"
	"testing"

	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/stretchr/testify/require"
)

func TestNewStreamBackendFromValues_InMemoryDefaults(t *testing.T) {
	backend, err := NewStreamBackendFromValues(context.Background(), values.New())
	require.NoError(t, err)
	require.NotNil(t, backend)
	require.NotNil(t, backend.EventRouter())
	require.NotNil(t, backend.Publisher())

	sub, closeSub, err := backend.BuildSubscriber(context.Background(), "conv-1")
	require.NoError(t, err)
	require.NotNil(t, sub)
	require.False(t, closeSub)
}

func TestStreamBackend_BuildSubscriberRequiresConvID(t *testing.T) {
	backend, err := NewStreamBackendFromValues(context.Background(), values.New())
	require.NoError(t, err)

	_, _, err = backend.BuildSubscriber(context.Background(), "")
	require.ErrorContains(t, err, "convID is empty")
}
