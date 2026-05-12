//go:build embed

package spa

import "embed"

// Assets contains the Glazed help browser SPA frontend files.
// Built from the glazed-spa release artifact, fetched by `make fetch-spa`.
//
//go:embed dist
var Assets embed.FS
