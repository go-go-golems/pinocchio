package webchat

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

// RunLoop is a backend loop strategy for a conversation.
type RunLoop func(ctx context.Context, eng engine.Engine, t *turns.Turn, reg geptools.ToolRegistry, opts map[string]any) (*turns.Turn, error)

// EventSinkWrapper allows callers to wrap or replace the default event sink.
type EventSinkWrapper func(convID string, req infruntime.ConversationRuntimeRequest, sink events.EventSink) (events.EventSink, error)

// Router wires HTTP endpoints, registries and conversation lifecycle.
type Router struct {
	baseCtx  context.Context
	parsed   *values.Values
	mux      *http.ServeMux
	staticFS fs.FS

	// event router (in-memory or Redis)
	router *events.EventRouter
	// stream backend abstraction for publisher/subscriber construction.
	streamBackend StreamBackend

	// registries
	mwFactories   map[string]infruntime.MiddlewareBuilder
	toolFactories map[string]infruntime.ToolRegistrar

	// shared deps
	db            *sql.DB
	timelineStore chatstore.TimelineStore
	// split timeline service surface for hydration reads.
	timelineService *TimelineService
	turnStore       chatstore.TurnStore

	// conversations
	cm *ConvManager
	// split service APIs.
	chatService *ChatService
	streamHub   *StreamHub

	// runtime flags
	idleTimeoutSec int
	// enableDebugRoutes controls registration of /api/debug/* handlers.
	// Default is false (debug routes disabled).
	enableDebugRoutes bool

	// step mode control (shared; not conversation-owned)
	stepCtrl *toolloop.StepController

	// app-owned runtime wiring
	runtimeComposer infruntime.RuntimeBuilder

	// optional overrides for conv manager hooks
	buildSubscriberOverride    func(convID string) (message.Subscriber, bool, error)
	timelineUpsertHookOverride func(*Conversation) func(entity *timelinepb.TimelineEntityV2, version uint64)

	// optional event sink wrapper
	eventSinkWrapper EventSinkWrapper
}
