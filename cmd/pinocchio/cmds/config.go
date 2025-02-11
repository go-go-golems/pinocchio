package cmds

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-go-golems/clay/pkg/cmds/repositories"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/pinocchio/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rs/zerolog/log"
)

// commands for manipulating the config
//
// - x - add/rm/print a repository entry
// - set ai keys
// - add profile / remove profile / update profile
//
// layers that are loaded from the config file:
// (from cobra.go)
// - ai-chat
// - ai-client
// - openai-chat
// - claude-chat

type ConfigCommand struct {
	*cobra.Command
}

func NewConfigGroupCommand(helpSystem *help.HelpSystem) (*cobra.Command, error) {
	cmd := &ConfigCommand{}

	cobraCmd := &cobra.Command{
		Use:   "config",
		Short: "Commands for manipulating configuration and profiles",
	}

	cobraCmd.AddCommand(repositories.NewRepositoriesGroupCommand())
	err := repositories.AddDocToHelpSystem(helpSystem)
	if err != nil {
		return nil, err
	}

	cobraCmd.AddCommand(cmd.newListCommand())
	cobraCmd.AddCommand(cmd.newGetCommand())
	cobraCmd.AddCommand(cmd.newSetCommand())
	cobraCmd.AddCommand(cmd.newDeleteCommand())
	cobraCmd.AddCommand(cmd.newEditCommand())

	cmd.Command = cobraCmd
	return cobraCmd, nil
}

// getEditor returns a new ConfigEditor instance for the current config file
func (c *ConfigCommand) getEditor() (*config.ConfigEditor, error) {
	configPath := viper.ConfigFileUsed()
	log.Debug().Str("config_path", configPath).Msg("using config file")

	editor, err := config.NewConfigEditor(configPath)
	if err != nil {
		return nil, fmt.Errorf("could not create config editor: %w", err)
	}

	return editor, nil
}

func (c *ConfigCommand) newListCommand() *cobra.Command {
	var concise bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configuration keys and values",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, err := c.getEditor()
			if err != nil {
				return err
			}

			if concise {
				keys := editor.ListKeys()
				for _, key := range keys {
					fmt.Println(key)
				}
				return nil
			}

			settings := editor.GetAll()
			for key, value := range settings {
				fmt.Printf("%s: %s\n", key, config.FormatValue(value))
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&concise, "concise", "c", true, "Only show keys")
	return cmd
}

func (c *ConfigCommand) newGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, err := c.getEditor()
			if err != nil {
				return err
			}

			key := args[0]
			value, err := editor.Get(key)
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", config.FormatValue(value))
			return nil
		},
	}
}

func (c *ConfigCommand) newSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, err := c.getEditor()
			if err != nil {
				return err
			}

			key := args[0]
			value := args[1]

			if err := editor.Set(key, value); err != nil {
				return err
			}

			return editor.Save()
		},
	}
}

func (c *ConfigCommand) newDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, err := c.getEditor()
			if err != nil {
				return err
			}

			key := args[0]

			if err := editor.Delete(key); err != nil {
				return err
			}

			return editor.Save()
		},
	}
}

func (c *ConfigCommand) newEditCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the configuration file in your default editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}

			configPath := viper.ConfigFileUsed()
			if configPath == "" {
				var err error
				configPath, err = config.GetDefaultConfigPath()
				if err != nil {
					return err
				}
			}

			// Ensure the directory exists
			if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
				return fmt.Errorf("could not create config directory: %w", err)
			}

			editCmd := exec.Command(editor, configPath)
			editCmd.Stdin = os.Stdin
			editCmd.Stdout = os.Stdout
			editCmd.Stderr = os.Stderr

			return editCmd.Run()
		},
	}
}
