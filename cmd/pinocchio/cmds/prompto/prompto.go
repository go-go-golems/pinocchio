package prompto

import (
	"fmt"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/prompto/cmd/prompto/cmds"
	"github.com/spf13/cobra"
)

var promptoCmd = &cobra.Command{
	Use:   "prompto",
	Short: "prompto generates prompting context from a list of repositories",
	Long:  "prompto loads a list of repositories from a yaml config file and generates prompting context from them",
}

// loadPromptoConfig creates a new viper instance for prompto's config
func loadPromptoConfig() ([]string, error) {
	promptoViper, err := clay.InitViperInstanceWithAppName("prompto", "")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize prompto viper: %w", err)
	}

	if err := promptoViper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading prompto config: %w", err)
	}

	return promptoViper.GetStringSlice("repositories"), nil
}

func InitPromptoCmd(helpSystem *help.HelpSystem) (*cobra.Command, error) {
	repositories, err := loadPromptoConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load prompto repositories: %w", err)
	}

	options := cmds.NewCommandOptions(repositories)
	for _, cmd := range cmds.NewCommands(options) {
		promptoCmd.AddCommand(cmd)
	}

	command, err := cmds.NewConfigGroupCommand(helpSystem)
	if err != nil {
		return nil, err
	}
	promptoCmd.AddCommand(command)

	return promptoCmd, nil
}
