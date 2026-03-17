package profileswitch

import (
	"context"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Backend runs real inference using the current profile resolution.
//
// Profile switching swaps the session.Builder while the session is idle.
type Backend struct {
	sess *session.Session
	mgr  *Manager

	sink      events.EventSink
	persister enginebuilder.TurnPersister

	alwaysMiddlewares []middleware.Middleware

	mu       sync.RWMutex
	resolved Resolved
}

var _ interface {
	Start(context.Context, string) (tea.Cmd, error)
	Interrupt()
	Kill()
	IsFinished() bool
} = (*Backend)(nil)

func NewBackend(mgr *Manager, sink events.EventSink, persister enginebuilder.TurnPersister, always []middleware.Middleware) (*Backend, error) {
	if mgr == nil {
		return nil, errors.New("profileswitch backend: manager is nil")
	}
	if sink == nil {
		return nil, errors.New("profileswitch backend: sink is nil")
	}

	sess := session.NewSession()
	b := &Backend{
		sess:              sess,
		mgr:               mgr,
		sink:              sink,
		persister:         persister,
		alwaysMiddlewares: append([]middleware.Middleware(nil), always...),
	}
	return b, nil
}

func (b *Backend) SessionID() string {
	if b == nil || b.sess == nil {
		return ""
	}
	return b.sess.SessionID
}

func (b *Backend) SetTurnPersister(p enginebuilder.TurnPersister) {
	if b == nil {
		return
	}
	b.persister = p
	// The builder is recreated on each profile switch; applyResolved will pick up b.persister.
	// If there's an active builder already, update it opportunistically.
	if b.sess != nil && b.sess.Builder != nil {
		if builder, ok := b.sess.Builder.(*enginebuilder.Builder); ok && builder != nil {
			builder.Persister = p
		}
	}
}

func (b *Backend) SetSeedTurn(t *turns.Turn) error {
	if b == nil || b.sess == nil || t == nil {
		return nil
	}
	if b.sess.IsRunning() {
		return errors.New("cannot seed while streaming")
	}
	seed := t.Clone()

	sid, ok, err := turns.KeyTurnMetaSessionID.Get(seed.Metadata)
	if err != nil || !ok || strings.TrimSpace(sid) == "" {
		sid = session.NewSession().SessionID
		_ = turns.KeyTurnMetaSessionID.Set(&seed.Metadata, sid)
	}

	// Replace the session while preserving the current builder.
	sess := &session.Session{Builder: b.sess.Builder}
	sess.SessionID = strings.TrimSpace(sid)
	sess.Append(seed)
	b.sess = sess
	return nil
}

func (b *Backend) Current() Resolved {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.resolved
}

func (b *Backend) InitDefaultProfile(ctx context.Context, profileSlug string) (Resolved, error) {
	if b == nil || b.mgr == nil {
		return Resolved{}, errors.New("profileswitch backend: not initialized")
	}
	res, err := b.mgr.Switch(ctx, profileSlug)
	if err != nil {
		return Resolved{}, err
	}
	if err := b.applyResolved(res); err != nil {
		return Resolved{}, err
	}
	return res, nil
}

func (b *Backend) SwitchProfile(ctx context.Context, profileSlug string) (Resolved, error) {
	if b == nil || b.mgr == nil || b.sess == nil {
		return Resolved{}, errors.New("profileswitch backend: not initialized")
	}
	if b.sess.IsRunning() {
		return Resolved{}, errors.New("cannot switch profile while streaming")
	}
	res, err := b.mgr.Switch(ctx, profileSlug)
	if err != nil {
		return Resolved{}, err
	}
	if err := b.applyResolved(res); err != nil {
		return Resolved{}, err
	}
	return res, nil
}

func (b *Backend) applyResolved(res Resolved) error {
	if b == nil || b.sess == nil {
		return errors.New("profileswitch backend: not initialized")
	}
	if res.StepSettings == nil {
		return errors.New("profileswitch backend: resolved step settings is nil")
	}

	eng, err := factory.NewEngineFromStepSettings(res.StepSettings)
	if err != nil {
		return err
	}

	mws := make([]middleware.Middleware, 0, 2+len(b.alwaysMiddlewares))
	mws = append(mws, b.alwaysMiddlewares...)
	if strings.TrimSpace(res.SystemPrompt) != "" {
		mws = append(mws, middleware.NewSystemPromptMiddleware(res.SystemPrompt))
	}

	builder := &enginebuilder.Builder{
		Base:        eng,
		Middlewares: mws,
		EventSinks:  []events.EventSink{b.sink},
		Persister:   b.persister,
	}

	b.mu.Lock()
	b.resolved = res
	b.mu.Unlock()

	b.sess.Builder = builder
	return nil
}

func (b *Backend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
	if b == nil || b.sess == nil {
		return nil, errors.New("profileswitch backend: not initialized")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("empty prompt")
	}
	if b.sess.IsRunning() {
		return nil, errors.New("already running")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	t, err := b.sess.AppendNewTurnFromUserPrompt(prompt)
	if err != nil {
		return nil, err
	}

	// Persist profile/runtime attribution on the turn itself (canonical source of truth).
	res := b.Current()
	attrib := map[string]any{
		"runtime_key":         res.RuntimeKey.String(),
		"profile_slug":        res.ProfileSlug.String(),
		"registry_slug":       res.RegistrySlug.String(),
		"profile_version":     res.ProfileVersion,
		"runtime_fingerprint": res.RuntimeFingerprint,
	}
	_ = turns.KeyTurnMetaRuntime.Set(&t.Metadata, attrib)

	handle, err := b.sess.StartInference(ctx)
	if err != nil {
		return nil, err
	}

	return func() tea.Msg {
		updated, err := handle.Wait()
		if err != nil {
			log.Error().Err(err).Msg("inference failed")
		}
		if updated != nil {
			log.Debug().Int("blocks", len(updated.Blocks)).Msg("inference completed")
		}
		// BackendFinishedMsg is emitted by the forwarder (EventFinal/EventError), not here.
		return nil
	}, nil
}

func (b *Backend) Interrupt() {
	if b == nil || b.sess == nil {
		return
	}
	_ = b.sess.CancelActive()
}

func (b *Backend) Kill() {
	if b == nil || b.sess == nil {
		return
	}
	_ = b.sess.CancelActive()
}

func (b *Backend) IsFinished() bool {
	return b == nil || b.sess == nil || !b.sess.IsRunning()
}
