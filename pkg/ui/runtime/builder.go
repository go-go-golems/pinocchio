package runtime

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/ui"
	"github.com/pkg/errors"
)

// ChatBuilder constructs chat UI programs.
type ChatBuilder struct {
	ctx            context.Context
	engineFactory  factory.EngineFactory
	settings       *settings.InferenceSettings
	router         *events.EventRouter
	programOptions []tea.ProgramOption
	modelOptions   []boba_chat.ModelOption
	seedTurn       *turns.Turn
}

func NewChatBuilder() *ChatBuilder {
	return &ChatBuilder{ctx: context.Background()}
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

func (b *ChatBuilder) WithSettings(s *settings.InferenceSettings) *ChatBuilder {
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

type ChatSession struct {
	Router  *events.EventRouter
	Backend *ui.EngineBackend

	handler func(*message.Message) error
	program *tea.Program
}

func (cs *ChatSession) BindHandlerWithProgram(p *tea.Program) {
	cs.program = p
	if cs.Backend != nil {
		cs.Backend.AttachProgram(p)
	}
	cs.handler = ui.StepChatForwardFunc(p)
}

func (cs *ChatSession) AttachProgram(p *tea.Program) {
	cs.BindHandlerWithProgram(p)
}

func (cs *ChatSession) EventHandler() func(*message.Message) error {
	return cs.handler
}

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

	uiSink := middleware.NewWatermillSink(b.router.Publisher, "ui")
	eng, err := b.engineFactory.CreateEngine(b.settings)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create engine")
	}

	backend := ui.NewEngineBackend(eng, uiSink)
	model := boba_chat.InitialModel(backend, b.modelOptions...)
	program := tea.NewProgram(model, b.programOptions...)
	backend.AttachProgram(program)

	sess := &ChatSession{Router: b.router, Backend: backend}
	sess.BindHandlerWithProgram(program)
	if b.seedTurn != nil {
		backend.SetSeedTurn(b.seedTurn)
	}
	return sess, program, nil
}
