package evtstream

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	// DefaultEventBusTopic is the default Watermill topic used by evtstream.
	DefaultEventBusTopic = "evtstream.events"

	MetadataKeyEventName    = "evtstream_event_name"
	MetadataKeySessionID    = "evtstream_session_id"
	MetadataKeyPartitionKey = "evtstream_partition_key"
	MetadataKeyPublishedOrd = "evtstream_published_ordinal"
	MetadataKeyStreamID     = "evtstream_stream_id"
)

// PartitionKeyForSession returns the partition key that backends should use to preserve
// publish/consume order per SessionId.
func PartitionKeyForSession(sid SessionId) string {
	return string(sid)
}

type busConfig struct {
	publisher      message.Publisher
	subscriber     message.Subscriber
	topic          string
	messageMutator BusMessageMutator
	observer       BusObserver
}

// BusOption configures evtstream's Watermill integration.
type BusOption func(*busConfig) error

// BusMessageMutator can attach backend-specific metadata before publish.
// It is useful for test/lab setups that want to inject synthetic stream ids.
type BusMessageMutator func(ctx context.Context, ev Event, msg *message.Message) error

// BusRecord captures publish/consume metadata for a Watermill message.
type BusRecord struct {
	MessageID string            `json:"messageId"`
	Topic     string            `json:"topic"`
	Metadata  map[string]string `json:"metadata"`
}

// BusObserver receives publish/consume observations from the bus adapter and consumer.
type BusObserver interface {
	Published(ctx context.Context, ev Event, rec BusRecord)
	Consumed(ctx context.Context, ev Event, rec BusRecord)
}

// BusObserverHooks adapts callbacks to BusObserver.
type BusObserverHooks struct {
	OnPublished func(ctx context.Context, ev Event, rec BusRecord)
	OnConsumed  func(ctx context.Context, ev Event, rec BusRecord)
}

func (h BusObserverHooks) Published(ctx context.Context, ev Event, rec BusRecord) {
	if h.OnPublished != nil {
		h.OnPublished(ctx, ev, rec)
	}
}

func (h BusObserverHooks) Consumed(ctx context.Context, ev Event, rec BusRecord) {
	if h.OnConsumed != nil {
		h.OnConsumed(ctx, ev, rec)
	}
}

func WithBusTopic(topic string) BusOption {
	return func(cfg *busConfig) error {
		if topic == "" {
			return fmt.Errorf("event bus topic is empty")
		}
		cfg.topic = topic
		return nil
	}
}

func WithBusMessageMutator(mutator BusMessageMutator) BusOption {
	return func(cfg *busConfig) error {
		cfg.messageMutator = mutator
		return nil
	}
}

func WithBusObserver(observer BusObserver) BusOption {
	return func(cfg *busConfig) error {
		cfg.observer = observer
		return nil
	}
}

// WithEventBus configures a Watermill publisher/subscriber pair owned by the application.
func WithEventBus(pub message.Publisher, sub message.Subscriber, opts ...BusOption) HubOption {
	return func(h *Hub) error {
		if pub == nil {
			return fmt.Errorf("event bus publisher is nil")
		}
		if sub == nil {
			return fmt.Errorf("event bus subscriber is nil")
		}
		cfg := &busConfig{
			publisher:  pub,
			subscriber: sub,
			topic:      DefaultEventBusTopic,
		}
		for _, opt := range opts {
			if opt == nil {
				continue
			}
			if err := opt(cfg); err != nil {
				return err
			}
		}
		h.bus = cfg
		return nil
	}
}

type eventEnvelope struct {
	Name      string          `json:"name"`
	SessionID string          `json:"sessionId"`
	Payload   json.RawMessage `json:"payload"`
}

type watermillEventPublisher struct {
	hub *Hub
}

func (p watermillEventPublisher) Publish(ctx context.Context, ev Event) error {
	if p.hub == nil {
		return fmt.Errorf("hub is nil")
	}
	if p.hub.bus == nil {
		return fmt.Errorf("event bus is not configured")
	}
	if ev.SessionId == "" {
		return fmt.Errorf("event %q missing session id", ev.Name)
	}
	if ev.Payload == nil {
		return fmt.Errorf("event %q payload is nil", ev.Name)
	}
	if err := p.hub.validatePayloadType(p.hub.reg.events, "event", ev.Name, ev.Payload); err != nil {
		return err
	}

	payload, err := p.hub.reg.MarshalProtoJSON(ev.Payload)
	if err != nil {
		return fmt.Errorf("marshal event %q payload: %w", ev.Name, err)
	}
	body, err := json.Marshal(eventEnvelope{
		Name:      ev.Name,
		SessionID: string(ev.SessionId),
		Payload:   payload,
	})
	if err != nil {
		return fmt.Errorf("marshal event %q envelope: %w", ev.Name, err)
	}

	msg := message.NewMessage(uuid.NewString(), body)
	msg.SetContext(ctx)
	msg.Metadata.Set(MetadataKeyEventName, ev.Name)
	msg.Metadata.Set(MetadataKeySessionID, string(ev.SessionId))
	msg.Metadata.Set(MetadataKeyPartitionKey, PartitionKeyForSession(ev.SessionId))
	msg.Metadata.Set(MetadataKeyPublishedOrd, "0")
	if p.hub.bus.messageMutator != nil {
		if err := p.hub.bus.messageMutator(ctx, Event{Name: ev.Name, SessionId: ev.SessionId, Payload: proto.Clone(ev.Payload)}, msg); err != nil {
			return fmt.Errorf("mutate event %q message metadata: %w", ev.Name, err)
		}
	}
	if err := p.hub.bus.publisher.Publish(p.hub.bus.topic, msg); err != nil {
		return fmt.Errorf("publish event %q: %w", ev.Name, err)
	}
	if p.hub.bus.observer != nil {
		p.hub.bus.observer.Published(ctx, Event{Name: ev.Name, SessionId: ev.SessionId, Payload: proto.Clone(ev.Payload)}, newBusRecord(msg, p.hub.bus.topic))
	}
	return nil
}

func decodeEventEnvelope(reg *SchemaRegistry, payload []byte) (Event, error) {
	var env eventEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return Event{}, fmt.Errorf("decode event envelope: %w", err)
	}
	if env.Name == "" {
		return Event{}, fmt.Errorf("event envelope missing name")
	}
	if env.SessionID == "" {
		return Event{}, fmt.Errorf("event envelope missing session id")
	}
	msg, err := reg.instantiate(reg.events, "event", env.Name)
	if err != nil {
		return Event{}, err
	}
	if len(env.Payload) > 0 && string(env.Payload) != "null" {
		if err := protojson.Unmarshal(env.Payload, msg); err != nil {
			return Event{}, fmt.Errorf("decode event %q payload: %w", env.Name, err)
		}
	}
	return Event{Name: env.Name, SessionId: SessionId(env.SessionID), Payload: msg}, nil
}

func newBusRecord(msg *message.Message, topic string) BusRecord {
	rec := BusRecord{Topic: topic}
	if msg == nil {
		return rec
	}
	rec.MessageID = msg.UUID
	rec.Metadata = cloneWatermillMetadata(msg.Metadata)
	return rec
}

func cloneWatermillMetadata(metadata message.Metadata) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(metadata))
	for k, v := range metadata {
		out[k] = v
	}
	return out
}
