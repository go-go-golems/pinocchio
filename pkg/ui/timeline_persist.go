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
	"google.golang.org/protobuf/types/known/structpb"
)

// StepTimelinePersistFunc stores timeline snapshots from the UI event topic into the configured TimelineStore.
// Persistence is best-effort: serialization/storage errors are logged but do not fail chat execution.
func StepTimelinePersistFunc(store chatstore.TimelineStore, convID string) func(msg *message.Message) error {
	var version atomic.Uint64
	return StepTimelinePersistFuncWithVersion(store, convID, &version)
}

// StepTimelinePersistFuncWithVersion is like StepTimelinePersistFunc, but allows the caller to provide a shared
// monotonic version counter (useful when multiple components need to upsert into the same conversation timeline).
func StepTimelinePersistFuncWithVersion(store chatstore.TimelineStore, convID string, version *atomic.Uint64) func(msg *message.Message) error {
	if version == nil {
		version = &atomic.Uint64{}
	}
	// Serialize store writes to avoid SQLITE_BUSY under concurrent handler dispatch.
	var storeMu sync.Mutex

	var mu sync.Mutex
	assistantSeen := map[string]bool{}
	assistantContent := map[string]string{}
	thinkingSeen := map[string]bool{}
	thinkingContent := map[string]string{}

	attribFromExtra := func(extra map[string]interface{}) map[string]any {
		if len(extra) == 0 {
			return nil
		}
		out := map[string]any{}
		if s, ok := extra["runtime_key"].(string); ok && strings.TrimSpace(s) != "" {
			out["runtime_key"] = strings.TrimSpace(s)
		}
		if s, ok := extra["runtime_fingerprint"].(string); ok && strings.TrimSpace(s) != "" {
			out["runtime_fingerprint"] = strings.TrimSpace(s)
		}
		if s, ok := extra["profile.slug"].(string); ok && strings.TrimSpace(s) != "" {
			out["profile.slug"] = strings.TrimSpace(s)
		}
		if s, ok := extra["profile.registry"].(string); ok && strings.TrimSpace(s) != "" {
			out["profile.registry"] = strings.TrimSpace(s)
		}
		switch v := extra["profile.version"].(type) {
		case uint64:
			if v > 0 {
				// #nosec G115
				out["profile.version"] = int64(v)
			}
		case int64:
			if v > 0 {
				out["profile.version"] = v
			}
		case int:
			if v > 0 {
				out["profile.version"] = int64(v)
			}
		case float64:
			if v > 0 {
				out["profile.version"] = v
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}

	currentTextID := ""
	currentTextActive := false

	upsertEntity := func(ctx context.Context, entityID string, kind string, propsMap map[string]any) error {
		if store == nil || strings.TrimSpace(convID) == "" || strings.TrimSpace(entityID) == "" {
			return nil
		}
		storeMu.Lock()
		defer storeMu.Unlock()
		seq := version.Add(1)
		props, err := structpb.NewStruct(propsMap)
		if err != nil {
			return err
		}
		entity := &timelinepb.TimelineEntityV2{
			Id:    entityID,
			Kind:  strings.TrimSpace(kind),
			Props: props,
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

		canonicalEntityID := func(corr events.Correlation) string {
			if strings.TrimSpace(corr.SegmentID) != "" {
				return strings.TrimSpace(corr.SegmentID)
			}
			if strings.TrimSpace(corr.CorrelationKey) != "" {
				return strings.TrimSpace(corr.CorrelationKey)
			}
			return entityID
		}
		reasoningEntityID := func(corr events.Correlation) string {
			if strings.TrimSpace(corr.SegmentID) != "" {
				return strings.TrimSpace(corr.SegmentID)
			}
			if strings.TrimSpace(corr.CorrelationKey) != "" {
				return strings.TrimSpace(corr.CorrelationKey)
			}
			return entityID + ":thinking"
		}

		attrib := attribFromExtra(md.Extra)

		persistMessage := func(id string, role string, content string, streaming bool) {
			// Watermill message contexts can be canceled unexpectedly (for example by ack/teardown ordering),
			// which can cause best-effort persistence to flake. Persist with a detached, bounded context.
			persistCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			props := map[string]any{
				"schemaVersion": 2,
				"role":          role,
				"content":       content,
				"streaming":     streaming,
			}
			for k, v := range attrib {
				props[k] = v
			}

			if err := upsertEntity(persistCtx, id, "message", props); err != nil {
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
		case *events.EventTextSegmentStarted:
			// Do not create an empty assistant entry on segment start.
			currentTextID = canonicalEntityID(e.Correlation())
			currentTextActive = true
		case *events.EventTextDelta:
			textID := canonicalEntityID(e.Correlation())
			currentTextID = textID
			currentTextActive = true
			if strings.TrimSpace(e.Text) == "" {
				break
			}
			mu.Lock()
			assistantSeen[textID] = true
			assistantContent[textID] = e.Text
			mu.Unlock()
			persistMessage(textID, "assistant", e.Text, true)
		case *events.EventTextSegmentFinished:
			textID := canonicalEntityID(e.Correlation())
			currentTextID = textID
			currentTextActive = false
			mu.Lock()
			seen := assistantSeen[textID]
			content := e.Text
			if strings.TrimSpace(content) == "" {
				content = assistantContent[textID]
			}
			if strings.TrimSpace(content) != "" {
				assistantSeen[textID] = true
				assistantContent[textID] = content
			}
			mu.Unlock()
			if strings.TrimSpace(content) == "" && !seen {
				break
			}
			persistMessage(textID, "assistant", content, false)
		case *events.EventInterrupt:
			textID := strings.TrimSpace(currentTextID)
			if textID == "" {
				textID = entityID
			}
			mu.Lock()
			seen := assistantSeen[textID]
			content := e.Text
			if strings.TrimSpace(content) == "" {
				content = assistantContent[textID]
			}
			if strings.TrimSpace(content) != "" {
				assistantSeen[textID] = true
				assistantContent[textID] = content
			}
			mu.Unlock()
			if strings.TrimSpace(content) == "" && !seen {
				break
			}
			currentTextActive = false
			persistMessage(textID, "assistant", content, false)
		case *events.EventError:
			textID := strings.TrimSpace(currentTextID)
			if currentTextActive && textID != "" {
				mu.Lock()
				seen := assistantSeen[textID]
				content := assistantContent[textID]
				mu.Unlock()
				if strings.TrimSpace(content) != "" || seen {
					currentTextActive = false
					persistMessage(textID, "assistant", content, false)
					break
				}
			}
			errText := "**Error**\n\n" + e.ErrorString
			mu.Lock()
			assistantSeen[entityID] = true
			assistantContent[entityID] = errText
			mu.Unlock()
			currentTextActive = false
			persistMessage(entityID, "assistant", errText, false)
		case *events.EventInfo:
			if strings.TrimSpace(e.Message) == "profile-switched" {
				from, _ := e.Data["from"].(string)
				to, _ := e.Data["to"].(string)
				props := map[string]any{
					"schemaVersion": 1,
					"from":          strings.TrimSpace(from),
					"to":            strings.TrimSpace(to),
				}
				for k, v := range attrib {
					props[k] = v
				}
				// Watermill message contexts can be canceled unexpectedly (ack/teardown ordering).
				// Persist with a detached, bounded context to keep best-effort storage stable.
				persistCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				// best-effort store as dedicated entity kind
				if err := upsertEntity(persistCtx, entityID, "profile_switch", props); err != nil {
					log.Warn().Err(err).
						Str("component", "timeline_persist").
						Str("conv_id", convID).
						Str("entity_id", entityID).
						Msg("timeline upsert failed (profile_switch)")
				}
				break
			}
		case *events.EventReasoningSegmentStarted:
			thinkID := reasoningEntityID(e.Correlation())
			mu.Lock()
			thinkingSeen[thinkID] = true
			content := thinkingContent[thinkID]
			mu.Unlock()
			persistMessage(thinkID, "thinking", content, true)
		case *events.EventReasoningDelta:
			thinkID := reasoningEntityID(e.Correlation())
			if strings.TrimSpace(e.Text) == "" {
				break
			}
			mu.Lock()
			thinkingSeen[thinkID] = true
			thinkingContent[thinkID] = e.Text
			mu.Unlock()
			persistMessage(thinkID, "thinking", e.Text, true)
		case *events.EventReasoningSegmentFinished:
			thinkID := reasoningEntityID(e.Correlation())
			mu.Lock()
			seen := thinkingSeen[thinkID]
			content := e.Text
			if strings.TrimSpace(content) == "" {
				content = thinkingContent[thinkID]
			}
			mu.Unlock()
			if !seen && strings.TrimSpace(content) == "" {
				break
			}
			persistMessage(thinkID, "thinking", content, false)
		}

		return nil
	}
}
