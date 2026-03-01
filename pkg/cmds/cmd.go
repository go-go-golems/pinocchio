package cmds

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"

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
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	pinui "github.com/go-go-golems/pinocchio/pkg/ui"
	"github.com/go-go-golems/pinocchio/pkg/ui/runtime"
	"github.com/google/uuid"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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

// conversation manager removed; no-op left intentionally for compatibility if referenced elsewhere

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

	// Update step settings from parsed layers
	stepSettings, err := settings.NewStepSettings()
	if err != nil {
		return errors.Wrap(err, "failed to create step settings")
	}
	err = stepSettings.UpdateFromParsedValues(parsedValues)
	if err != nil {
		return errors.Wrap(err, "failed to update step settings from parsed layers")
	}

	// Create image paths from helper settings
	imagePaths := make([]string, len(helpersSettings.Images))
	for i, img := range helpersSettings.Images {
		imagePaths[i] = img.Path
	}

	// No conversation manager preview; print path handled by RunWithOptions

	// Determine run mode based on helper settings
	runMode := run.RunModeBlocking
	if helpersSettings.StartInChat {
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

	// Run with options
	_, err = g.RunWithOptions(ctx,
		run.WithStepSettings(stepSettings),
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

// runEngineAndCollectMessages handles the actual engine execution and message collection
func (g *PinocchioCommand) runEngineAndCollectMessages(ctx context.Context, rc *run.RunContext, sinks []events.EventSink) error {
	// Create engine
	engine, err := rc.EngineFactory.CreateEngine(rc.StepSettings)
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
	if rc.Router == nil {
		return nil, errors.New("chat mode requires a router")
	}

	chatConvID := "cli-" + uuid.NewString()
	timelineStore, turnStore, closeStores, err := openChatPersistenceStores(rc.Persistence)
	if err != nil {
		return nil, errors.Wrap(err, "open chat persistence stores")
	}
	defer closeStores()

	isOutputTerminal := isatty.IsTerminal(os.Stdout.Fd())

	options := []tea.ProgramOption{
		tea.WithMouseCellMotion(),
	}
	if !isOutputTerminal {
		options = append(options, tea.WithOutput(os.Stderr))
	} else {
		options = append(options, tea.WithAltScreen())
	}

	// Enable streaming for the UI
	rc.StepSettings.Chat.Stream = true

	// Start router in a goroutine
	eg := errgroup.Group{}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	f := func() {
		cancel()
		defer func(Router *events.EventRouter) {
			_ = Router.Close()
		}(rc.Router)
	}

	eg.Go(func() error {
		defer f()
		ret := rc.Router.Run(ctx)
		if ret != nil {
			return ret
		}
		return nil
	})

	eg.Go(func() error {
		defer f()
		var err error

		// Wait for router to be ready
		<-rc.Router.Running()

		// If we're in interactive mode, run initial blocking step
		if rc.RunMode == run.RunModeInteractive {
			// Create options for initial step with chat topic
			chatSink := middleware.NewWatermillSink(rc.Router.Publisher, "chat")

			// Add default printer for initial step
			if rc.UISettings == nil || rc.UISettings.Output == "" || rc.UISettings.Output == "text" {
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
			err := rc.Router.RunHandlers(ctx)
			if err != nil {
				return err
			}

			err = g.runEngineAndCollectMessages(ctx, rc, []events.EventSink{chatSink})
			if err != nil {
				return err
			}

			// If we're not in interactive mode or it's explicitly disabled, return early
			if rc.UISettings != nil && rc.UISettings.NonInteractive {
				return nil
			}

			// Check if we should ask for chat continuation
			askChat := (isOutputTerminal || rc.UISettings != nil && rc.UISettings.ForceInteractive) && (rc.UISettings == nil || !rc.UISettings.NonInteractive)
			if !askChat {
				return nil
			}

			// Ask user if they want to continue in chat mode
			continueInChat, err := askForChatContinuation()
			if err != nil {
				return err
			}

			if !continueInChat {
				return nil
			}
		}

		startInChat := rc.UISettings != nil && rc.UISettings.StartInChat

		// Build program and session via unified builder
		sess, p, err := runtime.NewChatBuilder().
			WithContext(ctx).
			WithEngineFactory(rc.EngineFactory).
			WithSettings(rc.StepSettings).
			WithRouter(rc.Router).
			WithProgramOptions(options...).
			WithModelOptions(
				bobatea_chat.WithTitle("pinocchio"),
			).
			BuildProgram()
		if err != nil {
			return err
		}

		if turnStore != nil {
			sess.Backend.SetTurnPersister(
				newCLITurnStorePersister(turnStore, chatConvID, sess.Backend.SessionID(), "final"),
			)
		}
		if timelineStore != nil {
			rc.Router.AddHandler("ui-persist", "ui", pinui.StepTimelinePersistFunc(timelineStore, chatConvID))
		}

		// Register bound UI event handler and run handlers
		rc.Router.AddHandler("ui", "ui", sess.EventHandler())
		err = rc.Router.RunHandlers(ctx)
		if err != nil {
			return err
		}

		seedGroup := errgroup.Group{}

		// Seed backend Turn after router is running so the timeline shows prior context
		seedGroup.Go(func() error {
			<-rc.Router.Running()
			// Prefer seeding with the existing first Q/A from a prior blocking run when present
			if rc.ResultTurn != nil {
				sess.Backend.SetSeedTurn(rc.ResultTurn)
				return nil
			}
			// Otherwise seed from initial system/blocks to provide context in a fresh chat
			seed, err := buildInitialTurnFromBlocksRendered(g.SystemPrompt, g.Blocks, "", rc.Variables, rc.ImagePaths)
			if err == nil {
				sess.Backend.SetSeedTurn(seed)
				return nil
			}
			// Fallback without rendering on error
			fallbackSeed, _ := buildInitialTurnFromBlocks(g.SystemPrompt, g.Blocks, "", nil)
			sess.Backend.SetSeedTurn(fallbackSeed)
			return nil
		})

		// If we're starting directly in chat mode, pre-fill the prompt text (if any), then submit.
		if startInChat {
			seedGroup.Go(func() error {
				<-rc.Router.Running()
				// Render prompt before auto-submit in chat
				promptText := strings.TrimSpace(g.Prompt)
				if promptText != "" && rc.Variables != nil {
					if rendered, err := renderTemplateString("prompt", promptText, rc.Variables); err == nil {
						promptText = rendered
					}
				}
				if promptText != "" {
					log.Debug().Int("len", len(promptText)).Msg("Auto-start: submitting rendered prompt after router.Running")
					p.Send(bobatea_chat.ReplaceInputTextMsg{Text: promptText})
					p.Send(bobatea_chat.SubmitMessageMsg{})
				} else {
					log.Debug().Msg("Auto-start enabled, but no prompt text found; skipping submit")
				}
				return nil
			})
		}

		_, err = p.Run()
		if seedErr := seedGroup.Wait(); seedErr != nil && err == nil {
			err = seedErr
		}
		return err
	})

	err = eg.Wait()
	if err != nil {
		return nil, err
	}

	// Return resulting Turn when available
	return rc.ResultTurn, nil
}

// Helper function to ask user about continuing in chat mode
func askForChatContinuation() (bool, error) {
	tty_, err := bobatea_chat.OpenTTY()
	if err != nil {
		return false, err
	}
	defer func() {
		err := tty_.Close()
		if err != nil {
			fmt.Println("Failed to close tty:", err)
		}
	}()

	ui := &input.UI{
		Writer: tty_,
		Reader: tty_,
	}

	query := "\nDo you want to continue in chat? [y/n]"
	answer, err := ui.Ask(query, &input.Options{
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
		fmt.Println("Failed to get user input:", err)
		return false, err
	}

	return answer == "y" || answer == "Y", nil
}
