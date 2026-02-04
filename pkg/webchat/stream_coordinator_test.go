package webchat

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/require"

	"github.com/go-go-golems/geppetto/pkg/events"
)

type stubSubscriber struct {
	ch chan *message.Message
}

func (s *stubSubscriber) Subscribe(_ context.Context, _ string) (<-chan *message.Message, error) {
	return s.ch, nil
}

func (s *stubSubscriber) Close() error {
	close(s.ch)
	return nil
}

func TestDeriveSeqFromStreamID(t *testing.T) {
	seq, ok := deriveSeqFromStreamID("1700000000000-2")
	require.True(t, ok)
	require.Equal(t, uint64(1700000000000*1_000_000+2), seq)

	_, ok = deriveSeqFromStreamID("bad")
	require.False(t, ok)
}

func TestStreamCoordinator_UsesStreamIDSequence(t *testing.T) {
	ch := make(chan *message.Message, 1)
	sub := &stubSubscriber{ch: ch}
	seqCh := make(chan uint64, 2)

	sc := NewStreamCoordinator("c1", sub, nil, func(_ events.Event, cur StreamCursor, _ []byte) {
		seqCh <- cur.Seq
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, sc.Start(ctx))

	msg := message.NewMessage("1", []byte(`{"type":"log","level":"info","message":"hi"}`))
	msg.Metadata.Set("xid", "1700000000000-2")
	ch <- msg
	close(ch)

	select {
	case got := <-seqCh:
		require.Equal(t, uint64(1700000000000*1_000_000+2), got)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for stream coordinator")
	}
}

func TestStreamCoordinator_FallsBackToLocalSeq(t *testing.T) {
	ch := make(chan *message.Message, 1)
	sub := &stubSubscriber{ch: ch}
	seqCh := make(chan uint64, 2)

	sc := NewStreamCoordinator("c1", sub, nil, func(_ events.Event, cur StreamCursor, _ []byte) {
		seqCh <- cur.Seq
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, sc.Start(ctx))

	minBase := uint64(time.Now().UnixMilli()) * 1_000_000
	msg := message.NewMessage("1", []byte(`{"type":"log","level":"info","message":"hi"}`))
	msg.Metadata.Set("xid", "bad")
	ch <- msg
	close(ch)

	select {
	case got := <-seqCh:
		require.GreaterOrEqual(t, got, minBase)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for stream coordinator")
	}
}
