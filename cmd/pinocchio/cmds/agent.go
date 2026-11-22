package cmds

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/modelsource"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/agentserver"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/presets"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/scripted"
	pinocchiocmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/agentcli"
	"github.com/spf13/cobra"
)

type agentServeSettings struct {
	Address       string `glazed:"address"`
	Presets       string `glazed:"presets"`
	DefaultPreset string `glazed:"default-preset"`
	StateDir      string `glazed:"state-dir"`
	Prefix        string `glazed:"prefix"`
	Engine        string `glazed:"engine"`
}

// agentMiddlewares is pinocchio's standard chain (flags, args, PINOCCHIO_*
// env, config, defaults) with Pi's own variable inserted after the flags.
func agentMiddlewares(parsed *values.Values, cmd *cobra.Command, args []string) ([]sources.Middleware, error) {
	base, err := pinocchiocmds.GetPinocchioCommandMiddlewares(parsed, cmd, args)
	if err != nil {
		return nil, err
	}
	return agentcli.WithPiEnv(base), nil
}

func loadModelSet(ctx context.Context, o modelsource.LoadOptions) (*modelsource.Set, error) {
	set, notes, err := agentcli.LoadModels(ctx, o)
	for _, n := range notes {
		log.Info().Msg("model source skipped: " + n)
	}
	return set, err
}

// AgentServeCommand is `pinocchio agent serve`.
type AgentServeCommand struct {
	*cmds.CommandDescription
}

var _ cmds.BareCommand = (*AgentServeCommand)(nil)

// NewAgentServeCommand describes `pinocchio agent serve`.
func NewAgentServeCommand() (*AgentServeCommand, error) {
	sections, err := agentcli.ModelSections("pinocchio")
	if err != nil {
		return nil, err
	}
	return &AgentServeCommand{CommandDescription: cmds.NewCommandDescription(
		"serve",
		cmds.WithShort("Serve the agent HTTP API (sessions, SSE events, browser-executed tools)"),
		cmds.WithLong(`Start the agent service: pinocchio's chat HTTP API (pkg/chatapp/httpapi)
with one preset per session. A page creates a session, registers the tools it
implements, sends messages and reads events over SSE; when the model calls a
tool, the service asks the page to run it and waits for the result. The model
loop and the conversation stay in the service.

Presets are YAML files in --presets (name, systemPrompt, model, thinking,
maxTokens, maxRounds, toolTimeout, resultCut, budget.tokensPerSession,
pageInstructions). Models come from engine profiles and Pi's agent directory
(see 'pinocchio help geppetto-model-sources'); credentials never reach the page.
See 'pinocchio help pinocchio-agent-serve' for the API.

The service listens on 127.0.0.1 by default and has no authentication of its
own; put the application's own server in front of it (same origin).

  GET    /v1/health · /v1/presets · /v1/models
  GET    /v1/sessions?label=k:v                 sessions with running
  POST   /v1/sessions                           {labels: {preset?, model?, thinking?}, title?, data: {instructions?}}
  GET    /v1/sessions/{id}                      metadata, running, timeline snapshot
  DELETE /v1/sessions/{id}
  POST   /v1/sessions/{id}/tools/manifest       {revision, clientInstanceId, connectionId, tools}
  POST   /v1/sessions/{id}/messages             {prompt} -> 202; 409 run_active
  GET    /v1/sessions/{id}/events               SSE: snapshot, then ui events
  POST   /v1/sessions/{id}/tools/results        {toolCallId, toolName, status, result, error?, executor}
  POST   /v1/sessions/{id}/stop`),
		cmds.WithFlags(
			fields.New("address", fields.TypeString, fields.WithDefault("127.0.0.1:8790"),
				fields.WithHelp("Address to listen on")),
			fields.New("presets", fields.TypeString, fields.WithRequired(true),
				fields.WithHelp("Directory of preset YAML files")),
			fields.New("default-preset", fields.TypeString,
				fields.WithHelp("Preset of sessions created without one (default: the only preset)")),
			fields.New("state-dir", fields.TypeString,
				fields.WithHelp("Directory for sessions, conversations and timelines (default ~/.local/state/pinocchio/agent)")),
			fields.New("prefix", fields.TypeString, fields.WithDefault(agentserver.DefaultPrefix),
				fields.WithHelp("Path prefix of the API")),
			fields.New("engine", fields.TypeChoice, fields.WithChoices("models", "fake"), fields.WithDefault("models"),
				fields.WithHelp("models: resolve each session's model; fake: scripted test engine")),
		),
		cmds.WithSections(sections...),
	)}, nil
}

func defaultStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no home directory; pass --state-dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "pinocchio", "agent"), nil
}

// Run starts the service and blocks until the context ends or SIGINT/SIGTERM.
func (c *AgentServeCommand) Run(ctx context.Context, parsed *values.Values) error {
	s := &agentServeSettings{}
	if err := parsed.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return fmt.Errorf("decode agent serve settings: %w", err)
	}
	ps, err := presets.Load(s.Presets)
	if err != nil {
		return err
	}
	dir := s.StateDir
	if dir == "" {
		if dir, err = defaultStateDir(); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("state directory: %w", err)
	}
	opts := []agentserver.Option{agentserver.WithStateDir(dir), agentserver.WithDefaultPreset(s.DefaultPreset), agentserver.WithPrefix(s.Prefix)}
	switch s.Engine {
	case "fake":
		opts = append(opts, agentserver.WithEngine(scripted.Model, func() (engine.Engine, error) { return &scripted.Engine{Delay: 15 * time.Millisecond}, nil }))
	default:
		lo, err := agentcli.DecodeModelOptions(parsed)
		if err != nil {
			return err
		}
		set, err := loadModelSet(ctx, lo)
		if err != nil {
			return err
		}
		defer func() { _ = set.Close() }()
		opts = append(opts, agentserver.WithModels(set))
	}
	srv, err := agentserver.New(ps, opts...)
	if err != nil {
		return err
	}
	defer func() { _ = srv.Close() }()
	ln, err := net.Listen("tcp", s.Address)
	if err != nil {
		return err
	}
	if host, _, _ := net.SplitHostPort(s.Address); host == "" || host == "0.0.0.0" || host == "::" {
		log.Warn().Str("address", s.Address).Msg("agent service is listening on all interfaces; it has no authentication of its own")
	}
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	_, _ = fmt.Fprintf(os.Stderr, "agent service on http://%s%s · presets: %s · state in %s · engine: %s\n", ln.Addr(), s.Prefix, strings.Join(presets.Names(ps), ", "), dir, s.Engine)
	sig, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-sig.Done()
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second) // ctx may already be done
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// AgentModelsCommand is `pinocchio agent models`: one row per usable model.
type AgentModelsCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = (*AgentModelsCommand)(nil)

// NewAgentModelsCommand describes `pinocchio agent models`.
func NewAgentModelsCommand() (*AgentModelsCommand, error) {
	sections, err := agentcli.ModelSections("pinocchio")
	if err != nil {
		return nil, err
	}
	return &AgentModelsCommand{CommandDescription: cmds.NewCommandDescription(
		"models",
		cmds.WithShort("List the models the agent service can use (no secrets)"),
		cmds.WithSections(sections...),
	)}, nil
}

// RunIntoGlazeProcessor emits one row per model: ref, source, api, context
// window, output limit, reasoning support and thinking levels.
func (c *AgentModelsCommand) RunIntoGlazeProcessor(ctx context.Context, parsed *values.Values, gp middlewares.Processor) error {
	lo, err := agentcli.DecodeModelOptions(parsed)
	if err != nil {
		return err
	}
	set, err := loadModelSet(ctx, lo)
	if err != nil {
		return err
	}
	defer func() { _ = set.Close() }()
	ms, err := set.Models(ctx)
	for _, m := range ms {
		if aerr := gp.AddRow(ctx, types.NewRow(
			types.MRP("ref", m.Ref),
			types.MRP("source", m.Source),
			types.MRP("api", m.API),
			types.MRP("context_window", m.ContextWindow),
			types.MRP("max_output", m.MaxOutputTokens),
			types.MRP("reasoning", m.Reasoning),
			types.MRP("thinking", strings.Join(m.ThinkingLevels, ",")),
		)); aerr != nil {
			return aerr
		}
	}
	return err
}

// NewAgentCommand creates the `pinocchio agent` group.
func NewAgentCommand() (*cobra.Command, error) {
	root := &cobra.Command{Use: "agent", Short: "Run agents whose tools execute in a browser page"}
	serve, err := NewAgentServeCommand()
	if err != nil {
		return nil, err
	}
	models, err := NewAgentModelsCommand()
	if err != nil {
		return nil, err
	}
	for _, c := range []cmds.Command{serve, models} {
		cc, err := cli.BuildCobraCommandFromCommand(c,
			cli.WithCobraMiddlewaresFunc(agentMiddlewares),
			cli.WithCobraShortHelpSections(schema.DefaultSlug, agentcli.ModelsSlug, agentcli.PiSlug))
		if err != nil {
			return nil, err
		}
		root.AddCommand(cc)
	}
	return root, nil
}
