package cmds

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"

	geppettobootstrap "github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/glazed/pkg/helpers/templating"

	"github.com/go-go-golems/geppetto/pkg/events"

	tea "github.com/charmbracelet/bubbletea"
	bobatea_chat "github.com/go-go-golems/bobatea/pkg/chat"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatapprpcjsonl "github.com/go-go-golems/pinocchio/pkg/chatapp/rpc/jsonl"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	pinui "github.com/go-go-golems/pinocchio/pkg/ui"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/google/uuid"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func renderTemplateString(name, text string, vars map[string]interface{}) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}
	tpl, err := templating.CreateTemplate(name).Parse(text)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tpl.Execute(&b, vars); err != nil {
		return "", err
	}
	return b.String(), nil
}

// SimpleMessage represents a minimal YAML message that will be converted to a user block
type SimpleMessage struct {
	Text string `yaml:"text"`
}

// buildInitialTurnFromBlocks constructs a Turn from system prompt, pre-seeded blocks, and an optional user prompt
func buildInitialTurnFromBlocks(systemPrompt string, blocks []turns.Block, userPrompt string, imagePaths []string) (*turns.Turn, error) {
	t := &turns.Turn{}
	if strings.TrimSpace(systemPrompt) != "" {
		turns.AppendBlock(t, turns.NewSystemTextBlock(systemPrompt))
	}
	if len(blocks) > 0 {
		turns.AppendBlocks(t, blocks...)
	}
	if len(imagePaths) > 0 {
		imgs, err := imagePathsToTurnImages(imagePaths)
		if err != nil {
			return nil, err
		}
		turns.AppendBlock(t, turns.NewUserMultimodalBlock(userPrompt, imgs))
	}
	if strings.TrimSpace(userPrompt) != "" {
		turns.AppendBlock(t, turns.NewUserTextBlock(userPrompt))
	}
	return t, nil
}

// renderBlocks renders text payloads in blocks using vars
func renderBlocks(blocks []turns.Block, vars map[string]interface{}) ([]turns.Block, error) {
	if len(blocks) == 0 {
		return blocks, nil
	}
	out := make([]turns.Block, 0, len(blocks))
	for _, b := range blocks {
		nb := b
		if txt, ok := b.Payload[turns.PayloadKeyText].(string); ok {
			rt, err := renderTemplateString("message", txt, vars)
			if err != nil {
				return nil, err
			}
			if nb.Payload == nil {
				nb.Payload = map[string]any{}
			}
			nb.Payload[turns.PayloadKeyText] = rt
		}
		out = append(out, nb)
	}
	return out, nil
}

func buildInitialTurnFromBlocksRendered(systemPrompt string, blocks []turns.Block, userPrompt string, vars map[string]interface{}, imagePaths []string) (*turns.Turn, error) {
	sp, err := renderTemplateString("system-prompt", systemPrompt, vars)
	if err != nil {
		return nil, err
	}
	rblocks, err := renderBlocks(blocks, vars)
	if err != nil {
		return nil, err
	}
	up, err := renderTemplateString("prompt", userPrompt, vars)
	if err != nil {
		return nil, err
	}
	return buildInitialTurnFromBlocks(sp, rblocks, up, imagePaths)
}

// buildInitialTurn constructs a seed Turn for the command from system + blocks + user prompt using vars.
func (g *PinocchioCommand) buildInitialTurn(vars map[string]interface{}, imagePaths []string) (*turns.Turn, error) {
	return buildInitialTurnFromBlocksRendered(g.SystemPrompt, g.Blocks, g.Prompt, vars, imagePaths)
}

type PinocchioCommandDescription struct {
	Name      string                 `yaml:"name"`
	Short     string                 `yaml:"short"`
	Long      string                 `yaml:"long,omitempty"`
	Flags     []*fields.Definition   `yaml:"flags,omitempty"`
	Arguments []*fields.Definition   `yaml:"arguments,omitempty"`
	Layers    []schema.Section       `yaml:"layers,omitempty"`
	Type      string                 `yaml:"type,omitempty"`
	Tags      []string               `yaml:"tags,omitempty"`
	Metadata  map[string]interface{} `yaml:"metadata,omitempty"`

	Prompt       string   `yaml:"prompt,omitempty"`
	Messages     []string `yaml:"messages,omitempty"`
	SystemPrompt string   `yaml:"system-prompt,omitempty"`
}

