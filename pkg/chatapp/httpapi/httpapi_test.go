package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	sessionstreamv1 "github.com/go-go-golems/sessionstream/pkg/sessionstream/pb/proto/sessionstream/v1"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// --- engines and resolvers -------------------------------------------------

// answerEngine streams "Answer: <last user text>" in chunks, waiting delay
// before each one, and stops with ctx.Err() when the run is cancelled.
type answerEngine struct {
	chunk int
	delay time.Duration
	usage *gepevents.Usage
	seen  chan<- *turns.Turn
}

func (e answerEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	if e.seen != nil {
		e.seen <- t.Clone()
	}
	prompt := ""
	for i := len(t.Blocks) - 1; i >= 0; i-- {
		if t.Blocks[i].Kind == turns.BlockKindUser {
			prompt, _ = t.Blocks[i].Payload[turns.PayloadKeyText].(string)
			break
		}
	}
	answer := "Answer: " + prompt
	chunk := e.chunk
	if chunk <= 0 {
		chunk = len(answer)
	}
	meta := gepevents.EventMetadata{}
	corr := gepevents.Correlation{RunID: "run-1", ProviderCallID: "provider-call-1", SegmentID: "segment-1"}
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	sent := ""
	for i, n := 0, 1; i < len(answer); i, n = i+chunk, n+1 {
		if e.delay > 0 {
			select {
			case <-ctx.Done():
				return t, ctx.Err()
			case <-time.After(e.delay):
			}
		}
		end := min(i+chunk, len(answer))
		sent += answer[i:end]
		gepevents.PublishEventToContext(ctx, gepevents.NewTextDeltaEvent(meta, corr, answer[i:end], sent, int64(n)))
	}
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentFinishedEvent(meta, corr, answer, "stop"))
	if e.usage != nil {
		gepevents.PublishEventToContext(ctx, gepevents.NewProviderCallFinishedEvent(meta, corr, "stop", "", e.usage, nil, false))
	}
	turns.AppendBlock(t, turns.NewAssistantTextBlock(answer))
	return t, nil
}

func engineResolver(e answerEngine) RuntimeResolver {
	return RuntimeResolverFunc(func(context.Context, *http.Request, sessions.Info, MessageRequest) (*infruntime.ComposedRuntime, error) {
		return &infruntime.ComposedRuntime{Engine: e}, nil
	})
}

// --- server helpers ---------------------------------------------------------

func newTestServer(t *testing.T, opts ...Option) (*Server, *httptest.Server) {
	t.Helper()
	all := append([]Option{WithRuntimeResolver(engineResolver(answerEngine{}))}, opts...)
	srv, err := New(all...)
	require.NoError(t, err)
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		httpSrv.Close()
		_ = srv.Close()
	})
	return srv, httpSrv
}

func do(t *testing.T, method, url, body string, header ...string) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, out
}

func createSession(t *testing.T, base, body string, header ...string) SessionResponse {
	t.Helper()
	resp, out := do(t, http.MethodPost, base+"/api/chat/sessions", body, header...)
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(out))
	var created SessionResponse
	require.NoError(t, json.Unmarshal(out, &created))
	return created
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var e ErrorResponse
	require.NoError(t, json.Unmarshal(body, &e), string(body))
	return e.Code
}

func getSession(t *testing.T, base, id string) SessionResponse {
	t.Helper()
	resp, out := do(t, http.MethodGet, base+"/api/chat/sessions/"+id, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, string(out))
	var got SessionResponse
	require.NoError(t, json.Unmarshal(out, &got))
	return got
}

