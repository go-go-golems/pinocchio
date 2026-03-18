package tuidemo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	enginefactory "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
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
		fixtureID         string
		profileSlug       string
		profileRegistries string
		logLevel          string
		listFixtures      bool
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

			stepSettings, closeRuntime, err := ResolveInferenceSettings(ctx, profileSlug, profileRegistries)
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
	root.Flags().StringVar(&profileSlug, "profile", "", "Optional profile slug to resolve from profile registries")
	root.Flags().StringVar(&profileRegistries, "profile-registries", "", "Optional comma-separated profile registry sources (yaml/sqlite/sqlite-dsn)")
	root.Flags().StringVar(&logLevel, "log-level", "info", "Log level (trace|debug|info|warn|error)")
	root.Flags().BoolVar(&listFixtures, strings.TrimSpace(spec.ListFlag), false, strings.TrimSpace(spec.ListUsage))

	return root.Execute()
}

func (s CLISpec) listValues() []string {
	if s.ListValues == nil {
		return nil
	}
	return s.ListValues()
}
