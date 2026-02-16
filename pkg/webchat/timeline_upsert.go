package webchat

import timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"

// TimelineUpsertHook exposes the timeline upsert hook for external use.
func (r *Router) TimelineUpsertHook(conv *Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64) {
	if r != nil && r.timelineUpsertHookOverride != nil {
		return r.timelineUpsertHookOverride(conv)
	}
	return r.timelineUpsertHookDefault(conv)
}

func (r *Router) timelineUpsertHookDefault(conv *Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64) {
	if r == nil || conv == nil {
		return nil
	}
	return func(entity *timelinepb.TimelineEntityV1, version uint64) {
		r.emitTimelineUpsert(conv, entity, version)
	}
}

func (r *Router) emitTimelineUpsert(conv *Conversation, entity *timelinepb.TimelineEntityV1, version uint64) {
	if r == nil || conv == nil || entity == nil {
		return
	}
	payload, err := protoToRaw(&timelinepb.TimelineUpsertV1{
		ConvId:  conv.ID,
		Version: version,
		Entity:  entity,
	})
	if err != nil {
		return
	}
	env := map[string]any{
		"sem": true,
		"event": map[string]any{
			"type": "timeline.upsert",
			"id":   entity.Id,
			"seq":  version,
			"data": payload,
		},
	}
	if r.cm != nil {
		_ = NewWSPublisher(r.cm).PublishJSON(r.baseCtx, conv.ID, env)
	}
}
