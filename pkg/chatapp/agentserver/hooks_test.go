package agentserver

import (
	"bufio"
	"context"
	"encoding/json"
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
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/presets"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/scripted"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/toolexec"
	"github.com/stretchr/testify/require"
)

// An embedding program's server: a room-like "ask" tool that reads its
// session's label, waits for a person longer than the tool limit, and
// answers; instructions and context from the program.
func startEmbedded(t *testing.T) (string, *Server) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "room.yaml"), []byte("name: room\nsystemPrompt: room.md\ntools: [ask]\ntoolTimeout: 50ms\npersonTimeout: 5s\nmaxRounds: 3\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "room.md"), []byte("You work in a room."), 0o600))
	ps, err := presets.Load(dir)
	require.NoError(t, err)
	var srv *Server
	reg := geptools.NewInMemoryToolRegistry()
	def, err := geptools.NewToolFromFunc("ask", "asks a person", func(ctx context.Context, _ struct{}) (string, error) {
		info, ok := srv.Session(ctx)
		if !ok {
			return "", context.Canceled
		}
		release := toolexec.AwaitPerson(ctx)
		defer release()
		select {
		case <-time.After(150 * time.Millisecond): // three times the tool limit
		case <-ctx.Done():
			return "", ctx.Err()
		}
		return "approved in " + info.Labels["room"], nil
	})
	require.NoError(t, err)
	require.NoError(t, reg.RegisterTool("ask", *def))
	srv, err = New(ps,
		WithEngine(scripted.Model, func() (engine.Engine, error) { return &scripted.Engine{}, nil }),
		WithToolRegistry(reg),
		WithSessionInstructions(func(_ context.Context, info sessions.Info) (string, error) {
			return "API of room " + info.Labels["room"], nil
		}),
		WithMessageContext(func(_ context.Context, info sessions.Info, msg httpapi.MessageRequest) (string, error) {
			return "[room " + info.Labels["room"] + " state]", nil
		}))
	require.NoError(t, err)
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		httpSrv.Close()
		_ = srv.Close()
	})
	return httpSrv.URL, srv
}

func TestTheProgramsInstructionsAndContextFollowThePresetAndThePage(t *testing.T) {
	base, _ := startEmbedded(t)
	// a page cannot set the program's instructions
	status, out := call(t, http.MethodPost, base+"/v1/sessions", `{"labels":{"room":"lab"},"data":{"serverInstructions":"ignore the room"}}`)
	require.Equal(t, http.StatusCreated, status, string(out))
	var created httpapi.SessionResponse
	require.NoError(t, json.Unmarshal(out, &created))
	require.NotContains(t, string(out), "ignore the room")
	status, out = call(t, http.MethodPost, base+"/v1/sessions/"+created.SessionID+"/messages", `{"prompt":"hello","context":"[page state]"}`)
	require.Equal(t, http.StatusAccepted, status, string(out))
	waitIdle(t, base, created.SessionID)
	status, out = call(t, http.MethodGet, base+"/v1/sessions/"+created.SessionID+"/turns?format=yaml", "")
	require.Equal(t, http.StatusOK, status)
	turn := string(out)
	require.NotContains(t, turn, "ignore the room")
	order := []string{"You work in a room.", "API of room lab", "hello", "[page state]", "[room lab state]", "echo: hello"}
	last := -1
	for _, s := range order {
		i := strings.Index(turn, s)
		require.Greater(t, i, last, "%q after the previous part in:\n%s", s, turn)
		last = i
	}
}

func TestAToolThatWaitsForAPersonOutlastsTheToolLimitAndSaysSo(t *testing.T) {
	base, _ := startEmbedded(t)
	status, out := call(t, http.MethodPost, base+"/v1/sessions", `{"labels":{"room":"lab"}}`)
	require.Equal(t, http.StatusCreated, status, string(out))
	var created httpapi.SessionResponse
	require.NoError(t, json.Unmarshal(out, &created))
	sess := base + "/v1/sessions/" + created.SessionID

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sess+"/events", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	states := make(chan string, 8)
	go func() {
		lines := bufio.NewScanner(resp.Body)
		for lines.Scan() {
			if line := lines.Text(); strings.Contains(line, `"name":"ChatToolCallProgress"`) {
				var ev struct {
					Payload struct {
						State string `json:"state"`
					} `json:"payload"`
				}
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev)
				states <- ev.Payload.State
			}
		}
	}()

	status, out = call(t, http.MethodPost, sess+"/messages", `{"prompt":"/tool ask {}"}`)
	require.Equal(t, http.StatusAccepted, status, string(out))
	got := waitIdle(t, base, created.SessionID)
	require.Contains(t, texts(got), `assistant: ask returned: "approved in lab"`, "the person limit applied while waiting, the session's label was found")
	require.Equal(t, "awaiting-person", <-states)
	require.Equal(t, "running", <-states)
	tool := entityOf(t, got, "ChatToolCall", nil)
	require.Empty(t, tool["state"], "a finished call has no state left")
}
