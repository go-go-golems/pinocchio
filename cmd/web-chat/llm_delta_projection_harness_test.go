package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
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

type harnessLLMDeltaEngine struct {
	messageID  uuid.UUID
	cumulative string
}

func (e harnessLLMDeltaEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	messageID := e.messageID
	if messageID == uuid.Nil {
		messageID = uuid.New()
	}
	cumulative := strings.TrimSpace(e.cumulative)
	if cumulative == "" {
		cumulative = "hello"
	}
	meta := events.EventMetadata{ID: messageID}
	events.PublishEventToContext(ctx, events.NewStartEvent(meta))
	events.PublishEventToContext(ctx, events.NewPartialCompletionEvent(meta, cumulative, cumulative))
	return t, nil
}

func configureHarnessTimelineScript(t *testing.T, script string) {
	t.Helper()
	webchat.ClearTimelineHandlers()
	webchat.RegisterDefaultTimelineHandlers()
	webchat.ClearTimelineRuntime()

	scriptPath := filepath.Join(t.TempDir(), "timeline-harness.js")
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o600))
	require.NoError(t, configureTimelineJSScripts([]string{scriptPath}))

	t.Cleanup(func() {
		webchat.ClearTimelineHandlers()
		webchat.RegisterDefaultTimelineHandlers()
		webchat.ClearTimelineRuntime()
	})
}

func newLLMDeltaProjectionHarnessServer(t *testing.T, eng engine.Engine) *httptest.Server {
	t.Helper()

	if eng == nil {
		eng = harnessLLMDeltaEngine{}
	}

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
			Engine:             eng,
			RuntimeKey:         runtimeKey,
			RuntimeFingerprint: "fp-" + runtimeKey,
			SeedSystemPrompt:   "seed",
			// Keep Sink nil so Router installs default Watermill sink per conversation topic.
			Sink: nil,
		}, nil
	})

	webchatSrv, err := webchat.NewServer(context.Background(), parsed, staticFS, webchat.WithRuntimeComposer(runtimeComposer))
	require.NoError(t, err)

	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("default"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"}},
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
	timelineLogger := log.With().Str("component", "webchat-harness-test").Str("route", "/api/timeline").Logger()
	timelineHandler := webhttp.NewTimelineHandler(webchatSrv.TimelineService(), timelineLogger)
	appMux.HandleFunc("/api/timeline", timelineHandler)
	appMux.HandleFunc("/api/timeline/", timelineHandler)
	appMux.Handle("/api/", webchatSrv.APIHandler())
	appMux.Handle("/", webchatSrv.UIHandler())

	return httptest.NewServer(appMux)
}

func submitHarnessPrompt(t *testing.T, baseURL string, convID string, prompt string) {
	t.Helper()
	body := map[string]any{
		"prompt":  prompt,
		"conv_id": convID,
	}
	b, err := json.Marshal(body)
	require.NoError(t, err)
	resp, err := http.Post(baseURL+"/chat/default", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func decodeSnapshotEntities(raw any) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	entities := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entity, ok := item.(map[string]any)
		if !ok {
			continue
		}
		entities = append(entities, entity)
	}
	return entities
}

