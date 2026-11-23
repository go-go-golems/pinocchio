// Package webclient carries the browser client of chatapp's HTTP API: the
// TypeScript source (src/), its tests (test/) and the built ES module
// (chatapp-client.js), which httpapi serves at {prefix}/client.js. Because the
// file travels with the Go module, a page that loads it from its chatapp
// server always runs the client of that server's version.
//
// Rebuild after changing src/: make webclient (CI fails when the committed
// file differs from a fresh build). TypeScript bundles can import the source
// from the module directory instead:
//
//	$(go list -m -f '{{.Dir}}' github.com/go-go-golems/pinocchio)/pkg/chatapp/webclient/src/index.ts
package webclient

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"net/http"
	"strings"
)

// JS is the built client (chatapp-client.js).
//
//go:embed chatapp-client.js
var JS []byte

var etag = func() string {
	sum := sha256.Sum256(JS)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}()

// Handler serves JS as a JavaScript module, with an ETag of its content.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")
		if strings.Contains(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(JS)
	})
}
