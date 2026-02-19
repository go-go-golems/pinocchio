package webchat

import (
	"context"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"google.golang.org/protobuf/encoding/protojson"
)

func registerBuiltinTimelineHandlers() {
	RegisterTimelineHandler("chat.message", builtinChatMessageTimelineHandler)
}

func builtinChatMessageTimelineHandler(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, _ int64) error {
	var msg timelinepb.MessageSnapshotV1
	if err := protojson.Unmarshal(ev.Data, &msg); err != nil {
		return nil
	}
	if msg.SchemaVersion == 0 {
		msg.SchemaVersion = 1
	}
	return p.Upsert(ctx, ev.Seq, timelineEntityV2FromProtoMessage(ev.ID, "message", &msg))
}
