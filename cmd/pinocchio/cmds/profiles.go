package cmds

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/go-go-golems/pinocchio/pkg/profiles"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type ProfilesCommand struct {
	*cobra.Command
}

func NewProfilesCommand() (*cobra.Command, error) {
	cmd := &ProfilesCommand{}

	cobraCmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage pinocchio profiles",
	}

	cobraCmd.AddCommand(cmd.newListCommand())
	cobraCmd.AddCommand(cmd.newGetCommand())
	cobraCmd.AddCommand(cmd.newSetCommand())
	cobraCmd.AddCommand(cmd.newDeleteCommand())
	cobraCmd.AddCommand(cmd.newEditCommand())
	cobraCmd.AddCommand(cmd.newInitCommand())
	cobraCmd.AddCommand(cmd.newDuplicateCommand())

	cmd.Command = cobraCmd
	return cobraCmd, nil
}

// getEditor returns a new ProfilesEditor instance for the current profiles file
func (c *ProfilesCommand) getEditor() (*profiles.ProfilesEditor, error) {
	profilesPath, err := profiles.GetDefaultProfilesPath()
	if err != nil {
		return nil, fmt.Errorf("could not get profiles path: %w", err)
	}

	log.Debug().Str("profiles_path", profilesPath).Msg("using profiles file")
	editor, err := profiles.NewProfilesEditor(profilesPath)
	if err != nil {
		return nil, fmt.Errorf("could not create profiles editor: %w", err)
	}

	return editor, nil
}

func (c *ProfilesCommand) newListCommand() *cobra.Command {
	var concise bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, err := c.getEditor()
			if err != nil {
				return err
			}

			profiles, contents, err := editor.ListProfiles()
			if err != nil {
				return err
			}

			if concise {
				for _, profile := range profiles {
					fmt.Println(profile)
				}
				return nil
			}

			// Show full profile contents
			for _, profile := range profiles {
				fmt.Printf("%s:\n", profile)
				layers := contents[profile]
				// Sort layers for consistent output
				for layerName, settings := range layers {
					fmt.Printf("  %s:\n", layerName)
					// Sort settings for consistent output
					for key, value := range settings {
						fmt.Printf("    %s: %s\n", key, value)
					}
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&concise, "concise", "c", false, "Only show profile names")
	return cmd
}

func (c *ProfilesCommand) newGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <profile> [layer] [key]",
		Short: "Get profile settings",
		Args:  cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, err := c.getEditor()
			if err != nil {
				return err
			}

			profile := args[0]

			if len(args) == 1 {
				// Show all layers
				layers, err := editor.GetProfileLayers(profile)
				if err != nil {
					return err
				}

				for pair := layers.Oldest(); pair != nil; pair = pair.Next() {
					fmt.Printf("%s:\n", pair.Key)
					settings := pair.Value
					for settingPair := settings.Oldest(); settingPair != nil; settingPair = settingPair.Next() {
						fmt.Printf("  %s: %s\n", settingPair.Key, settingPair.Value)
					}
				}
				return nil
			}

			layer := args[1]
			if len(args) == 2 {
				// Show all settings for layer
				layers, err := editor.GetProfileLayers(profile)
				if err != nil {
					return err
				}

				settings, ok := layers.Get(layer)
				if !ok {
					return fmt.Errorf("layer %s not found in profile %s", layer, profile)
				}

				for pair := settings.Oldest(); pair != nil; pair = pair.Next() {
					fmt.Printf("%s: %s\n", pair.Key, pair.Value)
				}
				return nil
			}

			// Get specific value
			key := args[2]
			value, err := editor.GetLayerValue(profile, layer, key)
			if err != nil {
				return err
			}

			fmt.Println(value)
			return nil
		},
	}
}

func (c *ProfilesCommand) newSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <profile> <layer> <key> <value>",
		Short: "Set a profile setting",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, err := c.getEditor()
			if err != nil {
				return err
			}

			profile := args[0]
			layer := args[1]
			key := args[2]
			value := args[3]

			if err := editor.SetLayerValue(profile, layer, key, value); err != nil {
				return err
			}

			return editor.Save()
		},
	}
}

