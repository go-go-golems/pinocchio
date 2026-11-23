package chatapp

import (
	"context"
	"sync"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns"
)

// guardEngine runs a run's round guard before every model call of the run,
// whichever path geppetto takes (the tool loop, its final answer, or a
// plain call when the run has no tools). Rounds count model calls from 0,
// as the tool loop does; the final answer at the limit is call maxRounds.
// One wrapper per run: the count is the run's.
func guardEngine(inner engine.Engine, guard toolloop.RoundGuard) engine.Engine {
	if guard == nil || inner == nil {
		return inner
	}
	return &guardedEngine{inner: inner, guard: guard}
}

type guardedEngine struct {
	inner engine.Engine
	guard toolloop.RoundGuard

	mu    sync.Mutex
	round int
}

var _ engine.Engine = (*guardedEngine)(nil)

func (g *guardedEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	g.mu.Lock()
	round := g.round
	g.round++
	g.mu.Unlock()
	if err := g.guard(ctx, t, round); err != nil {
		return t, err // the turn so far stays the conversation
	}
	return g.inner.RunInference(ctx, t)
}
