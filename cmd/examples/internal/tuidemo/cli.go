package tuidemo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	enginefactory "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
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
	cliSection, err := newCLISection(spec)
	if err != nil {
		return err
	}
	profileSettingsSection, err := profilebootstrap.NewProfileSettingsSection()
	if err != nil {
		return err
	}

	root := &cobra.Command{
		Use:   strings.TrimSpace(spec.Use),
		Short: strings.TrimSpace(spec.Short),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliValues, err := cliSection.(schema.CobraSection).ParseSectionFromCobraCommand(cmd)
			if err != nil {
				return err
			}
			configFile, _ := cliValues.GetField("config-file")
			logLevel, _ := cliValues.GetField("log-level")
			fixtureID, _ := cliValues.GetField(strings.TrimSpace(spec.FixtureFlag))
			listFixtures, _ := cliValues.GetField(strings.TrimSpace(spec.ListFlag))

			zerolog.TimeFieldFormat = time.StampMilli
			if parsed, err := zerolog.ParseLevel(strings.TrimSpace(fmt.Sprint(logLevel))); err == nil {
				zerolog.SetGlobalLevel(parsed)
			}

			if listValue, ok := listFixtures.(bool); ok && listValue {
				for _, value := range spec.listValues() {
					fmt.Println(value)
				}
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			parsed, err := buildParsedCLIValues(profileSettingsSection, cmd, strings.TrimSpace(fmt.Sprint(configFile)))
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
			return spec.Run(ctx, strings.TrimSpace(fmt.Sprint(fixtureID)), engineInstance)
		},
	}

	if err := cliSection.(schema.CobraSection).AddSectionToCobraCommand(root); err != nil {
		return err
	}
	if err := profileSettingsSection.(schema.CobraSection).AddSectionToCobraCommand(root); err != nil {
		return err
	}

	return root.Execute()
}

func newCLISection(spec CLISpec) (schema.Section, error) {
	return schema.NewSection(schema.DefaultSlug, "Flags",
		schema.WithFields(
			fields.New(
				strings.TrimSpace(spec.FixtureFlag),
				fields.TypeString,
				fields.WithDefault(spec.FixtureDefault),
				fields.WithHelp(strings.TrimSpace(spec.FixtureUsage)),
			),
			fields.New(
				"config-file",
				fields.TypeString,
				fields.WithHelp("Path to Pinocchio config file"),
			),
			fields.New(
				"log-level",
				fields.TypeString,
				fields.WithDefault("info"),
				fields.WithHelp("Log level (trace|debug|info|warn|error)"),
			),
			fields.New(
				strings.TrimSpace(spec.ListFlag),
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp(strings.TrimSpace(spec.ListUsage)),
			),
		),
	)
}

func buildParsedCLIValues(profileSettingsSection schema.Section, cmd *cobra.Command, configFile string) (*values.Values, error) {
	profileValues, err := profileSettingsSection.(schema.CobraSection).ParseSectionFromCobraCommand(cmd)
	if err != nil {
		return nil, err
	}
	profileSettings := &profilebootstrap.ProfileSettings{}
	if err := profileValues.DecodeInto(profileSettings); err != nil {
		return nil, err
	}

	return profilebootstrap.NewCLISelectionValues(profilebootstrap.CLISelectionInput{
		ConfigFile:        configFile,
		Profile:           profileSettings.Profile,
		ProfileRegistries: profileSettings.ProfileRegistries,
	})
}

func (s CLISpec) listValues() []string {
	if s.ListValues == nil {
		return nil
	}
	return s.ListValues()
}