type PinocchioCommand struct {
	*glazedcmds.CommandDescription `yaml:",inline"`
	Prompt                         string        `yaml:"prompt,omitempty"`
	Blocks                         []turns.Block `yaml:"-"`
	SystemPrompt                   string        `yaml:"system-prompt,omitempty"`
	EngineFactory                  factory.EngineFactory
	BaseInferenceSettings          *settings.InferenceSettings
}

var _ glazedcmds.WriterCommand = &PinocchioCommand{}

type PinocchioCommandOption func(*PinocchioCommand)

func WithPrompt(prompt string) PinocchioCommandOption {
	return func(g *PinocchioCommand) {
		g.Prompt = prompt
	}
}

func WithBlocks(blocks []turns.Block) PinocchioCommandOption {
	return func(g *PinocchioCommand) {
		g.Blocks = blocks
	}
}

func WithSystemPrompt(systemPrompt string) PinocchioCommandOption {
	return func(g *PinocchioCommand) {
		g.SystemPrompt = systemPrompt
	}
}

func WithBaseInferenceSettings(base *settings.InferenceSettings) PinocchioCommandOption {
	return func(g *PinocchioCommand) {
		if base == nil {
			g.BaseInferenceSettings = nil
			return
		}
		g.BaseInferenceSettings = base.Clone()
	}
}

func NewPinocchioCommand(
	description *glazedcmds.CommandDescription,
	options ...PinocchioCommandOption,
) (*PinocchioCommand, error) {
	helpersParameterLayer, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		return nil, err
	}

	description.Schema.PrependSections(helpersParameterLayer)

	ret := &PinocchioCommand{
		CommandDescription: description,
	}

	for _, option := range options {
		option(ret)
	}

	return ret, nil
}

