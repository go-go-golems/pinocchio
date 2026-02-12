package cmds

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/catter/pkg"
	"github.com/go-go-golems/pinocchio/pkg/filefilter"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
)

type CatterStatsSettings struct {
	Stats         []string `glazed:"stats"`
	PrintFilters  bool     `glazed:"print-filters"`
	FilterYAML    string   `glazed:"filter-yaml"`
	FilterProfile string   `glazed:"filter-profile"`
	Glazed        bool     `glazed:"glazed"`
	Paths         []string `glazed:"paths"`
}

type CatterStatsCommand struct {
	*cmds.CommandDescription
}

func NewCatterStatsCommand() (*CatterStatsCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedSection()
	if err != nil {
		return nil, fmt.Errorf("could not create Glazed parameter layer: %w", err)
	}

	fileFilterLayer, err := filefilter.NewFileFilterParameterLayer()
	if err != nil {
		return nil, fmt.Errorf("could not create file filter parameter layer: %w", err)
	}

	return &CatterStatsCommand{
		CommandDescription: cmds.NewCommandDescription(
			"stats",
			cmds.WithShort("Print statistics for files and directories"),
			cmds.WithLong("A CLI tool to print statistics for files and directories, including token counts and sizes."),
			cmds.WithFlags(
				fields.New(
					"stats",
					fields.TypeStringList,
					fields.WithHelp("Types of statistics to show: overview, dir, full"),
					fields.WithShortFlag("s"),
					fields.WithDefault([]string{"overview"}),
				),
				fields.New(
					"print-filters",
					fields.TypeBool,
					fields.WithHelp("Print configured filters"),
					fields.WithDefault(false),
				),
				fields.New(
					"filter-yaml",
					fields.TypeString,
					fields.WithHelp("Path to YAML file containing filter configuration"),
				),
				fields.New(
					"filter-profile",
					fields.TypeString,
					fields.WithHelp("Name of the filter profile to use"),
				),
				fields.New(
					"glazed",
					fields.TypeBool,
					fields.WithHelp("Enable Glazed structured output"),
					fields.WithDefault(true),
				),
			),
			cmds.WithArguments(
				fields.New(
					"paths",
					fields.TypeStringList,
					fields.WithHelp("Paths to process"),
					fields.WithDefault([]string{"."}),
				),
			),
			cmds.WithSections(
				glazedParameterLayer,
				fileFilterLayer,
			),
		),
	}, nil
}

func (c *CatterStatsCommand) RunIntoGlazeProcessor(ctx context.Context, parsedLayers *values.Values, gp middlewares.Processor) error {
	s := &CatterStatsSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
	if err != nil {
		return fmt.Errorf("error initializing settings: %w", err)
	}

	ff, err := createFileFilter(parsedLayers, s.FilterYAML, s.FilterProfile)
	if err != nil {
		return err
	}

	if len(s.Paths) < 1 {
		s.Paths = append(s.Paths, ".")
	}

	stats := pkg.NewStats()
	err = stats.ComputeStats(s.Paths, ff)
	if err != nil {
		return fmt.Errorf("error computing stats: %w", err)
	}

	config := pkg.Config{}
	for _, statType := range s.Stats {
		switch strings.ToLower(statType) {
		case "overview":
			config.OutputFlags |= pkg.OutputOverview
		case "dir":
			config.OutputFlags |= pkg.OutputDirStructure
		case "full":
			config.OutputFlags |= pkg.OutputFullStructure
		default:
			_, _ = fmt.Fprintf(os.Stderr, "Unknown stat type: %s\n", statType)
		}
	}

	if !s.Glazed {
		gp = nil
	}
	err = stats.PrintStats(config, gp)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error printing stats: %v\n", err)
	}

	return nil
}