func waitForHarnessEntities(
	t *testing.T,
	baseURL string,
	convID string,
	predicate func([]map[string]any) bool,
) []map[string]any {
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

func TestLLMDeltaProjectionHarness_NonConsumingReducerAddsSideProjection(t *testing.T) {
	messageID := uuid.New()
	configureHarnessTimelineScript(t, `
const p = require("pinocchio");
p.timeline.registerSemReducer("llm.delta", function(ev) {
  return {
    consume: false,
    upserts: [{
      id: ev.id + "-delta-projection",
      kind: "llm.delta.projection",
      props: { cumulative: ev.data && ev.data.cumulative }
    }]
  };
});
`)
	srv := newLLMDeltaProjectionHarnessServer(t, harnessLLMDeltaEngine{
		messageID:  messageID,
		cumulative: "hello",
	})
	defer srv.Close()

	convID := "conv-llm-delta-nonconsume"
	submitHarnessPrompt(t, srv.URL, convID, "start projection test")

	entities := waitForHarnessEntities(t, srv.URL, convID, func(entities []map[string]any) bool {
		_, messageFound := findEntityByID(entities, messageID.String())
		_, sideFound := findEntityByID(entities, messageID.String()+"-delta-projection")
		return messageFound && sideFound
	})

	messageEntity, ok := findEntityByID(entities, messageID.String())
	require.True(t, ok)
	messageProps, _ := messageEntity["props"].(map[string]any)
	require.Equal(t, "hello", messageProps["content"])

	sideEntity, ok := findEntityByID(entities, messageID.String()+"-delta-projection")
	require.True(t, ok)
	sideProps, _ := sideEntity["props"].(map[string]any)
	require.Equal(t, "hello", sideProps["cumulative"])
}

func TestLLMDeltaProjectionHarness_ConsumingReducerSuppressesBuiltinDeltaProjection(t *testing.T) {
	messageID := uuid.New()
	configureHarnessTimelineScript(t, `
const p = require("pinocchio");
p.timeline.registerSemReducer("llm.delta", function(ev) {
  return {
    consume: true,
    upserts: [{
      id: ev.id + "-delta-consumed",
      kind: "llm.delta.consumed",
      props: { cumulative: ev.data && ev.data.cumulative }
    }]
  };
});
`)
	srv := newLLMDeltaProjectionHarnessServer(t, harnessLLMDeltaEngine{
		messageID:  messageID,
		cumulative: "hello",
	})
	defer srv.Close()

	convID := "conv-llm-delta-consume"
	submitHarnessPrompt(t, srv.URL, convID, "start consume test")

	entities := waitForHarnessEntities(t, srv.URL, convID, func(entities []map[string]any) bool {
		_, messageFound := findEntityByID(entities, messageID.String())
		_, consumedFound := findEntityByID(entities, messageID.String()+"-delta-consumed")
		return messageFound && consumedFound
	})

	messageEntity, ok := findEntityByID(entities, messageID.String())
	require.True(t, ok)
	messageProps, _ := messageEntity["props"].(map[string]any)
	require.Equal(t, "", messageProps["content"], fmt.Sprintf("expected llm.delta consume=true to block builtin delta projection for message %s", messageID))

	consumedEntity, ok := findEntityByID(entities, messageID.String()+"-delta-consumed")
	require.True(t, ok)
	consumedProps, _ := consumedEntity["props"].(map[string]any)
	require.Equal(t, "hello", consumedProps["cumulative"])
}

func TestLLMDeltaProjectionHarness_HandlerRunsBeforeReducer(t *testing.T) {
	messageID := uuid.New()
	configureHarnessTimelineScript(t, `
const p = require("pinocchio");
var deltaSeen = 0;
p.timeline.onSem("llm.delta", function(ev) {
  if (ev && ev.type === "llm.delta") {
    deltaSeen = deltaSeen + 1;
  }
});
p.timeline.registerSemReducer("llm.delta", function(ev) {
  return {
    consume: false,
    upserts: [{
      id: ev.id + "-handler-order",
      kind: "llm.delta.handler.order",
      props: { seen: deltaSeen }
    }]
  };
});
`)
	srv := newLLMDeltaProjectionHarnessServer(t, harnessLLMDeltaEngine{
		messageID:  messageID,
		cumulative: "hello",
	})
	defer srv.Close()

	convID := "conv-llm-delta-handler-order"
	submitHarnessPrompt(t, srv.URL, convID, "start handler order test")

	entities := waitForHarnessEntities(t, srv.URL, convID, func(entities []map[string]any) bool {
		_, found := findEntityByID(entities, messageID.String()+"-handler-order")
		return found
	})

	orderEntity, ok := findEntityByID(entities, messageID.String()+"-handler-order")
	require.True(t, ok)
	props, _ := orderEntity["props"].(map[string]any)
	require.Equal(t, float64(1), props["seen"])
}
