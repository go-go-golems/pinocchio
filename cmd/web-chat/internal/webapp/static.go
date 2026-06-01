package webapp

import (
	"io/fs"
	"net/http"
	"strings"

	zlog "github.com/rs/zerolog/log"
)

func RegisterStaticUIHandlers(mux *http.ServeMux, staticFS fs.FS) {
	if mux == nil || staticFS == nil {
		return
	}
	logger := zlog.With().Str("component", "web-chat").Logger()
	if staticSub, err := fs.Sub(staticFS, "static"); err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	} else {
		logger.Warn().Err(err).Msg("failed to mount /static/ asset handler")
	}
	if distAssets, err := fs.Sub(staticFS, "static/dist/assets"); err == nil {
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(distAssets))))
	} else {
		logger.Warn().Err(err).Msg("no built dist assets found under static/dist/assets")
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			http.NotFound(w, r)
			return
		}
		if b, err := fs.ReadFile(staticFS, "static/dist/index.html"); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		if b, err := fs.ReadFile(staticFS, "static/index.html"); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		http.Error(w, "index not found", http.StatusInternalServerError)
	})
}
