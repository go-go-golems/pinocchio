package webchat

import (
	"database/sql"
	"errors"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/gorilla/websocket"
)

// RouterOption configures optional dependencies for a Router.
type RouterOption func(*Router) error

func WithConversationRequestResolver(resolver ConversationRequestResolver) RouterOption {
	return func(r *Router) error {
		if resolver == nil {
			return errors.New("conversation request resolver is nil")
		}
		r.requestResolver = resolver
		return nil
	}
}

func WithWebSocketUpgrader(u websocket.Upgrader) RouterOption {
	return func(r *Router) error {
		r.upgrader = u
		return nil
	}
}

func WithProfileRegistry(reg ProfileRegistry) RouterOption {
	return func(r *Router) error {
		if reg == nil {
			return errors.New("profile registry is nil")
		}
		r.profiles = reg
		return nil
	}
}

func WithConvManager(cm *ConvManager) RouterOption {
	return func(r *Router) error {
		if cm == nil {
			return errors.New("conv manager is nil")
		}
		r.cm = cm
		return nil
	}
}

func WithEventRouter(er *events.EventRouter) RouterOption {
	return func(r *Router) error {
		if er == nil {
			return errors.New("event router is nil")
		}
		r.router = er
		return nil
	}
}

func WithBuildSubscriber(fn func(convID string) (message.Subscriber, bool, error)) RouterOption {
	return func(r *Router) error {
		if fn == nil {
			return errors.New("build subscriber is nil")
		}
		r.buildSubscriberOverride = fn
		return nil
	}
}

func WithTimelineUpsertHook(fn func(*Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64)) RouterOption {
	return func(r *Router) error {
		if fn == nil {
			return errors.New("timeline upsert hook is nil")
		}
		r.timelineUpsertHookOverride = fn
		return nil
	}
}

func WithStepController(sc *toolloop.StepController) RouterOption {
	return func(r *Router) error {
		if sc == nil {
			return errors.New("step controller is nil")
		}
		r.stepCtrl = sc
		return nil
	}
}

func WithDB(db *sql.DB) RouterOption {
	return func(r *Router) error {
		r.db = db
		return nil
	}
}

func WithTimelineStore(s chatstore.TimelineStore) RouterOption {
	return func(r *Router) error {
		if s == nil {
			return errors.New("timeline store is nil")
		}
		r.timelineStore = s
		if r.cm != nil {
			r.cm.SetTimelineStore(s)
		}
		return nil
	}
}

// WithTurnStore configures a durable turn snapshot store.
func WithTurnStore(s chatstore.TurnStore) RouterOption {
	return func(r *Router) error {
		if s == nil {
			return errors.New("turn store is nil")
		}
		r.turnStore = s
		return nil
	}
}

// WithEventSinkWrapper allows callers to wrap the default event sink (e.g., FilteringSink).
func WithEventSinkWrapper(fn EventSinkWrapper) RouterOption {
	return func(r *Router) error {
		if fn == nil {
			return errors.New("event sink wrapper is nil")
		}
		r.eventSinkWrapper = fn
		return nil
	}
}

// WithDebugRoutesEnabled toggles registration of debug API endpoints.
// When disabled, /api/debug/* and legacy /debug/* routes are not mounted.
func WithDebugRoutesEnabled(enabled bool) RouterOption {
	return func(r *Router) error {
		r.disableDebugRoutes = !enabled
		return nil
	}
}
