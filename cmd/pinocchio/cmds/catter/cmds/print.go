package cmds

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-go-golems/clay/pkg/filefilter"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/catter/pkg"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
)

type CatterPrintSettings struct {
	MaxTotalSize  int64    `glazed.parameter:"max-total-size"`
	List          bool     `glazed.parameter:"list"`
	Delimiter     string   `glazed.parameter:"delimiter"`
	MaxLines      int      `glazed.parameter:"max-lines"`
	MaxTokens     int      `glazed.parameter:"max-tokens"`
	PrintFilters  bool     `glazed.parameter:"print-filters"`
	FilterYAML    string   `glazed.parameter:"filter-yaml"`
	FilterProfile string   `glazed.parameter:"filter-profile"`
	Glazed        bool     `glazed.parameter:"glazed"`
	ArchiveFile   string   `glazed.parameter:"archive-file"`
	ArchivePrefix string   `glazed.parameter:"archive-prefix"`
	Paths         []string `glazed.parameter:"paths"`
}

type CatterPrintCommand struct {
	*cmds.CommandDescription
}

func NewCatterPrintCommand() (*CatterPrintCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, fmt.Errorf("could not create Glazed parameter layer: %w", err)
	}

	fileFilterLayer, err := filefilter.NewFileFilterParameterLayer()
	if err != nil {
		return nil, fmt.Errorf("could not create file filter parameter layer: %w", err)
	}

	return &CatterPrintCommand{
		CommandDescription: cmds.NewCommandDescription(
			"print",
			cmds.WithShort("Print or archive file contents with token counting"),
			cmds.WithLong("A CLI tool to print or archive file contents, recursively process directories, and count tokens for LLM context preparation."),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"max-total-size",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Maximum total size of all files in bytes (default no limit)"),
					parameters.WithDefault(0),
				),
				parameters.NewParameterDefinition(
					"list",
					parameters.ParameterTypeBool,
					parameters.WithHelp("List filenames only without printing content"),
					parameters.WithDefault(false),
					parameters.WithShortFlag("l"),
				),
				parameters.NewParameterDefinition(
					"delimiter",
					parameters.ParameterTypeString,
					parameters.WithHelp("Type of delimiter to use between files (text output only): default, xml, markdown, simple, begin-end"),
					parameters.WithDefault("default"),
					parameters.WithShortFlag("d"),
				),
				parameters.NewParameterDefinition(
					"max-lines",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Maximum number of lines to print per file (0 for no limit, text output only)"),
					parameters.WithDefault(0),
				),
				parameters.NewParameterDefinition(
					"max-tokens",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Maximum number of tokens to print per file (0 for no limit, text output only)"),
					parameters.WithDefault(0),
				),
				parameters.NewParameterDefinition(
					"print-filters",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Print configured filters"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"filter-yaml",
					parameters.ParameterTypeString,
					parameters.WithHelp("Path to YAML file containing filter configuration"),
				),
				parameters.NewParameterDefinition(
					"filter-profile",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the filter profile to use"),
				),
				parameters.NewParameterDefinition(
					"glazed",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Enable Glazed structured output (ignored if --archive-file is used)"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"archive-file",
					parameters.ParameterTypeString,
					parameters.WithHelp("Path to the output archive file. Format (zip or tar.gz) inferred from extension."),
					parameters.WithDefault(""),
					parameters.WithShortFlag("a"),
				),
				parameters.NewParameterDefinition(
					"archive-prefix",
					parameters.ParameterTypeString,
					parameters.WithHelp("Directory prefix to add to files within the archive (e.g., 'myproject/')"),
					parameters.WithDefault(""),
				),
			),
			cmds.WithArguments(
				parameters.NewParameterDefinition(
					"paths",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Paths to process"),
					parameters.WithDefault([]string{"."}),
				),
			),
			cmds.WithLayersList(
				glazedParameterLayer,
				fileFilterLayer,
			),
		),
	}, nil
}

func (c *CatterPrintCommand) RunIntoGlazeProcessor(ctx context.Context, parsedLayers *layers.ParsedLayers, gp middlewares.Processor) error {
	s := &CatterPrintSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return fmt.Errorf("error initializing settings: %w", err)
	}

	outputFormat := "text"
	outputFile := s.ArchiveFile
	isArchiveOutput := outputFile != ""

	if isArchiveOutput {
		if strings.HasSuffix(outputFile, ".zip") {
			outputFormat = "zip"
		} else if strings.HasSuffix(outputFile, ".tar.gz") || strings.HasSuffix(outputFile, ".tgz") {
			outputFormat = "tar.gz"
		} else {
			return fmt.Errorf("unsupported archive file extension for %s. Use .zip, .tar.gz, or .tgz", outputFile)
		}

		if s.Glazed {
			_, _ = fmt.Fprintln(os.Stderr, "Warning: --glazed is ignored when --archive-file is specified.")
			s.Glazed = false
		}
		if s.List {
			return fmt.Errorf("--list cannot be used with --archive-file")
		}
	}

	archivePrefix := s.ArchivePrefix
	if archivePrefix != "" && !strings.HasSuffix(archivePrefix, "/") {
		archivePrefix += "/"
	}

	ff, err := createFileFilter(parsedLayers, s.FilterYAML, s.FilterProfile)
	if err != nil {
		return err
	}

	fileProcessorOptions := []pkg.FileProcessorOption{
		pkg.WithMaxTotalSize(s.MaxTotalSize),
		pkg.WithListOnly(s.List),
		pkg.WithDelimiterType(s.Delimiter),
		pkg.WithMaxLines(s.MaxLines),
		pkg.WithMaxTokens(s.MaxTokens),
		pkg.WithFileFilter(ff),
		pkg.WithPrintFilters(s.PrintFilters),
		pkg.WithOutputFormat(outputFormat),
		pkg.WithOutputFile(outputFile),
		pkg.WithArchivePrefix(archivePrefix),
	}

	if !isArchiveOutput && s.Glazed {
		fileProcessorOptions = append(fileProcessorOptions, pkg.WithProcessor(gp))
	}

	fp := pkg.NewFileProcessor(fileProcessorOptions...)

	if len(s.Paths) < 1 {
		s.Paths = append(s.Paths, ".")
	}

	return fp.ProcessPaths(s.Paths)
}

func createFileFilter(parsedLayers *layers.ParsedLayers, filterYAML, filterProfile string) (*filefilter.FileFilter, error) {
	layer, ok := parsedLayers.Get(filefilter.FileFilterSlug)
	if !ok {
		return nil, fmt.Errorf("file filter layer not found")
	}
	ff, err := filefilter.CreateFileFilterFromSettings(layer)
	if err != nil {
		return nil, fmt.Errorf("error creating file filter: %w", err)
	}

	if filterYAML != "" {
		ff, err = filefilter.LoadFromFile(filterYAML, filterProfile)
		if err != nil {
			return nil, fmt.Errorf("error loading filter configuration from YAML: %w", err)
		}
	} else {
		if _, err := os.Stat(".catter-filter.yaml"); err == nil {
			ff, err = filefilter.LoadFromFile(".catter-filter.yaml", filterProfile)
			if err != nil {
				return nil, fmt.Errorf("error loading default filter configuration: %w", err)
			}
		}
	}

	return ff, nil
}
