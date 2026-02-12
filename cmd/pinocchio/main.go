package main

import (
	"embed"
	"fmt"
	layers2 "github.com/go-go-golems/geppetto/pkg/layers"
	"os"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/clay/pkg/repositories"
	"github.com/go-go-golems/geppetto/pkg/doc"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/catter"
	catter_doc "github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/catter/pkg/doc"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/helpers"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/kagi"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/openai"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/temporizer"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/tokens"
	pinocchio_docs "github.com/go-go-golems/pinocchio/cmd/pinocchio/doc"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	pkg_doc "github.com/go-go-golems/pinocchio/pkg/doc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	clay_profiles "github.com/go-go-golems/clay/pkg/cmds/profiles"
	clay_repositories "github.com/go-go-golems/clay/pkg/cmds/repositories"
	glazedConfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/rs/zerolog/log"

	// New command management import
	clay_commandmeta "github.com/go-go-golems/clay/pkg/cmds/commandmeta"
)

var version = "dev"

//go:embed prompts/*
var promptsFS embed.FS

var rootCmd = &cobra.Command{
	Use:     "pinocchio",
	Short:   "pinocchio is a tool to run LLM applications",
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromCobra(cmd)
	},
}

func main() {
	// Initialize logging as early as possible (before command discovery / repository loading),
	// but without parsing the full Cobra flagset before subcommands are registered.
	//
	// In particular, avoid calling rootCmd.ParseFlags(os.Args[1:]) here:
	// - it returns `pflag: help requested` for --help, which should not be treated as fatal
	// - it fails on subcommand flags like --print-parsed-parameters before commands are registered
	if err := logging.InitEarlyLoggingFromArgs(os.Args[1:], "pinocchio"); err != nil {
		fmt.Printf("Could not initialize early logger: %v\n", err)
		os.Exit(1)
	}

	helpSystem, err := initRootCmd()
	cobra.CheckErr(err)

	// first, check if the args are "run-command file.yaml",
	// because we need to load the file and then run the command itself.
	// we need to do this before cobra, because we don't know which flags to load yet
	if len(os.Args) >= 3 && os.Args[1] == "run-command" && os.Args[2] != "--help" {
		bytes, err := os.ReadFile(os.Args[2])
		if err != nil {
			fmt.Printf("Could not read file: %v\n", err)
			os.Exit(1)
		}
		cmds_, err := cmds.LoadFromYAML(bytes)
		if err != nil {
			fmt.Printf("Could not load command: %v\n", err)
			os.Exit(1)
		}
		if len(cmds_) != 1 {
			fmt.Printf("Expected exactly one command, got %d", len(cmds_))
		}

		cobraCommand, err := cmds.BuildCobraCommandWithGeppettoMiddlewares(cmds_[0])
		if err != nil {
			fmt.Printf("Could not build cobra command: %v\n", err)
			os.Exit(1)
		}

		// _, err = initRootCmd()
		// cobra.CheckErr(err)

		rootCmd.AddCommand(cobraCommand)
		restArgs := os.Args[3:]
		os.Args = append([]string{os.Args[0], cobraCommand.Use}, restArgs...)
	} else {
		err = initAllCommands(helpSystem)
		cobra.CheckErr(err)
	}

	log.Debug().Msg("Executing pinocchio")

	err = rootCmd.Execute()
	cobra.CheckErr(err)
}

var runCommandCmd = &cobra.Command{
	Use:   "run-command",
	Short: "Run a command from a file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		panic(errors.Errorf("not implemented"))
	},
}

func initRootCmd() (*help.HelpSystem, error) {
	helpSystem := help.NewHelpSystem()
	err := doc.AddDocToHelpSystem(helpSystem)
	cobra.CheckErr(err)

	err = pkg_doc.AddDocToHelpSystem(helpSystem)
	cobra.CheckErr(err)

	err = catter_doc.AddDocToHelpSystem(helpSystem)
	cobra.CheckErr(err)

	err = pinocchio_docs.AddDocToHelpSystem(helpSystem)
	cobra.CheckErr(err)

	help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

	err = clay.InitGlazed("pinocchio", rootCmd)
	cobra.CheckErr(err)

	rootCmd.AddCommand(runCommandCmd)
	rootCmd.AddCommand(pinocchio_cmds.NewCodegenCommand())
	return helpSystem, nil
}

// loadRepositoriesFromConfig reads repository paths from the config file
func loadRepositoriesFromConfig() []string {
	configPath, err := glazedConfig.ResolveAppConfigPath("pinocchio", "")
	if err != nil || configPath == "" {
		return []string{}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Debug().Err(err).Str("config", configPath).Msg("Could not read config file for repositories")
		return []string{}
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Debug().Err(err).Str("config", configPath).Msg("Could not parse config file")
		return []string{}
	}

	repos, ok := config["repositories"].([]interface{})
	if !ok {
		return []string{}
	}

	repositoryPaths := make([]string, 0, len(repos))
	for _, repo := range repos {
		if repoStr, ok := repo.(string); ok {
			repositoryPaths = append(repositoryPaths, repoStr)
		}
	}

	return repositoryPaths
}