func waitForFinished(t *testing.T, base, id string) SessionResponse {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		got := getSession(t, base, id)
		if got.Snapshot != nil && got.Snapshot.Status == "finished" {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for a finished snapshot of %s", id)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func entityTexts(snap *serverkit.SessionSnapshotResponse) map[string]string {
	out := map[string]string{}
	for _, entity := range snap.Entities {
		payload, ok := entity.Payload.(map[string]any)
		if !ok {
			continue
		}
		role, _ := payload["role"].(string)
		if text, ok := payload["text"].(string); ok && text != "" {
			out[role] = text
		}
		if content, ok := payload["content"].(string); ok && content != "" {
			out[role] = content
		}
	}
	return out
}

// --- sessions ---------------------------------------------------------------

func TestSnapshotStatusDoesNotFinishBeforeAssistant(t *testing.T) {
	entities := []serverkit.SnapshotEntity{
		{Payload: map[string]any{"role": "user", "status": ""}},
		{Payload: map[string]any{"role": "thinking", "status": "finished"}},
	}
	require.Equal(t, "streaming", snapshotStatus(entities))
	entities = append(entities, serverkit.SnapshotEntity{Payload: map[string]any{"role": "assistant", "status": "finished"}})
	require.Equal(t, "finished", snapshotStatus(entities))
}

func TestNewRequiresARuntimeResolver(t *testing.T) {
	_, err := New()
	require.ErrorContains(t, err, "runtime resolver is required")
}

func TestSessionLifecycle(t *testing.T) {
	_, httpSrv := newTestServer(t)
	base := httpSrv.URL

	created := createSession(t, base, `{"title":"Main room","labels":{"room":"main"},"profile":"ignored"}`)
	require.NotEmpty(t, created.SessionID)
	require.Equal(t, created.SessionID, created.Session.ID)
	require.Equal(t, "Main room", created.Session.Title)
	createSession(t, base, `{"labels":{"room":"t-1"}}`)

	resp, out := do(t, http.MethodGet, base+"/api/chat/sessions?label=room:main", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list SessionListResponse
	require.NoError(t, json.Unmarshal(out, &list))
	require.Len(t, list.Sessions, 1)
	require.Equal(t, created.SessionID, list.Sessions[0].ID)

	resp, _ = do(t, http.MethodPost, base+"/api/chat/sessions/"+created.SessionID+"/messages", `{"prompt":"Explain ordinals\nin plain language"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	got := waitForFinished(t, base, created.SessionID)
	require.Equal(t, "Answer: Explain ordinals\nin plain language", entityTexts(got.Snapshot)["assistant"])
	require.Equal(t, "Explain ordinals\nin plain language", entityTexts(got.Snapshot)["user"])
	require.Equal(t, "Main room", got.Session.Title, "a set title is kept")

	resp, _ = do(t, http.MethodDelete, base+"/api/chat/sessions/"+created.SessionID, "")
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp, out = do(t, http.MethodGet, base+"/api/chat/sessions/"+created.SessionID, "")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, CodeSessionNotFound, errorCode(t, out))
}

func TestFirstMessageTitlesAnUntitledSession(t *testing.T) {
	_, httpSrv := newTestServer(t)
	created := createSession(t, httpSrv.URL, ``)
	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+created.SessionID+"/messages", `{"prompt":"\n  Plan the week  \nthen more"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.Equal(t, "Plan the week", waitForFinished(t, httpSrv.URL, created.SessionID).Session.Title)
}

func TestClientChosenSessionIDs(t *testing.T) {
	_, httpSrv := newTestServer(t)
	require.Equal(t, "room-1", createSession(t, httpSrv.URL, `{"id":"room-1"}`).SessionID)
	resp, out := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions", `{"id":"room-1"}`)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	require.Equal(t, CodeSessionExists, errorCode(t, out))
	resp, out = do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions", `{"id":"no spaces"}`)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, CodeInvalidSessionID, errorCode(t, out))
}

func TestUnknownSessionsAre404UnlessAutoCreated(t *testing.T) {
	_, strict := newTestServer(t)
	resp, out := do(t, http.MethodPost, strict.URL+"/api/chat/sessions/never-made/messages", `{"prompt":"hi"}`)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, CodeSessionNotFound, errorCode(t, out))

	srv, auto := newTestServer(t, WithAutoCreateSessions(true))
	resp, _ = do(t, http.MethodPost, auto.URL+"/api/chat/sessions/kept-by-client/messages", `{"prompt":"hi"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	waitForFinished(t, auto.URL, "kept-by-client")
	info, err := srv.Sessions().Get(context.Background(), "kept-by-client")
	require.NoError(t, err)
	require.Equal(t, "hi", info.Title)

	resp, _ = do(t, http.MethodDelete, auto.URL+"/api/chat/sessions/not-there", "")
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "delete never creates")
}

// --- messages ---------------------------------------------------------------

func TestSecondMessageWhileRunningIsRefusedWith409(t *testing.T) {
	_, httpSrv := newTestServer(t, WithRuntimeResolver(engineResolver(answerEngine{chunk: 2, delay: 20 * time.Millisecond})))
	id := createSession(t, httpSrv.URL, ``).SessionID
	url := httpSrv.URL + "/api/chat/sessions/" + id

	resp, _ := do(t, http.MethodPost, url+"/messages", `{"prompt":"a long answer please"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.True(t, getSession(t, httpSrv.URL, id).Running)
	resp, out := do(t, http.MethodPost, url+"/messages", `{"prompt":"me too"}`)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	require.Equal(t, CodeRunActive, errorCode(t, out))

	resp, _ = do(t, http.MethodPost, url+"/stop", "")
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	deadline := time.Now().Add(3 * time.Second)
	for {
		got := getSession(t, httpSrv.URL, id)
		if !got.Running {
			require.Equal(t, "a long answer please", entityTexts(got.Snapshot)["user"], "the refused message left no trace")
			require.NotEqual(t, "Answer: a long answer please", entityTexts(got.Snapshot)["assistant"], "the run was stopped before it finished")
			break
		}
		require.True(t, time.Now().Before(deadline), "run did not stop")
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMessageValidation(t *testing.T) {
	_, httpSrv := newTestServer(t)
	id := createSession(t, httpSrv.URL, ``).SessionID
	for _, body := range []string{
		`{"prompt":"","attachments":[{"attachment_id":"  "}]}`,
		`{"prompt":"hi","attachments":[{"attachment_id":""}]}`,
		`{"prompt":"","attachments":[]}`,
		`{"prompt":`,
	} {
		resp, out := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", body)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, body)
		require.Equal(t, CodeInvalidRequest, errorCode(t, out), body)
	}
	_, small := newTestServer(t, WithMaxBodyBytes(16))
	id = createSession(t, small.URL, ``).SessionID
	resp, out := do(t, http.MethodPost, small.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"far more than sixteen bytes"}`)
	require.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	require.Equal(t, CodeRequestTooLarge, errorCode(t, out))
}

type quotaError struct{}

func (quotaError) Error() string { return "the session's budget is used up" }
func (quotaError) Code() string  { return "quota_exceeded" }

func TestResolverSeesSessionAndMessageAndItsCodedErrorsAnswer400(t *testing.T) {
	type seen struct {
		info sessions.Info
		msg  MessageRequest
	}
	got := make(chan seen, 1)
	resolver := RuntimeResolverFunc(func(_ context.Context, _ *http.Request, s sessions.Info, msg MessageRequest) (*infruntime.ComposedRuntime, error) {
		got <- seen{s, msg}
		var extra struct {
			Agent string `json:"agent"`
		}
		if err := json.Unmarshal(msg.Raw, &extra); err != nil {
			return nil, err
		}
		if extra.Agent == "broke" {
			return nil, quotaError{}
		}
		return &infruntime.ComposedRuntime{Engine: answerEngine{}}, nil
	})
	_, httpSrv := newTestServer(t, WithRuntimeResolver(resolver))
	id := createSession(t, httpSrv.URL, `{"labels":{"room":"main"}}`).SessionID

	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"hi","profile":"p1","agent":"coder"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	s := <-got
	require.Equal(t, "main", s.info.Labels["room"])
	require.Equal(t, "p1", s.msg.Profile)
	require.Contains(t, string(s.msg.Raw), `"agent":"coder"`)
	waitForFinished(t, httpSrv.URL, id)

	resp, out := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"hi","agent":"broke"}`)
	<-got
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "quota_exceeded", errorCode(t, out))
}

func TestBlocksHookAddsBlocksAfterTheMessage(t *testing.T) {
	seen := make(chan *turns.Turn, 1)
	_, httpSrv := newTestServer(t,
		WithRuntimeResolver(engineResolver(answerEngine{seen: seen})),
		WithBlocks(func(_ context.Context, s sessions.Info, msg MessageRequest) ([]turns.Block, error) {
			return []turns.Block{turns.NewUserTextBlock("[state] room=" + s.Labels["room"])}, nil
		}))
	id := createSession(t, httpSrv.URL, `{"labels":{"room":"main"}}`).SessionID
	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"hello"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	turn := <-seen
	require.Len(t, turn.Blocks, 2)
	require.Equal(t, "hello", turn.Blocks[0].Payload[turns.PayloadKeyText])
	require.Equal(t, "[state] room=main", turn.Blocks[1].Payload[turns.PayloadKeyText])
}

func TestHistoryAndSessionIDReachTheRuntime(t *testing.T) {
	prior := &turns.Turn{ID: "turn-prior"}
	turns.AppendBlock(prior, turns.NewUserTextBlock("previous question"))
	turns.AppendBlock(prior, turns.NewAssistantTextBlock("previous answer"))
	payload, err := serde.ToYAML(prior, serde.Options{})
	require.NoError(t, err)
	seen := make(chan *turns.Turn, 1)
	_, httpSrv := newTestServer(t,
		WithAutoCreateSessions(true),
		WithTurnStore(&fakeTurnStore{snapshot: &chatstore.TurnSnapshot{ConvID: "sess-history", SessionID: "sess-history", TurnID: "turn-prior", Phase: "final", Payload: string(payload)}}),
		WithRuntimeResolver(engineResolver(answerEngine{seen: seen})))

	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/sess-history/messages", `{"prompt":"follow up"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	select {
	case turn := <-seen:
		require.Len(t, turn.Blocks, 3)
		require.Equal(t, "previous question", turn.Blocks[0].Payload[turns.PayloadKeyText])
		require.Equal(t, "previous answer", turn.Blocks[1].Payload[turns.PayloadKeyText])
		require.Equal(t, "follow up", turn.Blocks[2].Payload[turns.PayloadKeyText])
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the runtime")
	}
}

func TestUsageIsRecordedOnTheSession(t *testing.T) {
	srv, httpSrv := newTestServer(t, WithRuntimeResolver(engineResolver(answerEngine{usage: &gepevents.Usage{InputTokens: 50, OutputTokens: 5}})))
	id := createSession(t, httpSrv.URL, ``).SessionID
	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"count me"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	waitForFinished(t, httpSrv.URL, id)
	info, err := srv.Sessions().Get(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, sessions.Usage{Input: 50, Output: 5, Calls: 1}, info.Usage)
}

// --- auth -------------------------------------------------------------------

func headerAuth(r *http.Request) (Principal, error) {
	user := r.Header.Get("X-User")
	if user == "" {
		user = r.URL.Query().Get("user")
	}
	if user == "" {
		return Principal{}, errors.New("no user")
	}
	return Principal{ID: user}, nil
}

func TestAuthenticationAndOwnership(t *testing.T) {
	_, httpSrv := newTestServer(t, WithAuthenticator(headerAuth))
	base := httpSrv.URL

	resp, out := do(t, http.MethodPost, base+"/api/chat/sessions", `{}`)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, CodeUnauthenticated, errorCode(t, out))

	alice := createSession(t, base, `{}`, "X-User", "alice")
	require.Equal(t, "alice", alice.Session.Owner)
	createSession(t, base, `{}`, "X-User", "bob")

	resp, out = do(t, http.MethodGet, base+"/api/chat/sessions/"+alice.SessionID, "", "X-User", "bob")
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, CodeForbidden, errorCode(t, out))
	resp, _ = do(t, http.MethodPost, base+"/api/chat/sessions/"+alice.SessionID+"/messages", `{"prompt":"hi"}`, "X-User", "bob")
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	resp, out = do(t, http.MethodGet, base+"/api/chat/sessions", "", "X-User", "alice")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list SessionListResponse
	require.NoError(t, json.Unmarshal(out, &list))
	require.Len(t, list.Sessions, 1, "a principal lists only its own sessions")

	conn := dialWS(t, base, "?user=bob")
	writeClientFrame(t, conn, map[string]any{"subscribe": map[string]any{"sessionId": alice.SessionID, "sinceSnapshotOrdinal": "0"}})
	frame := readServerFrame(t, conn)
	require.NotNil(t, frame.GetError())
	require.Equal(t, "subscribe_denied", frame.GetError().GetCode())
}

// An authenticator that lets anonymous requests through (empty principal):
// they see only ownerless sessions, never another person's.
func TestAnonymousListingShowsNoOnesSessions(t *testing.T) {
	anonymousOK := func(r *http.Request) (Principal, error) { return Principal{ID: r.Header.Get("X-User")}, nil }
	_, httpSrv := newTestServer(t, WithAuthenticator(anonymousOK))
	base := httpSrv.URL
	alice := createSession(t, base, `{"title":"alice's secrets"}`, "X-User", "alice")
	open := createSession(t, base, `{}`)

	resp, out := do(t, http.MethodGet, base+"/api/chat/sessions", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotContains(t, string(out), alice.SessionID)
	require.NotContains(t, string(out), "alice's secrets")
	require.Contains(t, string(out), open.SessionID)
}

// A deleted session's conversation outlives it, so its id is never given
// out again: no one can re-create it and read the old timeline.
func TestADeletedSessionsIDCannotBeTakenOver(t *testing.T) {
	srv, httpSrv := newTestServer(t, WithAuthenticator(headerAuth), WithAutoCreateSessions(true))
	base := httpSrv.URL
	alice := createSession(t, base, `{"id":"s-alice"}`, "X-User", "alice")
	resp, _ := do(t, http.MethodPost, base+"/api/chat/sessions/"+alice.SessionID+"/messages", `{"prompt":"my secret"}`, "X-User", "alice")
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.NoError(t, srv.Service().WaitIdle(context.Background(), sessionstream.SessionId(alice.SessionID)))
	resp, _ = do(t, http.MethodDelete, base+"/api/chat/sessions/"+alice.SessionID, "", "X-User", "alice")
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp, out := do(t, http.MethodPost, base+"/api/chat/sessions", `{"id":"s-alice"}`, "X-User", "mallory")
	require.Equal(t, http.StatusConflict, resp.StatusCode, string(out))
	resp, out = do(t, http.MethodGet, base+"/api/chat/sessions/s-alice", "", "X-User", "mallory")
	require.NotEqual(t, http.StatusOK, resp.StatusCode, "auto-creation does not reopen it either")
	require.NotContains(t, string(out), "my secret")
}

// --- transports -------------------------------------------------------------

func dialWS(t *testing.T, base, query string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/chat/ws"+query, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	require.NotNil(t, readServerFrame(t, conn).GetHello())
	return conn
}

func TestWebSocketSnapshotAndLiveEvent(t *testing.T) {
	_, httpSrv := newTestServer(t)
	id := createSession(t, httpSrv.URL, ``).SessionID
	conn := dialWS(t, httpSrv.URL, "")
	writeClientFrame(t, conn, map[string]any{"subscribe": map[string]any{"sessionId": id, "sinceSnapshotOrdinal": "0"}})
	snap := readServerFrame(t, conn)
	require.NotNil(t, snap.GetSnapshot())
	require.Equal(t, id, snap.GetSnapshot().GetSessionId())
	require.NotNil(t, readServerFrame(t, conn).GetSubscribed())

	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"hello over websocket"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if frame := readServerFrame(t, conn); frame.GetUiEvent() != nil {
			require.Equal(t, id, frame.GetUiEvent().GetSessionId())
			return
		}
	}
	t.Fatal("expected a ui-event frame")
}

type sseEvent struct {
	event string
	id    string
	data  string
}

// readSSE parses a stream into events on a channel until it ends.
func readSSE(t *testing.T, url string) (<-chan sseEvent, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	out := make(chan sseEvent, 1024)
	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()
		scanner := bufio.NewScanner(resp.Body)
		var cur sseEvent
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				if cur.event != "" {
					out <- cur
				}
				cur = sseEvent{}
			case strings.HasPrefix(line, "event: "):
				cur.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "id: "):
				cur.id = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "data: "):
				cur.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return out, cancel
}

