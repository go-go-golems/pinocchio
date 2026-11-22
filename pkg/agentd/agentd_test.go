package agentd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAgent(t *testing.T, extra string) map[string]*AgentConfig {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sys.md"), []byte("You are a test agent."), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("name: test\nsystemPrompt: sys.md\npageInstructions: true\nmodel: fake:scripted\nmaxRounds: 6\ntoolTimeout: 2s\nresultCut: 40\n"+extra), 0o600))
	agents, err := LoadAgents(dir)
	require.NoError(t, err)
	return agents
}

type client struct {
	t    *testing.T
	base string
}

func newServer(t *testing.T, agents map[string]*AgentConfig, store *Store) (*client, *Server) {
	t.Helper()
	srv, err := NewServer(context.Background(), Options{Agents: agents, Engine: func() (engine.Engine, error) { return &FakeEngine{}, nil }, EngineModel: FakeModel, Store: store, Heartbeat: 200 * time.Millisecond})
	require.NoError(t, err)
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	return &client{t: t, base: hs.URL}, srv
}

func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, c.base+path, rd)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	if out != nil && resp.StatusCode < 300 && resp.StatusCode != 204 {
		require.NoError(c.t, json.NewDecoder(resp.Body).Decode(out))
	}
	return resp.StatusCode
}

// events reads SSE events after `after` until pred returns true or the timeout passes.
func (c *client) events(sid string, after int64, timeout time.Duration, pred func(Event) bool) []Event {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", c.base+"/v1/sessions/"+sid+"/events", nil)
	req.Header.Set("Last-Event-ID", fmt.Sprint(after))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var out []Event
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev Event
		require.NoError(c.t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev))
		out = append(out, ev)
		if pred(ev) {
			return out
		}
	}
	c.t.Fatalf("event stream ended without the expected event; got %d events: %v", len(out), types(out))
	return nil
}

func types(evs []Event) []string {
	out := []string{}
	for _, e := range evs {
		out = append(out, e.Type)
	}
	return out
}

func runEnded(e Event) bool {
	return e.Type == EvRunFinished || e.Type == EvRunFailed || e.Type == EvRunCancelled
}

func create(t *testing.T, c *client) SessionView {
	var v SessionView
	require.Equal(t, 201, c.do("POST", "/v1/sessions", map[string]any{"agent": "test", "instructions": "API: eval(code)."}, &v))
	return v
}

func manifest(t *testing.T, c *client, sid string, tools ...map[string]any) map[string]any {
	var ack map[string]any
	require.Equal(t, 200, c.do("POST", "/v1/sessions/"+sid+"/tools/manifest", map[string]any{
		"revision": 1, "clientInstanceId": "tab-1", "connectionId": "conn-1", "tools": tools,
	}, &ack))
	return ack["executor"].(map[string]any)
}

var evalTool = map[string]any{"name": "eval", "description": "Run JavaScript", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"code": map[string]any{"type": "string"}}}}