func initAllCommands(helpSystem *help.HelpSystem) error {
	repositoryPaths := loadRepositoriesFromConfig()

	defaultDirectory := "$HOME/.pinocchio/prompts"
	repositoryPaths = append(repositoryPaths, defaultDirectory)

	loader := &cmds.PinocchioCommandLoader{}

	directories := []repositories.Directory{
		{
			FS:               promptsFS,
			RootDirectory:    "prompts",
			RootDocDirectory: "prompts/doc",
			Name:             "pinocchio",
			SourcePrefix:     "embed",
		}}

	for _, repositoryPath := range repositoryPaths {
		dir := os.ExpandEnv(repositoryPath)
		// check if dir exists
		if fi, err := os.Stat(dir); os.IsNotExist(err) || !fi.IsDir() {
			continue
		}
		directories = append(directories, repositories.Directory{
			FS:               os.DirFS(dir),
			RootDirectory:    ".",
			RootDocDirectory: "doc",
			WatchDirectory:   dir,
			Name:             dir,
			SourcePrefix:     "file",
		})
	}

	repositories_ := []*repositories.Repository{
		repositories.NewRepository(
			repositories.WithDirectories(directories...),
			repositories.WithCommandLoader(loader),
		),
	}

	allCommands, err := repositories.LoadRepositories(
		helpSystem,
		rootCmd,
		repositories_,
		cli.WithCobraMiddlewaresFunc(layers2.GetCobraCommandGeppettoMiddlewares),
		cli.WithCobraShortHelpSections(schema.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
		cli.WithProfileSettingsSection(),
		cli.WithCreateCommandSettingsSection(),
	)
	if err != nil {
		return err
	}

	rootCmd.AddCommand(openai.OpenaiCmd)

	tokens.RegisterCommands(rootCmd)

	kagiCmd := kagi.RegisterKagiCommands()
	rootCmd.AddCommand(kagiCmd)

	// Create and add the unified command management group
	commandManagementCmd, err := clay_commandmeta.NewCommandManagementCommandGroup(allCommands)
	if err != nil {
		return fmt.Errorf("failed to initialize command management commands: %w", err)
	}
	rootCmd.AddCommand(commandManagementCmd)

	// Add profiles command from clay
	profilesCmd, err := clay_profiles.NewProfilesCommand("pinocchio", pinocchioInitialProfilesContent)
	if err != nil {
		// Use fmt.Errorf for consistent error handling
		return fmt.Errorf("error initializing profiles command: %w", err)
	}
	rootCmd.AddCommand(profilesCmd)

	// Create and add the repositories command group
	rootCmd.AddCommand(clay_repositories.NewRepositoriesGroupCommand())

	catter.AddToRootCommand(rootCmd)

	clipCommand, err := pinocchio_cmds.NewClipCommand()
	if err != nil {
		return err
	}
	cobraClipCommand, err := cli.BuildCobraCommandFromCommand(clipCommand,
		cli.WithCobraMiddlewaresFunc(layers2.GetCobraCommandGeppettoMiddlewares),
	)
	if err != nil {
		return err
	}
	rootCmd.AddCommand(cobraClipCommand)

	// Add temporizer command
	temporizerCmd := temporizer.NewTemporizerCommand()
	rootCmd.AddCommand(temporizerCmd)

	// Add helper commands
	err = helpers.RegisterHelperCommands(rootCmd)
	if err != nil {
		return err
	}

	return nil
}

// pinocchioInitialProfilesContent provides the default YAML content for a new pinocchio profiles file.
func pinocchioInitialProfilesContent() string {
	return `# Pinocchio Profiles Configuration
#
# This file contains profile configurations for Pinocchio.
# Each profile can override layer parameters for different components (like AI models).
# Profiles allow you to easily switch between different model providers, API keys,
# or specific model settings.
#
# Profiles are selected using the --profile <profile-name> flag.
#
# Example:
#
# anyscale-mixtral:
#   # Override settings for the 'openai-chat' layer (used by OpenAI compatible APIs)
#   openai-chat:
#     openai-base-url: https://api.endpoints.anyscale.com/v1
#     openai-api-key: "YOUR_ANYSCALE_API_KEY" # Replace with your key or use environment variable
#   # Override settings for the general 'ai-chat' layer
#   ai-chat:
#     ai-engine: mistralai/Mixtral-8x7B-Instruct-v0.1
#     ai-api-type: openai
#     # You could override temperature, max tokens etc. here too
#     # temperature: 0.5
#
# openai-gpt4:
#   openai-chat:
#     # openai-base-url defaults to OpenAI, no need to set normally
#     openai-api-key: "YOUR_OPENAI_API_KEY" # Replace with your key or use environment variable
#   ai-chat:
#     ai-engine: gpt-4-turbo
#     ai-api-type: openai
#
# You can manage this file using the 'pinocchio profiles' commands:
# - list: List all profiles
# - get <profile> [layer] [key]: Get profile settings
# - set <profile> <layer> <key> <value>: Set a profile setting
# - delete <profile> [layer] [key]: Delete a profile, layer, or setting
# - edit: Open this file in your editor
# - init: Create this file if it doesn't exist
# - duplicate <source> <new>: Copy an existing profile
`
}
