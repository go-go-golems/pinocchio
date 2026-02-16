package webchat

import (
	"context"
	"testing"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/stretchr/testify/require"
)

func TestTimelineService_Snapshot(t *testing.T) {
	store := chatstore.NewInMemoryTimelineStore(10)
	t.Cleanup(func() { _ = store.Close() })

	err := store.Upsert(context.Background(), "c1", 1, &timelinepb.TimelineEntityV1{
		Id:   "m1",
		Kind: "message",
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{
				Role:    "user",
				Content: "hello",
			},
		},
	})
	require.NoError(t, err)

	svc := NewTimelineService(store)
	snap, err := svc.Snapshot(context.Background(), "c1", 0, 10)
	require.NoError(t, err)
	require.Equal(t, uint64(1), snap.Version)
	require.Len(t, snap.Entities, 1)
}

func TestTimelineService_SnapshotFailsWhenDisabled(t *testing.T) {
	svc := NewTimelineService(nil)
	_, err := svc.Snapshot(context.Background(), "c1", 0, 10)
	require.ErrorContains(t, err, "not enabled")
}
