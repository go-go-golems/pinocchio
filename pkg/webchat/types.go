package webchat

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/gorilla/websocket"
)

// MiddlewareFactory creates a middleware instance from an arbitrary config object.
type MiddlewareFactory func(cfg any) middleware.Middleware

// ToolFactory registers a tool into a registry.
type ToolFactory func(reg geptools.ToolRegistry) error

// RunLoop is a backend loop strategy for a conversation.
type RunLoop func(ctx context.Context, eng engine.Engine, t *turns.Turn, reg geptools.ToolRegistry, opts map[string]any) (*turns.Turn, error)

// EventSinkWrapper allows callers to wrap or replace the default event sink.
type EventSinkWrapper func(convID string, req RuntimeComposeRequest, sink events.EventSink) (events.EventSink, error)

// MiddlewareUse declares a middleware to attach and its config.
type MiddlewareUse struct {
	Name   string
	Config any
}

// Router wires HTTP endpoints, registries and conversation lifecycle.
type Router struct {
	baseCtx  context.Context
	parsed   *values.Values
	mux      *http.ServeMux
	staticFS fs.FS

	// event router (in-memory or Redis)
	router *events.EventRouter

	// registries
	mwFactories   map[string]MiddlewareFactory
	toolFactories map[string]ToolFactory

	// shared deps
	db            *sql.DB
	timelineStore chatstore.TimelineStore
	turnStore     chatstore.TurnStore

	// ws
	upgrader websocket.Upgrader

	// conversations
	cm *ConvManager

	// runtime flags
	usesRedis      bool
	redisAddr      string
	idleTimeoutSec int
	// disableDebugRoutes disables registration of /api/debug/* and legacy /debug/* handlers.
	// Default is false (debug routes enabled).
	disableDebugRoutes bool

	// step mode control (shared; not conversation-owned)
	stepCtrl *toolloop.StepController

	// request policy
	requestResolver ConversationRequestResolver
	runtimeComposer RuntimeComposer

	// optional overrides for conv manager hooks
	buildSubscriberOverride    func(convID string) (message.Subscriber, bool, error)
	timelineUpsertHookOverride func(*Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64)

	// optional event sink wrapper
	eventSinkWrapper EventSinkWrapper
}