// RunIntoWriter runs the command and writes the output into the given writer.
func (g *PinocchioCommand) RunIntoWriter(
	ctx context.Context,
	parsedValues *values.Values,
	w io.Writer,
) error {
	// Get helpers settings from parsed layers
	helpersSettings := &cmdlayers.HelpersSettings{}
	err := parsedValues.DecodeSectionInto(cmdlayers.GeppettoHelpersSlug, helpersSettings)
	if err != nil {
		return errors.Wrap(err, "failed to initialize helpers settings")
	}

	// Update inference settings from parsed layers
	stepSettings, err := settings.NewInferenceSettings()
	if err != nil {
		return errors.Wrap(err, "failed to create inference settings")
	}
	err = stepSettings.UpdateFromParsedValues(parsedValues)
	if err != nil {
		return errors.Wrap(err, "failed to update inference settings from parsed layers")
	}

	profileSettings := profilebootstrap.ProfileSettings{}
	var resolvedEngineSettings *profilebootstrap.ResolvedCLIEngineSettings

	var baseSettings *settings.InferenceSettings
	var baseErr error
	if g.BaseInferenceSettings != nil {
		baseSettings, baseErr = baseSettingsFromParsedValuesWithBase(parsedValues, g.BaseInferenceSettings)
	} else {
		baseSettings, baseErr = baseSettingsFromParsedValues(parsedValues)
	}
	if baseErr == nil && baseSettings != nil {
		// If the UI forces streaming, keep base settings aligned.
		if baseSettings.Chat != nil {
			baseSettings.Chat.Stream = stepSettings.Chat != nil && stepSettings.Chat.Stream
		}
		resolvedEngineSettings, err = profilebootstrap.ResolveCLIEngineSettingsFromBase(ctx, baseSettings, parsedValues, nil)
		if err != nil {
			return errors.Wrap(err, "resolve engine profile settings for command run")
		}
		if resolvedEngineSettings.Close != nil {
			defer resolvedEngineSettings.Close()
		}
		if resolvedEngineSettings.ProfileRuntime != nil {
			profileSettings = resolvedEngineSettings.ProfileRuntime.ProfileSettings
		}
		if resolvedEngineSettings.BaseInferenceSettings != nil {
			baseSettings = resolvedEngineSettings.BaseInferenceSettings
		}
		if resolvedEngineSettings.FinalInferenceSettings != nil {
			stepSettings = resolvedEngineSettings.FinalInferenceSettings
		}
	} else {
		resolvedProfileRuntime, err := profilebootstrap.ResolveCLIProfileRuntime(ctx, parsedValues)
		if err != nil {
			return errors.Wrap(err, "resolve profile runtime for command run")
		}
		if resolvedProfileRuntime != nil {
			if resolvedProfileRuntime.Close != nil {
				defer resolvedProfileRuntime.Close()
			}
			profileSettings = resolvedProfileRuntime.ProfileSettings
		}
	}

	// Create image paths from helper settings
	imagePaths := make([]string, len(helpersSettings.Images))
	for i, img := range helpersSettings.Images {
		imagePaths[i] = img.Path
	}

	// No conversation manager preview; print path handled by RunWithOptions

	// Determine run mode based on helper settings
	runMode := run.RunModeBlocking
	if helpersSettings.RPC || strings.EqualFold(helpersSettings.Output, "jsonl") {
		runMode = run.RunModeRPCJSONL
	} else if helpersSettings.StartInChat {
		runMode = run.RunModeChat
	} else if helpersSettings.Interactive {
		runMode = run.RunModeInteractive
	}

	// Create UI settings from helper settings
	uiSettings := &run.UISettings{
		Interactive:      helpersSettings.Interactive,
		ForceInteractive: helpersSettings.ForceInteractive,
		NonInteractive:   helpersSettings.NonInteractive,
		StartInChat:      helpersSettings.StartInChat,
		PrintPrompt:      helpersSettings.PrintPrompt,
		Output:           helpersSettings.Output,
		RPC:              helpersSettings.RPC,
		WithMetadata:     helpersSettings.WithMetadata,
		FullOutput:       helpersSettings.FullOutput,
	}

	router, err := events.NewEventRouter()
	if err != nil {
		return err
	}

	// If we're just printing the prompt, render and print the seed Turn and return
	if helpersSettings.PrintPrompt {
		seed, err := g.buildInitialTurn(getDefaultTemplateVariables(parsedValues), imagePaths)
		if err != nil {
			return err
		}
		turns.FprintTurn(w, seed)
		return nil
	}
	if helpersSettings.PrintInferenceSettings {
		if resolvedEngineSettings == nil {
			resolvedEngineSettings = &profilebootstrap.ResolvedCLIEngineSettings{
				BaseInferenceSettings:  baseSettings,
				FinalInferenceSettings: stepSettings,
			}
		}
		_, err := geppettobootstrap.HandleInferenceDebugOutput(
			w,
			profilebootstrap.BootstrapConfig(),
			parsedValues,
			geppettobootstrap.InferenceDebugSettings{
				PrintInferenceSettings: true,
			},
			&geppettobootstrap.ResolvedInferenceTrace{
				FinalInferenceSettings: resolvedEngineSettings.FinalInferenceSettings,
				ResolvedEngineProfile:  resolvedEngineSettings.ResolvedEngineProfile,
			},
			geppettobootstrap.InferenceDebugOutputOptions{
				CommandBase: g.BaseInferenceSettings,
			},
		)
		return err
	}

	// Run with options
	_, err = g.RunWithOptions(ctx,
		run.WithInferenceSettings(stepSettings),
		run.WithBaseSettings(baseSettings),
		run.WithProfileSelection(profileSettings.Profile, strings.Join(profileSettings.ProfileRegistries, ",")),
		run.WithEngineFactory(g.EngineFactory),
		run.WithWriter(w),
		run.WithRunMode(runMode),
		run.WithUISettings(uiSettings),
		run.WithPersistenceSettings(run.PersistenceSettings{
			TimelineDSN: helpersSettings.TimelineDSN,
			TimelineDB:  helpersSettings.TimelineDB,
			TurnsDSN:    helpersSettings.TurnsDSN,
			TurnsDB:     helpersSettings.TurnsDB,
		}),
		run.WithRouter(router),
		run.WithVariables(getDefaultTemplateVariables(parsedValues)),
		run.WithImagePaths(imagePaths),
	)
	if err != nil {
		return err
	}

	return nil
}

