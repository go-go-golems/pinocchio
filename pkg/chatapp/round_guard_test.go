package chatapp

import (
	"context"
	"errors"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/stretchr/testify/require"
)

type countingEngine struct{ calls int }

func (e *countingEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	e.calls++
	return t, nil
}

func TestTheGuardRunsBeforeEveryModelCallAndCanStopOne(t *testing.T) {
	inner := &countingEngine{}
	var seen []int
	stop := errors.New("budget")
	eng := guardEngine(inner, func(_ context.Context, _ *turns.Turn, round int) error {
		seen = append(seen, round)
		if round == 2 {
			return stop
		}
		return nil
	})
	turn := &turns.Turn{}
	for i := 0; i < 2; i++ {
		_, err := eng.RunInference(context.Background(), turn)
		require.NoError(t, err)
	}
	got, err := eng.RunInference(context.Background(), turn)
	require.ErrorIs(t, err, stop)
	require.Same(t, turn, got, "the turn so far is returned with the guard's error")
	require.Equal(t, []int{0, 1, 2}, seen, "rounds count model calls from 0")
	require.Equal(t, 2, inner.calls, "the stopped call never reaches the engine")
	require.Same(t, inner, guardEngine(inner, nil), "no guard, no wrapper")
}
