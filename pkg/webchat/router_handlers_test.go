package webchat

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
)

type nilEventRouterStreamBackend struct{}

func (nilEventRouterStreamBackend) EventRouter() *events.EventRouter {
	return nil
}

func (nilEventRouterStreamBackend) Publisher() message.Publisher {
	return nil
}

func (nilEventRouterStreamBackend) BuildSubscriber(context.Context, string) (message.Subscriber, bool, error) {
	return nil, false, errors.New("not implemented")
}

func (nilEventRouterStreamBackend) Close() error {
	return nil
}

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

func TestAPIHandler_DoesNotOwnChatOrWSRoutes(t *testing.T) {
	r := &Router{
		cm: &ConvManager{conns: map[string]*Conversation{}},
	}
	h := r.APIHandler()

	chatReq := httptest.NewRequest(http.MethodPost, "http://example.com/chat", nil)
	chatRec := httptest.NewRecorder()
	h.ServeHTTP(chatRec, chatReq)
	require.Equal(t, http.StatusNotFound, chatRec.Code)

	wsReq := httptest.NewRequest(http.MethodGet, "http://example.com/ws?conv_id=c1", nil)
	wsRec := httptest.NewRecorder()
	h.ServeHTTP(wsRec, wsReq)
	require.Equal(t, http.StatusNotFound, wsRec.Code)
}

func TestNewRouterFromDeps_RequiresRuntimeComposer(t *testing.T) {
	_, err := NewRouterFromDeps(context.Background(), RouterDeps{
		StaticFS:      fstest.MapFS{},
		StreamBackend: mustNewInMemoryStreamBackend(t),
	})
	require.ErrorContains(t, err, "runtime composer is not configured")
}

func TestNewRouterFromDeps_RejectsNilEventRouter(t *testing.T) {
	_, err := NewRouterFromDeps(context.Background(), RouterDeps{
		StaticFS:      fstest.MapFS{},
		StreamBackend: nilEventRouterStreamBackend{},
	}, WithRuntimeComposer(stubRuntimeComposer()))
	require.ErrorContains(t, err, "stream backend event router is nil")
}
