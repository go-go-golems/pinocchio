package webchat

import (
	"context"
	"database/sql"
	"embed"
	"net/http"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/gorilla/websocket"
)

// MiddlewareFactory creates a middleware instance from an arbitrary config object.
type MiddlewareFactory func(cfg any) middleware.Middleware

// ToolFactory registers a tool into a registry.
type ToolFactory func(reg geptools.ToolRegistry) error

// RunLoop is a backend loop strategy for a conversation.
type RunLoop func(ctx context.Context, eng engine.Engine, t *turns.Turn, reg geptools.ToolRegistry, opts map[string]any) (*turns.Turn, error)

// MiddlewareUse declares a middleware to attach and its config.
type MiddlewareUse struct {
	Name   string
	Config any
}

// Profile describes how to build engines and run loops for a chat namespace.
type Profile struct {
	Slug           string
	DefaultPrompt  string
	DefaultTools   []string
	DefaultMws     []MiddlewareUse
	LoopName       string
	AllowOverrides bool
}

// ProfileRegistry stores profiles by slug.
type ProfileRegistry interface {
	Add(p *Profile) error
	Get(slug string) (*Profile, bool)
	List() []*Profile
}

// simple in-memory implementation
type inMemoryProfileRegistry struct{ profiles map[string]*Profile }

func newInMemoryProfileRegistry() *inMemoryProfileRegistry {
	return &inMemoryProfileRegistry{profiles: map[string]*Profile{}}
}
func (r *inMemoryProfileRegistry) Add(p *Profile) error { r.profiles[p.Slug] = p; return nil }
func (r *inMemoryProfileRegistry) Get(slug string) (*Profile, bool) {
	p, ok := r.profiles[slug]
	return p, ok
}
func (r *inMemoryProfileRegistry) List() []*Profile {
	out := make([]*Profile, 0, len(r.profiles))
	for _, p := range r.profiles {
		out = append(out, p)
	}
	return out
}

// Router wires HTTP, profiles, registries and conversation lifecycle.
type Router struct {
	baseCtx  context.Context
	parsed   *layers.ParsedLayers
	mux      *http.ServeMux
	staticFS embed.FS

	// event router (in-memory or Redis)
	router *events.EventRouter

	// registries
	mwFactories   map[string]MiddlewareFactory
	toolFactories map[string]ToolFactory

	// shared deps
	db *sql.DB

	// profiles
	profiles ProfileRegistry

	// ws
	upgrader websocket.Upgrader

	// conversations
	cm *ConvManager

	// runtime flags
	usesRedis      bool
	redisAddr      string
	idleTimeoutSec int

	// step mode control (shared; not conversation-owned)
	stepCtrl *toolloop.StepController

	// request policy
	engineFromReqBuilder EngineFromReqBuilder
}