// collectUntilRunEnds returns the snapshot ordinal and the ui events of a
// stream, up to and including the run's end.
func collectUntilRunEnds(t *testing.T, events <-chan sseEvent) (string, []UIEventMessage) {
	t.Helper()
	var snapshotOrdinal string
	var ui []UIEventMessage
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			require.True(t, ok, "stream ended early")
			switch ev.event {
			case "snapshot":
				var snap serverkit.SessionSnapshotResponse
				require.NoError(t, json.Unmarshal([]byte(ev.data), &snap))
				snapshotOrdinal = snap.SnapshotOrdinal
			case "ui":
				var msg UIEventMessage
				require.NoError(t, json.Unmarshal([]byte(ev.data), &msg))
				require.Equal(t, msg.Ordinal, ev.id)
				ui = append(ui, msg)
				if msg.Name == "ChatRunFinished" || msg.Name == "ChatRunStopped" || msg.Name == "ChatRunFailed" {
					return snapshotOrdinal, ui
				}
			default:
				t.Fatalf("unexpected event %q: %s", ev.event, ev.data)
			}
		case <-timeout:
			t.Fatalf("no end of run; got %d ui events", len(ui))
		}
	}
}

func ordinalOf(t *testing.T, s string) uint64 {
	t.Helper()
	var n uint64
	_, err := fmt.Sscan(s, &n)
	require.NoError(t, err)
	return n
}

