package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

type integrationNoopEngine struct{}

func (integrationNoopEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	return t, nil
}

type integrationNoopSink struct{}

func (integrationNoopSink) PublishEvent(events.Event) error { return nil }

type integrationFakeRunner struct{}

func (integrationFakeRunner) Start(ctx context.Context, req webchat.StartRequest) (webchat.StartResult, error) {
	props, err := structpb.NewStruct(map[string]any{
		"runner":  "fake",
		"content": "fake runner emitted timeline entity",
	})
	if err != nil {
		return webchat.StartResult{}, err
	}
	if err := req.Timeline.Upsert(ctx, &timelinepb.TimelineEntityV2{
		Id:    "fake-" + req.ConvID,
		Kind:  "runner.status",
		Props: props,
	}, 1); err != nil {
		return webchat.StartResult{}, err
	}
	return webchat.StartResult{
		Response: map[string]any{
			"status":     "started",
			"runner":     "fake",
			"conv_id":    req.ConvID,
			"session_id": req.SessionID,
		},
	}, nil
}

func newAppOwnedIntegrationServer(t *testing.T) *httptest.Server {
	t.Helper()

	parsed := values.New()
	staticFS := fstest.MapFS{
		"static/index.html": {Data: []byte("<html><body>ok</body></html>")},
	}
	runtimeComposer := infruntime.RuntimeBuilderFunc(func(_ context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		runtimeKey := strings.TrimSpace(req.ProfileKey)
		if runtimeKey == "" {
			runtimeKey = "default"
		}
		return infruntime.ComposedRuntime{
			Engine:             integrationNoopEngine{},
			Sink:               integrationNoopSink{},
			RuntimeKey:         runtimeKey,
			RuntimeFingerprint: "fp-" + runtimeKey,
			SeedSystemPrompt:   "seed",
		}, nil
	})

	webchatSrv, err := webchat.NewServer(context.Background(), parsed, staticFS, webchat.WithRuntimeComposer(runtimeComposer))
	require.NoError(t, err)

	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.EngineProfile{Slug: gepprofiles.MustEngineProfileSlug("default"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"}},
		&gepprofiles.EngineProfile{
			Slug:    gepprofiles.MustEngineProfileSlug("agent"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"},
		},
	)
	require.NoError(t, err)
	requestResolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug), nil)
	chatHandler := webhttp.NewChatHandler(webchatSrv.ChatService(), requestResolver)
	runnerHandler := func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		plan, err := requestResolver.Resolve(req)
		if err != nil {
			http.Error(w, "failed to resolve request", http.StatusInternalServerError)
			return
		}
		if strings.TrimSpace(plan.Prompt) == "" {
			http.Error(w, "missing prompt", http.StatusBadRequest)
			return
		}
		result, err := webchatSrv.ChatService().StartPromptWithRunner(req.Context(), webchatSrv.ChatService().NewLLMLoopRunner(), webchat.StartPromptWithRunnerInput{
			Runtime:        plan.RuntimeRequest(),
			IdempotencyKey: webhttp.IdempotencyKeyFromRequest(req, nil),
			Payload: webchat.LLMLoopStartPayload{
				Prompt: plan.Prompt,
			},
			Metadata: map[string]any{"route": "chat-runner"},
		})
		if err != nil {
			http.Error(w, "runner start failed", http.StatusInternalServerError)
			return
		}
		if result.HTTPStatus > 0 {
			w.WriteHeader(result.HTTPStatus)
		}
		_ = json.NewEncoder(w).Encode(result.Response)
	}
	fakeRunnerHandler := func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		plan, err := requestResolver.Resolve(req)
		if err != nil {
			http.Error(w, "failed to resolve request", http.StatusInternalServerError)
			return
		}
		_, startReq, err := webchatSrv.ChatService().PrepareRunnerStart(req.Context(), webchat.PrepareRunnerStartInput{
			Runtime:  plan.RuntimeRequest(),
			Metadata: map[string]any{"route": "fake-runner"},
		})
		if err != nil {
			http.Error(w, "prepare runner start failed", http.StatusInternalServerError)
			return
		}
		result, err := integrationFakeRunner{}.Start(req.Context(), startReq)
		if err != nil {
			http.Error(w, "runner start failed", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(result.Response)
	}
	wsHandler := webhttp.NewWSHandler(
		webchatSrv.StreamHub(),
		requestResolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)

	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
	appMux.HandleFunc("/chat-runner", runnerHandler)
	appMux.HandleFunc("/chat-runner/", runnerHandler)
	appMux.HandleFunc("/fake-runner", fakeRunnerHandler)
	appMux.HandleFunc("/fake-runner/", fakeRunnerHandler)
	appMux.HandleFunc("/ws", wsHandler)
	registerProfileAPIHandlers(appMux, requestResolver)
	timelineLogger := log.With().Str("component", "webchat-test").Str("route", "/api/timeline").Logger()
	timelineHandler := webhttp.NewTimelineHandler(webchatSrv.TimelineService(), timelineLogger)
	appMux.HandleFunc("/api/timeline", timelineHandler)
	appMux.HandleFunc("/api/timeline/", timelineHandler)
	appMux.Handle("/api/", webchatSrv.APIHandler())
	appMux.Handle("/", webchatSrv.UIHandler())

	httpSrv := httptest.NewServer(appMux)
	return httpSrv
}

