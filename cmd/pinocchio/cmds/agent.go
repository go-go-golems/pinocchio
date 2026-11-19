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
	"github.com/go-go-golems/pinocchio/pkg/agentd"
	"github.com/spf13/cobra"
)

// NewAgentCommand creates `pinocchio agent`, the agent service commands.
func NewAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run agents whose tools execute in a browser page",
	}
	cmd.AddCommand(newAgentServeCommand(), newAgentModelsCommand())
	return cmd
}

type agentFlags struct {
	address  string
	agents   string
	stateDir string
	engine   string
	piDir    string
	profiles []string
	appName  string
	noPi     bool
	noProf   bool
}

func (f *agentFlags) modelFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.piDir, "pi-dir", "", "Pi agent directory (default $PI_CODING_AGENT_DIR or ~/.pi/agent)")
	cmd.Flags().StringSliceVar(&f.profiles, "profile-registries", nil, "Engine profile registries (yaml:PATH, sqlite:PATH); default <config dir>/<app>/profiles.yaml")
	cmd.Flags().StringVar(&f.appName, "app-name", "pinocchio", "Application name for the default profiles.yaml location")
	cmd.Flags().BoolVar(&f.noPi, "no-pi", false, "Do not load models from Pi's agent directory")
	cmd.Flags().BoolVar(&f.noProf, "no-profiles", false, "Do not load engine profiles")
}

func (f *agentFlags) loadModels(ctx context.Context) (*modelsource.Set, error) {
	set, notes, err := modelsource.Load(ctx, modelsource.LoadOptions{PiDir: f.piDir, SkipPi: f.noPi, ProfileSources: f.profiles, AppName: f.appName, SkipProfiles: f.noProf})
	for _, n := range notes {
		log.Info().Msg("model source skipped: " + n)
	}
	return set, err
}

func newAgentServeCommand() *cobra.Command {
	f := &agentFlags{}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the agent HTTP API (sessions, SSE events, browser-executed tools)",
		Long: `Start the agent service. A page creates a session for an agent, registers
the tools it implements, sends turns and reads events over SSE; when the
model calls a tool, the service asks the page to run it and waits for the
result. The model loop and the conversation stay in the service.

Agents are YAML files in --agents (name, systemPrompt, model, limits, tool
approval modes). Models come from engine profiles and Pi's agent directory
(see 'pinocchio help geppetto-model-sources'); credentials never reach the page.

The service listens on 127.0.0.1 by default; put the application's own server
in front of it (same origin) rather than exposing it.

  GET  /v1/health · /v1/agents · /v1/models · /v1/sessions
  POST /v1/sessions                          {agent, model?, thinking?, instructions?}
  GET  /v1/sessions/{id}                     conversation for hydration
  POST /v1/sessions/{id}/tools/manifest      {revision, clientInstanceId, connectionId, tools}
  POST /v1/sessions/{id}/turns               {text, context?}
  GET  /v1/sessions/{id}/events              SSE; Last-Event-ID or ?after= resumes
  POST /v1/sessions/{id}/tools/{callId}/result {status, value, error?, executor}
  POST /v1/sessions/{id}/cancel

`,
		RunE: func(cmd *cobra.Command, args []string) error { return runAgentServe(cmd.Context(), f) },
	}
	cmd.Flags().StringVar(&f.address, "address", "127.0.0.1:8790", "Address to listen on")
	cmd.Flags().StringVar(&f.agents, "agents", "", "Directory of agent YAML files (required)")
	cmd.Flags().StringVar(&f.stateDir, "state-dir", "", "Directory for saved sessions (default <state dir>/pinocchio/agentd)")
	cmd.Flags().StringVar(&f.engine, "engine", "models", "models (resolve each session's model) or fake (scripted test engine)")
	f.modelFlags(cmd)
	_ = cmd.MarkFlagRequired("agents")
	return cmd
}

func defaultStateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "pinocchio", "agentd")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "pinocchio", "agentd")
	}
	return filepath.Join(".pinocchio", "agentd")
}

func runAgentServe(ctx context.Context, f *agentFlags) error {
	if ctx == nil {
		ctx = context.Background()
	}
	agents, err := agentd.LoadAgents(f.agents)
	if err != nil {
		return err
	}
	dir := f.stateDir
	if dir == "" {
		dir = defaultStateDir()
	}
	store, err := agentd.NewStore(dir)
	if err != nil {
		return err
	}
	opts := agentd.Options{Agents: agents, Store: store}
	switch f.engine {
	case "fake":
		opts.Engine = func() (engine.Engine, error) { return &agentd.FakeEngine{Delay: 15 * time.Millisecond}, nil }
		opts.EngineModel = agentd.FakeModel
	case "models", "":
		set, err := f.loadModels(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = set.Close() }()
		opts.Models = set
	default:
		return fmt.Errorf("--engine must be models or fake, got %q", f.engine)
	}
	srv, err := agentd.NewServer(ctx, opts)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", f.address)
	if err != nil {
		return err
	}
	if host, _, _ := net.SplitHostPort(f.address); host == "" || host == "0.0.0.0" || host == "::" {
		log.Warn().Str("address", f.address).Msg("agent service is listening on all interfaces; it has no authentication of its own")
	}
	names := []string{}
	for n := range agents {
		names = append(names, n)
	}
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintf(os.Stderr, "agent service on http://%s · agents: %s · sessions in %s · engine: %s\n", ln.Addr(), strings.Join(names, ", "), dir, f.engine)
	sig, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-sig.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func newAgentModelsCommand() *cobra.Command {
	f := &agentFlags{}
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List the models the agent service can use (no secrets)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			set, err := f.loadModels(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = set.Close() }()
			ms, err := set.Models(ctx)
			for _, m := range ms {
				fmt.Printf("%-44s %-18s ctx=%-8d out=%-7d reasoning=%v\n", m.Ref, m.API, m.ContextWindow, m.MaxOutputTokens, m.Reasoning)
			}
			return err
		},
	}
	f.modelFlags(cmd)
	return cmd
}
