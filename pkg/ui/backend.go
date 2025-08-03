package ui

import (
	"context"
	"fmt"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	conversation2 "github.com/go-go-golems/bobatea/pkg/chat/conversation"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type StepBackend struct {
	step       chat.Step
	stepResult steps.StepResult[*conversation.Message]
}

var _ boba_chat.Backend = &StepBackend{}

func (s *StepBackend) Start(ctx context.Context, msgs []*conversation.Message) (tea.Cmd, error) {
	if !s.IsFinished() {
		return nil, errors.New("Step is already running")
	}

	stepResult, err := s.step.Start(ctx, msgs)
	if err != nil {

		return tea.Batch(
				func() tea.Msg {
					return boba_chat.BackendFinishedMsg{}
				}),
			nil
	}

	s.stepResult = stepResult

	return func() tea.Msg {
		if s.IsFinished() {
			return nil
		}
		stepChannel := s.stepResult.GetChannel()
		for range stepChannel {
			// just wait for the step to finish, progress is handled through the published events
		}

		s.stepResult = nil
		return boba_chat.BackendFinishedMsg{}
	}, nil
}

func NewStepBackend(step chat.Step) *StepBackend {
	return &StepBackend{
		step: step,
	}
}

func (s *StepBackend) Interrupt() {
	if s.stepResult != nil {
		s.stepResult.Cancel()
	} else {
		log.Warn().Msg("Step is not running")
	}
}

func (s *StepBackend) Kill() {
	if s.stepResult != nil {
		s.stepResult.Cancel()
		s.stepResult = nil
	} else {
		log.Debug().Msg("Step is not running")
	}
}

func (s *StepBackend) IsFinished() bool {
	return s.stepResult == nil
}

// EngineBackend provides a Backend implementation using the new Engine-first architecture.
// This replaces StepBackend for new code using the inference.Engine interface.
type EngineBackend struct {
	engine    inference.Engine
	eventSink inference.EventSink
	isRunning bool
	cancel    context.CancelFunc
}

var _ boba_chat.Backend = &EngineBackend{}

// NewEngineBackend creates a new EngineBackend with the given engine and event sink.
// The eventSink is used to publish events during inference for UI updates.
func NewEngineBackend(engine inference.Engine, eventSink inference.EventSink) *EngineBackend {
	return &EngineBackend{
		engine:    engine,
		eventSink: eventSink,
		isRunning: false,
	}
}

// Start executes inference using the engine and publishes events through the sink.
// This method implements the boba_chat.Backend interface.
func (e *EngineBackend) Start(ctx context.Context, msgs []*conversation.Message) (tea.Cmd, error) {
	if e.isRunning {
		return nil, errors.New("Engine is already running")
	}

	// Create cancellable context for this inference run
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.isRunning = true

	// Create engine with the event sink if provided
	var engine inference.Engine = e.engine
	if e.eventSink != nil {
		// Configure engine with event sink
		engineWithSink, err := e.configureEngineWithSink(engine)
		if err != nil {
			e.isRunning = false
			e.cancel = nil
			return tea.Batch(
				func() tea.Msg {
					return boba_chat.BackendFinishedMsg{}
				}), nil
		}
		engine = engineWithSink
	}

	return func() tea.Msg {
		if !e.isRunning {
			return nil
		}

		// Run inference
		_, err := engine.RunInference(ctx, msgs)
		
		// Mark as finished
		e.isRunning = false
		e.cancel = nil

		if err != nil {
			log.Error().Err(err).Msg("Engine inference failed")
		}

		return boba_chat.BackendFinishedMsg{}
	}, nil
}

// configureEngineWithSink configures the engine to use the event sink.
// This is a helper method that creates a new engine instance with the sink configured.
func (e *EngineBackend) configureEngineWithSink(engine inference.Engine) (inference.Engine, error) {
	// For now, we return the engine as-is since the Engine interface doesn't expose 
	// configuration methods. In a real implementation, this would require creating 
	// a new engine instance with the sink configured, or extending the Engine interface.
	// The event publishing should happen within the specific Engine implementations.
	return engine, nil
}

// Interrupt attempts to cancel the current inference operation.
func (e *EngineBackend) Interrupt() {
	if e.cancel != nil {
		e.cancel()
	} else {
		log.Warn().Msg("Engine is not running")
	}
}

// Kill forcefully cancels the current inference operation.
func (e *EngineBackend) Kill() {
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		e.isRunning = false
	} else {
		log.Debug().Msg("Engine is not running")
	}
}

// IsFinished returns whether the engine is currently running an inference operation.
func (e *EngineBackend) IsFinished() bool {
	return !e.isRunning
}

// StepChatForwardFunc is a function that forwards watermill messages to the UI by
// trasnforming them into bubbletea messages and injecting them into the program `p`.
func StepChatForwardFunc(p *tea.Program) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		msg.Ack()

		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			log.Error().Err(err).Str("payload", string(msg.Payload)).Msg("Failed to parse event")
			return err
		}

		eventMetadata := e.Metadata()
		metadata := conversation2.StreamMetadata{
			ID:            eventMetadata.ID,
			ParentID:      eventMetadata.ParentID,
			StepMetadata:  e.StepMetadata(),
			EventMetadata: &eventMetadata,
		}
		log.Debug().Interface("event", e).Msg("Dispatching event to UI")
		switch e_ := e.(type) {
		case *events.EventError:
			p.Send(conversation2.StreamCompletionError{
				StreamMetadata: metadata,
				Err:            errors.New(e_.ErrorString),
			})
		case *events.EventPartialCompletion:
			p.Send(conversation2.StreamCompletionMsg{
				StreamMetadata: metadata,
				Delta:          e_.Delta,
				Completion:     e_.Completion,
			})
		case *events.EventFinal:
			p.Send(conversation2.StreamDoneMsg{
				StreamMetadata: metadata,
				Completion:     e_.Text,
			})

		case *events.EventInterrupt:
			p_, ok := events.ToTypedEvent[events.EventInterrupt](e)
			if !ok {
				return errors.New("payload is not of type EventInterrupt")
			}
			p.Send(conversation2.StreamDoneMsg{
				StreamMetadata: metadata,
				Completion:     p_.Text,
			})

		case *events.EventToolCall:
			p.Send(conversation2.StreamDoneMsg{
				StreamMetadata: metadata,
				Completion:     fmt.Sprintf("%s(%s)", e_.ToolCall.Name, e_.ToolCall.Input),
			})
		case *events.EventToolResult:
			p.Send(conversation2.StreamDoneMsg{
				StreamMetadata: metadata,
				Completion:     fmt.Sprintf("Result: %s", e_.ToolResult.Result),
			})

		case *events.EventPartialCompletionStart:
			p.Send(conversation2.StreamStartMsg{
				StreamMetadata: metadata,
			})
		}

		return nil
	}
}
