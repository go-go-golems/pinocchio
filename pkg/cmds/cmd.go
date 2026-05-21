package cmds

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/plugins"
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
	"github.com/tcnksm/go-input"
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
	runMode := determineRunMode(helpersSettings)

	// Create UI settings from helper settings
	uiSettings := &run.UISettings{
		Interactive:      helpersSettings.Interactive,
		ForceInteractive: helpersSettings.ForceInteractive,
		NonInteractive:   helpersSettings.NonInteractive,
		StartInChat:      helpersSettings.StartInChat,
		PrintPrompt:      helpersSettings.PrintPrompt,
		Output:           helpersSettings.Output,
		RPC:              helpersSettings.RPC,
		DebugEventsJSONL: strings.TrimSpace(helpersSettings.DebugEventsJSONL),
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

func determineRunMode(settings *cmdlayers.HelpersSettings) run.RunMode {
	if settings == nil {
		return run.RunModeBlocking
	}
	if settings.RPC || strings.EqualFold(settings.Output, "jsonl") {
		return run.RunModeRPCJSONL
	}
	if settings.StartInChat {
		return run.RunModeChat
	}
	if settings.ForceInteractive || (settings.Interactive && !settings.NonInteractive) {
		return run.RunModeInteractive
	}
	return run.RunModeBlocking
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
		return g.runBlockingMaybeContinueInChat(ctx, runCtx)
	case run.RunModeRPCJSONL:
		return g.runRPCJSONL(ctx, runCtx)
	case run.RunModeInteractive:
		return g.runInteractive(ctx, runCtx)
	case run.RunModeChat:
		return g.runChat(ctx, runCtx)
	default:
		return nil, errors.Errorf("unknown run mode: %v", runCtx.RunMode)
	}
}

func (g *PinocchioCommand) runBlockingMaybeContinueInChat(ctx context.Context, rc *run.RunContext) (*turns.Turn, error) {
	result, err := g.runBlockingOnce(ctx, rc)
	if err != nil {
		return nil, err
	}
	if !shouldAskForChatContinuation(rc, false) {
		return result, nil
	}
	continueInChat, err := askForChatContinuation()
	if err != nil {
		return nil, err
	}
	if !continueInChat {
		return result, nil
	}
	rc.ResultTurn = result
	rc.RunMode = run.RunModeChat
	return g.runChat(ctx, rc)
}

func (g *PinocchioCommand) runInteractive(ctx context.Context, rc *run.RunContext) (*turns.Turn, error) {
	result, err := g.runBlockingOnce(ctx, rc)
	if err != nil {
		return nil, err
	}
	if !shouldAskForChatContinuation(rc, true) {
		return result, nil
	}
	continueInChat, err := askForChatContinuation()
	if err != nil {
		return nil, err
	}
	if !continueInChat {
		return result, nil
	}
	rc.ResultTurn = result
	rc.RunMode = run.RunModeChat
	return g.runChat(ctx, rc)
}

func (g *PinocchioCommand) runBlockingOnce(ctx context.Context, rc *run.RunContext) (*turns.Turn, error) {
	if rc.UISettings != nil && strings.TrimSpace(rc.UISettings.DebugEventsJSONL) != "" {
		return g.runBlockingWithDebugEvents(ctx, rc)
	}
	return g.runBlocking(ctx, rc)
}

