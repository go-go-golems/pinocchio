package sessions

import (
	"context"
	"errors"

	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// RecordUsage returns a chatapp usage observer that adds each provider call's
// usage to the session's Info.Usage. Install it with
// chatapp.WithUsageObserver(sessions.RecordUsage(store)). Sessions that are not
// in the store are ignored; other store errors are logged.
func RecordUsage(store Store) chatapp.UsageObserver {
	return func(ctx context.Context, sid sessionstream.SessionId, u *chatappv1.UsageInfo) {
		if store == nil || u == nil {
			return
		}
		add := UsageFromProto(u)
		err := store.Update(ctx, string(sid), func(info *Info) error {
			info.Usage = info.Usage.Add(add)
			return nil
		})
		if err != nil && !errors.Is(err, ErrNotFound) {
			log.Warn().Err(err).Str("session", string(sid)).Msg("sessions: record usage")
		}
	}
}

// UsageFromProto converts one provider call's chatapp usage, counting it as
// one call.
func UsageFromProto(u *chatappv1.UsageInfo) Usage {
	if u == nil {
		return Usage{}
	}
	return Usage{
		Input:         int(u.GetInputTokens()),
		Output:        int(u.GetOutputTokens()),
		Cached:        int(u.GetCachedTokens()),
		CacheCreation: int(u.GetCacheCreationInputTokens()),
		CacheRead:     int(u.GetCacheReadInputTokens()),
		Calls:         1,
	}
}
