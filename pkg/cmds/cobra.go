package cmds

import (
	"context"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"github.com/spf13/cobra"
)

func GetPinocchioCommandMiddlewares(parsedCommandSections *values.Values, cmd *cobra.Command, args []string) ([]sources.Middleware, error) {
	cfg := profilebootstrap.BootstrapConfig()
	return []sources.Middleware{
		sources.FromCobra(cmd, fields.WithSource("cobra")),
		sources.FromArgs(args, fields.WithSource("arguments")),
		sources.FromEnv(cfg.EnvPrefix, fields.WithSource("env")),
		sources.FromConfigPlanBuilder(
			func(_ context.Context, _ *values.Values) (*glazedconfig.Plan, error) {
				return cfg.ConfigPlanBuilder(parsedCommandSections)
			},
			sources.WithConfigFileMapper(cfg.ConfigFileMapper),
			sources.WithParseOptions(fields.WithSource("config")),
		),
		sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	}, nil
}

func BuildCobraCommandWithGeppettoMiddlewares(
	cmd cmds.Command,
	options ...cli.CobraOption,
) (*cobra.Command, error) {
	config := cli.CobraParserConfig{
		MiddlewaresFunc:   GetPinocchioCommandMiddlewares,
		ShortHelpSections: []string{schema.DefaultSlug, cmdlayers.GeppettoHelpersSlug},
	}

	options_ := append([]cli.CobraOption{
		cli.WithParserConfig(config),
	}, options...)

	return cli.BuildCobraCommand(cmd, options_...)
}
