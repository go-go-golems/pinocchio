package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	appserver "github.com/go-go-golems/pinocchio/cmd/web-chat/app"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/profiles"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

const migrationCaptureDirEnv = "WEBCHAT_MIGRATION_CAPTURE_DIR"

type comparisonRuntimeResolver struct {
	completion string
}

func (r comparisonRuntimeResolver) Resolve(context.Context, *http.Request, string, string) (*infruntime.ComposedRuntime, error) {
	return &infruntime.ComposedRuntime{Engine: comparisonRuntimeEngine(r)}, nil
}

type comparisonRuntimeEngine struct {
	completion string
}

type legacyProfileRequestResolver struct {
	*profiles.RequestResolver
}

func newLegacyProfileRequestResolver(profileRegistry gepprofiles.Registry, defaultRegistry gepprofiles.RegistrySlug) *legacyProfileRequestResolver {
	return &legacyProfileRequestResolver{
		RequestResolver: profiles.NewRequestResolver(profileRegistry, defaultRegistry, nil),
	}
}

func (r *legacyProfileRequestResolver) Resolve(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	if req == nil {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request"}
	}
	switch req.Method {
	case http.MethodGet:
		return r.resolveWS(req)
	case http.MethodPost:
		return r.resolveChat(req)
	default:
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{Status: http.StatusMethodNotAllowed, ClientMsg: "method not allowed"}
	}
}

func (r *legacyProfileRequestResolver) resolveWS(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
	if convID == "" {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "missing conv_id"}
	}
	profileSlug, err := r.ResolveProfileSelection(req.Context(), "", "", "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	registrySlug, err := r.ResolveRegistrySelection("", "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedProfile, err := r.ResolveEffectiveProfile(req.Context(), registrySlug, profileSlug)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	plan, err := r.BuildConversationPlan(req.Context(), convID, "", "", resolvedProfile)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	return resolvedConversationRequestFromPlan(plan), nil
}

func (r *legacyProfileRequestResolver) resolveChat(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	var body webhttp.ChatRequestBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request", Err: err}
	}
	if body.Prompt == "" && body.Text != "" {
		body.Prompt = body.Text
	}
	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
	}
	profileSlug, err := r.ResolveProfileSelection(req.Context(), "", body.Profile, "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	registrySlug, err := r.ResolveRegistrySelection(body.Registry, "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedProfile, err := r.ResolveEffectiveProfile(req.Context(), registrySlug, profileSlug)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	plan, err := r.BuildConversationPlan(req.Context(), convID, body.Prompt, strings.TrimSpace(body.IdempotencyKey), resolvedProfile)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	return resolvedConversationRequestFromPlan(plan), nil
}

func resolvedConversationRequestFromPlan(plan *profiles.ConversationPlan) webhttp.ResolvedConversationRequest {
	if plan == nil || plan.Runtime == nil {
		return webhttp.ResolvedConversationRequest{}
	}
	return webhttp.ResolvedConversationRequest{
		ConvID:                    plan.ConvID,
		RuntimeKey:                plan.Runtime.RuntimeKey,
		RuntimeFingerprint:        plan.Runtime.RuntimeFingerprint,
		ProfileVersion:            plan.Runtime.ProfileVersion,
		ResolvedInferenceSettings: profiles.CloneResolvedInferenceSettings(plan.Runtime.InferenceSettings),
		ResolvedRuntime:           profiles.ToRuntimeTransport(plan.Runtime),
		ProfileMetadata:           profiles.CopyMetadataMap(plan.Runtime.ProfileMetadata),
		Prompt:                    plan.Prompt,
		IdempotencyKey:            plan.IdempotencyKey,
	}
}

func registerLegacyProfileAPIHandlers(mux *http.ServeMux, resolver *profiles.RequestResolver) {
	if mux == nil || resolver == nil || resolver.Registry() == nil {
		return
	}
	profiles.RegisterAPIHandlers(mux, resolver.Registry(), profiles.APIOptions{
		DefaultRegistrySlug:             resolver.DefaultRegistrySlug(),
		EnableCurrentProfileCookieRoute: true,
		CurrentProfileCookieName:        "chat_profile",
		ExtensionSchemas: []profiles.ExtensionSchemaDocument{{
			Key: "webchat.starter_suggestions@v1",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
						"default": []any{},
					},
				},
				"required":             []any{"items"},
				"additionalProperties": false,
			},
		}},
	})
}

func (e comparisonRuntimeEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	completion := strings.TrimSpace(e.completion)
	if completion == "" {
		completion = "comparison runtime response"
	}
	meta := gepevents.EventMetadata{}
	gepevents.PublishEventToContext(ctx, gepevents.NewStartEvent(meta))
	gepevents.PublishEventToContext(ctx, gepevents.NewPartialCompletionEvent(meta, completion, completion))
	return t, nil
}

