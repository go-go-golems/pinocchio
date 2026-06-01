package profiles

import (
	"net/http"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
)

// RegisterAPIHandlers mounts the profile API routes on the provided mux.
func RegisterAPIHandlers(mux *http.ServeMux, profileRegistry gepprofiles.Registry, opts APIOptions) {
	if mux == nil || profileRegistry == nil {
		return
	}
	opts.normalize()

	registerSchemaHandlers(mux, opts)
	registerProfileHandlers(mux, profileRegistry, opts)
	if opts.EnableCurrentProfileCookieRoute {
		registerCurrentProfileHandler(mux, profileRegistry, opts)
	}
}
