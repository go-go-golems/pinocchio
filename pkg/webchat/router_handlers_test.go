package webchat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/stretchr/testify/require"
)

func TestUIHandler_ServesIndexFromStaticFS(t *testing.T) {
	staticFS := fstest.MapFS{
		"static/index.html": {Data: []byte("<html>ok</html>")},
	}
	r := &Router{staticFS: staticFS}

	h := r.UIHandler()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "ok")
}

func TestAPIHandler_DoesNotServeIndex(t *testing.T) {
	r := &Router{
		cm: &ConvManager{conns: map[string]*Conversation{}},
	}

	h := r.APIHandler()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNewRouter_RequiresRuntimeComposer(t *testing.T) {
	_, err := NewRouter(context.Background(), values.New(), fstest.MapFS{})
	require.ErrorContains(t, err, "runtime composer is not configured")
}
