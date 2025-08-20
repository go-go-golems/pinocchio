package runtime

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/ui"
	"github.com/pkg/errors"
)

// HandlerContext provides runtime objects for building a Watermill handler.
type HandlerContext struct {
	Session *ChatSession
	Program *tea.Program
	Router  *events.EventRouter
}

// HandlerFactory produces a Watermill handler bound to the provided context.
// This allows custom handlers to access the Bubble Tea program and session.
type HandlerFactory func(HandlerContext) func(*message.Message) error

// ChatBuilder constructs chat UI components and programs for CLI and embedding.
type ChatBuilder struct {
	ctx            context.Context
	engineFactory  factory.EngineFactory
	settings       *settings.StepSettings
	router         *events.EventRouter
	programOptions []tea.ProgramOption
	modelOptions   []boba_chat.ModelOption
	seedTurn       *turns.Turn
	handlerFactory HandlerFactory
}

// NewChatBuilder returns a new builder with defaults.
func NewChatBuilder() *ChatBuilder {
	return &ChatBuilder{
		ctx:            context.Background(),
		programOptions: nil,
		modelOptions:   nil,
	}
}

func (b *ChatBuilder) WithContext(ctx context.Context) *ChatBuilder {
	if ctx != nil {
		b.ctx = ctx
	}
	return b
}

func (b *ChatBuilder) WithEngineFactory(f factory.EngineFactory) *ChatBuilder {
	b.engineFactory = f
	return b
}

func (b *ChatBuilder) WithSettings(s *settings.StepSettings) *ChatBuilder {
	b.settings = s
	return b
}

func (b *ChatBuilder) WithRouter(r *events.EventRouter) *ChatBuilder {
	b.router = r
	return b
}

func (b *ChatBuilder) WithProgramOptions(opts ...tea.ProgramOption) *ChatBuilder {
	b.programOptions = append(b.programOptions, opts...)
	return b
}

func (b *ChatBuilder) WithModelOptions(opts ...boba_chat.ModelOption) *ChatBuilder {
	b.modelOptions = append(b.modelOptions, opts...)
	return b
}

func (b *ChatBuilder) WithSeedTurn(t *turns.Turn) *ChatBuilder {
	b.seedTurn = t
	return b
}

// WithEventHandler allows callers to provide a ready-made handler.
// For access to Program and Session, prefer WithHandlerFactory.
func (b *ChatBuilder) WithEventHandler(h func(*message.Message) error) *ChatBuilder {
	b.handlerFactory = func(_ HandlerContext) func(*message.Message) error { return h }
	return b
}

// WithHandlerFactory sets a factory that will be invoked once the Program exists
// to construct the final Watermill handler with access to Session and Program.
func (b *ChatBuilder) WithHandlerFactory(f HandlerFactory) *ChatBuilder {
	b.handlerFactory = f
	return b
}

// ChatSession holds references to runtime components and exposes a bound event handler.
type ChatSession struct {
	Router  *events.EventRouter
	Backend *ui.EngineBackend

	handler func(*message.Message) error

	// program is set by BuildProgram automatically, or via AttachProgram when embedding
	program *tea.Program
}

// BindHandlerWithProgram binds the Watermill handler using either the custom
// HandlerFactory or the default StepChatForwardFunc, and attaches the program
// to the backend for initial entity emissions.
func (cs *ChatSession) BindHandlerWithProgram(p *tea.Program) {
	cs.program = p
	if cs.Backend != nil {
		cs.Backend.AttachProgram(p)
	}
	if cs.handler == nil {
		// No pre-bound handler, use default
		cs.handler = ui.StepChatForwardFunc(p)
	}
}

// AttachProgram attaches a Bubble Tea program for the event handler to target.
func (cs *ChatSession) AttachProgram(p *tea.Program) {
	cs.BindHandlerWithProgram(p)
}

// EventHandler returns the bound Watermill->UI handler.
func (cs *ChatSession) EventHandler() func(*message.Message) error {
	return cs.handler
}

// BuildProgram creates engine, backend, chat model and a ready-to-run Bubble Tea program.
// It also binds the UI event handler to the returned session.
func (b *ChatBuilder) BuildProgram() (*ChatSession, *tea.Program, error) {
	if b.engineFactory == nil {
		return nil, nil, errors.New("engine factory is required")
	}
	if b.settings == nil {
		return nil, nil, errors.New("settings are required")
	}
	if b.router == nil {
		return nil, nil, errors.New("router is required; use WithRouter")
	}

	// Create engine with a UI sink attached
	uiSink := middleware.NewWatermillSink(b.router.Publisher, "ui")
	eng, err := b.engineFactory.CreateEngine(b.settings, engine.WithSink(uiSink))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create engine")
	}

	backend := ui.NewEngineBackend(eng)

	model := boba_chat.InitialModel(backend, b.modelOptions...)
	program := tea.NewProgram(model, b.programOptions...)

	// Attach program for seeding emissions and handler binding
	backend.AttachProgram(program)

	sess := &ChatSession{
		Router:  b.router,
		Backend: backend,
	}
	// Build or default the handler, then bind with the program
	if b.handlerFactory != nil {
		sess.handler = b.handlerFactory(HandlerContext{Session: sess, Program: program, Router: b.router})
	} else {
		// Default will be bound in BindHandlerWithProgram
		sess.handler = nil
	}
	sess.BindHandlerWithProgram(program)

	return sess, program, nil
}

// BuildComponents creates engine, backend and chat model for embedding. It returns a
// bound handler that will use the program attached later via ChatSession.AttachProgram.
func (b *ChatBuilder) BuildComponents() (*ChatSession, tea.Model, boba_chat.Backend, func(*message.Message) error, error) {
	if b.engineFactory == nil {
		return nil, nil, nil, nil, errors.New("engine factory is required")
	}
	if b.settings == nil {
		return nil, nil, nil, nil, errors.New("settings are required")
	}
	if b.router == nil {
		return nil, nil, nil, nil, errors.New("router is required; use WithRouter")
	}

	// Create engine with a UI sink attached
	uiSink := middleware.NewWatermillSink(b.router.Publisher, "ui")
	eng, err := b.engineFactory.CreateEngine(b.settings, engine.WithSink(uiSink))
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to create engine")
	}

	backend := ui.NewEngineBackend(eng)

	model := boba_chat.InitialModel(backend, b.modelOptions...)

	sess := &ChatSession{
		Router:  b.router,
		Backend: backend,
	}
	// Return a thin proxy that defers to the bound handler after the caller
	// binds the program using sess.BindHandlerWithProgram(p).
	handler := func(msg *message.Message) error {
		if sess.handler == nil {
			// If a factory was provided and a program is available, attempt to build now
			if b.handlerFactory != nil && sess.program != nil {
				sess.handler = b.handlerFactory(HandlerContext{Session: sess, Program: sess.program, Router: b.router})
			} else if sess.program != nil {
				sess.handler = ui.StepChatForwardFunc(sess.program)
			}
		}
		if sess.handler == nil {
			return errors.New("handler not bound; call BindHandlerWithProgram before running handlers")
		}
		return sess.handler(msg)
	}

	return sess, model, backend, handler, nil
}
