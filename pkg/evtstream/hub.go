package evtstream

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
)

// ProjectionErrorPolicy controls how local in-memory event processing reacts to projection failures.
type ProjectionErrorPolicy int

const (
	// ProjectionErrorPolicyAdvance advances the cursor even if a projection fails.
	ProjectionErrorPolicyAdvance ProjectionErrorPolicy = iota
	// ProjectionErrorPolicyFail stops processing and returns the projection error.
	ProjectionErrorPolicyFail
)

// Hub is the phase-0/1 substrate entrypoint.
type Hub struct {
	reg      *SchemaRegistry
	store    HydrationStore
	sessions *sessionRegistry
	commands *commandRegistry

	uiProjection       UIProjection
	timelineProjection TimelineProjection

	projectionPolicy ProjectionErrorPolicy

	mu           sync.Mutex
	localOrdinal map[SessionId]uint64
}

// HubOption configures a Hub.
type HubOption func(*Hub) error

func WithSchemaRegistry(r *SchemaRegistry) HubOption {
	return func(h *Hub) error {
		if r == nil {
			return fmt.Errorf("schema registry is nil")
		}
		h.reg = r
		return nil
	}
}

func WithHydrationStore(s HydrationStore) HubOption {
	return func(h *Hub) error {
		if s == nil {
			return fmt.Errorf("hydration store is nil")
		}
		h.store = s
		return nil
	}
}

func WithSessionMetadataFactory(f SessionMetadataFactory) HubOption {
	return func(h *Hub) error {
		h.sessions = newSessionRegistry(f)
		return nil
	}
}

func WithProjectionErrorPolicy(policy ProjectionErrorPolicy) HubOption {
	return func(h *Hub) error {
		h.projectionPolicy = policy
		return nil
	}
}

func NewHub(opts ...HubOption) (*Hub, error) {
	h := &Hub{
		reg:              NewSchemaRegistry(),
		store:            newNoopHydrationStore(),
		sessions:         newSessionRegistry(nil),
		commands:         newCommandRegistry(),
		projectionPolicy: ProjectionErrorPolicyAdvance,
		localOrdinal:     map[SessionId]uint64{},
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(h); err != nil {
			return nil, err
		}
	}
	return h, nil
}

func (h *Hub) RegisterCommand(name string, handler CommandHandler) error {
	if h == nil {
		return fmt.Errorf("hub is nil")
	}
	return h.commands.Register(name, handler)
}

func (h *Hub) RegisterUIProjection(p UIProjection) error {
	if h == nil {
		return fmt.Errorf("hub is nil")
	}
	if p == nil {
		return fmt.Errorf("ui projection is nil")
	}
	if h.uiProjection != nil {
		return fmt.Errorf("ui projection already registered")
	}
	h.uiProjection = p
	return nil
}

func (h *Hub) RegisterTimelineProjection(p TimelineProjection) error {
	if h == nil {
		return fmt.Errorf("hub is nil")
	}
	if p == nil {
		return fmt.Errorf("timeline projection is nil")
	}
	if h.timelineProjection != nil {
		return fmt.Errorf("timeline projection already registered")
	}
	h.timelineProjection = p
	return nil
}

// Submit executes a command through the in-memory phase-1 path.
func (h *Hub) Submit(ctx context.Context, sid SessionId, name string, payload proto.Message) error {
	if h == nil {
		return fmt.Errorf("hub is nil")
	}
	if payload == nil {
		return fmt.Errorf("submit payload for %q is nil", name)
	}
	if sid == "" {
		return fmt.Errorf("session id is empty")
	}
	if err := h.validatePayloadType(h.reg.commands, "command", name, payload); err != nil {
		return err
	}
	cmd := Command{Name: name, SessionId: sid, Payload: payload}
	return h.dispatch(ctx, cmd)
}

func (h *Hub) Snapshot(ctx context.Context, sid SessionId) (Snapshot, error) {
	if h == nil {
		return Snapshot{}, fmt.Errorf("hub is nil")
	}
	return h.store.Snapshot(ctx, sid, 0)
}

func (h *Hub) Session(ctx context.Context, sid SessionId) (*Session, error) {
	if h == nil {
		return nil, fmt.Errorf("hub is nil")
	}
	return h.sessions.GetOrCreate(ctx, sid)
}

func (h *Hub) Cursor(ctx context.Context, sid SessionId) (uint64, error) {
	if h == nil {
		return 0, fmt.Errorf("hub is nil")
	}
	return h.store.Cursor(ctx, sid)
}

func (h *Hub) dispatch(ctx context.Context, cmd Command) error {
	handler, ok := h.commands.Lookup(cmd.Name)
	if !ok {
		return fmt.Errorf("unknown command %q", cmd.Name)
	}
	sess, err := h.sessions.GetOrCreate(ctx, cmd.SessionId)
	if err != nil {
		return err
	}
	pub := localEventPublisher{hub: h}
	return handler(ctx, cmd, sess, pub)
}

type localEventPublisher struct {
	hub *Hub
}

func (p localEventPublisher) Publish(ctx context.Context, ev Event) error {
	if p.hub == nil {
		return fmt.Errorf("hub is nil")
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
	return p.hub.processEvent(ctx, ev)
}

func (h *Hub) processEvent(ctx context.Context, ev Event) error {
	ord := h.nextLocalOrdinal(ev.SessionId)
	ev.Ordinal = ord
	sess, err := h.sessions.GetOrCreate(ctx, ev.SessionId)
	if err != nil {
		return err
	}
	view, err := h.store.View(ctx, ev.SessionId)
	if err != nil {
		return err
	}

	var (
		entities []TimelineEntity
		uiErr    error
		tlErr    error
	)
	if h.uiProjection != nil {
		_, uiErr = h.uiProjection.Project(ctx, ev, sess, view)
	}
	if h.timelineProjection != nil {
		entities, tlErr = h.timelineProjection.Project(ctx, ev, sess, view)
	}

	if h.projectionPolicy == ProjectionErrorPolicyFail {
		if uiErr != nil {
			return uiErr
		}
		if tlErr != nil {
			return tlErr
		}
	}

	if err := h.store.Apply(ctx, ev.SessionId, ord, entities); err != nil {
		return err
	}
	return nil
}

func (h *Hub) nextLocalOrdinal(sid SessionId) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.localOrdinal[sid]++
	return h.localOrdinal[sid]
}

func (h *Hub) validatePayloadType(m map[string]proto.Message, kind, name string, payload proto.Message) error {
	prototype, ok := h.reg.lookup(m, name)
	if !ok {
		return fmt.Errorf("unknown %s %q", kind, name)
	}
	if prototype.ProtoReflect().Descriptor().FullName() != payload.ProtoReflect().Descriptor().FullName() {
		return fmt.Errorf("%s %q payload type mismatch: got %s want %s", kind, name, payload.ProtoReflect().Descriptor().FullName(), prototype.ProtoReflect().Descriptor().FullName())
	}
	return nil
}