func TestSSEHydratesThenStreamsWithoutGapOrDuplicate(t *testing.T) {
	_, httpSrv := newTestServer(t, WithRuntimeResolver(engineResolver(answerEngine{chunk: 1, delay: 2 * time.Millisecond})))
	id := createSession(t, httpSrv.URL, ``).SessionID
	url := httpSrv.URL + "/api/chat/sessions/" + id + "/events"

	// The first stream starts before anything happens: it sees every event.
	first, cancelFirst := readSSE(t, url)
	defer cancelFirst()
	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"stream me thirty-odd characters"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	// The second stream connects while the answer streams.
	time.Sleep(25 * time.Millisecond)
	second, cancelSecond := readSSE(t, url)
	defer cancelSecond()

	_, all := collectUntilRunEnds(t, first)
	snapOrd, late := collectUntilRunEnds(t, second)
	cut := ordinalOf(t, snapOrd)
	require.Positive(t, cut, "the second stream connected mid-run")

	var expected []UIEventMessage
	for _, msg := range all {
		if ordinalOf(t, msg.Ordinal) > cut {
			expected = append(expected, msg)
		}
	}
	require.NotEmpty(t, expected, "the run continued after the second stream connected")
	require.Equal(t, len(expected), len(late))
	for i := range expected {
		require.Equal(t, expected[i].Ordinal, late[i].Ordinal)
		require.Equal(t, expected[i].Name, late[i].Name)
	}
}

