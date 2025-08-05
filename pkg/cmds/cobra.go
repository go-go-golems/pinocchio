package cmds

import (
	layers2 "github.com/go-go-golems/geppetto/pkg/layers"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/spf13/cobra"
)

func BuildCobraCommandWithGeppettoMiddlewares(
	cmd cmds.Command,
	options ...cli.CobraOption,
) (*cobra.Command, error) {
	config := cli.CobraParserConfig{
		MiddlewaresFunc: layers2.GetCobraCommandGeppettoMiddlewares,
		ShortHelpLayers: []string{layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug},
	}

	options_ := append([]cli.CobraOption{
		cli.WithParserConfig(config),
	}, options...)

	return cli.BuildCobraCommand(cmd, options_...)
}