func TestAppOwnedChatRunnerHandler_Integration_DefaultProfilePath(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	reqBody := []byte(`{"prompt":"hello from runner path","conv_id":"conv-chat-runner-1"}`)
	resp, err := http.Post(srv.URL+"/chat-runner/default", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, "started", payload["status"])
	require.Equal(t, "conv-chat-runner-1", payload["conv_id"])
	require.NotEmpty(t, payload["session_id"])
	require.NotEmpty(t, payload["idempotency_key"])
}

func TestAppOwnedFakeRunner_Integration_TimelineHydrates(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	convID := "conv-fake-runner-1"
	reqBody := []byte(`{"conv_id":"` + convID + `"}`)
	resp, err := http.Post(srv.URL+"/fake-runner/default", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !found {
		timelineResp, err := http.Get(srv.URL + "/api/timeline?conv_id=" + convID)
		require.NoError(t, err)
		var snap map[string]any
		require.NoError(t, json.NewDecoder(timelineResp.Body).Decode(&snap))
		_ = timelineResp.Body.Close()

		entities, _ := snap["entities"].([]any)
		for _, raw := range entities {
			entity, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id, _ := entity["id"].(string)
			if id == "fake-"+convID {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(50 * time.Millisecond)
		}
	}
	require.True(t, found, "expected fake runner timeline entity")
}

func TestAppOwnedChatRunner_Integration_WebSocketReceivesChatMessage(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	convID := "conv-chat-runner-ws-1"
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?conv_id=" + convID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))

	reqBody := []byte(`{"prompt":"hello over runner websocket","conv_id":"` + convID + `"}`)
	resp, err := http.Post(srv.URL+"/chat-runner/default", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !found {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
		_, frame, readErr := conn.ReadMessage()
		if readErr != nil {
			if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
				continue
			}
			require.NoError(t, readErr)
		}
		if integrationSemEventType(frame) == "chat.message" {
			found = true
		}
	}
	require.True(t, found, "expected chat.message frame on websocket")
}

func TestAppOwnedChatHandler_Integration_DefaultProfilePath(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	reqBody := []byte(`{"prompt":"hello from integration test","conv_id":"conv-chat-1"}`)
	resp, err := http.Post(srv.URL+"/chat/default", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, "started", payload["status"])
	require.Equal(t, "conv-chat-1", payload["conv_id"])
	require.NotEmpty(t, payload["session_id"])
	require.NotEmpty(t, payload["idempotency_key"])
}

func TestAppOwnedWSHandler_Integration_HelloAndPong(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?conv_id=conv-ws-1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("ping")))

	seenPong := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !seenPong {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
		_, frame, readErr := conn.ReadMessage()
		if readErr != nil {
			if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
				continue
			}
			require.NoError(t, readErr)
		}
		if integrationSemEventType(frame) == "ws.pong" {
			seenPong = true
		}
	}
	require.True(t, seenPong, "expected ws.pong response to ping")
}

