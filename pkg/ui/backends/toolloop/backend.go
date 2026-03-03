package toolloop

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	geppettotoolloop "github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var _ boba_chat.Backend = (*ToolLoopBackend)(nil)

// ToolLoopBackend runs the tool-calling loop across turns and emits BackendFinishedMsg when done.
type ToolLoopBackend struct {
	reg  *tools.InMemoryToolRegistry
	sink events.EventSink
	hook geppettotoolloop.SnapshotHook

	sess *session.Session
}

func NewToolLoopBackend(eng engine.Engine, mws []middleware.Middleware, reg *tools.InMemoryToolRegistry, sink events.EventSink, hook geppettotoolloop.SnapshotHook) *ToolLoopBackend {
	loopCfg := geppettotoolloop.NewLoopConfig().WithMaxIterations(5)
	toolCfg := tools.DefaultToolConfig().WithExecutionTimeout(60 * time.Second)
	sess := session.NewSession()
	sess.Builder = enginebuilder.New(
		enginebuilder.WithBase(eng),
		enginebuilder.WithMiddlewares(mws...),
		enginebuilder.WithToolRegistry(reg),
		enginebuilder.WithLoopConfig(loopCfg),
		enginebuilder.WithToolConfig(toolCfg),
		enginebuilder.WithEventSinks(sink),
		enginebuilder.WithSnapshotHook(hook),
	)
	return &ToolLoopBackend{reg: reg, sink: sink, hook: hook, sess: sess}
}

func (b *ToolLoopBackend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
	if b == nil || b.sess == nil {
		return nil, errors.New("backend not initialized")
	}
	if b.sess.IsRunning() {
		return nil, errors.New("already running")
	}

	_, err := b.sess.AppendNewTurnFromUserPrompt(prompt)
	if err != nil {
		return nil, errors.Wrap(err, "append prompt turn")
	}

	handle, err := b.sess.StartInference(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "start inference")
	}

	return func() tea.Msg {
		_, waitErr := handle.Wait()
		if waitErr != nil {
			log.Error().Err(waitErr).Msg("tool loop failed")
		}
		return boba_chat.BackendFinishedMsg{}
	}, nil
}

func (b *ToolLoopBackend) Interrupt() {
	if b != nil && b.sess != nil {
		_ = b.sess.CancelActive()
	}
}

func (b *ToolLoopBackend) Kill() {
	if b != nil && b.sess != nil {
		_ = b.sess.CancelActive()
	}
}

func (b *ToolLoopBackend) IsFinished() bool {
	return b == nil || b.sess == nil || !b.sess.IsRunning()
}

// CurrentTurn returns the latest in-memory turn snapshot for this backend.
// Callers may mutate the returned Turn (e.g. seed Turn.Data) before starting inference.
func (b *ToolLoopBackend) CurrentTurn() *turns.Turn {
	if b == nil || b.sess == nil {
		return nil
	}
	return b.sess.Latest()
}
