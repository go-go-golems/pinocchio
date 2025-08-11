package main

import (
	"context"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/bobatea/pkg/repl"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/conversation/builder"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/tools"
	geppettolayers "github.com/go-go-golems/geppetto/pkg/layers"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	evalpkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/eval"
	toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	uipkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/ui"
	eventspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/xevents"
)

type SimpleAgentCmd struct{ *cmds.CommandDescription }

func NewSimpleAgentCmd() (*SimpleAgentCmd, error) {
	geLayers, err := geppettolayers.CreateGeppettoLayers()
	if err != nil {
		return nil, err
	}

	desc := cmds.NewCommandDescription(
		"simple-chat-agent",
		cmds.WithShort("Simple streaming chat agent with a calculator tool and a tiny REPL"),
		cmds.WithLayersList(geLayers...),
	)
	return &SimpleAgentCmd{CommandDescription: desc}, nil
}

// Tool implementations moved to pkg/tools

// Generative UI types moved to pkg/tools

// Styling moved to UI/xevents

// Pretty handlers removed (now in pkg/xevents)

// UI forwarder removed (now in pkg/xevents)

// Chat evaluator removed (now in pkg/eval)

// App model removed (now in pkg/ui)

func (c *SimpleAgentCmd) RunIntoWriter(ctx context.Context, parsed *layers.ParsedLayers, _ io.Writer) error {
	// Event router + sink
	router, err := events.NewEventRouter()
	if err != nil {
		return errors.Wrap(err, "router")
	}
	uiCh := make(chan interface{}, 1024)
	eventspkg.AddUIForwarder(router, uiCh)
	sink := middleware.NewWatermillSink(router.Publisher, "chat")

	// Engine
	eng, err := factory.NewEngineFromParsedLayers(parsed)
	if err != nil {
		return errors.Wrap(err, "engine")
	}

	// Tools: calculator + generative UI (integrated)
	registry := tools.NewInMemoryToolRegistry()
	if err := toolspkg.RegisterCalculatorTool(registry); err != nil {
		return errors.Wrap(err, "register calc tool")
	}
	// Channel to request UI forms from tools
	toolReqCh := make(chan toolspkg.ToolUIRequest, 4)
	if err := toolspkg.RegisterGenerativeUITool(registry, toolReqCh); err != nil {
		return errors.Wrap(err, "register generative-ui tool")
	}

	// Tools are provided per Turn via registry (handled in evaluator); no engine-level configuration needed

	// Conversation manager
	mb := builder.NewManagerBuilder().WithSystemPrompt("You are a helpful assistant. You can use tools.")
	manager, err := mb.Build()
	if err != nil {
		return errors.Wrap(err, "build conversation")
	}

	// Evaluator for REPL
	evaluator := evalpkg.NewChatEvaluator(eng, manager, registry, sink)
	replCfg := repl.DefaultConfig()
	replCfg.Title = "Chat REPL"
	replModel := repl.NewModel(evaluator, replCfg)

	// App model
	app := uipkg.NewAppModel(uiCh, replModel, toolReqCh)

	// Run router and Bubble Tea app
	eg, groupCtx := errgroup.WithContext(ctx)
	eg.Go(func() error { return router.Run(groupCtx) })
	eg.Go(func() error {
		<-router.Running()
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err := p.Run()
		return err
	})
	if err := eg.Wait(); err != nil {
		return err
	}
	log.Info().Msg("Finished")
	return nil
}

func main() {
	root := &cobra.Command{Use: "simple-chat-agent", PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := logging.InitLoggerFromViper(); err != nil {
			return err
		}
		return nil
	}}
	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, root)

	if err := clay.InitViper("pinocchio", root); err != nil {
		cobra.CheckErr(err)
	}

	c, err := NewSimpleAgentCmd()
	cobra.CheckErr(err)
	command, err := cli.BuildCobraCommand(c, cli.WithCobraMiddlewaresFunc(geppettolayers.GetCobraCommandGeppettoMiddlewares))
	cobra.CheckErr(err)
	root.AddCommand(command)
	cobra.CheckErr(root.Execute())
}
