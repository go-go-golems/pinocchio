package agentserver

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/httpapi"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/presets"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/scripted"
	"github.com/stretchr/testify/require"
)

func writePresets(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"bench.md": "You drive the workbench.",
		"bench.yaml": `name: bench
description: Workbench agent
systemPrompt: bench.md
pageInstructions: true
maxRounds: 2
`,
		"plain.yaml": `name: plain
tools: [noop]
maxRounds: 2
`,
		"tight.yaml": `name: tight
tools: [noop]
budget:
  tokensPerSession: 1
`,
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	return dir
}

func noopRegistry(t *testing.T) geptools.ToolRegistry {
	t.Helper()
	reg := geptools.NewInMemoryToolRegistry()
	def, err := geptools.NewToolFromFunc("noop", "does nothing", func(struct{}) (string, error) { return "ok", nil })
	require.NoError(t, err)
	require.NoError(t, reg.RegisterTool("noop", *def))
	return reg
}

func start(t *testing.T, presetDir, stateDir string) *httptest.Server {
	t.Helper()
	ps, err := presets.Load(presetDir)
	require.NoError(t, err)
	srv, err := New(ps,
		WithEngine(scripted.Model, func() (engine.Engine, error) { return &scripted.Engine{}, nil }),
		WithStateDir(stateDir),
		WithDefaultPreset("bench"),
		WithToolRegistry(noopRegistry(t)))
	require.NoError(t, err)
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		httpSrv.Close()
		_ = srv.Close()
	})
	return httpSrv
}

func call(t *testing.T, method, url, body string) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, out
}

func codeOf(t *testing.T, body []byte) string {
	t.Helper()
	var e httpapi.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &e), string(body))
	return e.Code
}

func waitIdle(t *testing.T, base, id string) httpapi.SessionResponse {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		status, out := call(t, http.MethodGet, base+"/v1/sessions/"+id, "")
		require.Equal(t, http.StatusOK, status, string(out))
		var got httpapi.SessionResponse
		require.NoError(t, json.Unmarshal(out, &got))
		if !got.Running && got.Snapshot != nil && len(got.Snapshot.Entities) > 1 {
			return got
		}
		require.True(t, time.Now().Before(deadline), "session %s did not finish", id)
		time.Sleep(10 * time.Millisecond)
	}
}

func texts(got httpapi.SessionResponse) []string {
	var out []string
	for _, e := range got.Snapshot.Entities {
		if p, ok := e.Payload.(map[string]any); ok {
			if s, ok := p["text"].(string); ok && s != "" {
				out = append(out, p["role"].(string)+": "+s)
			}
		}
	}
	return out
}

func TestInfoRoutes(t *testing.T) {
	base := start(t, writePresets(t), t.TempDir()).URL
	status, out := call(t, http.MethodGet, base+"/v1/health", "")
	require.Equal(t, http.StatusOK, status)
	require.JSONEq(t, `{"ok":true,"presets":3}`, string(out))

	status, out = call(t, http.MethodGet, base+"/v1/presets", "")
	require.Equal(t, http.StatusOK, status)
	var ps struct{ Presets []PresetView }
	require.NoError(t, json.Unmarshal(out, &ps))
	require.Len(t, ps.Presets, 3)
	require.Equal(t, PresetView{Name: "bench", Description: "Workbench agent", Model: "fake:scripted", MaxRounds: 2, PageInstructions: true, Default: true}, ps.Presets[0])

	status, out = call(t, http.MethodGet, base+"/v1/models", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, string(out), `"ref":"fake:scripted"`)
}

func TestCreateChecksThePresetAndTheInstructions(t *testing.T) {
	base := start(t, writePresets(t), t.TempDir()).URL
	status, out := call(t, http.MethodPost, base+"/v1/sessions", `{}`)
	require.Equal(t, http.StatusCreated, status, string(out))
	var created httpapi.SessionResponse
	require.NoError(t, json.Unmarshal(out, &created))
	require.Equal(t, "bench", created.Session.Labels["preset"], "the default preset")

	status, out = call(t, http.MethodPost, base+"/v1/sessions", `{"labels":{"preset":"poet"}}`)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, CodeUnknownPreset, codeOf(t, out))

	status, out = call(t, http.MethodPost, base+"/v1/sessions", `{"labels":{"preset":"plain"},"data":{"instructions":"x"}}`)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, CodeInstructionsNotAllowed, codeOf(t, out))

	status, out = call(t, http.MethodPost, base+"/v1/sessions", `{"data":{"instructions":"`+strings.Repeat("x", DefaultMaxPageInstructionsBytes+1)+`"}}`)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, CodePromptTooLarge, codeOf(t, out))
}