func TestSSEStreamEndsWhenTheSessionIsDeleted(t *testing.T) {
	_, httpSrv := newTestServer(t)
	id := createSession(t, httpSrv.URL, ``).SessionID
	events, cancel := readSSE(t, httpSrv.URL+"/api/chat/sessions/"+id+"/events")
	defer cancel()
	require.Equal(t, "snapshot", (<-events).event)
	resp, _ := do(t, http.MethodDelete, httpSrv.URL+"/api/chat/sessions/"+id, "")
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	ev := <-events
	require.Equal(t, "error", ev.event)
	require.Contains(t, ev.data, "session_deleted")
	_, open := <-events
	require.False(t, open)
}

func TestSSEBufferKeepsOnlyBatchesAfterTheSnapshot(t *testing.T) {
	hub := newSSEHub(DefaultSSEConfig())
	sub := hub.subscribe("s")
	for ord := uint64(3); ord <= 5; ord++ {
		require.NoError(t, hub.PublishUI(context.Background(), "s", ord, nil))
		sub.offer(uiBatch{ord: ord})
	}
	pending := sub.goLive(4)
	require.Len(t, pending, 1)
	require.Equal(t, uint64(5), pending[0].ord)
	sub.offer(uiBatch{ord: 6})
	require.Equal(t, uint64(6), (<-sub.queue).ord)
}

func TestSSESlowClientIsClosedNotBlocking(t *testing.T) {
	hub := newSSEHub(SSEConfig{QueueSize: 1, HydrationBufferLimit: 1})
	sub := hub.subscribe("s")
	sub.goLive(0)
	sub.offer(uiBatch{ord: 1})
	sub.offer(uiBatch{ord: 2}) // queue full: closed, not blocked
	<-sub.done
	require.Equal(t, "stream_overflow", sub.closeReason().Code)
}

// --- frontend tools -----------------------------------------------------------

