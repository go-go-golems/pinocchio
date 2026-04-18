package cmds

import (
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"github.com/spf13/cobra"
)

func BuildCobraCommandWithGeppettoMiddlewares(
	cmd cmds.Command,
	options ...cli.CobraOption,
) (*cobra.Command, error) {
	bootstrapCfg := profilebootstrap.BootstrapConfig()
	config := cli.CobraParserConfig{
		AppName: "pinocchio",
		ConfigPlanBuilder: func(parsed *values.Values, _ *cobra.Command, _ []string) (*glazedconfig.Plan, error) {
			return bootstrapCfg.ConfigPlanBuilder(parsed)
		},
		ShortHelpSections: []string{schema.DefaultSlug, cmdlayers.GeppettoHelpersSlug},
	}

	options_ := append([]cli.CobraOption{
		cli.WithParserConfig(config),
	}, options...)

	return cli.BuildCobraCommand(cmd, options_...)
}
