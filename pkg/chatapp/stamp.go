package chatapp

import (
	"context"
	"time"

	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// timeFields are the event fields Stamped fills: when something happened
// (at_ms), when a run started (started_at_ms) and when it ended
// (finished_at_ms). All are milliseconds since the epoch, server clock.
var timeFields = []protoreflect.Name{"at_ms", "started_at_ms", "finished_at_ms"}

// Stamped returns a publisher that sets every unset time field of an event's
// payload from the server's clock, then publishes it. chatapp and
// frontendtools wrap the publishers they are given with it, so an event's time
// is fixed when it is published; projections copy it into entities and never
// read a clock, which keeps a replayed timeline identical.
func Stamped(pub sessionstream.EventPublisher) sessionstream.EventPublisher {
	if pub == nil {
		return nil
	}
	if _, ok := pub.(stampedPublisher); ok {
		return pub
	}
	return stampedPublisher{next: pub, now: time.Now}
}

type stampedPublisher struct {
	next sessionstream.EventPublisher
	now  func() time.Time
}

func (p stampedPublisher) Publish(ctx context.Context, ev sessionstream.Event) error {
	StampTimes(ev.Payload, p.now())
	return p.next.Publish(ctx, ev)
}

// StampTimes sets msg's unset time fields (at_ms, started_at_ms,
// finished_at_ms) to now.
func StampTimes(msg proto.Message, now time.Time) {
	if msg == nil {
		return
	}
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()
	ms := now.UnixMilli()
	for _, name := range timeFields {
		fd := fields.ByName(name)
		if fd == nil || fd.Kind() != protoreflect.Int64Kind || fd.Cardinality() == protoreflect.Repeated {
			continue
		}
		if m.Get(fd).Int() == 0 {
			m.Set(fd, protoreflect.ValueOfInt64(ms))
		}
	}
}