func TestFrontendToolManifestEndpointPublishesTimelineEntity(t *testing.T) {
	manager := frontendtools.NewManager()
	_, httpSrv := newTestServer(t, WithFrontendTools(manager), WithChatPlugins(frontendtools.NewPlugin()), WithAutoCreateSessions(true))

	body := `{"clientInstanceId":"client-a","connectionId":"connection-a","revision":7,"tools":[{"name":"app.confirm_action","description":"Confirm an action","mode":"human","inputSchema":{"type":"object"},"available":true}]}`
	resp, out := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/sess-tools/tools/manifest", body)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(out))
	var manifest frontendToolManifestResponse
	require.NoError(t, json.Unmarshal(out, &manifest))
	require.True(t, manifest.Accepted)
	require.Equal(t, uint64(7), manifest.Revision)
	require.Equal(t, "client-a", manifest.Executor.ClientInstanceID)
	require.NotEmpty(t, manifest.Executor.AssignmentID)

	desc, ok := manager.Descriptor("sess-tools", "app.confirm_action")
	require.True(t, ok)
	require.Equal(t, toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_HUMAN, desc.Mode)
}

func TestFrontendToolRoutesWithoutManager(t *testing.T) {
	_, httpSrv := newTestServer(t, WithAutoCreateSessions(true))
	resp, out := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/s/tools/manifest", `{"tools":[]}`)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, CodeFrontendToolsDisabled, errorCode(t, out))
}

func TestFrontendToolResultErrorStatus(t *testing.T) {
	tests := []struct {
		err      error
		expected int
	}{
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorInvalidStatus}, http.StatusBadRequest},
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorExecutorMissing}, http.StatusBadRequest},
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorExecutorMismatch}, http.StatusConflict},
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorUnknownResult}, http.StatusNotFound},
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorLateResult}, http.StatusGone},
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorDuplicatePending}, http.StatusConflict},
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorSessionMismatch}, http.StatusConflict},
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorToolMismatch}, http.StatusConflict},
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorTerminalConflict}, http.StatusConflict},
		{&frontendtools.InvocationError{Code: frontendtools.InvocationErrorKeyReuse}, http.StatusConflict},
		{errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, frontendToolResultErrorStatus(tt.err), tt.err.Error())
	}
}

func TestFrontendToolResultEndpointRejectsUnsolicitedResult(t *testing.T) {
	_, httpSrv := newTestServer(t, WithFrontendTools(frontendtools.NewManager()), WithChatPlugins(frontendtools.NewPlugin()), WithAutoCreateSessions(true))
	body := `{"toolCallId":"call-1","toolName":"app.confirm_action","status":"success","result":{"approved":true},"executor":{"clientInstanceId":"client-a","connectionId":"connection-a","assignmentId":"assignment-a"}}`
	resp, out := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/sess-tools/tools/results", body)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, string(frontendtools.InvocationErrorUnknownResult), errorCode(t, out))
	require.Empty(t, getSession(t, httpSrv.URL, "sess-tools").Snapshot.Entities)
}

// --- exports and stores -------------------------------------------------------

func TestTimelineExportJSONDownload(t *testing.T) {
	_, httpSrv := newTestServer(t)
	id := createSession(t, httpSrv.URL, `{"id":"sess-export-1"}`).SessionID
	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"export this timeline"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	waitForFinished(t, httpSrv.URL, id)

	resp, out := do(t, http.MethodGet, httpSrv.URL+"/api/chat/sessions/"+id+"/timeline?format=json&view=entities&download=true", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	require.Contains(t, resp.Header.Get("Content-Disposition"), "pinocchio-sess-export-1-timeline.json")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(out, &payload))
	require.Equal(t, "sess-export-1", payload["session_id"])
	require.Len(t, payload["entities"], 2)
}

func TestTurnsExportMinitraceWithFileBackedDB(t *testing.T) {
	turnsDBPath := filepath.Join(t.TempDir(), "turns.db")
	turnStore, err := chatstore.NewSQLiteTurnStore(turnsDBPath)
	require.NoError(t, err)
	defer func() { _ = turnStore.Close() }()
	turn := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn, turns.NewUserTextBlock("pinocchio minitrace export"))
	payload, err := serde.ToYAML(turn, serde.Options{})
	require.NoError(t, err)
	require.NoError(t, turnStore.Save(context.Background(), "sess-minitrace", "sess-minitrace", "turn-1", "final", 1000, string(payload), chatstore.TurnSaveOptions{RuntimeKey: "gpt-5-mini"}))

	_, httpSrv := newTestServer(t, WithTurnStore(turnStore), WithTurnsDBPath(turnsDBPath), WithAutoCreateSessions(true))
	resp, out := do(t, http.MethodGet, httpSrv.URL+"/api/chat/sessions/sess-minitrace/turns?format=minitrace&download=true", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Disposition"), "pinocchio-sess-minitrace-turns.minitrace.json")
	var mt map[string]any
	require.NoError(t, json.Unmarshal(out, &mt))
	require.Equal(t, "minitrace-v0.2.0", mt["schema_version"])
}

