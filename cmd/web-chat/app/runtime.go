package app

import (
	"context"
	"net/http"

	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

// RuntimeResolver resolves the selected runtime for one canonical submit request.
// It is app-owned so cmd/web-chat can reuse its existing profile/runtime policy
// without pushing webchat-specific behavior into the shared sessionstream substrate.
type RuntimeResolver interface {
	Resolve(ctx context.Context, req *http.Request, profile string, registry string) (*infruntime.ComposedRuntime, error)
}
