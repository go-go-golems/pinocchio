package evtstream

import (
	"fmt"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// SchemaRegistry stores prototype protobuf messages keyed by logical names.
type SchemaRegistry struct {
	mu       sync.RWMutex
	commands map[string]proto.Message
	events   map[string]proto.Message
	uiEvents map[string]proto.Message
	entities map[string]proto.Message
}

func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		commands: map[string]proto.Message{},
		events:   map[string]proto.Message{},
		uiEvents: map[string]proto.Message{},
		entities: map[string]proto.Message{},
	}
}

func (r *SchemaRegistry) RegisterCommand(name string, msg proto.Message) error {
	return r.register(r.commands, "command", name, msg)
}

func (r *SchemaRegistry) RegisterEvent(name string, msg proto.Message) error {
	return r.register(r.events, "event", name, msg)
}

func (r *SchemaRegistry) RegisterUIEvent(name string, msg proto.Message) error {
	return r.register(r.uiEvents, "ui event", name, msg)
}

func (r *SchemaRegistry) RegisterTimelineEntity(kind string, msg proto.Message) error {
	return r.register(r.entities, "timeline entity", kind, msg)
}

func (r *SchemaRegistry) CommandSchema(name string) (proto.Message, bool) {
	return r.lookup(r.commands, name)
}

func (r *SchemaRegistry) EventSchema(name string) (proto.Message, bool) {
	return r.lookup(r.events, name)
}

func (r *SchemaRegistry) UIEventSchema(name string) (proto.Message, bool) {
	return r.lookup(r.uiEvents, name)
}

func (r *SchemaRegistry) TimelineEntitySchema(kind string) (proto.Message, bool) {
	return r.lookup(r.entities, kind)
}

func (r *SchemaRegistry) DecodeCommandJSON(name string, payload []byte) (proto.Message, error) {
	msg, err := r.instantiate(r.commands, "command", name)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return msg, nil
	}
	if err := protojson.Unmarshal(payload, msg); err != nil {
		return nil, fmt.Errorf("decode command %q: %w", name, err)
	}
	return msg, nil
}

func (r *SchemaRegistry) MarshalProtoJSON(msg proto.Message) ([]byte, error) {
	if msg == nil {
		return []byte("null"), nil
	}
	return protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   false,
	}.Marshal(msg)
}

func (r *SchemaRegistry) register(m map[string]proto.Message, kind, name string, msg proto.Message) error {
	if r == nil {
		return fmt.Errorf("schema registry is nil")
	}
	if name == "" {
		return fmt.Errorf("%s name is empty", kind)
	}
	if msg == nil {
		return fmt.Errorf("%s %q message is nil", kind, name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := m[name]; ok {
		return fmt.Errorf("%s %q already registered", kind, name)
	}
	m[name] = msg
	return nil
}

func (r *SchemaRegistry) lookup(m map[string]proto.Message, name string) (proto.Message, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	msg, ok := m[name]
	if !ok {
		return nil, false
	}
	return msg, true
}

func (r *SchemaRegistry) instantiate(m map[string]proto.Message, kind, name string) (proto.Message, error) {
	if r == nil {
		return nil, fmt.Errorf("schema registry is nil")
	}
	r.mu.RLock()
	prototype, ok := m[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown %s %q", kind, name)
	}
	return prototype.ProtoReflect().New().Interface(), nil
}