type legacyFlowCapture struct {
	RouteFamily      string         `json:"routeFamily"`
	SubmitPath       string         `json:"submitPath"`
	WebSocketPath    string         `json:"webSocketPath"`
	SnapshotPath     string         `json:"snapshotPath"`
	ConversationID   string         `json:"conversationId"`
	SubmitStatus     int            `json:"submitStatus"`
	HelloEventType   string         `json:"helloEventType"`
	RuntimeKey       string         `json:"runtimeKey,omitempty"`
	AssistantEntity  map[string]any `json:"assistantEntity,omitempty"`
	ExpectedBehavior []string       `json:"expectedBehavior"`
}

type canonicalFlowCapture struct {
	RouteFamily         string         `json:"routeFamily"`
	CreateSessionPath   string         `json:"createSessionPath"`
	SubmitPath          string         `json:"submitPath"`
	SnapshotPath        string         `json:"snapshotPath"`
	WebSocketPath       string         `json:"webSocketPath"`
	SessionID           string         `json:"sessionId"`
	CreateStatus        int            `json:"createStatus"`
	SubmitStatus        int            `json:"submitStatus"`
	HelloFrameType      string         `json:"helloFrameType"`
	SnapshotFrameType   string         `json:"snapshotFrameType"`
	SubscribedFrameType string         `json:"subscribedFrameType"`
	UIEventNames        []string       `json:"uiEventNames"`
	FinalSnapshot       map[string]any `json:"finalSnapshot"`
	ExpectedBehavior    []string       `json:"expectedBehavior"`
}

func writeJSONArtifactIfRequested(t *testing.T, filename string, payload any) {
	t.Helper()
	dir := strings.TrimSpace(os.Getenv(migrationCaptureDirEnv))
	if dir == "" {
		return
	}
	require.NoError(t, os.MkdirAll(dir, 0o755))
	body, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), body, 0o644))
}

func decodeJSONMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

func readWSJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	return decodeJSONMap(t, raw)
}

func waitForCanonicalUIEvents(t *testing.T, conn *websocket.Conn, limit int, stopName string) []string {
	t.Helper()
	if limit <= 0 {
		limit = 8
	}
	names := make([]string, 0, limit)
	deadline := time.Now().Add(3 * time.Second)
	for len(names) < limit && time.Now().Before(deadline) {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			require.NoError(t, err)
		}
		frame := decodeJSONMap(t, raw)
		if frame["type"] != "ui-event" {
			continue
		}
		name, _ := frame["name"].(string)
		if name == "" {
			continue
		}
		names = append(names, name)
		if stopName != "" && name == stopName {
			break
		}
	}
	return names
}

type legacyHarnessEngine struct {
	messageID  uuid.UUID
	cumulative string
}

func (e legacyHarnessEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	messageID := e.messageID
	if messageID == uuid.Nil {
		messageID = uuid.New()
	}
	cumulative := strings.TrimSpace(e.cumulative)
	if cumulative == "" {
		cumulative = "hello"
	}
	meta := gepevents.EventMetadata{ID: messageID}
	gepevents.PublishEventToContext(ctx, gepevents.NewStartEvent(meta))
	gepevents.PublishEventToContext(ctx, gepevents.NewPartialCompletionEvent(meta, cumulative, cumulative))
	return t, nil
}

type legacyHarnessTimelineService struct{}

func (legacyHarnessTimelineService) Snapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV2, error) {
	return &timelinepb.TimelineSnapshotV2{ConvId: convID, Version: 1}, nil
}

func newLegacyHarnessServer(t *testing.T, eng engine.Engine) *httptest.Server {
	t.Helper()
	if eng == nil {
		eng = legacyHarnessEngine{}
	}
	parsed := values.New()
	staticFS := fstest.MapFS{"static/index.html": {Data: []byte("<html><body>legacy</body></html>")}}
	runtimeComposer := infruntime.RuntimeBuilderFunc(func(_ context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		runtimeKey := strings.TrimSpace(req.ProfileKey)
		if runtimeKey == "" {
			runtimeKey = "default"
		}
		return infruntime.ComposedRuntime{Engine: eng, RuntimeKey: runtimeKey, RuntimeFingerprint: "fp-" + runtimeKey, SeedSystemPrompt: "seed"}, nil
	})
	webchatSrv, err := webchat.NewServer(context.Background(), parsed, staticFS, webchat.WithRuntimeComposer(runtimeComposer))
	require.NoError(t, err)
	profileRegistry, err := profiles.NewInMemoryProfileService("default", testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}))
	require.NoError(t, err)
	requestResolver := newLegacyProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(profiles.DefaultRegistrySlug))
	chatHandler := webhttp.NewChatHandler(webchatSrv.ChatService(), requestResolver)
	wsHandler := webhttp.NewWSHandler(webchatSrv.StreamHub(), requestResolver, websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }})
	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
	appMux.HandleFunc("/ws", wsHandler)
	registerLegacyProfileAPIHandlers(appMux, requestResolver.RequestResolver)
	timelineLogger := log.With().Str("component", "webchat-legacy-test").Str("route", "/api/timeline").Logger()
	timelineHandler := webhttp.NewTimelineHandler(webchatSrv.TimelineService(), timelineLogger)
	appMux.HandleFunc("/api/timeline", timelineHandler)
	appMux.HandleFunc("/api/timeline/", timelineHandler)
	appMux.Handle("/api/", webchatSrv.APIHandler())
	appMux.Handle("/", webchatSrv.UIHandler())
	srv := httptest.NewServer(appMux)
	t.Cleanup(func() {
		srv.Close()
		require.NoError(t, webchatSrv.Close())
	})
	return srv
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

