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

	"github.com/go-go-golems/geppetto/pkg/events"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

type integrationNoopEngine struct{}

func (integrationNoopEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	return t, nil
}

type integrationNoopSink struct{}

func (integrationNoopSink) PublishEvent(events.Event) error { return nil }

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
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("default"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"}},
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("agent"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"},
			Policy:  gepprofiles.PolicySpec{AllowOverrides: true},
		},
	)
	require.NoError(t, err)
	requestResolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))
	chatHandler := webhttp.NewChatHandler(webchatSrv.ChatService(), requestResolver)
	wsHandler := webhttp.NewWSHandler(
		webchatSrv.StreamHub(),
		requestResolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)

	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
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

func TestAppOwnedProfileAPI_CRUDLifecycle_ContractShape(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	listResp, err := http.Get(srv.URL + "/api/chat/profiles")
	require.NoError(t, err)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listed []map[string]any
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))
	require.GreaterOrEqual(t, len(listed), 2)
	assertProfileListItemContract(t, listed[0])
	assertProfileListItemContract(t, listed[1])
	require.Equal(t, "agent", listed[0]["slug"])
	require.Equal(t, "default", listed[1]["slug"])

	createResp, err := http.Post(srv.URL+"/api/chat/profiles", "application/json", strings.NewReader(`{
		"slug":"analyst",
		"display_name":"Analyst",
		"description":"Integration analyst profile",
		"runtime":{"system_prompt":"You are analyst"},
		"extensions":{"WebChat.Starter_Suggestions@V1":{"items":["hello"]}},
		"set_default":true
	}`))
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
	assertProfileDocumentContract(t, created)
	require.Equal(t, "analyst", created["slug"])
	require.Equal(t, true, created["is_default"])

	getResp, err := http.Get(srv.URL + "/api/chat/profiles/analyst")
	require.NoError(t, err)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusOK, getResp.StatusCode)
	var got map[string]any
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&got))
	assertProfileDocumentContract(t, got)
	require.Equal(t, "analyst", got["slug"])
	extensions, ok := got["extensions"].(map[string]any)
	require.True(t, ok)
	_, ok = extensions["webchat.starter_suggestions@v1"]
	require.True(t, ok)

	patchReq, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/chat/profiles/analyst", strings.NewReader(`{
		"display_name":"Analyst V2",
		"extensions":{"webchat.starter_suggestions@v1":{"items":["updated"]}},
		"expected_version":1
	}`))
	require.NoError(t, err)
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := http.DefaultClient.Do(patchReq)
	require.NoError(t, err)
	defer patchResp.Body.Close()
	require.Equal(t, http.StatusOK, patchResp.StatusCode)
	var patched map[string]any
	require.NoError(t, json.NewDecoder(patchResp.Body).Decode(&patched))
	assertProfileDocumentContract(t, patched)
	metadata, ok := patched["metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(2), metadata["version"])

	setDefaultResp, err := http.Post(srv.URL+"/api/chat/profiles/default/default", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer setDefaultResp.Body.Close()
	require.Equal(t, http.StatusOK, setDefaultResp.StatusCode)
	var defaultDoc map[string]any
	require.NoError(t, json.NewDecoder(setDefaultResp.Body).Decode(&defaultDoc))
	assertProfileDocumentContract(t, defaultDoc)
	require.Equal(t, "default", defaultDoc["slug"])
	require.Equal(t, true, defaultDoc["is_default"])

	deleteReq, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/chat/profiles/analyst?expected_version=2", nil)
	require.NoError(t, err)
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	require.NoError(t, err)
	defer deleteResp.Body.Close()
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	getDeletedResp, err := http.Get(srv.URL + "/api/chat/profiles/analyst")
	require.NoError(t, err)
	defer getDeletedResp.Body.Close()
	require.Equal(t, http.StatusNotFound, getDeletedResp.StatusCode)
}

func TestAppOwnedProfileSelection_InFlightConversation_RebuildsRuntime(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	selectDefaultResp, err := http.Post(srv.URL+"/api/chat/profile", "application/json", strings.NewReader(`{"slug":"default"}`))
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

	selectAgentResp, err := http.Post(srv.URL+"/api/chat/profile", "application/json", strings.NewReader(`{"slug":"agent"}`))
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

	selectResp, err := http.Post(srv.URL+"/api/chat/profile", "application/json", strings.NewReader(`{"slug":"agent"}`))
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

func assertProfileListItemContract(t *testing.T, item map[string]any) {
	t.Helper()
	require.NotEmpty(t, item["slug"])
	assertAllowedContractKeys(
		t,
		item,
		"slug",
		"display_name",
		"description",
		"default_prompt",
		"extensions",
		"is_default",
		"version",
	)
}

func assertProfileDocumentContract(t *testing.T, doc map[string]any) {
	t.Helper()
	require.NotEmpty(t, doc["registry"])
	require.NotEmpty(t, doc["slug"])
	_, hasDefault := doc["is_default"]
	require.True(t, hasDefault)
	assertAllowedContractKeys(
		t,
		doc,
		"registry",
		"slug",
		"display_name",
		"description",
		"runtime",
		"policy",
		"metadata",
		"extensions",
		"is_default",
	)
}

func assertAllowedContractKeys(t *testing.T, payload map[string]any, allowed ...string) {
	t.Helper()
	allowedSet := map[string]struct{}{}
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range payload {
		if _, ok := allowedSet[key]; !ok {
			t.Fatalf("unexpected profile API contract key: %s", key)
		}
	}
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
