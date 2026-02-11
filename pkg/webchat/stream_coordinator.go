package webchat

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/events"
)

type StreamCursor struct {
	StreamID string
	Seq      uint64
}

// StreamCoordinator owns the subscriber that feeds events, translates them into SEM frames,
// and dispatches callbacks in-order.
type StreamCoordinator struct {
	convID     string
	subscriber message.Subscriber

	onEvent func(events.Event, StreamCursor)
	onFrame func(events.Event, StreamCursor, []byte)

	seq atomic.Uint64

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

func NewStreamCoordinator(
	convID string,
	subscriber message.Subscriber,
	onEvent func(events.Event, StreamCursor),
	onFrame func(events.Event, StreamCursor, []byte),
) *StreamCoordinator {
	return &StreamCoordinator{
		convID:     convID,
		subscriber: subscriber,
		onEvent:    onEvent,
		onFrame:    onFrame,
	}
}

func (sc *StreamCoordinator) Start(ctx context.Context) error {
	if sc == nil || sc.subscriber == nil {
		return nil
	}
	sc.mu.Lock()
	if sc.running {
		sc.mu.Unlock()
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	sc.cancel = cancel
	sc.running = true
	sc.mu.Unlock()

	go sc.consume(runCtx)
	return nil
}

func (sc *StreamCoordinator) Stop() {
	if sc == nil {
		return
	}
	sc.mu.Lock()
	if sc.cancel != nil {
		sc.cancel()
	}
	sc.cancel = nil
	sc.running = false
	sc.mu.Unlock()
}

func (sc *StreamCoordinator) Close() {
	if sc == nil {
		return
	}
	sc.Stop()
	if sc.subscriber != nil {
		if err := sc.subscriber.Close(); err != nil {
			log.Warn().Err(err).Str("component", "webchat").Str("conv_id", sc.convID).Msg("stream coordinator: subscriber close failed")
		}
	}
}

func (sc *StreamCoordinator) IsRunning() bool {
	if sc == nil {
		return false
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.running
}

func (sc *StreamCoordinator) consume(ctx context.Context) {
	ch, err := sc.subscriber.Subscribe(ctx, topicForConv(sc.convID))
	if err != nil {
		log.Error().Err(err).Str("component", "webchat").Str("conv_id", sc.convID).Msg("stream coordinator: subscribe failed")
		sc.Stop()
		return
	}
	log.Info().Str("component", "webchat").Str("conv_id", sc.convID).Msg("stream coordinator: started")
	for msg := range ch {
		ev, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			log.Warn().Err(err).Str("component", "webchat").Str("conv_id", sc.convID).Msg("stream coordinator: failed to decode event")
			msg.Ack()
			continue
		}

		streamID := extractStreamID(msg)
		seq := sc.nextSeq(streamID)

		cur := StreamCursor{
			StreamID: streamID,
			Seq:      seq,
		}

		if sc.onEvent != nil {
			sc.onEvent(ev, cur)
		}
		if sc.onFrame != nil {
			for _, frame := range SemanticEventsFromEventWithCursor(ev, cur) {
				if len(frame) == 0 {
					continue
				}
				sc.onFrame(ev, cur, frame)
			}
		}
		msg.Ack()
	}
	log.Info().Str("component", "webchat").Str("conv_id", sc.convID).Msg("stream coordinator: stopped")
	sc.mu.Lock()
	sc.running = false
	sc.cancel = nil
	sc.mu.Unlock()
}

func (sc *StreamCoordinator) nextSeq(streamID string) uint64 {
	if streamID != "" {
		if derived, ok := deriveSeqFromStreamID(streamID); ok {
			for {
				current := sc.seq.Load()
				next := derived
				if next <= current {
					next = current + 1
				}
				if sc.seq.CompareAndSwap(current, next) {
					return next
				}
			}
		}
	}
	for {
		current := sc.seq.Load()
		now := uint64(time.Now().UnixMilli()) * 1_000_000
		next := now
		if next <= current {
			next = current + 1
		}
		if sc.seq.CompareAndSwap(current, next) {
			return next
		}
	}
}

func extractStreamID(msg *message.Message) string {
	if msg == nil || msg.Metadata == nil {
		return ""
	}
	keys := []string{"xid", "redis_xid"}
	for _, k := range keys {
		if v := msg.Metadata.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func deriveSeqFromStreamID(streamID string) (uint64, bool) {
	parts := strings.Split(streamID, "-")
	if len(parts) != 2 {
		return 0, false
	}
	ms, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	seq, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return ms*1_000_000 + seq, true
}

func SemanticEventsFromEventWithCursor(e events.Event, cur StreamCursor) [][]byte {
	frames := SemanticEventsFromEvent(e)
	if len(frames) == 0 {
		return nil
	}
	out := make([][]byte, 0, len(frames))
	for _, b := range frames {
		if len(b) == 0 {
			continue
		}
		var env map[string]any
		if err := json.Unmarshal(b, &env); err != nil {
			out = append(out, b)
			continue
		}
		evObj, _ := env["event"].(map[string]any)
		if evObj == nil {
			out = append(out, b)
			continue
		}
		evObj["seq"] = cur.Seq
		if cur.StreamID != "" {
			evObj["stream_id"] = cur.StreamID
		}
		env["event"] = evObj
		rebuilt, err := json.Marshal(env)
		if err != nil {
			out = append(out, b)
			continue
		}
		out = append(out, rebuilt)
	}
	return out
}
