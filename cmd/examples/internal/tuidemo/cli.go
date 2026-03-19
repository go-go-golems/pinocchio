package tuidemo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	enginefactory "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type CLISpec struct {
	Use            string
	Short          string
	FixtureFlag    string
	FixtureDefault string
	FixtureUsage   string
	ListFlag       string
	ListUsage      string
	ListValues     func() []string
	Run            func(ctx context.Context, fixtureID string, engineInstance engine.Engine) error
}

func ExecuteCLI(spec CLISpec) error {
	var (
		fixtureID    string
		configFile   string
		logLevel     string
		listFixtures bool
	)

	root := &cobra.Command{
		Use:   strings.TrimSpace(spec.Use),
		Short: strings.TrimSpace(spec.Short),
		RunE: func(cmd *cobra.Command, args []string) error {
			zerolog.TimeFieldFormat = time.StampMilli
			if parsed, err := zerolog.ParseLevel(strings.TrimSpace(logLevel)); err == nil {
				zerolog.SetGlobalLevel(parsed)
			}

			if listFixtures {
				for _, value := range spec.listValues() {
					fmt.Println(value)
				}
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			parsed, err := buildParsedCLIValues(cmd, strings.TrimSpace(configFile))
			if err != nil {
				return errors.Wrap(err, "build parsed CLI values")
			}

			stepSettings, closeRuntime, err := ResolveInferenceSettings(ctx, parsed)
			if err != nil {
				return errors.Wrap(err, "resolve inference settings")
			}
			if closeRuntime != nil {
				defer closeRuntime()
			}

			engineInstance, err := enginefactory.NewEngineFromSettings(stepSettings)
			if err != nil {
				return errors.Wrap(err, "create engine")
			}

			if spec.Run == nil {
				return fmt.Errorf("run hook is nil")
			}
			return spec.Run(ctx, fixtureID, engineInstance)
		},
	}

	root.Flags().StringVar(&fixtureID, strings.TrimSpace(spec.FixtureFlag), spec.FixtureDefault, strings.TrimSpace(spec.FixtureUsage))
	root.Flags().StringVar(&configFile, "config-file", "", "Path to Pinocchio config file")
	root.Flags().StringVar(&logLevel, "log-level", "info", "Log level (trace|debug|info|warn|error)")
	root.Flags().BoolVar(&listFixtures, strings.TrimSpace(spec.ListFlag), false, strings.TrimSpace(spec.ListUsage))
	profileSettingsSection, err := profilebootstrap.NewProfileSettingsSection()
	if err != nil {
		return err
	}
	if err := profileSettingsSection.(schema.CobraSection).AddSectionToCobraCommand(root); err != nil {
		return err
	}

	return root.Execute()
}

func buildParsedCLIValues(cmd *cobra.Command, configFile string) (*values.Values, error) {
	ret := values.New()

	commandSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}
	commandValues, err := values.NewSectionValues(commandSection)
	if err != nil {
		return nil, err
	}
	if trimmed := strings.TrimSpace(configFile); trimmed != "" {
		if err := values.WithFieldValue("config-file", trimmed, fields.WithSource("cli"))(commandValues); err != nil {
			return nil, err
		}
	}
	ret.Set(cli.CommandSettingsSlug, commandValues)

	profileSection, err := profilebootstrap.NewProfileSettingsSection()
	if err != nil {
		return nil, err
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		return nil, err
	}
	if profileFlag := cmd.Flags().Lookup("profile"); profileFlag != nil {
		if profile := strings.TrimSpace(profileFlag.Value.String()); profile != "" {
			if err := values.WithFieldValue("profile", profile, fields.WithSource("cli"))(profileValues); err != nil {
				return nil, err
			}
		}
	}
	if registries, err := cmd.Flags().GetStringSlice("profile-registries"); err == nil && len(registries) > 0 {
		if err := values.WithFieldValue("profile-registries", registries, fields.WithSource("cli"))(profileValues); err != nil {
			return nil, err
		}
	}
	ret.Set(profilebootstrap.ProfileSettingsSectionSlug, profileValues)

	return ret, nil
}

func (s CLISpec) listValues() []string {
	if s.ListValues == nil {
		return nil
	}
	return s.ListValues()
}
