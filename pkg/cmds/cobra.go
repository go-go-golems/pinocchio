package cmds

import (
	sections2 "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/spf13/cobra"
)

func BuildCobraCommandWithGeppettoMiddlewares(
	cmd cmds.Command,
	options ...cli.CobraOption,
) (*cobra.Command, error) {
	config := cli.CobraParserConfig{
		MiddlewaresFunc:   sections2.GetCobraCommandGeppettoMiddlewares,
		ShortHelpSections: []string{schema.DefaultSlug, cmdlayers.GeppettoHelpersSlug},
	}

	options_ := append([]cli.CobraOption{
		cli.WithParserConfig(config),
	}, options...)

	return cli.BuildCobraCommand(cmd, options_...)
}
