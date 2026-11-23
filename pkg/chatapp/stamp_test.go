package chatapp

import (
	"testing"
	"time"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	"github.com/stretchr/testify/require"
)

func TestStampTimesFillsOnlyUnsetTimeFields(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	started := &chatappv1.ChatRunStarted{MessageId: "m"}
	StampTimes(started, now)
	require.Equal(t, now.UnixMilli(), started.GetStartedAtMs())

	kept := &chatappv1.ChatRunFinished{FinishedAtMs: 42}
	StampTimes(kept, now)
	require.Equal(t, int64(42), kept.GetFinishedAtMs(), "a time set by the publisher stays")

	patch := &chatappv1.ChatTextPatch{Text: "x"}
	StampTimes(patch, now)
	require.Equal(t, now.UnixMilli(), patch.GetAtMs())

	StampTimes(nil, now) // no payload: nothing to do
}