func decodeSnapshotEntities(raw any) []map[string]any {
	items, _ := raw.([]any)
	entities := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entity, _ := item.(map[string]any)
		if len(entity) > 0 {
			entities = append(entities, entity)
		}
	}
	return entities
}

func waitForHarnessEntities(t *testing.T, baseURL string, convID string, predicate func([]map[string]any) bool) []map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last []map[string]any
	for time.Now().Before(deadline) {
		endpoint := baseURL + "/api/timeline?conv_id=" + url.QueryEscape(convID)
		resp, err := http.Get(endpoint)
		require.NoError(t, err)
		var snap map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&snap))
		_ = resp.Body.Close()
		entities := decodeSnapshotEntities(snap["entities"])
		last = entities
		if predicate(entities) {
			return entities
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for timeline entities (last=%v)", last)
	return nil
}

func findEntityByID(entities []map[string]any, id string) (map[string]any, bool) {
	for _, entity := range entities {
		eid, _ := entity["id"].(string)
		if eid == id {
			return entity, true
		}
	}
	return nil, false
}

func captureLegacyFlow(t *testing.T, prompt string) legacyFlowCapture {
	t.Helper()
	messageID := uuid.New()
	srv := newLegacyHarnessServer(t, legacyHarnessEngine{
		messageID:  messageID,
		cumulative: "Answer: " + prompt,
	})
	defer srv.Close()

	convID := "legacy-conv-1"
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?conv_id=" + convID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	hello := readWSJSON(t, conn)
	require.Equal(t, "ws.hello", integrationSemEventType(mustMarshalJSON(t, hello)))

	body := []byte(`{"prompt":"` + prompt + `","conv_id":"` + convID + `"}`)
	resp, err := http.Post(srv.URL+"/chat/default", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	entities := waitForHarnessEntities(t, srv.URL, convID, func(entities []map[string]any) bool {
		_, found := findEntityByID(entities, messageID.String())
		return found
	})
	assistantEntity, found := findEntityByID(entities, messageID.String())
	require.True(t, found)

	return legacyFlowCapture{
		RouteFamily:     "legacy-webchat",
		SubmitPath:      "/chat/default",
		WebSocketPath:   "/ws?conv_id=" + convID,
		SnapshotPath:    "/api/timeline?conv_id=" + convID,
		ConversationID:  convID,
		SubmitStatus:    resp.StatusCode,
		HelloEventType:  integrationSemEventType(mustMarshalJSON(t, hello)),
		RuntimeKey:      integrationSemRuntimeKey(mustMarshalJSON(t, hello)),
		AssistantEntity: assistantEntity,
		ExpectedBehavior: []string{
			"submit prompt via legacy /chat route",
			"attach websocket by conv_id query string",
			"hydrate transcript from /api/timeline",
			"assistant transcript entity eventually contains final content",
		},
	}
}

func captureCanonicalFlow(t *testing.T, prompt string) canonicalFlowCapture {
	t.Helper()
	canonicalApp, err := appserver.NewServer(appserver.WithRuntimeResolver(comparisonRuntimeResolver{completion: "Answer: " + prompt}))
	require.NoError(t, err)
	defer func() { _ = canonicalApp.Close() }()
	profileRegistry, err := profiles.NewInMemoryProfileService(
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
	)
	require.NoError(t, err)
	resolver := profiles.NewRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(profiles.DefaultRegistrySlug), nil)
	appConfigJS, err := runtimeConfigScript("", false)
	require.NoError(t, err)
	appFS := fstest.MapFS{"static/index.html": {Data: []byte("<html><body>canonical comparison</body></html>")}}
	mux := buildAppMux(appFS, appConfigJS, resolver, canonicalApp)
	httpSrv := httptest.NewServer(mux)
	defer httpSrv.Close()

	createResp, err := http.Post(httpSrv.URL+"/api/chat/sessions", "application/json", strings.NewReader(`{"profile":"default"}`))
	require.NoError(t, err)
	defer func() { _ = createResp.Body.Close() }()
	require.Equal(t, http.StatusOK, createResp.StatusCode)
	var created struct {
		SessionID string `json:"sessionId"`
	}
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
	require.NotEmpty(t, created.SessionID)

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/chat/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	hello := readWSJSON(t, conn)
	require.Equal(t, "hello", hello["type"])

	require.NoError(t, conn.WriteJSON(map[string]any{
		"type":         "subscribe",
		"sessionId":    created.SessionID,
		"sinceOrdinal": "0",
	}))
	snapshotFrame := readWSJSON(t, conn)
	require.Equal(t, "snapshot", snapshotFrame["type"])
	subscribedFrame := readWSJSON(t, conn)
	require.Equal(t, "subscribed", subscribedFrame["type"])

	submitPath := "/api/chat/sessions/" + created.SessionID + "/messages"
	submitResp, err := http.Post(httpSrv.URL+submitPath, "application/json", strings.NewReader(`{"prompt":"`+prompt+`","profile":"default"}`))
	require.NoError(t, err)
	defer func() { _ = submitResp.Body.Close() }()
	require.Equal(t, http.StatusOK, submitResp.StatusCode)
	uiEventNames := waitForCanonicalUIEvents(t, conn, 12, "ChatMessageFinished")
	require.NotEmpty(t, uiEventNames)

	snapshotPath := "/api/chat/sessions/" + created.SessionID
	time.Sleep(100 * time.Millisecond)
	finalResp, err := http.Get(httpSrv.URL + snapshotPath)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, finalResp.StatusCode)
	var finalSnap map[string]any
	require.NoError(t, json.NewDecoder(finalResp.Body).Decode(&finalSnap))
	_ = finalResp.Body.Close()

	return canonicalFlowCapture{
		RouteFamily:         "canonical-evtstream",
		CreateSessionPath:   "/api/chat/sessions",
		SubmitPath:          submitPath,
		SnapshotPath:        snapshotPath,
		WebSocketPath:       "/api/chat/ws",
		SessionID:           created.SessionID,
		CreateStatus:        createResp.StatusCode,
		SubmitStatus:        submitResp.StatusCode,
		HelloFrameType:      fmt.Sprintf("%v", hello["type"]),
		SnapshotFrameType:   fmt.Sprintf("%v", snapshotFrame["type"]),
		SubscribedFrameType: fmt.Sprintf("%v", subscribedFrame["type"]),
		UIEventNames:        uiEventNames,
		FinalSnapshot:       finalSnap,
		ExpectedBehavior: []string{
			"create or resume session explicitly",
			"subscribe websocket by client frame instead of conv_id query",
			"receive snapshot then subscribed then ui-event frames",
			"read final transcript from canonical session snapshot endpoint",
		},
	}
}