func shouldAskForChatContinuation(rc *run.RunContext, force bool) bool {
	if rc == nil {
		return false
	}
	if rc.UISettings != nil && rc.UISettings.NonInteractive {
		return false
	}
	if force || (rc.UISettings != nil && rc.UISettings.ForceInteractive) {
		// Explicit interactive modes are operator requests, not scripting compatibility
		// shims. They intentionally proceed to /dev/tty prompting even when stdout is
		// redirected; callers that need a guaranteed non-prompting scripted path should
		// use --non-interactive or avoid --interactive/--force-interactive.
		return true
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}

func commandRunnerOptions(fanout sessionstream.UIFanout) chatapp.RunnerOptions {
	return chatapp.RunnerOptions{
		UIFanout: fanout,
		Plugins: []chatapp.ChatPlugin{
			plugins.NewReasoningPlugin(),
			plugins.NewToolCallPlugin(),
		},
	}
}

func askForChatContinuation() (bool, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer func() { _ = tty.Close() }()

	ui := &input.UI{Writer: tty, Reader: tty}
	answer, err := ui.Ask("\nDo you want to continue in chat? [y/n]", &input.Options{
		Default:  "y",
		Required: true,
		Loop:     true,
		ValidateFunc: func(answer string) error {
			switch answer {
			case "y", "Y", "n", "N":
				return nil
			default:
				return errors.Errorf("please enter 'y' or 'n'")
			}
		},
	})
	if err != nil {
		return false, err
	}
	return answer == "y" || answer == "Y", nil
}

// runBlockingWithDebugEvents keeps stdout in normal text mode while routing the
// inference through chatapp/sessionstream so projected UI events can be recorded
// to --debug-events-jsonl for diagnostics.
func (g *PinocchioCommand) runBlockingWithDebugEvents(ctx context.Context, rc *run.RunContext) (*turns.Turn, error) {
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

	debugFanout, closeDebug, err := openDebugEventsFanout(rc.UISettings)
	if err != nil {
		return nil, err
	}
	defer closeDebug()
	if debugFanout == nil {
		return g.runBlocking(ctx, rc)
	}
	if err := writeHelloAll(sid, []string{"ui-events", "snapshot", "done", "debug-log"}, debugFanout); err != nil {
		return nil, err
	}

	statusFanout := newRunStatusFanout(debugFanout)
	runner, err := chatapp.NewRunner(commandRunnerOptions(statusFanout))
	if err != nil {
		_ = writeTerminalErrorDoneAll(sid, "runner_init_failed", err, debugFanout)
		return nil, err
	}
	defer func() { _ = runner.Close() }()
	initialSnap, err := runner.Service.Snapshot(ctx, sid)
	if err != nil {
		_ = writeTerminalErrorDoneAll(sid, "initial_snapshot_failed", err, debugFanout)
		return nil, err
	}
	if err := writeSnapshotAll(initialSnap, debugFanout); err != nil {
		return nil, err
	}

	engine, err := rc.EngineFactory.CreateEngine(rc.InferenceSettings)
	if err != nil {
		err = fmt.Errorf("failed to create engine: %w", err)
		_ = writeTerminalErrorDoneAll(sid, "engine_init_failed", err, debugFanout)
		return nil, err
	}
	req := chatapp.PromptRequest{Prompt: prompt, InitialTurn: seed, Runtime: &infruntime.ComposedRuntime{Engine: engine}}
	if err := runner.Service.SubmitPromptRequest(ctx, sid, req); err != nil {
		_ = writeTerminalErrorDoneAll(sid, "submit_failed", err, debugFanout)
		return nil, err
	}
	if err := runner.Service.WaitIdle(ctx, sid); err != nil {
		_ = writeTerminalErrorDoneAll(sid, "wait_failed", err, debugFanout)
		return nil, err
	}
	snap, err := runner.Service.Snapshot(ctx, sid)
	if err != nil {
		_ = writeTerminalErrorDoneAll(sid, "snapshot_failed", err, debugFanout)
		return nil, err
	}
	if err := writeSnapshotAll(snap, debugFanout); err != nil {
		return nil, err
	}
	status, runErr := statusFanout.Result()
	if runErr != nil {
		_ = writeErrorAll(sid, "run_failed", runErr, true, debugFanout)
		_ = writeDoneAll(sid, status, debugFanout)
		return nil, runErr
	}
	if err := writeDoneAll(sid, status, debugFanout); err != nil {
		return nil, err
	}

	result := turnFromCommandSnapshot(seed, snap)
	rc.ResultTurn = result
	if err := writeBlockingTextOutput(rc.Writer, result); err != nil {
		return nil, err
	}
	return result, nil
}

func writeBlockingTextOutput(w io.Writer, t *turns.Turn) error {
	if w == nil || t == nil {
		return nil
	}
	for i := len(t.Blocks) - 1; i >= 0; i-- {
		block := t.Blocks[i]
		if block.Role != turns.RoleAssistant || block.Payload == nil {
			continue
		}
		if text, ok := block.Payload[turns.PayloadKeyText].(string); ok && strings.TrimSpace(text) != "" {
			_, err := fmt.Fprintln(w, text)
			return err
		}
	}
	return nil
}

func turnFromCommandSnapshot(seed *turns.Turn, snap sessionstream.Snapshot) *turns.Turn {
	out := &turns.Turn{}
	if seed != nil {
		for _, block := range seed.Blocks {
			if block.Role == turns.RoleUser || block.Role == turns.RoleAssistant {
				continue
			}
			turns.AppendBlock(out, block)
		}
	}
	entities := append([]sessionstream.TimelineEntity(nil), snap.Entities...)
	sort.SliceStable(entities, func(i, j int) bool { return entities[i].CreatedOrdinal < entities[j].CreatedOrdinal })
	for _, entity := range entities {
		msg, ok := entity.Payload.(*chatappv1.ChatMessageEntity)
		if !ok || msg == nil {
			continue
		}
		text := strings.TrimSpace(firstNonEmptyString(msg.GetContent(), msg.GetText()))
		if text == "" {
			continue
		}
		switch msg.GetRole() {
		case "user":
			turns.AppendBlock(out, turns.NewUserTextBlock(text))
		case "assistant":
			turns.AppendBlock(out, turns.NewAssistantTextBlock(text))
		}
	}
	return out
}

func snapshotFromTurnForHydration(sid sessionstream.SessionId, seed *turns.Turn) sessionstream.Snapshot {
	snap := sessionstream.Snapshot{SessionId: sid}
	if seed == nil {
		return snap
	}
	ordinal := uint64(1)
	for idx, block := range seed.Blocks {
		if block.Payload == nil {
			continue
		}
		role := ""
		switch block.Role {
		case turns.RoleUser:
			role = "user"
		case turns.RoleAssistant:
			role = "assistant"
		default:
			continue
		}
		text, _ := block.Payload[turns.PayloadKeyText].(string)
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		id := fmt.Sprintf("seed-%s-%d", role, idx)
		status := "accepted"
		if role == "assistant" {
			status = "finished"
		}
		snap.Entities = append(snap.Entities, sessionstream.TimelineEntity{
			Kind:           "ChatMessage",
			Id:             id,
			CreatedOrdinal: ordinal,
			Payload: &chatappv1.ChatMessageEntity{
				MessageId: id,
				Role:      role,
				Content:   text,
				Text:      text,
				Status:    status,
			},
		})
		ordinal++
	}
	return snap
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func shouldUsePrettyTextPrinter(settings *run.UISettings) bool {
	if settings == nil || strings.TrimSpace(settings.Output) == "" {
		return true
	}
	return strings.EqualFold(settings.Output, "text") && !settings.WithMetadata && !settings.FullOutput
}

// runBlocking handles blocking execution mode using Engine directly
func (g *PinocchioCommand) runBlocking(ctx context.Context, rc *run.RunContext) (*turns.Turn, error) {
	// If we have a router, set up watermill sink for event publishing
	var sinks []events.EventSink
	if rc.Router != nil {
		watermillSink := middleware.NewWatermillSink(rc.Router.Publisher, "chat")
		sinks = []events.EventSink{watermillSink}

		// Add default printer if none is set. Human text output should use the
		// pretty streaming printer; the structured printer's text mode is intended
		// for event debugging and prints verbose info payloads.
		if shouldUsePrettyTextPrinter(rc.UISettings) {
			rc.Router.AddHandler("chat", "chat", pinocchioStepPrinterFunc("", rc.Writer))
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
	debugFanout, closeDebug, err := openDebugEventsFanout(rc.UISettings)
	if err != nil {
		return nil, err
	}
	defer closeDebug()
	liveFanout := sessionstream.UIFanout(fanout)
	if debugFanout != nil {
		liveFanout, err = pinui.NewMultiUIFanout(fanout, debugFanout)
		if err != nil {
			return nil, err
		}
	}
	if err := writeHelloAll(sid, []string{"ui-events", "snapshot", "done"}, fanout, debugFanout); err != nil {
		return nil, err
	}

	statusFanout := newRunStatusFanout(liveFanout)
	runner, err := chatapp.NewRunner(commandRunnerOptions(statusFanout))
	if err != nil {
		_ = writeTerminalErrorDoneAll(sid, "runner_init_failed", err, fanout, debugFanout)
		return nil, err
	}
	defer func() { _ = runner.Close() }()
	initialSnap, err := runner.Service.Snapshot(ctx, sid)
	if err != nil {
		_ = writeTerminalErrorDoneAll(sid, "initial_snapshot_failed", err, fanout, debugFanout)
		return nil, err
	}
	if err := writeSnapshotAll(initialSnap, fanout, debugFanout); err != nil {
		return nil, err
	}

	engine, err := rc.EngineFactory.CreateEngine(rc.InferenceSettings)
	if err != nil {
		err = fmt.Errorf("failed to create engine: %w", err)
		_ = writeTerminalErrorDoneAll(sid, "engine_init_failed", err, fanout, debugFanout)
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
		_ = writeTerminalErrorDoneAll(sid, "submit_failed", err, fanout, debugFanout)
		return nil, err
	}
	if err := runner.Service.WaitIdle(ctx, sid); err != nil {
		_ = writeTerminalErrorDoneAll(sid, "wait_failed", err, fanout, debugFanout)
		return nil, err
	}
	snap, err := runner.Service.Snapshot(ctx, sid)
	if err != nil {
		_ = writeTerminalErrorDoneAll(sid, "snapshot_failed", err, fanout, debugFanout)
		return nil, err
	}
	if err := writeSnapshotAll(snap, fanout, debugFanout); err != nil {
		return nil, err
	}
	status, runErr := statusFanout.Result()
	if runErr != nil {
		_ = writeErrorAll(sid, "run_failed", runErr, true, fanout, debugFanout)
		_ = writeDoneAll(sid, status, fanout, debugFanout)
		return nil, runErr
	}
	if err := writeDoneAll(sid, status, fanout, debugFanout); err != nil {
		return nil, err
	}
	return seed, nil
}

func openDebugEventsFanout(settings *run.UISettings) (*chatapprpcjsonl.UIFanout, func(), error) {
	noop := func() {}
	if settings == nil || strings.TrimSpace(settings.DebugEventsJSONL) == "" {
		return nil, noop, nil
	}
	path := strings.TrimSpace(settings.DebugEventsJSONL)
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, noop, fmt.Errorf("create debug events jsonl directory: %w", err)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, noop, fmt.Errorf("create debug events jsonl file: %w", err)
	}
	fanout, err := chatapprpcjsonl.NewUIFanout(file)
	if err != nil {
		_ = file.Close()
		return nil, noop, err
	}
	return fanout, func() { _ = file.Close() }, nil
}

func writeHelloAll(sid sessionstream.SessionId, capabilities []string, fanouts ...*chatapprpcjsonl.UIFanout) error {
	for _, fanout := range fanouts {
		if fanout == nil {
			continue
		}
		if err := fanout.WriteHello(sid, capabilities); err != nil {
			return err
		}
	}
	return nil
}

func writeSnapshotAll(snap sessionstream.Snapshot, fanouts ...*chatapprpcjsonl.UIFanout) error {
	for _, fanout := range fanouts {
		if fanout == nil {
			continue
		}
		if err := fanout.WriteSnapshot(snap); err != nil {
			return err
		}
	}
	return nil
}

func writeErrorAll(sid sessionstream.SessionId, code string, err error, terminal bool, fanouts ...*chatapprpcjsonl.UIFanout) error {
	for _, fanout := range fanouts {
		if fanout == nil {
			continue
		}
		if writeErr := fanout.WriteError(sid, code, err, terminal); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func writeDoneAll(sid sessionstream.SessionId, status string, fanouts ...*chatapprpcjsonl.UIFanout) error {
	for _, fanout := range fanouts {
		if fanout == nil {
			continue
		}
		if err := fanout.WriteDone(sid, status); err != nil {
			return err
		}
	}
	return nil
}

func writeTerminalErrorDoneAll(sid sessionstream.SessionId, code string, err error, fanouts ...*chatapprpcjsonl.UIFanout) error {
	if writeErr := writeErrorAll(sid, code, err, true, fanouts...); writeErr != nil {
		return writeErr
	}
	return writeDoneAll(sid, "failed", fanouts...)
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

	seed := rc.ResultTurn
	if seed == nil {
		var err error
		seed, err = buildInitialTurnFromBlocksRendered(g.SystemPrompt, g.Blocks, "", rc.Variables, rc.ImagePaths)
		if err != nil {
			return nil, err
		}
	}
	sid := commandSessionID(seed)
	eng, err := rc.EngineFactory.CreateEngine(rc.InferenceSettings)
	if err != nil {
		return nil, err
	}

	debugFanout, closeDebug, err := openDebugEventsFanout(rc.UISettings)
	if err != nil {
		return nil, err
	}
	defer closeDebug()
	if err := writeHelloAll(sid, []string{"ui-events", "snapshot", "done", "debug-log"}, debugFanout); err != nil {
		return nil, err
	}

	fanoutProxy := pinui.NewUIFanoutProxy()
	runner, err := chatapp.NewRunner(commandRunnerOptions(fanoutProxy))
	if err != nil {
		_ = writeErrorAll(sid, "runner_init_failed", err, true, debugFanout)
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
	liveTarget := sessionstream.UIFanout(uiFanout)
	if debugFanout != nil {
		liveTarget, err = pinui.NewMultiUIFanout(uiFanout, debugFanout)
		if err != nil {
			return nil, err
		}
	}
	statusFanout := newRunStatusFanout(liveTarget)
	if err := fanoutProxy.SetTarget(statusFanout); err != nil {
		return nil, err
	}
	var hydrationSnapshots []sessionstream.Snapshot
	if snap, err := runner.Service.Snapshot(ctx, sid); err == nil {
		_ = writeSnapshotAll(snap, debugFanout)
		if len(snap.Entities) > 0 {
			hydrationSnapshots = append(hydrationSnapshots, snap)
		}
	}
	if rc.ResultTurn != nil {
		// Continuation mode seeds chat with the already-produced blocking answer. The
		// sessionstream store starts empty for this TUI session, so hydrate bobatea
		// directly from the result turn to make the prior exchange visible without
		// replaying it through the backend or issuing a second provider call.
		hydrationSnapshots = append(hydrationSnapshots, snapshotFromTurnForHydration(sid, rc.ResultTurn))
	}

	autoSubmitInitialPrompt := rc.ResultTurn == nil && (rc.RunMode == run.RunModeInteractive || (rc.UISettings != nil && rc.UISettings.StartInChat))
	if len(hydrationSnapshots) > 0 || autoSubmitInitialPrompt {
		go func() {
			// Bubble Tea Program.Send blocks until the program is running. Keep all
			// startup UI messages in this goroutine so continuation hydration cannot
			// deadlock before p.Run() has a chance to enter the event loop.
			for _, snap := range hydrationSnapshots {
				_ = uiFanout.HydrateSnapshot(snap)
			}
			if !autoSubmitInitialPrompt {
				return
			}
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
		_ = writeErrorAll(sid, "tui_failed", err, true, debugFanout)
		return nil, err
	}
	if snap, err := runner.Service.Snapshot(ctx, sid); err == nil {
		_ = writeSnapshotAll(snap, debugFanout)
	}
	status, runErr := statusFanout.Result()
	if runErr != nil {
		_ = writeErrorAll(sid, "run_failed", runErr, true, debugFanout)
		_ = writeDoneAll(sid, status, debugFanout)
		return nil, runErr
	}
	if err := writeDoneAll(sid, status, debugFanout); err != nil {
		return nil, err
	}
	return rc.ResultTurn, nil
}
