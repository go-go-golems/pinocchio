package webapp

import (
	"net/http"
	"strings"

	zlog "github.com/rs/zerolog/log"
)

func MountRoot(root string, appMux http.Handler, appConfigJS string) http.Handler {
	if appMux == nil {
		return http.NotFoundHandler()
	}
	if root == "" || root == "/" {
		return appMux
	}
	prefix := root
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	parent := http.NewServeMux()
	parent.HandleFunc("/app-config.js", buildAppConfigHandler(appConfigJS))
	parent.Handle(prefix, http.StripPrefix(strings.TrimRight(prefix, "/"), appMux))
	zlog.Info().Str("root", prefix).Msg("mounted webchat under custom root")
	return parent
}