func TestABrowserToolRunsThroughTheScriptAndTheSystemPromptStays(t *testing.T) {
	stateDir := t.TempDir()
	base := start(t, writePresets(t), stateDir).URL
	status, out := call(t, http.MethodPost, base+"/v1/sessions", `{"labels":{"preset":"bench"},"data":{"instructions":"API: bench.help()"}}`)
	require.Equal(t, http.StatusCreated, status, string(out))
	var created httpapi.SessionResponse
	require.NoError(t, json.Unmarshal(out, &created))
	sess := base + "/v1/sessions/" + created.SessionID

	status, out = call(t, http.MethodPost, sess+"/tools/manifest", `{"clientInstanceId":"tab","connectionId":"conn","revision":1,"tools":[{"name":"eval","mode":"frontend","inputSchema":{"type":"object"},"available":true}]}`)
	require.Equal(t, http.StatusOK, status, string(out))
	var manifest struct {
		Executor json.RawMessage `json:"executor"`
	}
	require.NoError(t, json.Unmarshal(out, &manifest))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sess+"/events", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	lines := bufio.NewScanner(resp.Body)

	status, out = call(t, http.MethodPost, sess+"/messages", `{"prompt":"/tool eval {\"code\":\"1+1\"}"}`)
	require.Equal(t, http.StatusAccepted, status, string(out))
	callID := ""
	for callID == "" && lines.Scan() {
		line := lines.Text()
		if !strings.HasPrefix(line, "data: ") || !strings.Contains(line, "ChatFrontendToolCallRequested") {
			continue
		}
		var ev struct {
			Payload struct {
				ToolCallID string `json:"toolCallId"`
			} `json:"payload"`
		}
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev))
		callID = ev.Payload.ToolCallID
	}
	require.NotEmpty(t, callID)
	status, out = call(t, http.MethodPost, sess+"/tools/results", `{"toolCallId":"`+callID+`","toolName":"eval","status":"success","result":2,"executor":`+string(manifest.Executor)+`}`)
	require.Equal(t, http.StatusOK, status, string(out))
	require.Contains(t, texts(waitIdle(t, base, created.SessionID)), "assistant: eval returned: 2")

	status, out = call(t, http.MethodGet, sess+"/turns?format=yaml", "")
	require.Equal(t, http.StatusOK, status, string(out))
	require.Equal(t, 1, strings.Count(string(out), "kind: system"), "one system block")
	require.Less(t, strings.Index(string(out), "You drive the workbench."), strings.Index(string(out), "API: bench.help()"), "the preset's prompt, then the page's instructions")
	require.Less(t, strings.Index(string(out), "API: bench.help()"), strings.Index(string(out), "kind: user"), "before the conversation")
}

func TestTheRoundLimitEndsWithAFinalAnswer(t *testing.T) {
	base := start(t, writePresets(t), t.TempDir()).URL
	status, out := call(t, http.MethodPost, base+"/v1/sessions", `{"labels":{"preset":"plain"}}`)
	require.Equal(t, http.StatusCreated, status, string(out))
	var created httpapi.SessionResponse
	require.NoError(t, json.Unmarshal(out, &created))
	sess := base + "/v1/sessions/" + created.SessionID

	status, _ = call(t, http.MethodPost, sess+"/messages", `{"prompt":"/tools 5 noop {}"}`)
	require.Equal(t, http.StatusAccepted, status)
	require.Contains(t, texts(waitIdle(t, base, created.SessionID)), "assistant: final answer after 2 tool calls")

	status, out = call(t, http.MethodGet, sess+"/turns?format=yaml", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, string(out), "You have used all 2 tool rounds of this turn.", "the final-round instruction stays in the conversation")
}

func TestTheBudgetStopsARunBetweenRounds(t *testing.T) {
	base := start(t, writePresets(t), t.TempDir()).URL
	status, out := call(t, http.MethodPost, base+"/v1/sessions", `{"labels":{"preset":"tight"}}`)
	require.Equal(t, http.StatusCreated, status, string(out))
	var created httpapi.SessionResponse
	require.NoError(t, json.Unmarshal(out, &created))

	status, _ = call(t, http.MethodPost, base+"/v1/sessions/"+created.SessionID+"/messages", `{"prompt":"/tools 5 noop {}"}`)
	require.Equal(t, http.StatusAccepted, status)
	got := waitIdle(t, base, created.SessionID)
	require.Len(t, texts(got), 2)
	require.True(t, strings.HasPrefix(texts(got)[1], "error: the session used "), texts(got))
	require.True(t, strings.HasSuffix(texts(got)[1], " of its 1 tokens"), texts(got))
}

func TestARestartKeepsSessionsAndTheirConversations(t *testing.T) {
	presetDir, stateDir := writePresets(t), t.TempDir()
	first := start(t, presetDir, stateDir)
	status, out := call(t, http.MethodPost, first.URL+"/v1/sessions", `{"title":"kept"}`)
	require.Equal(t, http.StatusCreated, status, string(out))
	var created httpapi.SessionResponse
	require.NoError(t, json.Unmarshal(out, &created))
	status, _ = call(t, http.MethodPost, first.URL+"/v1/sessions/"+created.SessionID+"/messages", `{"prompt":"remember me"}`)
	require.Equal(t, http.StatusAccepted, status)
	waitIdle(t, first.URL, created.SessionID)
	first.Close()

	second := start(t, presetDir, stateDir)
	status, _ = call(t, http.MethodPost, second.URL+"/v1/sessions/"+created.SessionID+"/messages", `{"prompt":"again"}`)
	require.Equal(t, http.StatusAccepted, status)
	got := waitIdle(t, second.URL, created.SessionID)
	require.Equal(t, "kept", got.Session.Title)
	for deadline := time.Now().Add(5 * time.Second); len(texts(got)) < 4; got = waitIdle(t, second.URL, created.SessionID) {
		require.True(t, time.Now().Before(deadline), "conversation: %v", texts(got))
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, []string{"user: remember me", "assistant: echo: remember me", "user: again", "assistant: echo: again"}, texts(got))

	status, out = call(t, http.MethodGet, second.URL+"/v1/sessions/"+created.SessionID+"/turns?format=yaml", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, string(out), "remember me", "the second run saw the first one's history")
}
