package chatapp

import (
	"context"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// UIEventTransformer transforms already-projected UI events immediately before
// they are published to a UI fanout. It operates on derived delivery events,
// not canonical backend events or timeline entities.
type UIEventTransformer interface {
	TransformUIEvents(context.Context, sessionstream.Event, []sessionstream.UIEvent) ([]sessionstream.UIEvent, error)
}

// UIEventTransformerFunc adapts a function to UIEventTransformer.
type UIEventTransformerFunc func(context.Context, sessionstream.Event, []sessionstream.UIEvent) ([]sessionstream.UIEvent, error)

func (f UIEventTransformerFunc) TransformUIEvents(ctx context.Context, source sessionstream.Event, events []sessionstream.UIEvent) ([]sessionstream.UIEvent, error) {
	return f(ctx, source, events)
}

// CompactChatTextDeltaTransformer replaces rich ChatTextPatch UI events with
// compact ChatTextDelta payloads. Other UI events pass through unchanged.
// Canonical ChatTextPatch events remain available to the timeline projection.
func CompactChatTextDeltaTransformer() UIEventTransformer {
	return UIEventTransformerFunc(func(_ context.Context, _ sessionstream.Event, events []sessionstream.UIEvent) ([]sessionstream.UIEvent, error) {
		if len(events) == 0 {
			return nil, nil
		}
		out := make([]sessionstream.UIEvent, 0, len(events))
		for _, event := range events {
			if event.Name != EventChatTextPatch {
				out = append(out, event)
				continue
			}
			patch, ok := event.Payload.(*chatappv1.ChatTextPatch)
			if !ok || patch == nil {
				out = append(out, event)
				continue
			}
			out = append(out, sessionstream.UIEvent{
				Name: UIEventChatTextDelta,
				Payload: &chatappv1.ChatTextDelta{
					MessageId: patch.GetMessageId(),
					Text:      patch.GetText(),
					Mode:      patch.GetMode(),
					Final:     patch.GetFinal(),
				},
			})
		}
		return out, nil
	})
}

func applyUIEventTransformers(ctx context.Context, source sessionstream.Event, events []sessionstream.UIEvent, transformers []UIEventTransformer) ([]sessionstream.UIEvent, error) {
	current := events
	for _, transformer := range transformers {
		if transformer == nil {
			continue
		}
		next, err := transformer.TransformUIEvents(ctx, source, current)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}