func getDefaultTemplateVariables(parsedValues *values.Values) map[string]interface{} {
	ret := map[string]interface{}{}
	defaultSectionValues, ok := parsedValues.Get(values.DefaultSlug)
	if !ok {
		return ret
	}
	defaultSectionValues.Fields.ForEach(func(key string, value *fields.FieldValue) {
		ret[key] = value.Value
	})
	return ret
}

// RunWithOptions executes the command with the given options
func (g *PinocchioCommand) RunWithOptions(ctx context.Context, options ...run.RunOption) (*turns.Turn, error) {
	runCtx := run.NewRunContext()

	// Apply options
	for _, opt := range options {
		if err := opt(runCtx); err != nil {
			return nil, err
		}
	}

	// ConversationManager optional during migration; prefer Turn-based flows

	if runCtx.UISettings != nil && runCtx.UISettings.PrintPrompt {
		// Build a preview turn from initial blocks using rendered templates
		t, err := g.buildInitialTurn(runCtx.Variables, runCtx.ImagePaths)
		if err != nil {
			return nil, err
		}
		return t, nil
	}

	// Create engine factory if not provided
	if runCtx.EngineFactory == nil {
		runCtx.EngineFactory = factory.NewStandardEngineFactory()
	}

	// Verify router for chat mode
	if (runCtx.RunMode == run.RunModeChat || runCtx.RunMode == run.RunModeInteractive) && runCtx.Router == nil {
		return nil, errors.New("chat mode requires a router")
	}

	switch runCtx.RunMode {
	case run.RunModeBlocking:
		return g.runBlocking(ctx, runCtx)
	case run.RunModeRPCJSONL:
		return g.runRPCJSONL(ctx, runCtx)
	case run.RunModeInteractive, run.RunModeChat:
		return g.runChat(ctx, runCtx)
	default:
		return nil, errors.Errorf("unknown run mode: %v", runCtx.RunMode)
	}
}

// runBlocking handles blocking execution mode using Engine directly
func (g *PinocchioCommand) runBlocking(ctx context.Context, rc *run.RunContext) (*turns.Turn, error) {
	// If we have a router, set up watermill sink for event publishing
	var sinks []events.EventSink
	if rc.Router != nil {
		watermillSink := middleware.NewWatermillSink(rc.Router.Publisher, "chat")
		sinks = []events.EventSink{watermillSink}

		// Add default printer if none is set
		if rc.UISettings == nil || rc.UISettings.Output == "" {
			rc.Router.AddHandler("chat", "chat", events.StepPrinterFunc("", rc.Writer))
		} else {
			printer := events.NewStructuredPrinter(rc.Writer, events.PrinterOptions{
				Format:          events.PrinterFormat(rc.UISettings.Output),
				Name:            "",
				IncludeMetadata: rc.UISettings.WithMetadata,
				Full:            rc.UISettings.FullOutput,
			})
			rc.Router.AddHandler("chat", "chat", printer)
		}

		// Start router
		eg := errgroup.Group{}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		eg.Go(func() error {
			defer cancel()
			defer func(Router *events.EventRouter) {
				_ = Router.Close()
			}(rc.Router)
			return rc.Router.Run(ctx)
		})

		eg.Go(func() error {
			defer cancel()
			<-rc.Router.Running()
			return g.runEngineAndCollectMessages(ctx, rc, sinks)
		})

		err := eg.Wait()
		if err != nil {
			return nil, err
		}
	} else {
		// No router, just run the engine directly using Turns
		err := g.runEngineAndCollectMessages(ctx, rc, nil)
		if err != nil {
			return nil, err
		}
	}

	// Return resulting Turn when available
	return rc.ResultTurn, nil
}

