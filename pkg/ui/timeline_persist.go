package ui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/rs/zerolog/log"
)

// StepTimelinePersistFunc stores timeline snapshots from the UI event topic into the configured TimelineStore.
// Persistence is best-effort: serialization/storage errors are logged but do not fail chat execution.
func StepTimelinePersistFunc(store chatstore.TimelineStore, convID string) func(msg *message.Message) error {
	var version atomic.Uint64

	var mu sync.Mutex
	assistantSeen := map[string]bool{}
	assistantContent := map[string]string{}
	thinkingSeen := map[string]bool{}
	thinkingContent := map[string]string{}

	upsertMessage := func(ctx context.Context, entityID string, role string, content string, streaming bool) error {
		if store == nil || strings.TrimSpace(convID) == "" || strings.TrimSpace(entityID) == "" {
			return nil
		}
		seq := version.Add(1)
		entity := &timelinepb.TimelineEntityV1{
			Id:   entityID,
			Kind: "message",
			Snapshot: &timelinepb.TimelineEntityV1_Message{
				Message: &timelinepb.MessageSnapshotV1{
					SchemaVersion: 1,
					Role:          role,
					Content:       content,
					Streaming:     streaming,
				},
			},
		}
		return store.Upsert(ctx, convID, seq, entity)
	}

	return func(msg *message.Message) error {
		msg.Ack()

		ev, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			log.Warn().Err(err).Str("component", "timeline_persist").Msg("failed to decode event payload")
			return nil
		}

		md := ev.Metadata()
		entityID := strings.TrimSpace(md.ID.String())
		if entityID == "" {
			return nil
		}

		ctx := msg.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		persist := func(id string, role string, content string, streaming bool) {
			persistCtx := ctx
			cancel := func() {}
			if persistCtx.Err() != nil {
				// During shutdown, Watermill message contexts can be canceled before the queue drains.
				// Use a short detached context so final timeline upserts can still land without log spam.
				persistCtx, cancel = context.WithTimeout(context.Background(), 250*time.Millisecond)
			}
			defer cancel()

			if err := upsertMessage(persistCtx, id, role, content, streaming); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				log.Warn().Err(err).
					Str("component", "timeline_persist").
					Str("conv_id", convID).
					Str("entity_id", id).
					Str("role", role).
					Msg("timeline upsert failed")
			}
		}

		switch e := ev.(type) {
		case *events.EventPartialCompletionStart:
			// Do not create an empty assistant entry on start.
			_ = e
		case *events.EventPartialCompletion:
			if strings.TrimSpace(e.Completion) == "" {
				break
			}
			mu.Lock()
			assistantSeen[entityID] = true
			assistantContent[entityID] = e.Completion
			mu.Unlock()
			persist(entityID, "assistant", e.Completion, true)
		case *events.EventFinal:
			mu.Lock()
			seen := assistantSeen[entityID]
			content := e.Text
			if strings.TrimSpace(content) == "" {
				content = assistantContent[entityID]
			}
			if strings.TrimSpace(content) != "" {
				assistantSeen[entityID] = true
				assistantContent[entityID] = content
			}
			mu.Unlock()
			if strings.TrimSpace(content) == "" && !seen {
				break
			}
			persist(entityID, "assistant", content, false)
		case *events.EventInterrupt:
			mu.Lock()
			seen := assistantSeen[entityID]
			content := e.Text
			if strings.TrimSpace(content) == "" {
				content = assistantContent[entityID]
			}
			if strings.TrimSpace(content) != "" {
				assistantSeen[entityID] = true
				assistantContent[entityID] = content
			}
			mu.Unlock()
			if strings.TrimSpace(content) == "" && !seen {
				break
			}
			persist(entityID, "assistant", content, false)
		case *events.EventError:
			errText := "**Error**\n\n" + e.ErrorString
			mu.Lock()
			assistantSeen[entityID] = true
			assistantContent[entityID] = errText
			mu.Unlock()
			persist(entityID, "assistant", errText, false)
		case *events.EventInfo:
			thinkID := entityID + ":thinking"
			switch strings.TrimSpace(e.Message) {
			case "thinking-started":
				mu.Lock()
				thinkingSeen[thinkID] = true
				content := thinkingContent[thinkID]
				mu.Unlock()
				persist(thinkID, "thinking", content, true)
			case "thinking-ended":
				mu.Lock()
				seen := thinkingSeen[thinkID]
				content := thinkingContent[thinkID]
				mu.Unlock()
				if !seen && strings.TrimSpace(content) == "" {
					break
				}
				persist(thinkID, "thinking", content, false)
			}
		case *events.EventThinkingPartial:
			thinkID := entityID + ":thinking"
			if strings.TrimSpace(e.Completion) == "" {
				break
			}
			mu.Lock()
			thinkingSeen[thinkID] = true
			thinkingContent[thinkID] = e.Completion
			mu.Unlock()
			persist(thinkID, "thinking", e.Completion, true)
		}

		return nil
	}
}