func TestAppOwnedChatHandler_Integration_UserMessageProjectedViaStream(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	convID := "conv-chat-proj-1"
	prompt := "hello from chat.message projection"
	reqBody := []byte(`{"prompt":"` + prompt + `","conv_id":"` + convID + `"}`)
	resp, err := http.Post(srv.URL+"/chat/default", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	turnID, _ := payload["turn_id"].(string)
	require.NotEmpty(t, turnID)
	expectedID := "user-" + turnID

	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !found {
		timelineResp, err := http.Get(srv.URL + "/api/timeline?conv_id=" + convID)
		require.NoError(t, err)
		var snap map[string]any
		require.NoError(t, json.NewDecoder(timelineResp.Body).Decode(&snap))
		_ = timelineResp.Body.Close()

		entities, _ := snap["entities"].([]any)
		for _, raw := range entities {
			entity, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id, _ := entity["id"].(string)
			if id != expectedID {
				continue
			}
			props, _ := entity["props"].(map[string]any)
			if props == nil {
				continue
			}
			role, _ := props["role"].(string)
			content, _ := props["content"].(string)
			streaming, _ := props["streaming"].(bool)
			if role == "user" && content == prompt && !streaming {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(50 * time.Millisecond)
		}
	}
	require.True(t, found, "expected user timeline entity projected from chat.message stream event")
}

func TestAppOwnedProfileSelection_InFlightConversation_RebuildsRuntime(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	selectDefaultResp, err := http.Post(srv.URL+"/api/chat/profile", "application/json", strings.NewReader(`{"profile":"default","registry":"default"}`))
	require.NoError(t, err)
	defer selectDefaultResp.Body.Close()
	require.Equal(t, http.StatusOK, selectDefaultResp.StatusCode)
	defaultCookie := mustProfileCookie(t, selectDefaultResp)

	const convID = "conv-inflight-profile-switch-1"
	chatReqDefault, err := http.NewRequest(
		http.MethodPost,
		srv.URL+"/chat",
		strings.NewReader(`{"prompt":"start default","conv_id":"`+convID+`"}`),
	)
	require.NoError(t, err)
	chatReqDefault.Header.Set("Content-Type", "application/json")
	chatReqDefault.AddCookie(defaultCookie)
	chatRespDefault, err := http.DefaultClient.Do(chatReqDefault)
	require.NoError(t, err)
	defer chatRespDefault.Body.Close()
	require.Equal(t, http.StatusOK, chatRespDefault.StatusCode)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?conv_id=" + convID
	defaultWSHeaders := http.Header{}
	defaultWSHeaders.Add("Cookie", defaultCookie.String())
	defaultConn, _, err := websocket.DefaultDialer.Dial(wsURL, defaultWSHeaders)
	require.NoError(t, err)
	require.NoError(t, defaultConn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, defaultHelloFrame, err := defaultConn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(defaultHelloFrame))
	require.Equal(t, "default", integrationSemRuntimeKey(defaultHelloFrame))
	_ = defaultConn.Close()

	selectAgentResp, err := http.Post(srv.URL+"/api/chat/profile", "application/json", strings.NewReader(`{"profile":"agent","registry":"default"}`))
	require.NoError(t, err)
	defer selectAgentResp.Body.Close()
	require.Equal(t, http.StatusOK, selectAgentResp.StatusCode)
	agentCookie := mustProfileCookie(t, selectAgentResp)

	chatReqAgent, err := http.NewRequest(
		http.MethodPost,
		srv.URL+"/chat",
		strings.NewReader(`{"prompt":"switch to agent","conv_id":"`+convID+`"}`),
	)
	require.NoError(t, err)
	chatReqAgent.Header.Set("Content-Type", "application/json")
	chatReqAgent.AddCookie(agentCookie)
	chatRespAgent, err := http.DefaultClient.Do(chatReqAgent)
	require.NoError(t, err)
	defer chatRespAgent.Body.Close()
	require.Equal(t, http.StatusOK, chatRespAgent.StatusCode)

	agentWSHeaders := http.Header{}
	agentWSHeaders.Add("Cookie", agentCookie.String())
	agentConn, _, err := websocket.DefaultDialer.Dial(wsURL, agentWSHeaders)
	require.NoError(t, err)
	require.NoError(t, agentConn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, agentHelloFrame, err := agentConn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(agentHelloFrame))
	require.Equal(t, "agent", integrationSemRuntimeKey(agentHelloFrame))
	_ = agentConn.Close()
}

func TestAppOwnedProfileSelection_AffectsNextConversationCreation(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	selectResp, err := http.Post(srv.URL+"/api/chat/profile", "application/json", strings.NewReader(`{"profile":"agent","registry":"default"}`))
	require.NoError(t, err)
	defer selectResp.Body.Close()
	require.Equal(t, http.StatusOK, selectResp.StatusCode)
	agentCookie := mustProfileCookie(t, selectResp)

	const convID = "conv-profile-select-next-1"
	chatReq, err := http.NewRequest(
		http.MethodPost,
		srv.URL+"/chat",
		strings.NewReader(`{"prompt":"hello agent","conv_id":"`+convID+`"}`),
	)
	require.NoError(t, err)
	chatReq.Header.Set("Content-Type", "application/json")
	chatReq.AddCookie(agentCookie)
	chatResp, err := http.DefaultClient.Do(chatReq)
	require.NoError(t, err)
	defer chatResp.Body.Close()
	require.Equal(t, http.StatusOK, chatResp.StatusCode)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?conv_id=" + convID
	headers := http.Header{}
	headers.Add("Cookie", agentCookie.String())
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))
	require.Equal(t, "agent", integrationSemRuntimeKey(helloFrame))
	_ = conn.Close()
}