// runRPCJSONL executes the command through chatapp/sessionstream and writes one
// protobuf JSON RpcLine per stdout line. This path is script-friendly and shares
// projected chat UI events with web-chat instead of printing raw Geppetto events.
func (g *PinocchioCommand) runRPCJSONL(ctx context.Context, rc *run.RunContext) (*turns.Turn, error) {
	if rc.Writer == nil {
		rc.Writer = io.Discard
	}
	if rc.EngineFactory == nil {
		rc.EngineFactory = factory.NewStandardEngineFactory()
	}

	seed, err := g.buildInitialTurn(rc.Variables, rc.ImagePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to render templates: %w", err)
	}
	sid := commandSessionID(seed)
	prompt := displayPromptForTurn(seed)
	if strings.TrimSpace(prompt) == "" {
		prompt = "(input turn)"
	}

	fanout, err := chatapprpcjsonl.NewUIFanout(rc.Writer)
	if err != nil {
		return nil, err
	}
	if err := fanout.WriteHello(sid, []string{"ui-events", "snapshot", "done"}); err != nil {
		return nil, err
	}

	runner, err := chatapp.NewRunner(chatapp.RunnerOptions{UIFanout: fanout})
	if err != nil {
		_ = fanout.WriteError(sid, "runner_init_failed", err, true)
		return nil, err
	}
	defer func() { _ = runner.Close() }()
	initialSnap, err := runner.Service.Snapshot(ctx, sid)
	if err != nil {
		_ = fanout.WriteError(sid, "initial_snapshot_failed", err, true)
		return nil, err
	}
	if err := fanout.WriteSnapshot(initialSnap); err != nil {
		return nil, err
	}

	engine, err := rc.EngineFactory.CreateEngine(rc.InferenceSettings)
	if err != nil {
		err = fmt.Errorf("failed to create engine: %w", err)
		_ = fanout.WriteError(sid, "engine_init_failed", err, true)
		return nil, err
	}

	req := chatapp.PromptRequest{
		Prompt:      prompt,
		InitialTurn: seed,
		Runtime: &infruntime.ComposedRuntime{
			Engine: engine,
		},
	}
	if err := runner.Service.SubmitPromptRequest(ctx, sid, req); err != nil {
		_ = fanout.WriteError(sid, "submit_failed", err, true)
		return nil, err
	}
	if err := runner.Service.WaitIdle(ctx, sid); err != nil {
		_ = fanout.WriteError(sid, "wait_failed", err, true)
		return nil, err
	}
	snap, err := runner.Service.Snapshot(ctx, sid)
	if err != nil {
		_ = fanout.WriteError(sid, "snapshot_failed", err, true)
		return nil, err
	}
	if err := fanout.WriteSnapshot(snap); err != nil {
		return nil, err
	}
	if err := fanout.WriteDone(sid, "ok"); err != nil {
		return nil, err
	}
	return seed, nil
}

func commandSessionID(seed *turns.Turn) sessionstream.SessionId {
	if seed != nil {
		if sid, ok, err := turns.KeyTurnMetaSessionID.Get(seed.Metadata); err == nil && ok && sid != "" {
			return sessionstream.SessionId(sid)
		}
		sid := uuid.NewString()
		_ = turns.KeyTurnMetaSessionID.Set(&seed.Metadata, sid)
		return sessionstream.SessionId(sid)
	}
	return sessionstream.SessionId(uuid.NewString())
}

