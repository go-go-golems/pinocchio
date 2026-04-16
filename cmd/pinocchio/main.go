package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	sections2 "github.com/go-go-golems/geppetto/pkg/sections"

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
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/tokens"
	pinocchio_docs "github.com/go-go-golems/pinocchio/cmd/pinocchio/doc"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	pkg_doc "github.com/go-go-golems/pinocchio/pkg/doc"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	clay_repositories "github.com/go-go-golems/clay/pkg/cmds/repositories"

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
		bytes, err := readRunCommandFile(os.Args[2])
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

func readRunCommandFile(pathArg string) ([]byte, error) {
	cleanPath := filepath.Clean(pathArg)
	if filepath.IsAbs(cleanPath) {
		root, err := os.OpenRoot(filepath.Dir(cleanPath))
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = root.Close()
		}()
		return root.ReadFile(filepath.Base(cleanPath))
	}

	root, err := os.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = root.Close()
	}()

	return root.ReadFile(cleanPath)
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

	profileSettingsSection, err := sections2.NewProfileSettingsSection()
	cobra.CheckErr(err)
	err = profileSettingsSection.(schema.CobraSection).AddSectionToCobraCommand(rootCmd)
	cobra.CheckErr(err)

	rootCmd.AddCommand(runCommandCmd)
	rootCmd.AddCommand(pinocchio_cmds.NewJSCommand())
	return helpSystem, nil
}

// loadRepositoriesFromConfig reads repository paths from the unified layered pinocchio config document.
func loadRepositoriesFromConfig() []string {
	repositoryPaths, err := profilebootstrap.ResolveRepositoryPaths()
	if err != nil {
		log.Debug().Err(err).Msg("Could not resolve repository paths from unified layered config")
		return []string{}
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
		cli.WithCobraMiddlewaresFunc(cmds.GetPinocchioCommandMiddlewares),
		cli.WithCobraShortHelpSections(schema.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
		cli.WithCreateCommandSettingsSection(),
	)
	if err != nil {
		return err
	}

	tokens.RegisterCommands(rootCmd)

	// Create and add the unified command management group
	commandManagementCmd, err := clay_commandmeta.NewCommandManagementCommandGroup(allCommands)
	if err != nil {
		return fmt.Errorf("failed to initialize command management commands: %w", err)
	}
	rootCmd.AddCommand(commandManagementCmd)

	// Create and add the repositories command group
	rootCmd.AddCommand(clay_repositories.NewRepositoriesGroupCommand())

	catter.AddToRootCommand(rootCmd)

	clipCommand, err := pinocchio_cmds.NewClipCommand()
	if err != nil {
		return err
	}
	cobraClipCommand, err := cli.BuildCobraCommandFromCommand(clipCommand,
		cli.WithCobraMiddlewaresFunc(cmds.GetPinocchioCommandMiddlewares),
	)
	if err != nil {
		return err
	}
	rootCmd.AddCommand(cobraClipCommand)

	return nil
}
