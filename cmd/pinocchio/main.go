package main

import (
	"embed"
	"fmt"
	layers2 "github.com/go-go-golems/geppetto/pkg/layers"
	"io"
	"os"
	"strings"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/clay/pkg/repositories"
	"github.com/go-go-golems/geppetto/pkg/doc"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/catter"
	catter_doc "github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/catter/pkg/doc"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/helpers"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/kagi"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/openai"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/prompto"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/temporizer"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/tokens"
	pinocchio_docs "github.com/go-go-golems/pinocchio/cmd/pinocchio/doc"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	pkg_doc "github.com/go-go-golems/pinocchio/pkg/doc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

func filterEarlyLoggingArgs(args []string) []string {
	// Keep only flags that affect early logging initialization.
	//
	// This avoids pflag stopping early when it encounters unknown flags (which is
	// expected before we register all cobra subcommands).
	allowedKV := map[string]struct{}{
		"--log-level":            {},
		"--log-file":             {},
		"--log-format":           {},
		"--logstash-host":        {},
		"--logstash-port":        {},
		"--logstash-protocol":    {},
		"--logstash-app-name":    {},
		"--logstash-environment": {},
	}
	allowedBool := map[string]struct{}{
		"--with-caller":         {},
		"--log-to-stdout":       {},
		"--logstash-enabled":    {},
		"--debug-early-flagset": {},
	}

	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]

		// Handle --flag=value form
		if strings.HasPrefix(a, "--") && strings.Contains(a, "=") {
			name := a[:strings.Index(a, "=")]
			if _, ok := allowedKV[name]; ok {
				out = append(out, a)
				continue
			}
			if _, ok := allowedBool[name]; ok {
				out = append(out, a)
				continue
			}
			continue
		}

		// Handle bare bool flags
		if _, ok := allowedBool[a]; ok {
			out = append(out, a)
			continue
		}

		// Handle --flag value form
		if _, ok := allowedKV[a]; ok {
			out = append(out, a)
			if i+1 < len(args) {
				out = append(out, args[i+1])
				i++
			}
			continue
		}
	}

	return out
}

func initEarlyLoggingFromArgs(args []string) error {
	// We want to initialize logging before we load/register commands, so that any
	// logging during command discovery respects --log-level etc.
	//
	// We cannot use rootCmd.ParseFlags() here because:
	// - it errors on --help ("pflag: help requested") and
	// - it would fail on unknown flags (all command-specific flags) before we
	//   have registered those commands.
	//
	// So: pre-parse ONLY logging flags from os.Args, ignoring everything else.
	fs := pflag.NewFlagSet("pinocchio-early-logging", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.SetInterspersed(true)

	// Defaults must match glazed/pkg/cmds/logging/layer.go:AddLoggingLayerToRootCommand
	logLevel := fs.String("log-level", "info", "Log level (trace, debug, info, warn, error, fatal)")
	logFile := fs.String("log-file", "", "Log file (default: stderr)")
	logFormat := fs.String("log-format", "text", "Log format (json, text)")
	withCaller := fs.Bool("with-caller", false, "Log caller information")
	logToStdout := fs.Bool("log-to-stdout", false, "Log to stdout even when log-file is set")

	logstashEnabled := fs.Bool("logstash-enabled", false, "Enable logging to Logstash")
	logstashHost := fs.String("logstash-host", "logstash", "Logstash host")
	logstashPort := fs.Int("logstash-port", 5044, "Logstash port")
	logstashProtocol := fs.String("logstash-protocol", "tcp", "Logstash protocol (tcp, udp)")
	logstashAppName := fs.String("logstash-app-name", "pinocchio", "Application name for Logstash logs")
	logstashEnvironment := fs.String("logstash-environment", "development", "Environment name for Logstash logs (development, staging, production)")

	debugEarlyFlagset := fs.Bool("debug-early-flagset", false, "Debug: print early logging flag parsing values and exit conditions")

	fs.ParseErrorsAllowlist.UnknownFlags = true
	// Always attempt parsing, but never fail early logging init on parsing errors.
	// The critical behavior is: default to info-level logging (quiet), and if we
	// successfully parse --log-level etc, honor them.
	filteredArgs := filterEarlyLoggingArgs(args)
	_ = fs.Parse(filteredArgs)

	if *debugEarlyFlagset || os.Getenv("PINOCCHIO_DEBUG_EARLY_FLAGSET") == "1" {
		fmt.Fprintf(os.Stderr, "pinocchio: early-logging filtered args: %q\n", filteredArgs)
		fmt.Fprintf(os.Stderr, "pinocchio: early-logging values:\n")
		// Print a stable set of known flags (even if not explicitly set).
		fmt.Fprintf(os.Stderr, "  --log-level=%q\n", *logLevel)
		fmt.Fprintf(os.Stderr, "  --log-file=%q\n", *logFile)
		fmt.Fprintf(os.Stderr, "  --log-format=%q\n", *logFormat)
		fmt.Fprintf(os.Stderr, "  --with-caller=%t\n", *withCaller)
		fmt.Fprintf(os.Stderr, "  --log-to-stdout=%t\n", *logToStdout)
		fmt.Fprintf(os.Stderr, "  --logstash-enabled=%t\n", *logstashEnabled)
		fmt.Fprintf(os.Stderr, "  --logstash-host=%q\n", *logstashHost)
		fmt.Fprintf(os.Stderr, "  --logstash-port=%d\n", *logstashPort)
		fmt.Fprintf(os.Stderr, "  --logstash-protocol=%q\n", *logstashProtocol)
		fmt.Fprintf(os.Stderr, "  --logstash-app-name=%q\n", *logstashAppName)
		fmt.Fprintf(os.Stderr, "  --logstash-environment=%q\n", *logstashEnvironment)
	}

	return logging.InitLoggerFromSettings(&logging.LoggingSettings{
		LogLevel:            *logLevel,
		LogFile:             *logFile,
		LogFormat:           *logFormat,
		WithCaller:          *withCaller,
		LogToStdout:         *logToStdout,
		LogstashEnabled:     *logstashEnabled,
		LogstashHost:        *logstashHost,
		LogstashPort:        *logstashPort,
		LogstashProtocol:    *logstashProtocol,
		LogstashAppName:     *logstashAppName,
		LogstashEnvironment: *logstashEnvironment,
	})
}

func main() {
	helpSystem, err := initRootCmd()
	cobra.CheckErr(err)

	// Initialize logging early from CLI args so that command loading/discovery
	// respects --log-level and defaults to quiet output (info) if unset.
	_ = initEarlyLoggingFromArgs(os.Args[1:])

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

	// Debug-only flag to print early logging parsing values. Hidden so it doesn't
	// pollute help output.
	rootCmd.PersistentFlags().Bool("debug-early-flagset", false, "Debug: print early logging flag parsing values")
	_ = rootCmd.PersistentFlags().MarkHidden("debug-early-flagset")

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
		cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
		cli.WithProfileSettingsLayer(),
		cli.WithCreateCommandSettingsLayer(),
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

	promptoCommand, err := prompto.InitPromptoCmd(helpSystem)
	if err != nil {
		return err
	}
	rootCmd.AddCommand(promptoCommand)

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
