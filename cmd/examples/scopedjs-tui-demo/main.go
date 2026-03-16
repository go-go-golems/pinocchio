package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/events"
	enginefactory "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	pinhelpers "github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
	toolloopbackend "github.com/go-go-golems/pinocchio/pkg/ui/backends/toolloop"
	agentforwarder "github.com/go-go-golems/pinocchio/pkg/ui/forwarders/agent"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func main() {
	var (
		workspaceID       string
		profileSlug       string
		profileRegistries string
		logLevel          string
		listWorkspaces    bool
	)

	root := &cobra.Command{
		Use:   "scopedjs-tui-demo",
		Short: "Bubble Tea demo for a scoped JavaScript project-ops tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			zerolog.TimeFieldFormat = time.StampMilli
			if parsed, err := zerolog.ParseLevel(strings.TrimSpace(logLevel)); err == nil {
				zerolog.SetGlobalLevel(parsed)
			}

			if listWorkspaces {
				for _, workspace := range availableDemoWorkspaces() {
					fmt.Println(workspace)
				}
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stepSettings, closeRuntime, err := resolveStepSettings(ctx, profileSlug, profileRegistries)
			if err != nil {
				return errors.Wrap(err, "resolve step settings")
			}
			if closeRuntime != nil {
				defer closeRuntime()
			}

			engineInstance, err := enginefactory.NewEngineFromStepSettings(stepSettings)
			if err != nil {
				return errors.Wrap(err, "create engine")
			}

			registry, meta, cleanup, err := buildDemoRegistry(ctx, workspaceID)
			if err != nil {
				return err
			}
			defer func() {
				if cleanup != nil {
					_ = cleanup()
				}
			}()

			goPubSub := gochannel.NewGoChannel(gochannel.Config{
				OutputChannelBuffer:            256,
				BlockPublishUntilSubscriberAck: false,
			}, watermill.NopLogger{})
			router, err := events.NewEventRouter(
				events.WithPublisher(goPubSub),
				events.WithSubscriber(goPubSub),
			)
			if err != nil {
				return errors.Wrap(err, "create event router")
			}
			defer func() { _ = router.Close() }()

			sink := middleware.NewWatermillSink(router.Publisher, "chat")
			middlewares := []middleware.Middleware{
				middleware.NewSystemPromptMiddleware(systemPrompt(meta)),
				middleware.NewToolResultReorderMiddleware(),
			}
			backend := toolloopbackend.NewToolLoopBackend(engineInstance, middlewares, registry, sink, nil)

			log.Info().
				Str("workspace_id", meta.WorkspaceID).
				Str("project_name", meta.ProjectName).
				Int("file_count", meta.FileCount).
				Int("task_count", meta.TaskCount).
				Int("note_count", meta.NoteCount).
				Str("fixture_label", meta.FixtureLabel).
				Msg("loaded scopedjs demo workspace")

			model := chat.InitialModel(backend,
				chat.WithTitle("scopedjs project ops demo"),
				chat.WithTimelineRegister(registerDemoRenderers),
				chat.WithStatusBarView(makeStatusBar(meta)),
			)

			program := tea.NewProgram(model, tea.WithAltScreen())
			router.AddHandler("ui-forward", "chat", agentforwarder.MakeUIForwarder(program))

			eg, groupCtx := errgroup.WithContext(ctx)
			eg.Go(func() error { return router.Run(groupCtx) })
			eg.Go(func() error {
				_, err := program.Run()
				cancel()
				return err
			})
			if err := eg.Wait(); err != nil {
				return err
			}
			return nil
		},
	}

	root.Flags().StringVar(&workspaceID, "workspace", "apollo", "Fixture workspace to scope the demo runtime to")
	root.Flags().StringVar(&profileSlug, "profile", "", "Optional profile slug to resolve from profile registries")
	root.Flags().StringVar(&profileRegistries, "profile-registries", "", "Optional comma-separated profile registry sources (yaml/sqlite/sqlite-dsn)")
	root.Flags().StringVar(&logLevel, "log-level", "info", "Log level (trace|debug|info|warn|error)")
	root.Flags().BoolVar(&listWorkspaces, "list-workspaces", false, "List available fake workspaces and exit")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveStepSettings(ctx context.Context, profileSlug string, profileRegistries string) (*aisettings.StepSettings, func(), error) {
	stepSettings, cleanup, err := resolveStepSettingsWithProfile(ctx, profileSlug, profileRegistries)
	if err != nil {
		return nil, nil, err
	}
	return stepSettings, cleanup, nil
}

func resolveStepSettingsWithProfile(ctx context.Context, profileSlug string, profileRegistries string) (*aisettings.StepSettings, func(), error) {
	base, _, err := pinhelpers.ResolveBaseStepSettings(nil)
	if err != nil {
		return nil, nil, err
	}

	profileSettings := pinhelpers.ResolveProfileSettings(nil)
	if v := strings.TrimSpace(profileSlug); v != "" {
		profileSettings.Profile = v
	}
	if v := strings.TrimSpace(profileRegistries); v != "" {
		profileSettings.ProfileRegistries = v
	}
	if profileSettings.ProfileRegistries == "" {
		if base.Chat == nil || base.Chat.Engine == nil || strings.TrimSpace(*base.Chat.Engine) == "" {
			return nil, nil, fmt.Errorf("no engine configured; set PINOCCHIO_* base settings or provide --profile-registries/--profile")
		}
		return base, nil, nil
	}

	specEntries, err := gepprofiles.ParseProfileRegistrySourceEntries(profileSettings.ProfileRegistries)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parse profile registry sources")
	}
	specs, err := gepprofiles.ParseRegistrySourceSpecs(specEntries)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parse profile registry specs")
	}
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, specs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "initialize profile registry")
	}

	input := gepprofiles.ResolveInput{BaseStepSettings: base}
	if profileSettings.Profile != "" {
		slug, err := gepprofiles.ParseProfileSlug(profileSettings.Profile)
		if err != nil {
			_ = chain.Close()
			return nil, nil, err
		}
		input.ProfileSlug = slug
	}

	resolved, err := chain.ResolveEffectiveProfile(ctx, input)
	if err != nil {
		_ = chain.Close()
		return nil, nil, err
	}
	return resolved.EffectiveStepSettings, func() {
		_ = chain.Close()
	}, nil
}

func makeStatusBar(meta demoMeta) func() string {
	barStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color("236")).
		Padding(0, 1)
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Background(lipgloss.Color("236"))
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("236")).
		Bold(true)
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("236")).
		Italic(true)

	return func() string {
		parts := []string{
			keyStyle.Render("workspace: ") + valueStyle.Render(meta.ProjectName),
			keyStyle.Render("files: ") + valueStyle.Render(fmt.Sprintf("%d", meta.FileCount)),
			keyStyle.Render("tasks: ") + valueStyle.Render(fmt.Sprintf("%d", meta.TaskCount)),
			keyStyle.Render("notes: ") + valueStyle.Render(fmt.Sprintf("%d", meta.NoteCount)),
			hintStyle.Render("try: summarize open tasks / create a dashboard note / register /tasks"),
		}
		return barStyle.Render(strings.Join(parts, "  "))
	}
}
