package httpapi

import (
	"context"
	"errors"
	"net/http"

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

// Authenticator turns a request into a Principal. An error answers 401.
type Authenticator func(r *http.Request) (Principal, error)

// Authorizer allows or denies an action. s is nil for ActionCreate and
// ActionList. An error answers 403.
type Authorizer func(ctx context.Context, p Principal, a Action, s *sessions.Info) error

// ErrForbidden is OwnerOnly's denial.
var ErrForbidden = errors.New("httpapi: forbidden")

// OwnerOnly allows everything on sessions without an owner and on sessions
// the principal owns. It is the default Authorizer; without an Authenticator
// every principal and every owner is anonymous, so everything is allowed.
func OwnerOnly(_ context.Context, p Principal, _ Action, s *sessions.Info) error {
	if s == nil || s.Owner == "" || s.Owner == p.ID {
		return nil
	}
	return ErrForbidden
}

type principalKey struct{}

func withPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext returns the principal of the request a context belongs
// to. Resolvers and hooks receive such a context.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

func (s *Server) authenticateRequest(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	if s.authenticate == nil {
		return Principal{}, true
	}
	p, err := s.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthenticated, err)
		return Principal{}, false
	}
	return p, true
}

// forbiddenError marks an Authorizer's denial.
type forbiddenError struct{ err error }

func (e forbiddenError) Error() string { return e.err.Error() }
func (e forbiddenError) Unwrap() error { return e.err }
