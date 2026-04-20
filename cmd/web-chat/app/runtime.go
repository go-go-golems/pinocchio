package app

import (
	"context"
	"net/http"

	chatapp "github.com/go-go-golems/pinocchio/pkg/evtstream/apps/chat"
)

// RuntimeResolver resolves the selected runtime for one canonical submit request.
// It is app-owned so cmd/web-chat can reuse its existing profile/runtime policy
// without pushing webchat-specific behavior into pkg/evtstream.
type RuntimeResolver interface {
	Resolve(ctx context.Context, req *http.Request, profile string, registry string) (*chatapp.ResolvedRuntime, error)
}