func snapshotText(snapshot map[string]any) string {
	if snapshot == nil {
		return ""
	}
	entities, _ := snapshot["entities"].([]any)
	for _, raw := range entities {
		entity, _ := raw.(map[string]any)
		payload, _ := entity["payload"].(map[string]any)
		text, _ := payload["text"].(string)
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func hasFinishedSnapshotText(snapshot map[string]any) bool {
	if snapshot == nil || snapshot["status"] != "finished" {
		return false
	}
	return strings.TrimSpace(snapshotText(snapshot)) != ""
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return body
}

func TestMigrationComparison_LegacyAndCanonicalHappyPath(t *testing.T) {
	legacy := captureLegacyFlow(t, "Explain ordinals in plain language")
	canonical := captureCanonicalFlow(t, "Explain ordinals in plain language")

	legacyProps, _ := legacy.AssistantEntity["props"].(map[string]any)
	require.NotNil(t, legacyProps)
	require.Equal(t, "/chat/default", legacy.SubmitPath)
	require.Equal(t, "ws.hello", legacy.HelloEventType)

	require.Equal(t, "/api/chat/sessions", canonical.CreateSessionPath)
	require.Equal(t, "hello", canonical.HelloFrameType)
	require.Equal(t, "snapshot", canonical.SnapshotFrameType)
	require.Equal(t, "subscribed", canonical.SubscribedFrameType)
	require.NotEmpty(t, canonical.UIEventNames)

	writeJSONArtifactIfRequested(t, "05-legacy-flow-transcript.json", legacy)
	writeJSONArtifactIfRequested(t, "06-canonical-flow-transcript.json", canonical)
}
