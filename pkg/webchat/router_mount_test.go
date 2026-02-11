package webchat

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouterMount_StripsPrefix(t *testing.T) {
	r := &Router{mux: http.NewServeMux()}
	r.HandleFunc("/chat", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	parent := http.NewServeMux()
	r.Mount(parent, "/api/webchat")

	srv := httptest.NewServer(parent)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/webchat/chat")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))
}

func TestRouterMount_RedirectsBasePath(t *testing.T) {
	r := &Router{mux: http.NewServeMux()}
	parent := http.NewServeMux()
	r.Mount(parent, "/api/webchat")

	srv := httptest.NewServer(parent)
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/api/webchat")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusPermanentRedirect, resp.StatusCode)
	require.Equal(t, "/api/webchat/", resp.Header.Get("Location"))
}
