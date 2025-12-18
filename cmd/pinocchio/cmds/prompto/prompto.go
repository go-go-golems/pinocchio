package prompto

import (
	"os"

	clay_repositories "github.com/go-go-golems/clay/pkg/cmds/repositories"
	appconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/prompto/cmd/prompto/cmds"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var promptoCmd = &cobra.Command{
	Use:   "prompto",
	Short: "prompto generates prompting context from a list of repositories",
	Long:  "prompto loads a list of repositories from a yaml config file and generates prompting context from them",
}

// loadPromptoConfig reads repositories from config file
func loadPromptoConfig() ([]string, error) {
	configPath, err := appconfig.ResolveAppConfigPath("prompto", "")
	if err != nil || configPath == "" {
		return []string{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return []string{}, nil
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return []string{}, nil
	}

	repositories := []string{}
	if repos, ok := config["repositories"].([]interface{}); ok {
		for _, repo := range repos {
			if str, ok := repo.(string); ok {
				repositories = append(repositories, str)
			}
		}
	}

	return repositories, nil
}

func InitPromptoCmd(helpSystem *help.HelpSystem) (*cobra.Command, error) {
	repositories, err := loadPromptoConfig()
	if err != nil {
		repositories = []string{}
	}

	options := cmds.NewCommandOptions(repositories)
	for _, cmd := range cmds.NewCommands(options) {
		promptoCmd.AddCommand(cmd)
	}

	promptoCmd.AddCommand(clay_repositories.NewRepositoriesGroupCommand())

	return promptoCmd, nil
}