func TestAppOwnedProfileSelection_LegacyCookie_AffectsNextConversationCreation(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	legacyCookie := &http.Cookie{Name: currentProfileCookieName, Value: "agent"}

	const convID = "conv-profile-select-legacy-cookie-1"
	chatReq, err := http.NewRequest(
		http.MethodPost,
		srv.URL+"/chat",
		strings.NewReader(`{"prompt":"hello legacy agent","conv_id":"`+convID+`"}`),
	)
	require.NoError(t, err)
	chatReq.Header.Set("Content-Type", "application/json")
	chatReq.AddCookie(legacyCookie)
	chatResp, err := http.DefaultClient.Do(chatReq)
	require.NoError(t, err)
	defer chatResp.Body.Close()
	require.Equal(t, http.StatusOK, chatResp.StatusCode)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?conv_id=" + convID
	headers := http.Header{}
	headers.Add("Cookie", legacyCookie.String())
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))
	require.Equal(t, "agent", integrationSemRuntimeKey(helloFrame))
	_ = conn.Close()
}

func TestProfileAPI_InvalidSlugAndRegistry_ReturnBadRequest(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	invalidRegistryResp, err := http.Get(srv.URL + "/api/chat/profiles?registry=invalid registry!")
	require.NoError(t, err)
	defer invalidRegistryResp.Body.Close()
	require.Equal(t, http.StatusBadRequest, invalidRegistryResp.StatusCode)

	invalidSlugResp, err := http.Post(
		srv.URL+"/api/chat/profile",
		"application/json",
		strings.NewReader(`{"profile":"not a valid slug!","registry":"default"}`),
	)
	require.NoError(t, err)
	defer invalidSlugResp.Body.Close()
	require.Equal(t, http.StatusBadRequest, invalidSlugResp.StatusCode)
}

func integrationSemEventType(frame []byte) string {
	var env struct {
		Event struct {
			Type string `json:"type"`
		} `json:"event"`
	}
	if err := json.Unmarshal(frame, &env); err != nil {
		return ""
	}
	return env.Event.Type
}

func integrationSemRuntimeKey(frame []byte) string {
	var env struct {
		Event struct {
			Data struct {
				RuntimeKey      string `json:"runtimeKey"`
				RuntimeKeySnake string `json:"runtime_key"`
			} `json:"data"`
		} `json:"event"`
	}
	if err := json.Unmarshal(frame, &env); err != nil {
		return ""
	}
	if env.Event.Data.RuntimeKey != "" {
		return env.Event.Data.RuntimeKey
	}
	return env.Event.Data.RuntimeKeySnake
}

func mustProfileCookie(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	require.NotNil(t, resp)
	for _, ck := range resp.Cookies() {
		if strings.TrimSpace(ck.Name) == "chat_profile" && strings.TrimSpace(ck.Value) != "" {
			return ck
		}
	}
	t.Fatalf("expected chat_profile cookie")
	return nil
}
