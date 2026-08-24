package frontendtools

import (
	"testing"
	"time"

	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestBoundedTerminalStoreEvictsByCountWithoutRefreshingOnRead(t *testing.T) {
	base := time.Unix(1_000, 0)
	store := newBoundedTerminalStore(2, time.Hour)
	first := terminalRecord("session", "call-1", base)
	second := terminalRecord("session", "call-2", base.Add(time.Second))
	third := terminalRecord("session", "call-3", base.Add(2*time.Second))

	store.add(first, base)
	store.add(second, base.Add(time.Second))
	_, ok := store.get(first.key, base.Add(1500*time.Millisecond))
	require.True(t, ok)
	store.add(third, base.Add(2*time.Second))

	_, ok = store.get(first.key, base.Add(2*time.Second))
	require.False(t, ok, "a retry lookup must not refresh insertion order")
	_, ok = store.get(second.key, base.Add(2*time.Second))
	require.True(t, ok)
	_, ok = store.get(third.key, base.Add(2*time.Second))
	require.True(t, ok)
	require.Equal(t, 2, store.len(base.Add(2*time.Second)))
}

func TestBoundedTerminalStoreEvictsAtTTLBoundary(t *testing.T) {
	base := time.Unix(2_000, 0)
	store := newBoundedTerminalStore(4, time.Minute)
	record := terminalRecord("session", "call-1", base)
	store.add(record, base)

	_, ok := store.get(record.key, base.Add(time.Minute-time.Nanosecond))
	require.True(t, ok)
	_, ok = store.get(record.key, base.Add(time.Minute))
	require.False(t, ok)
	require.Zero(t, store.len(base.Add(time.Minute)))
}

func TestNewManagerWithConfigRejectsUnboundedRetention(t *testing.T) {
	_, err := NewManagerWithConfig(ManagerConfig{TerminalMaxEntries: 0, TerminalTTL: time.Minute})
	require.Error(t, err)
	_, err = NewManagerWithConfig(ManagerConfig{TerminalMaxEntries: 1, TerminalTTL: 0})
	require.Error(t, err)

	manager, err := NewManagerWithConfig(ManagerConfig{TerminalMaxEntries: 1, TerminalTTL: time.Minute})
	require.NoError(t, err)
	require.NotNil(t, manager)
}

func terminalRecord(sessionID, toolCallID string, completedAt time.Time) *terminalCall {
	return &terminalCall{
		key:         pendingKey{sessionID: sessionstream.SessionId(sessionID), toolCallID: toolCallID},
		toolName:    "tool",
		status:      "success",
		origin:      terminalOriginResult,
		completedAt: completedAt,
	}
}
