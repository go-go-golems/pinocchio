package webclient_test

// The browser client mirrors the server's projections. This test makes sure
// they agree: it runs real conversations through agentserver with the
// scripted engine, records every event stream and the server's final
// snapshot, and runs the client's Node tests on those captures
// (test/agree.test.mjs replays each stream into a Timeline and compares it
// with the snapshot). Without node the Node part is skipped.
//
//	go test ./pkg/chatapp/webclient -run TestTheClientAgreesWithTheServer -keep-captures dir

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/agentserver"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/presets"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/scripted"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/webclient"
	"github.com/stretchr/testify/require"
)

var keepCaptures = flag.String("keep-captures", "", "also write the captures to this directory")

type frame struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type capture struct {
	Name    string            `json:"name"`
	Streams [][]frame         `json:"streams"` // each stream: a snapshot, then ui events
	Final   json.RawMessage   `json:"final"`   // the server's snapshot after everything
	Answers []json.RawMessage `json:"-"`
}

type harness struct {
	t    *testing.T
	base string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	write("page.yaml", "name: page\nmaxRounds: 3\n")
	write("server.yaml", "name: server\ntools: [noop]\nmaxRounds: 3\n")
	write("tight.yaml", "name: tight\ntools: [noop]\nbudget:\n  tokensPerSession: 1\n")
	ps, err := presets.Load(dir)
	require.NoError(t, err)
	reg := geptools.NewInMemoryToolRegistry()
	def, err := geptools.NewToolFromFunc("noop", "does nothing", func(struct{}) (string, error) { return "ok", nil })
	require.NoError(t, err)
	require.NoError(t, reg.RegisterTool("noop", *def))
	srv, err := agentserver.New(ps,
		agentserver.WithEngine(scripted.Model, func() (engine.Engine, error) { return &scripted.Engine{Delay: 15 * time.Millisecond}, nil }),
		agentserver.WithDefaultPreset("page"),
		agentserver.WithToolRegistry(reg),
		agentserver.WithStateDir(t.TempDir()))
	require.NoError(t, err)
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		httpSrv.Close()
		_ = srv.Close()
	})
	return &harness{t: t, base: httpSrv.URL + "/v1"}
}

func (h *harness) call(method, path, body string) (int, map[string]any) {
	h.t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, h.base+path, r)
	require.NoError(h.t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (h *harness) create(preset string) string {
	status, out := h.call(http.MethodPost, "/sessions", `{"labels":{"preset":"`+preset+`"}}`)
	require.Equal(h.t, http.StatusCreated, status, out)
	return out["sessionId"].(string)
}

// stream records one event stream until stop is called.
type stream struct {
	mu     sync.Mutex
	frames []frame
	cancel func()
	done   chan struct{}
}

func (h *harness) listen(id string) *stream {
	h.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.base+"/sessions/"+id+"/events", nil)
	require.NoError(h.t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	s := &stream{cancel: cancel, done: make(chan struct{})}
	gotSnapshot := make(chan struct{})
	go func() {
		defer close(s.done)
		defer func() { _ = resp.Body.Close() }()
		lines := bufio.NewScanner(resp.Body)
		lines.Buffer(make([]byte, 1<<20), 1<<24)
		event := ""
		for lines.Scan() {
			line := lines.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				s.mu.Lock()
				s.frames = append(s.frames, frame{Event: event, Data: json.RawMessage(strings.TrimPrefix(line, "data: "))})
				first := len(s.frames) == 1
				s.mu.Unlock()
				if first {
					close(gotSnapshot)
				}
			}
		}
	}()
	select {
	case <-gotSnapshot:
	case <-time.After(5 * time.Second):
		h.t.Fatal("no snapshot")
	}
	return s
}

func (s *stream) stop() []frame {
	s.cancel()
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]frame(nil), s.frames...)
}

func (s *stream) waitFor(t *testing.T, name string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		for _, f := range s.frames {
			var ev struct {
				Name    string         `json:"name"`
				Payload map[string]any `json:"payload"`
			}
			if f.Event == "ui" && json.Unmarshal(f.Data, &ev) == nil && ev.Name == name {
				s.mu.Unlock()
				return ev.Payload
			}
		}
		s.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no %s event", name)
	return nil
}

