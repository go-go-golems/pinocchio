package spa

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewHandler returns an http.Handler that serves the Glazed help browser SPA
// from the embedded assets. It implements SPA fallback routing (serves
// index.html for all unknown paths) so that client-side routing works.
func NewHandler() (http.Handler, error) {
	indexBytes, err := fs.ReadFile(Assets, "index.html")
	if err != nil {
		return nil, fmt.Errorf("reading SPA assets: %w (run 'make fetch-spa' and rebuild with -tags embed)", err)
	}

	fileServer := http.FileServer(http.FS(Assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		cleanPath := path.Clean("/" + r.URL.Path)
		if cleanPath == "/" {
			serveSPAIndex(w, r, indexBytes)
			return
		}

		assetPath := strings.TrimPrefix(cleanPath, "/")
		if _, err := fs.Stat(Assets, assetPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for all unknown paths (client-side routing).
		serveSPAIndex(w, r, indexBytes)
	}), nil
}

func serveSPAIndex(w http.ResponseWriter, r *http.Request, indexBytes []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(indexBytes)
	}
}