func TestEchoTurnStreamsAndPersistsHistory(t *testing.T) {
	c, _ := newServer(t, testAgent(t, ""), nil)
	s := create(t, c)
	assert.Equal(t, "fake:scripted", s.Model.Ref)
	var run map[string]any
	require.Equal(t, 202, c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": "hello there", "context": "tab: stage"}, &run))
	evs := c.events(s.ID, 0, 5*time.Second, runEnded)
	last := evs[len(evs)-1]
	require.Equal(t, EvRunFinished, last.Type, "%v", types(evs))
	assert.Equal(t, "echo: hello there", last.Data.(map[string]any)["text"])
	assert.Contains(t, types(evs), EvTextDelta)
	assert.Contains(t, types(evs), EvUsage)

	var v SessionView
	require.Equal(t, 200, c.do("GET", "/v1/sessions/"+s.ID, nil, &v))
	kinds := []string{}
	for _, m := range v.Messages {
		kinds = append(kinds, m.Kind)
	}
	assert.Equal(t, []string{"user", "context", "assistant"}, kinds)
	assert.Equal(t, "hello there", v.Title)
	assert.Equal(t, 1, v.Usage.Rounds)
}

func TestToolRoundTripThroughBrowser(t *testing.T) {
	c, _ := newServer(t, testAgent(t, ""), nil)
	s := create(t, c)
	exec := manifest(t, c, s.ID, evalTool)
	require.Equal(t, 202, c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": `/tool eval {"code":"1+1"}`}, nil))
	evs := c.events(s.ID, 0, 5*time.Second, func(e Event) bool { return e.Type == EvToolRequested })
	req := evs[len(evs)-1].Data.(map[string]any)
	assert.Equal(t, "eval", req["name"])
	assert.Equal(t, "auto", req["mode"])
	assert.Equal(t, map[string]any{"code": "1+1"}, req["input"])
	callID := req["callId"].(string)

	// the page answers
	assert.Equal(t, 204, c.do("POST", "/v1/sessions/"+s.ID+"/tools/"+callID+"/result", map[string]any{"status": "success", "value": "→ 2", "executor": exec, "toolName": "eval"}, nil))
	// a duplicate identical result is accepted idempotently; a different one conflicts
	assert.Equal(t, 204, c.do("POST", "/v1/sessions/"+s.ID+"/tools/"+callID+"/result", map[string]any{"status": "success", "value": "→ 2", "executor": exec, "toolName": "eval"}, nil))
	assert.Equal(t, 409, c.do("POST", "/v1/sessions/"+s.ID+"/tools/"+callID+"/result", map[string]any{"status": "success", "value": "→ 3", "executor": exec, "toolName": "eval"}, nil))

	evs = c.events(s.ID, evs[len(evs)-1].Seq, 5*time.Second, runEnded)
	last := evs[len(evs)-1]
	require.Equal(t, EvRunFinished, last.Type, "%v", types(evs))
	assert.Equal(t, `eval returned: "→ 2"`, last.Data.(map[string]any)["text"])
	assert.Contains(t, types(evs), EvToolFinished)

	var v SessionView
	c.do("GET", "/v1/sessions/"+s.ID, nil, &v)
	kinds := []string{}
	for _, m := range v.Messages {
		kinds = append(kinds, m.Kind)
	}
	assert.Equal(t, []string{"user", "tool_call", "tool_result", "assistant"}, kinds, "the conversation keeps the tool call and its result")
}

func TestResultCutAndToolError(t *testing.T) {
	c, _ := newServer(t, testAgent(t, ""), nil)
	s := create(t, c)
	exec := manifest(t, c, s.ID, evalTool)
	c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": `/tool eval {"code":"big"}`}, nil)
	evs := c.events(s.ID, 0, 5*time.Second, func(e Event) bool { return e.Type == EvToolRequested })
	callID := evs[len(evs)-1].Data.(map[string]any)["callId"].(string)
	c.do("POST", "/v1/sessions/"+s.ID+"/tools/"+callID+"/result", map[string]any{"status": "success", "value": strings.Repeat("x", 100), "executor": exec}, nil)
	evs = c.events(s.ID, 0, 5*time.Second, runEnded)
	text := evs[len(evs)-1].Data.(map[string]any)["text"].(string)
	assert.Contains(t, text, "cut at 40 of 100 characters", "the model sees a cut result")

	seq := evs[len(evs)-1].Seq
	c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": `/tool eval {"code":"throw"}`}, nil)
	evs = c.events(s.ID, seq, 5*time.Second, func(e Event) bool { return e.Type == EvToolRequested })
	callID = evs[len(evs)-1].Data.(map[string]any)["callId"].(string)
	c.do("POST", "/v1/sessions/"+s.ID+"/tools/"+callID+"/result", map[string]any{"status": "failed", "error": "ReferenceError: x is not defined", "executor": exec}, nil)
	evs = c.events(s.ID, seq, 5*time.Second, runEnded)
	assert.Contains(t, evs[len(evs)-1].Data.(map[string]any)["text"], "ReferenceError")
}

func TestToolTimeoutAndCancel(t *testing.T) {
	c, _ := newServer(t, testAgent(t, ""), nil)
	s := create(t, c)
	manifest(t, c, s.ID, evalTool)
	start := time.Now()
	c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": `/tool eval {"code":"never answered"}`}, nil)
	evs := c.events(s.ID, 0, 10*time.Second, runEnded)
	assert.Contains(t, evs[len(evs)-1].Data.(map[string]any)["text"], "did not answer within 2s")
	assert.Less(t, time.Since(start), 8*time.Second)

	seq := evs[len(evs)-1].Seq
	c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": "/sleep 5000 late"}, nil)
	assert.Equal(t, 409, c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": "second"}, nil), "one run per session")
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 202, c.do("POST", "/v1/sessions/"+s.ID+"/cancel", nil, nil))
	evs = c.events(s.ID, seq, 5*time.Second, runEnded)
	assert.Equal(t, EvRunCancelled, evs[len(evs)-1].Type)
}

func TestResumeAfterReconnectAndPersistence(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)
	agents := testAgent(t, "")
	c, _ := newServer(t, agents, store)
	s := create(t, c)
	c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": "one two three four"}, nil)
	all := c.events(s.ID, 0, 5*time.Second, runEnded)
	// reconnect from the middle: only later events are replayed
	mid := all[len(all)/2].Seq
	rest := c.events(s.ID, mid, 5*time.Second, runEnded)
	assert.Equal(t, all[len(all)/2+1:], rest)

	// a new server on the same store restores the conversation and the log
	c2, _ := newServer(t, agents, store)
	var v SessionView
	require.Equal(t, 200, c2.do("GET", "/v1/sessions/"+s.ID, nil, &v))
	require.Len(t, v.Messages, 2)
	assert.Equal(t, "echo: one two three four", v.Messages[1].Text)
	assert.Equal(t, all[len(all)-1].Seq, v.Seq)
	c2.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": "again", "context": "ctx 2"}, nil)
	evs := c2.events(s.ID, v.Seq, 5*time.Second, runEnded)
	assert.Equal(t, EvRunFinished, evs[len(evs)-1].Type)
}

func TestContextBlocksAreReplaced(t *testing.T) {
	c, _ := newServer(t, testAgent(t, ""), nil)
	s := create(t, c)
	seq := int64(0)
	for _, ctx := range []string{"state A", "state B"} {
		c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": "hi", "context": ctx}, nil)
		evs := c.events(s.ID, seq, 5*time.Second, runEnded)
		seq = evs[len(evs)-1].Seq
	}
	var v SessionView
	c.do("GET", "/v1/sessions/"+s.ID, nil, &v)
	ctxs := []string{}
	for _, m := range v.Messages {
		if m.Kind == "context" {
			ctxs = append(ctxs, m.Text)
		}
	}
	assert.Equal(t, []string{"state B"}, ctxs, "only the latest context stays in the conversation")
}

func TestManifestHumanModeAndRejections(t *testing.T) {
	c, _ := newServer(t, testAgent(t, "tools:\n  edit: human\n"), nil)
	s := create(t, c)
	manifest(t, c, s.ID, evalTool, map[string]any{"name": "edit", "description": "Edit a file"})
	c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": `/tool edit {"path":"a"}`}, nil)
	evs := c.events(s.ID, 0, 5*time.Second, func(e Event) bool { return e.Type == EvToolRequested })
	assert.Equal(t, "human", evs[len(evs)-1].Data.(map[string]any)["mode"])
	c.do("POST", "/v1/sessions/"+s.ID+"/cancel", nil, nil)

	assert.Equal(t, 400, c.do("POST", "/v1/sessions", map[string]any{"agent": "nope"}, nil))
	assert.Equal(t, 404, c.do("GET", "/v1/sessions/missing", nil, nil))
	var models map[string]any
	assert.Equal(t, 200, c.do("GET", "/v1/models", nil, &models))
}

func TestDeletedSessionIsNotSavedAgain(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)
	agents := testAgent(t, "")
	c, srv := newServer(t, agents, store)
	s := create(t, c)
	ss := srv.session(s.ID)
	require.NotNil(t, ss)
	c.do("POST", "/v1/sessions/"+s.ID+"/turns", map[string]any{"text": "/sleep 5000 late"}, nil)
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, 204, c.do("DELETE", "/v1/sessions/"+s.ID, nil, nil))
	// the cancelled run's goroutine saves when it returns; so does any late writer
	require.Eventually(t, func() bool { return ss.View(false).Running == "" }, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, store.Save(ss))
	_, err = os.Stat(store.path(s.ID))
	assert.True(t, os.IsNotExist(err), "the deleted session's file is gone")

	c2, _ := newServer(t, agents, store)
	assert.Equal(t, 404, c2.do("GET", "/v1/sessions/"+s.ID, nil, nil), "a restart does not restore it")
}

func TestRepeatPlanWithoutInput(t *testing.T) {
	n, input, ok := repeatPlan("/tools 2 eval")
	assert.True(t, ok)
	assert.Equal(t, 2, n)
	assert.Empty(t, input)
	n, input, ok = repeatPlan(`/tools 3 eval {"code":"1"}`)
	assert.True(t, ok)
	assert.Equal(t, 3, n)
	assert.Equal(t, "1", input["code"])
}
