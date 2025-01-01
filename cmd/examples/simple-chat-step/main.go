package main

import (
	"context"
	"fmt"
	"io"
	"os"
	_ "embed"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/middlewares"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

//go:embed test.yaml
var testYaml []byte

var rootCmd = &cobra.Command{
	Use:   "simple-chat-step",
	Short: "A simple chat step",
}

type TestCommand struct {
	*cmds.CommandDescription
	pinocchioCmd *pinocchio_cmds.GeppettoCommand
}

type TestCommandSettings struct {
	PinocchioProfile string `glazed.parameter:"pinocchio-profile"`
	Debug            bool   `glazed.parameter:"debug"`
}

func NewTestCommand(cmd *pinocchio_cmds.GeppettoCommand) *TestCommand {
	return &TestCommand{
		CommandDescription: cmds.NewCommandDescription("test2",
			cmds.WithShort("Test prompt"),
			cmds.WithFlags(
				parameters.NewParameterDefinition("pinocchio-profile",
					parameters.ParameterTypeString,
					parameters.WithHelp("Pinocchio profile"),
					parameters.WithDefault("default"),
				),
				parameters.NewParameterDefinition("debug",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Debug mode"),
					parameters.WithDefault(false),
				),
			),
		),
		pinocchioCmd: cmd,
	}
}

func ParseGeppettoLayersFromProfiles(c *pinocchio_cmds.GeppettoCommand, s *TestCommandSettings) (*layers.ParsedLayers, error) {
	xdgConfigPath, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	defaultProfileFile := fmt.Sprintf("%s/pinocchio/profiles.yaml", xdgConfigPath)
	middlewares_ := []middlewares.Middleware{}
	middlewares_ = append(middlewares_,
		middlewares.GatherFlagsFromProfiles(
			defaultProfileFile,
			defaultProfileFile,
			s.PinocchioProfile,
			parameters.WithParseStepSource("profiles"),
			parameters.WithParseStepMetadata(map[string]interface{}{
				"profileFile": defaultProfileFile,
				"profile":     s.PinocchioProfile,
			}),
		),
	)
	middlewares_ = append(middlewares_,
		middlewares.WrapWithWhitelistedLayers(
			[]string{
				settings.AiChatSlug,
				settings.AiClientSlug,
				openai.OpenAiChatSlug,
				claude.ClaudeChatSlug,
				cmdlayers.GeppettoHelpersSlug,
			},
			middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
		),
		middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
	)

	geppettoParsedLayers := layers.NewParsedLayers()
	err = middlewares.ExecuteMiddlewares(c.Description().Layers, geppettoParsedLayers, middlewares_...)
	if err != nil {
		return nil, err
	}

	return geppettoParsedLayers, nil
}

func (c *TestCommand) RunIntoWriter(ctx context.Context, parsedLayers *layers.ParsedLayers, w io.Writer) error {
	s := &TestCommandSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings")
	}

	geppettoParsedLayers, err := ParseGeppettoLayersFromProfiles(c.pinocchioCmd, s)
	if err != nil {
		return err
	}

	if s.Debug {
		// marshal geppettoParsedLayer to yaml and print it
		b_, err := yaml.Marshal(geppettoParsedLayers)
		if err != nil {
			return err
		}
		fmt.Println(string(b_))
		return nil
	}

	cmdCtx, _, err := c.pinocchioCmd.CreateCommandContextFromParsedLayers(geppettoParsedLayers)
	if err != nil {
		return err
	}

	printer := cmdCtx.SetupPrinter(os.Stdout)
	cmdCtx.Router.AddHandler("chat", "chat", printer)

	messages, err := cmdCtx.RunStepBlocking(ctx)
	if err != nil {
		return err
	}

	fmt.Println("\n--------------------------------")
	fmt.Println()

	for _, msg := range messages {
		if chatMsg, ok := msg.Content.(*conversation.ChatMessageContent); ok {
			fmt.Printf("%s: %s\n", chatMsg.Role, chatMsg.Text)
		} else {
			fmt.Printf("%s: %s\n", msg.Content.ContentType(), msg.Content.String())
		}
	}

	return nil
}

func main() {
	err := clay.InitViper("pinocchio", rootCmd)
	cobra.CheckErr(err)
	err = clay.InitLogger()
	cobra.CheckErr(err)

	commands, err := pinocchio_cmds.LoadFromYAML(testYaml)
	cobra.CheckErr(err)

	// Register the command as a normal cobra command and let it parse its step settings by itself
	cli.AddCommandsToRootCommand(rootCmd, commands, nil,
		cli.WithCobraMiddlewaresFunc(pinocchio_cmds.GetCobraCommandGeppettoMiddlewares),
		cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
	)

	if len(rootCmd.Commands()) == 1 {
		cmd := commands[0].(*pinocchio_cmds.GeppettoCommand)
		testCmd := NewTestCommand(cmd)
		command, err := cli.BuildCobraCommandFromWriterCommand(testCmd)
		cobra.CheckErr(err)
		rootCmd.AddCommand(command)
	}

	cobra.CheckErr(rootCmd.Execute())
}
