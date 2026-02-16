package webchat_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

type fakeResolver struct {
	plan webhttp.ConversationRequestPlan
	err  error
}

func (r fakeResolver) Resolve(_ *http.Request) (webhttp.ConversationRequestPlan, error) {
	if r.err != nil {
		return webhttp.ConversationRequestPlan{}, r.err
	}
	return r.plan, nil
}

type fakeChatHTTPService struct {
	lastIn webchat.SubmitPromptInput
	resp   webchat.SubmitPromptResult
	err    error
}

func (s *fakeChatHTTPService) SubmitPrompt(_ context.Context, in webchat.SubmitPromptInput) (webchat.SubmitPromptResult, error) {
	s.lastIn = in
	if s.err != nil {
		return webchat.SubmitPromptResult{}, s.err
	}
	return s.resp, nil
}

type fakeStreamHTTPService struct {
	handle *webchat.ConversationHandle
	err    error
}

func (s *fakeStreamHTTPService) ResolveAndEnsureConversation(_ context.Context, _ webchat.AppConversationRequest) (*webchat.ConversationHandle, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.handle != nil {
		return s.handle, nil
	}
	return &webchat.ConversationHandle{ConvID: "c1"}, nil
}

func (s *fakeStreamHTTPService) AttachWebSocket(_ context.Context, _ string, _ *websocket.Conn, _ webchat.WebSocketAttachOptions) error {
	return s.err
}

type fakeTimelineHTTPService struct {
	snap *timelinepb.TimelineSnapshotV1
	err  error
}

func (s *fakeTimelineHTTPService) Snapshot(_ context.Context, _ string, _ uint64, _ int) (*timelinepb.TimelineSnapshotV1, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.snap, nil
}

func TestNewChatHTTPHandler_SubmitPromptContract(t *testing.T) {
	svc := &fakeChatHTTPService{
		resp: webchat.SubmitPromptResult{
			HTTPStatus: http.StatusAccepted,
			Response: map[string]any{
				"status": "queued",
			},
		},
	}
	h := webhttp.NewChatHandler(svc, fakeResolver{
		plan: webhttp.ConversationRequestPlan{
			ConvID:     "conv-1",
			RuntimeKey: "default",
			Prompt:     "hello",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "http://example.com/chat", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, "conv-1", svc.lastIn.ConvID)
	require.Equal(t, "default", svc.lastIn.RuntimeKey)
	require.Equal(t, "hello", svc.lastIn.Prompt)
	require.NotEmpty(t, svc.lastIn.IdempotencyKey)

	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, "queued", out["status"])
}

func TestNewWSHTTPHandler_ResolverErrorContract(t *testing.T) {
	h := webhttp.NewWSHandler(&fakeStreamHTTPService{}, fakeResolver{
		err: &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "missing conv_id",
		},
	}, websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "missing conv_id")
}

func TestNewTimelineHTTPHandler_Contract(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("disabled service", func(t *testing.T) {
		h := webhttp.NewTimelineHandler(nil, logger)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/api/timeline?conv_id=c1", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("snapshot error", func(t *testing.T) {
		h := webhttp.NewTimelineHandler(&fakeTimelineHTTPService{err: errors.New("boom")}, logger)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/api/timeline?conv_id=c1", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("successful snapshot", func(t *testing.T) {
		h := webhttp.NewTimelineHandler(&fakeTimelineHTTPService{
			snap: &timelinepb.TimelineSnapshotV1{
				ConvId:  "c1",
				Version: 1,
				Entities: []*timelinepb.TimelineEntityV1{
					{
						Id:   "m1",
						Kind: "message",
						Snapshot: &timelinepb.TimelineEntityV1_Message{
							Message: &timelinepb.MessageSnapshotV1{Role: "assistant", Content: "ok"},
						},
					},
				},
			},
		}, logger)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/api/timeline?conv_id=c1", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		require.Contains(t, rec.Body.String(), "\"convId\":\"c1\"")
		require.Contains(t, rec.Body.String(), "\"id\":\"m1\"")
	})
}
