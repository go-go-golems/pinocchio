package ui

import (
	"context"
	"fmt"

	"github.com/go-go-golems/geppetto/pkg/events"

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
	log.Debug().Int("num_messages", len(msgs)).Msg("StepBackend.Start called")
	
	if !s.IsFinished() {
		log.Warn().Msg("Step is already running")
		return nil, errors.New("Step is already running")
	}

	log.Debug().Msg("Starting step")
	stepResult, err := s.step.Start(ctx, msgs)
	if err != nil {
		log.Error().Err(err).Msg("Step.Start failed")
		return tea.Batch(
				func() tea.Msg {
					log.Debug().Msg("Sending BackendFinishedMsg due to step error")
					return boba_chat.BackendFinishedMsg{}
				}),
			nil
	}

	log.Debug().Msg("Step.Start succeeded, storing stepResult")
	s.stepResult = stepResult

	return func() tea.Msg {
		log.Debug().Msg("StepBackend waiting goroutine started")
		
		if s.IsFinished() {
			log.Debug().Msg("Step is already finished, returning nil")
			return nil
		}
		
		stepChannel := s.stepResult.GetChannel()
		log.Debug().Msg("Got step channel, starting to wait for results")
		
		resultCount := 0
		for result := range stepChannel {
			resultCount++
			log.Debug().Int("result_count", resultCount).Bool("has_error", result.Error() != nil).Msg("Received result from step channel")
			if result.Error() != nil {
				log.Error().Err(result.Error()).Msg("Step result contains error")
			} else {
				log.Debug().Msg("Step result successful")
			}
			// just wait for the step to finish, progress is handled through the published events
		}
		
		log.Debug().Int("total_results", resultCount).Msg("Step channel closed, cleaning up")
		s.stepResult = nil
		log.Debug().Msg("Sending BackendFinishedMsg - step completed")
		return boba_chat.BackendFinishedMsg{}
	}, nil
}

func NewStepBackend(step chat.Step) *StepBackend {
	return &StepBackend{
		step: step,
	}
}

func (s *StepBackend) Interrupt() {
	log.Debug().Bool("is_running", s.stepResult != nil).Msg("StepBackend.Interrupt called")
	if s.stepResult != nil {
		log.Debug().Msg("Cancelling step result")
		s.stepResult.Cancel()
		log.Debug().Msg("Step result cancelled")
	} else {
		log.Warn().Msg("Step is not running")
	}
}

func (s *StepBackend) Kill() {
	log.Debug().Bool("is_running", s.stepResult != nil).Msg("StepBackend.Kill called")
	if s.stepResult != nil {
		log.Debug().Msg("Cancelling and clearing step result")
		s.stepResult.Cancel()
		s.stepResult = nil
		log.Debug().Msg("Step result cancelled and cleared")
	} else {
		log.Debug().Msg("Step is not running")
	}
}

func (s *StepBackend) IsFinished() bool {
	return s.stepResult == nil
}

// StepChatForwardFunc is a function that forwards watermill messages to the UI by
// trasnforming them into bubbletea messages and injecting them into the program `p`.
func StepChatForwardFunc(p *tea.Program) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		log.Debug().Int("payload_size", len(msg.Payload)).Str("payload", string(msg.Payload)).Msg("StepChatForwardFunc received watermill message")
		msg.Ack()

		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			log.Error().Err(err).Str("payload", string(msg.Payload)).Msg("Failed to parse event")
			return err
		}

		eventMetadata := e.Metadata()
		log.Debug().Str("event_type", fmt.Sprintf("%T", e)).Str("event_id", eventMetadata.ID.String()).Msg("Parsed event successfully")
		
		metadata := conversation2.StreamMetadata{
			ID:            eventMetadata.ID,
			ParentID:      eventMetadata.ParentID,
			StepMetadata:  e.StepMetadata(),
			EventMetadata: &eventMetadata,
		}
		log.Debug().Interface("event", e).Msg("Dispatching event to UI")
		
		switch e_ := e.(type) {
		case *events.EventError:
			log.Debug().Str("error", e_.ErrorString).Msg("Sending StreamCompletionError to UI")
			p.Send(conversation2.StreamCompletionError{
				StreamMetadata: metadata,
				Err:            errors.New(e_.ErrorString),
			})
			
		case *events.EventPartialCompletion:
			log.Debug().Str("delta", e_.Delta).Int("completion_length", len(e_.Completion)).Msg("Sending StreamCompletionMsg to UI")
			p.Send(conversation2.StreamCompletionMsg{
				StreamMetadata: metadata,
				Delta:          e_.Delta,
				Completion:     e_.Completion,
			})
			
		case *events.EventFinal:
			log.Debug().Int("text_length", len(e_.Text)).Msg("Sending StreamDoneMsg to UI (final)")
			p.Send(conversation2.StreamDoneMsg{
				StreamMetadata: metadata,
				Completion:     e_.Text,
			})

		case *events.EventInterrupt:
			log.Debug().Msg("Sending StreamDoneMsg to UI (interrupt)")
			p_, ok := events.ToTypedEvent[events.EventInterrupt](e)
			if !ok {
				return errors.New("payload is not of type EventInterrupt")
			}
			p.Send(conversation2.StreamDoneMsg{
				StreamMetadata: metadata,
				Completion:     p_.Text,
			})

		case *events.EventToolCall:
			log.Debug().Str("tool_name", e_.ToolCall.Name).Msg("Sending StreamDoneMsg to UI (tool call)")
			p.Send(conversation2.StreamDoneMsg{
				StreamMetadata: metadata,
				Completion:     fmt.Sprintf("%s(%s)", e_.ToolCall.Name, e_.ToolCall.Input),
			})
			
		case *events.EventToolResult:
			log.Debug().Int("result_length", len(e_.ToolResult.Result)).Msg("Sending StreamDoneMsg to UI (tool result)")
			p.Send(conversation2.StreamDoneMsg{
				StreamMetadata: metadata,
				Completion:     fmt.Sprintf("Result: %s", e_.ToolResult.Result),
			})

		case *events.EventPartialCompletionStart:
			log.Debug().Msg("Sending StreamStartMsg to UI")
			p.Send(conversation2.StreamStartMsg{
				StreamMetadata: metadata,
			})
			
		default:
			log.Warn().Str("event_type", fmt.Sprintf("%T", e)).Msg("Unknown event type, not dispatching to UI")
		}

		log.Debug().Str("event_type", fmt.Sprintf("%T", e)).Msg("Event dispatched to UI successfully")
		return nil
	}
}
