package webapp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type RuntimeConfig struct {
	BasePrefix string `json:"basePrefix"`
}

func NormalizeBasePrefix(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(p, "/")
}

func RuntimeConfigScript(basePrefix string) (string, error) {
	payload, err := json.Marshal(RuntimeConfig{
		BasePrefix: NormalizeBasePrefix(basePrefix),
	})
	if err != nil {
		return "", err
	}
	return "window.__PINOCCHIO_WEBCHAT_CONFIG__ = " + string(payload) + ";\n", nil
}

func buildAppConfigHandler(appConfigJS string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = io.WriteString(w, appConfigJS)
	}
}