func TestTurnsExportYAMLAndMinitraceMissingPath(t *testing.T) {
	_, httpSrv := newTestServer(t, WithAutoCreateSessions(true), WithTurnStore(&fakeTurnStore{snapshot: &chatstore.TurnSnapshot{
		ConvID: "sess-turn-export", SessionID: "sess-turn-export", TurnID: "turn-1", Phase: "final", RuntimeKey: "runtime-a", CreatedAtMs: 1000, Payload: "id: turn-1\n",
	}}))
	resp, out := do(t, http.MethodGet, httpSrv.URL+"/api/chat/sessions/sess-turn-export/turns?format=yaml&download=true", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/x-yaml", resp.Header.Get("Content-Type"))
	require.Contains(t, string(out), "turn_id: turn-1")
	require.Contains(t, string(out), "runtime_key: runtime-a")

	resp, _ = do(t, http.MethodGet, httpSrv.URL+"/api/chat/sessions/sess-turn-export/turns?format=minitrace&download=true", "")
	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

// The default in-memory turn store means a full export always has the turns.
func TestFullExportHasTheTimelineAndTheTurns(t *testing.T) {
	_, httpSrv := newTestServer(t, WithAutoCreateSessions(true))
	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/sess-full-export/messages", `{"prompt":"export bundle"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	waitForFinished(t, httpSrv.URL, "sess-full-export")
	resp, out := do(t, http.MethodGet, httpSrv.URL+"/api/chat/sessions/sess-full-export/export?format=json", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(out, &payload))
	require.Contains(t, payload, "timeline")
	require.Contains(t, payload, "turns")
}

func TestSQLiteStoresSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	open := func() (*Server, *httptest.Server) {
		store, err := sessions.NewSQLiteStore("file:" + filepath.Join(dir, "sessions.db") + "?_busy_timeout=5000")
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })
		return newTestServer(t,
			WithSessionStore(store),
			WithHydrationStoreSpec(serverkit.StoreSpec{Backend: serverkit.StoreBackendSQLite, Path: filepath.Join(dir, "timeline.db")}))
	}
	serverA, httpA := open()
	id := createSession(t, httpA.URL, `{"title":"kept"}`).SessionID
	resp, _ := do(t, http.MethodPost, httpA.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"persist across restart"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	waitForFinished(t, httpA.URL, id)
	httpA.Close()
	require.NoError(t, serverA.Close())

	_, httpB := open()
	got := getSession(t, httpB.URL, id)
	require.Equal(t, "kept", got.Session.Title)
	require.Equal(t, "finished", got.Snapshot.Status)
	require.Equal(t, "Answer: persist across restart", entityTexts(got.Snapshot)["assistant"])
}

// --- helpers -------------------------------------------------------------------

type fakeTurnStore struct {
	mu       sync.Mutex
	snapshot *chatstore.TurnSnapshot
}

func (s *fakeTurnStore) Save(context.Context, string, string, string, string, int64, string, chatstore.TurnSaveOptions) error {
	return nil
}

func (s *fakeTurnStore) List(context.Context, chatstore.TurnQuery) ([]chatstore.TurnSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snapshot == nil {
		return nil, nil
	}
	return []chatstore.TurnSnapshot{*s.snapshot}, nil
}

func (s *fakeTurnStore) LoadLatestTurn(context.Context, string, string) (*chatstore.TurnSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot, nil
}

func (s *fakeTurnStore) Close() error { return nil }

func readServerFrame(t *testing.T, conn *websocket.Conn) *sessionstreamv1.ServerFrame {
	t.Helper()
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	frame := &sessionstreamv1.ServerFrame{}
	require.NoError(t, protojson.Unmarshal(raw, frame))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	return frame
}

func writeClientFrame(t *testing.T, conn *websocket.Conn, payload map[string]any) {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, bytes.TrimSpace(body)))
}

// pageToolEngine calls the page's "page.read" browser tool (provider name
// "page_read") once, then answers with what the page returned.
type pageToolEngine struct{}

func (pageToolEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	for _, b := range t.Blocks {
		if b.Kind == turns.BlockKindToolUse {
			turns.AppendBlock(t, turns.NewAssistantTextBlock(fmt.Sprintf("the page says: %v", b.Payload[turns.PayloadKeyResult])))
			return t, nil
		}
	}
	turns.AppendBlock(t, turns.NewToolCallBlock("call-page-1", "page_read", map[string]any{"selector": "#title"}))
	return t, nil
}

