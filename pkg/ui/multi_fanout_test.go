package ui

import (
	"context"
	"testing"

	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestMultiUIFanoutPublishesToAllTargets(t *testing.T) {
	first := &recordingUIFanout{}
	second := &recordingUIFanout{}
	fanout, err := NewMultiUIFanout(first, nil, second)
	require.NoError(t, err)

	events := []sessionstream.UIEvent{{Name: "event"}}
	err = fanout.PublishUI(context.Background(), "sid", 42, events)
	require.NoError(t, err)

	require.Equal(t, []uint64{42}, first.ordinals)
	require.Equal(t, []uint64{42}, second.ordinals)
	require.Equal(t, "event", first.events[0][0].Name)
	require.Equal(t, "event", second.events[0][0].Name)
}

func TestMultiUIFanoutRequiresTarget(t *testing.T) {
	_, err := NewMultiUIFanout(nil)
	require.Error(t, err)
}

type recordingUIFanout struct {
	ordinals []uint64
	events   [][]sessionstream.UIEvent
}

func (f *recordingUIFanout) PublishUI(_ context.Context, _ sessionstream.SessionId, ord uint64, events []sessionstream.UIEvent) error {
	f.ordinals = append(f.ordinals, ord)
	f.events = append(f.events, append([]sessionstream.UIEvent(nil), events...))
	return nil
}