func displayPromptForTurn(seed *turns.Turn) string {
	if seed == nil {
		return ""
	}
	for i := len(seed.Blocks) - 1; i >= 0; i-- {
		block := seed.Blocks[i]
		if block.Role != turns.RoleUser || block.Payload == nil {
			continue
		}
		if text, ok := block.Payload[turns.PayloadKeyText].(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

// runEngineAndCollectMessages handles the actual engine execution and message collection
func (g *PinocchioCommand) runEngineAndCollectMessages(ctx context.Context, rc *run.RunContext, sinks []events.EventSink) error {
	// Create engine
	engine, err := rc.EngineFactory.CreateEngine(rc.InferenceSettings)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	// Build seed Turn directly from system + messages + prompt (rendered)
	seed, err := g.buildInitialTurn(rc.Variables, rc.ImagePaths)
	if err != nil {
		return fmt.Errorf("failed to render templates: %w", err)
	}

	runner, err := (&enginebuilder.Builder{
		Base:       engine,
		EventSinks: sinks,
	}).Build(ctx, func() string {
		if sid, ok, err := turns.KeyTurnMetaSessionID.Get(seed.Metadata); err == nil && ok && sid != "" {
			return sid
		}
		sid := uuid.NewString()
		_ = turns.KeyTurnMetaSessionID.Set(&seed.Metadata, sid)
		return sid
	}())
	if err != nil {
		return fmt.Errorf("failed to build runner: %w", err)
	}
	updatedTurn, err := runner.RunInference(ctx, seed)
	if err != nil {
		return fmt.Errorf("inference failed: %w", err)
	}
	// Store the updated Turn on the run context
	rc.ResultTurn = updatedTurn

	return nil
}

// runChat handles chat execution mode
func (g *PinocchioCommand) runChat(ctx context.Context, rc *run.RunContext) (*turns.Turn, error) {
	if rc.EngineFactory == nil {
		rc.EngineFactory = factory.NewStandardEngineFactory()
	}
	if rc.InferenceSettings == nil {
		return nil, errors.New("inference settings are required")
	}
	if rc.InferenceSettings.Chat != nil {
		rc.InferenceSettings.Chat.Stream = true
	}
	if rc.BaseSettings != nil && rc.BaseSettings.Chat != nil {
		rc.BaseSettings.Chat.Stream = true
	}

	seed, err := buildInitialTurnFromBlocksRendered(g.SystemPrompt, g.Blocks, "", rc.Variables, rc.ImagePaths)
	if err != nil {
		return nil, err
	}
	sid := commandSessionID(seed)
	eng, err := rc.EngineFactory.CreateEngine(rc.InferenceSettings)
	if err != nil {
		return nil, err
	}

	fanoutProxy := pinui.NewUIFanoutProxy()
	runner, err := chatapp.NewRunner(chatapp.RunnerOptions{UIFanout: fanoutProxy})
	if err != nil {
		return nil, err
	}
	defer func() { _ = runner.Close() }()

	backend, err := pinui.NewChatAppBackend(runner.Service, sid, &infruntime.ComposedRuntime{Engine: eng}, seed)
	if err != nil {
		return nil, err
	}

	isOutputTerminal := isatty.IsTerminal(os.Stdout.Fd())
	options := []tea.ProgramOption{tea.WithMouseCellMotion()}
	if !isOutputTerminal {
		options = append(options, tea.WithOutput(os.Stderr))
	} else {
		options = append(options, tea.WithAltScreen())
	}

	statusBar := func() string {
		profile := strings.TrimSpace(rc.Profile)
		if profile == "" {
			return ""
		}
		return "profile: " + profile
	}
	model := bobatea_chat.InitialModel(backend, bobatea_chat.WithTitle("pinocchio"), bobatea_chat.WithStatusBarView(statusBar))
	p := tea.NewProgram(model, options...)
	uiFanout, err := pinui.NewChatAppUIFanout(p)
	if err != nil {
		return nil, err
	}
	if err := fanoutProxy.SetTarget(uiFanout); err != nil {
		return nil, err
	}
	if snap, err := runner.Service.Snapshot(ctx, sid); err == nil {
		_ = uiFanout.HydrateSnapshot(snap)
	}

	if rc.RunMode == run.RunModeInteractive || (rc.UISettings != nil && rc.UISettings.StartInChat) {
		go func() {
			promptText := strings.TrimSpace(g.Prompt)
			if promptText != "" && rc.Variables != nil {
				if rendered, err := renderTemplateString("prompt", promptText, rc.Variables); err == nil {
					promptText = rendered
				}
			}
			if promptText != "" {
				p.Send(bobatea_chat.ReplaceInputTextMsg{Text: promptText})
				p.Send(bobatea_chat.SubmitMessageMsg{})
			}
		}()
	}

	if _, err := p.Run(); err != nil {
		return nil, err
	}
	return rc.ResultTurn, nil
}
