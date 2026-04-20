package evtstream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandRegistryRejectsDuplicateRegistration(t *testing.T) {
	r := newCommandRegistry()
	handler := func(context.Context, Command, *Session, EventPublisher) error { return nil }

	require.NoError(t, r.Register("LabStart", handler))
	require.Error(t, r.Register("LabStart", handler))
}

func TestCommandRegistryLookupMiss(t *testing.T) {
	r := newCommandRegistry()
	_, ok := r.Lookup("missing")
	require.False(t, ok)
}