func (h *harness) idle(id string) json.RawMessage {
	h.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, out := h.call(http.MethodGet, "/sessions/"+id, "")
		require.Equal(h.t, http.StatusOK, status)
		if running, _ := out["running"].(bool); !running {
			time.Sleep(100 * time.Millisecond) // let the streams receive the last events
			_, out = h.call(http.MethodGet, "/sessions/"+id, "")
			raw, err := json.Marshal(out["snapshot"])
			require.NoError(h.t, err)
			return raw
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("session %s did not finish", id)
	return nil
}

func (h *harness) send(id, body string) {
	h.t.Helper()
	status, out := h.call(http.MethodPost, "/sessions/"+id+"/messages", body)
	require.Equal(h.t, http.StatusAccepted, status, out)
}

func TestTheClientAgreesWithTheServer(t *testing.T) {
	h := newHarness(t)
	var captures []capture

	// thinking, a server tool, text; then a second answer in the same session
	{
		id := h.create("server")
		s := h.listen(id)
		h.send(id, `{"prompt":"/think plan the call -- /tool noop {}"}`)
		h.idle(id)
		h.send(id, `{"prompt":"thanks","context":"[page state]"}`)
		final := h.idle(id)
		captures = append(captures, capture{Name: "server-tool-two-answers", Streams: [][]frame{s.stop()}, Final: final})
	}
	// a browser tool, and a second stream that attaches in the middle of the answer
	{
		id := h.create("page")
		status, out := h.call(http.MethodPost, "/sessions/"+id+"/tools/manifest", `{"clientInstanceId":"tab","connectionId":"c1","revision":1,"tools":[{"name":"eval","mode":"frontend","inputSchema":{"type":"object"},"available":true}]}`)
		require.Equal(t, http.StatusOK, status, out)
		executor, _ := json.Marshal(out["executor"])
		s := h.listen(id)
		h.send(id, `{"prompt":"/think look first -- /tool eval {\"code\":\"6*7\"}"}`)
		req := s.waitFor(t, "ChatFrontendToolCallRequested")
		status, out = h.call(http.MethodPost, "/sessions/"+id+"/tools/results", `{"toolCallId":"`+req["toolCallId"].(string)+`","toolName":"eval","status":"success","result":{"value":42},"executor":`+string(executor)+`}`)
		require.Equal(t, http.StatusOK, status, out)
		s.waitFor(t, "ChatTextPatch")
		late := h.listen(id) // the answer is streaming: this stream starts with a partial snapshot
		final := h.idle(id)
		captures = append(captures, capture{Name: "browser-tool-and-late-attach", Streams: [][]frame{s.stop(), late.stop()}, Final: final})
	}
	// a stopped answer and a failed one
	{
		id := h.create("page")
		s := h.listen(id)
		h.send(id, `{"prompt":"/sleep 3000 never"}`)
		s.waitFor(t, "ChatRunStarted")
		status, out := h.call(http.MethodPost, "/sessions/"+id+"/stop", "")
		require.Equal(t, http.StatusAccepted, status, out)
		h.idle(id)
		final := h.idle(id)
		captures = append(captures, capture{Name: "stopped", Streams: [][]frame{s.stop()}, Final: final})

		id = h.create("tight")
		s = h.listen(id)
		h.send(id, `{"prompt":"/tool noop {}"}`)
		final = h.idle(id)
		captures = append(captures, capture{Name: "failed-quota", Streams: [][]frame{s.stop()}, Final: final})
	}

	dir := t.TempDir()
	if *keepCaptures != "" {
		dir = *keepCaptures
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	for _, c := range captures {
		raw, err := json.MarshalIndent(c, "", " ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, c.Name+".json"), raw, 0o600))
	}

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: the captures were made, the client was not checked against them")
	}
	cmd := exec.Command(node, "--test", "test/")
	cmd.Env = append(os.Environ(), "CHATAPP_CAPTURES="+dir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "client tests on the captures:\n%s", out)
}

// The served file is the embedded build.
func TestTheServerServesTheEmbeddedClient(t *testing.T) {
	h := newHarness(t)
	resp, err := http.Get(h.base + "/client.js")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/javascript; charset=utf-8", resp.Header.Get("Content-Type"))
	require.Equal(t, webclient.JS, body)
	require.Contains(t, string(body), "export {", "an ES module")
}