func TestBrowserToolRoundTripThroughHTTP(t *testing.T) {
	_, httpSrv := newTestServer(t,
		WithFrontendTools(frontendtools.NewManager()),
		WithChatPlugins(frontendtools.NewPlugin()),
		// No registry of its own: the page's tools are the run's only tools.
		WithRuntimeResolver(RuntimeResolverFunc(func(context.Context, *http.Request, sessions.Info, MessageRequest) (*infruntime.ComposedRuntime, error) {
			return &infruntime.ComposedRuntime{Engine: pageToolEngine{}}, nil
		})))
	base := httpSrv.URL + "/api/chat/sessions/"
	id := createSession(t, httpSrv.URL, ``).SessionID

	resp, out := do(t, http.MethodPost, base+id+"/tools/manifest", `{"clientInstanceId":"tab-1","connectionId":"conn-1","revision":1,"tools":[{"name":"page.read","description":"Read the page","mode":"frontend","inputSchema":{"type":"object","properties":{"selector":{"type":"string"}}},"available":true}]}`)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(out))
	var manifest frontendToolManifestResponse
	require.NoError(t, json.Unmarshal(out, &manifest))
	executor, err := json.Marshal(manifest.Executor)
	require.NoError(t, err)

	events, cancel := readSSE(t, base+id+"/events")
	defer cancel()
	resp, _ = do(t, http.MethodPost, base+id+"/messages", `{"prompt":"what is the title?"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var requested map[string]any
	timeout := time.After(3 * time.Second)
	for requested == nil {
		select {
		case ev := <-events:
			if ev.event != "ui" {
				continue
			}
			var msg struct {
				Name    string         `json:"name"`
				Payload map[string]any `json:"payload"`
			}
			require.NoError(t, json.Unmarshal([]byte(ev.data), &msg))
			if msg.Name == frontendtools.EventCallRequested {
				requested = msg.Payload
			}
		case <-timeout:
			t.Fatal("the browser tool call was never requested")
		}
	}
	require.Equal(t, "call-page-1", requested["toolCallId"])
	require.Equal(t, "page.read", requested["toolName"])

	// A string result, not an object: the model gets exactly that JSON value.
	resp, out = do(t, http.MethodPost, base+id+"/tools/results", fmt.Sprintf(`{"toolCallId":"call-page-1","toolName":"page.read","status":"success","result":"Hello from the page","executor":%s}`, executor))
	require.Equal(t, http.StatusOK, resp.StatusCode, string(out))

	got := waitForFinished(t, httpSrv.URL, id)
	require.Equal(t, `the page says: "Hello from the page"`, entityTexts(got.Snapshot)["assistant"])
}

type unknownPresetError struct{ name string }

func (e unknownPresetError) Error() string { return "unknown preset " + e.name }
func (unknownPresetError) Code() string    { return "unknown_preset" }

func TestCreateHookCompletesOrRejectsSessions(t *testing.T) {
	hook := func(_ context.Context, p Principal, info *sessions.Info) error {
		if info.Labels == nil {
			info.Labels = map[string]string{}
		}
		if info.Labels["preset"] == "" {
			info.Labels["preset"] = "coder"
		}
		if info.Labels["preset"] != "coder" {
			return unknownPresetError{info.Labels["preset"]}
		}
		info.ID = "hooks-cannot-change-ids"
		return nil
	}
	srv, httpSrv := newTestServer(t, WithCreateHook(hook), WithAutoCreateSessions(true))

	created := createSession(t, httpSrv.URL, `{"data":{"instructions":"be brief"}}`)
	require.Equal(t, "coder", created.Session.Labels["preset"])
	require.NotEqual(t, "hooks-cannot-change-ids", created.SessionID)
	require.JSONEq(t, `{"instructions":"be brief"}`, string(created.Session.Data))

	resp, out := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions", `{"labels":{"preset":"poet"}}`)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "unknown_preset", errorCode(t, out))

	resp, _ = do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/auto-1/messages", `{"prompt":"hi"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	info, err := srv.Sessions().Get(context.Background(), "auto-1")
	require.NoError(t, err)
	require.Equal(t, "coder", info.Labels["preset"], "auto-created sessions pass through the hook too")
}

func TestListShowsWhetherASessionRuns(t *testing.T) {
	_, httpSrv := newTestServer(t, WithRuntimeResolver(engineResolver(answerEngine{chunk: 1, delay: 20 * time.Millisecond})))
	busy := createSession(t, httpSrv.URL, ``).SessionID
	idle := createSession(t, httpSrv.URL, ``).SessionID
	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+busy+"/messages", `{"prompt":"take your time"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	resp, out := do(t, http.MethodGet, httpSrv.URL+"/api/chat/sessions", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list SessionListResponse
	require.NoError(t, json.Unmarshal(out, &list))
	running := map[string]bool{}
	for _, s := range list.Sessions {
		running[s.ID] = s.Running
	}
	require.Equal(t, map[string]bool{busy: true, idle: false}, running)
	resp, _ = do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+busy+"/stop", "")
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func TestASessionRemembersItsConversation(t *testing.T) {
	seen := make(chan *turns.Turn, 2)
	_, httpSrv := newTestServer(t, WithRuntimeResolver(engineResolver(answerEngine{seen: seen})))
	id := createSession(t, httpSrv.URL, ``).SessionID
	for _, prompt := range []string{"first", "second"} {
		resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"`+prompt+`"}`)
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
		<-seen
		waitForFinished(t, httpSrv.URL, id)
		for getSession(t, httpSrv.URL, id).Running {
			time.Sleep(5 * time.Millisecond)
		}
	}
	resp, _ := do(t, http.MethodPost, httpSrv.URL+"/api/chat/sessions/"+id+"/messages", `{"prompt":"third"}`)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	turn := <-seen
	require.Len(t, turn.Blocks, 5, "first, its answer, second, its answer, third")
	require.Equal(t, "third", turn.Blocks[4].Payload[turns.PayloadKeyText])
}
