// Package sessionguard is the only way httpapi's request handlers reach
// session metadata. A Guard holds the store; handlers get an Access bound to
// the request's principal, and every Access operation authorizes before it
// touches the store. A Session (the result of an authorized Open or Create)
// is what Update and Delete act on, so a handler cannot mutate a session it
// was not authorized for: there is no method that takes a bare id.
package sessionguard

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
)

// Principal is who makes a request. An empty ID is anonymous.
type Principal struct {
	ID    string
	Attrs map[string]string
}

// Action is what a request does to a session.
type Action string

const (
	ActionCreate Action = "create" // session is nil
	ActionList   Action = "list"   // session is nil
	ActionRead   Action = "read"   // snapshot, events, websocket, exports
	ActionSend   Action = "send"   // a message
	ActionStop   Action = "stop"
	ActionDelete Action = "delete"
	ActionTools  Action = "tools" // browser-tool manifest and results
)

// Authorizer allows or denies an action. s is nil for ActionCreate and
// ActionList.
type Authorizer func(ctx context.Context, p Principal, a Action, s *sessions.Info) error

// CreateHook runs before a session is stored; it may change everything but
// the id and the owner.
type CreateHook func(ctx context.Context, p Principal, info *sessions.Info) error

// ForbiddenError is an Authorizer's denial (or a Session used for an action
// it was not opened for).
type ForbiddenError struct{ Err error }

func (e ForbiddenError) Error() string { return e.Err.Error() }
func (e ForbiddenError) Unwrap() error { return e.Err }

// CreateRejectedError is a CreateHook's refusal.
type CreateRejectedError struct{ Err error }

func (e CreateRejectedError) Error() string { return e.Err.Error() }
func (e CreateRejectedError) Unwrap() error { return e.Err }

// Guard holds the session store.
type Guard struct {
	store      sessions.Store
	authorize  Authorizer
	createHook CreateHook
	autoCreate bool
}

// Option configures a Guard.
type Option func(*Guard)

// WithCreateHook runs hook before every create (explicit or automatic).
func WithCreateHook(hook CreateHook) Option { return func(g *Guard) { g.createHook = hook } }

// WithAutoCreate makes Open create a missing session for actions other than
// stop and delete.
func WithAutoCreate(on bool) Option { return func(g *Guard) { g.autoCreate = on } }

// New guards store with authorize (required).
func New(store sessions.Store, authorize Authorizer, opts ...Option) *Guard {
	g := &Guard{store: store, authorize: authorize}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	return g
}

// For binds the guard to a principal.
func (g *Guard) For(p Principal) Access { return Access{g: g, p: p} }

// Unchecked is the store itself, for programs embedding the server
// (httpapi.Server.Sessions). Request handlers must not call it; a test in
// httpapi checks that only Sessions does.
func (g *Guard) Unchecked() sessions.Store { return g.store }

// Access is the guard bound to one principal.
type Access struct {
	g *Guard
	p Principal
}

// Principal is the principal the access is bound to.
func (a Access) Principal() Principal { return a.p }

func (a Access) check(ctx context.Context, action Action, info *sessions.Info) error {
	if err := a.g.authorize(ctx, a.p, action, info); err != nil {
		return ForbiddenError{err}
	}
	return nil
}

// Create stores a new session owned by the principal: authorized for
// create, completed by the create hook (which cannot change the id or the
// owner), read back from the store.
func (a Access) Create(ctx context.Context, info sessions.Info) (Session, error) {
	if err := a.check(ctx, ActionCreate, nil); err != nil {
		return Session{}, err
	}
	id := info.ID
	info.Owner = a.p.ID
	if a.g.createHook != nil {
		if err := a.g.createHook(ctx, a.p, &info); err != nil {
			return Session{}, CreateRejectedError{err}
		}
	}
	info.ID, info.Owner = id, a.p.ID
	if err := a.g.store.Create(ctx, info); err != nil {
		return Session{}, err
	}
	stored, err := a.g.store.Get(ctx, id)
	if err != nil {
		return Session{}, err
	}
	return Session{info: stored, a: a, action: ActionCreate}, nil
}

// Open returns the session for an action, authorized for that action. With
// auto-creation it creates a missing session first (not for stop or delete).
func (a Access) Open(ctx context.Context, id string, action Action) (Session, error) {
	info, err := a.g.store.Get(ctx, id)
	if errors.Is(err, sessions.ErrNotFound) && a.g.autoCreate && action != ActionDelete && action != ActionStop {
		if _, err := a.Create(ctx, sessions.Info{ID: id}); err != nil && !errors.Is(err, sessions.ErrExists) {
			return Session{}, err
		}
		info, err = a.g.store.Get(ctx, id)
	}
	if err != nil {
		return Session{}, err
	}
	if err := a.check(ctx, action, &info); err != nil {
		return Session{}, err
	}
	return Session{info: info, a: a, action: action}, nil
}

// List returns the principal's sessions that match f: authorized for list,
// limited to the principal's own sessions whatever f says, and each one
// authorized as a read.
func (a Access) List(ctx context.Context, f sessions.Filter) ([]sessions.Info, error) {
	if err := a.check(ctx, ActionList, nil); err != nil {
		return nil, err
	}
	f.Owner, f.AllOwners = a.p.ID, false
	list, err := a.g.store.List(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]sessions.Info, 0, len(list))
	for i := range list {
		if a.check(ctx, ActionRead, &list[i]) == nil {
			out = append(out, list[i])
		}
	}
	return out, nil
}

// Session is a session the principal was authorized to act on.
type Session struct {
	info   sessions.Info
	a      Access
	action Action
}

// Info is the session's metadata as it was opened.
func (s Session) Info() sessions.Info { return s.info }

// ID is the session's id.
func (s Session) ID() string { return s.info.ID }

// Action is what the session was opened for.
func (s Session) Action() Action { return s.action }

// Update applies fn to the stored session; only a session opened to change
// it (send, tools, stop) may be updated, and fn cannot change its owner.
func (s Session) Update(ctx context.Context, fn func(*sessions.Info) error) error {
	switch s.action {
	case ActionSend, ActionTools, ActionStop:
	case ActionCreate, ActionList, ActionRead, ActionDelete:
		return ForbiddenError{fmt.Errorf("sessionguard: a session opened for %s cannot be updated", s.action)}
	default:
		return ForbiddenError{fmt.Errorf("sessionguard: unknown action %q", s.action)}
	}
	owner := s.info.Owner
	return s.a.g.store.Update(ctx, s.info.ID, func(info *sessions.Info) error {
		if err := fn(info); err != nil {
			return err
		}
		info.Owner = owner
		return nil
	})
}

// Delete deletes the session; only a session opened for delete may be.
func (s Session) Delete(ctx context.Context) error {
	if s.action != ActionDelete {
		return ForbiddenError{fmt.Errorf("sessionguard: a session opened for %s cannot be deleted", s.action)}
	}
	return s.a.g.store.Delete(ctx, s.info.ID)
}