func (c *ProfilesCommand) newDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <profile> [layer] [key]",
		Short: "Delete a profile, layer, or setting",
		Args:  cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, err := c.getEditor()
			if err != nil {
				return err
			}

			profile := args[0]

			if len(args) == 1 {
				// Delete entire profile
				if err := editor.DeleteProfile(profile); err != nil {
					return err
				}
			} else if len(args) == 3 {
				// Delete specific setting
				layer := args[1]
				key := args[2]
				if err := editor.DeleteLayerValue(profile, layer, key); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("must specify either profile or profile, layer, and key")
			}

			return editor.Save()
		},
	}
}

func (c *ProfilesCommand) newEditCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the profiles file in your default editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}

			profilesPath, err := profiles.GetDefaultProfilesPath()
			if err != nil {
				return err
			}

			// Ensure the directory exists
			if err := os.MkdirAll(profilesPath[:len(profilesPath)-len("/profiles.yaml")], 0755); err != nil {
				return fmt.Errorf("could not create profiles directory: %w", err)
			}

			editCmd := exec.Command(editor, profilesPath)
			editCmd.Stdin = os.Stdin
			editCmd.Stdout = os.Stdout
			editCmd.Stderr = os.Stderr

			return editCmd.Run()
		},
	}
}

func (c *ProfilesCommand) newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a new profiles file",
		RunE: func(cmd *cobra.Command, args []string) error {
			profilesPath, err := profiles.GetDefaultProfilesPath()
			if err != nil {
				return err
			}

			// Check if file already exists
			if _, err := os.Stat(profilesPath); err == nil {
				return fmt.Errorf("profiles file already exists at %s", profilesPath)
			}

			// Ensure the directory exists
			if err := os.MkdirAll(profilesPath[:len(profilesPath)-len("/profiles.yaml")], 0755); err != nil {
				return fmt.Errorf("could not create profiles directory: %w", err)
			}

			// Create initial profiles file with documentation
			initialContent := `# Pinocchio Profiles Configuration
#
# This file contains profile configurations for Pinocchio.
# Each profile can override layer parameters for different components.
#
# Example:
#
# mixtral:
#   openai-chat:
#     openai-base-url: https://api.endpoints.anyscale.com/v1
#     openai-api-key: XXX
#   ai-chat:
#     ai-engine: mistralai/Mixtral-8x7B-Instruct-v0.1
#     ai-api-type: openai
#
# mistral:
#   openai-chat:
#     openai-base-url: https://api.endpoints.anyscale.com/v1
#     openai-api-key: XXX
#   ai-chat:
#     ai-engine: mistralai/Mistral-7B-Instruct-v0.1
#     ai-api-type: openai
#
# You can manage this file using the 'pinocchio profiles' commands:
# - list: List all profiles
# - get: Get profile settings
# - set: Set a profile setting
# - delete: Delete a profile or setting
# - edit: Open this file in your editor
`
			if err := os.WriteFile(profilesPath, []byte(initialContent), 0644); err != nil {
				return fmt.Errorf("could not write profiles file: %w", err)
			}

			fmt.Printf("Created new profiles file at %s\n", profilesPath)
			return nil
		},
	}
}

func (c *ProfilesCommand) newDuplicateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "duplicate <source-profile> <new-profile>",
		Short: "Duplicate an existing profile with a new name",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, err := c.getEditor()
			if err != nil {
				return err
			}

			sourceProfile := args[0]
			newProfile := args[1]

			if err := editor.DuplicateProfile(sourceProfile, newProfile); err != nil {
				return err
			}

			if err := editor.Save(); err != nil {
				return err
			}

			fmt.Printf("Duplicated profile %s to %s\n", sourceProfile, newProfile)
			return nil
		},
	}
}
