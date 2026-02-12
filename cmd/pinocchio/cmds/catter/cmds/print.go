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

type CatterPrintSettings struct {
	MaxTotalSize  int64    `glazed:"max-total-size"`
	List          bool     `glazed:"list"`
	Delimiter     string   `glazed:"delimiter"`
	MaxLines      int      `glazed:"max-lines"`
	MaxTokens     int      `glazed:"max-tokens"`
	PrintFilters  bool     `glazed:"print-filters"`
	FilterYAML    string   `glazed:"filter-yaml"`
	FilterProfile string   `glazed:"filter-profile"`
	Glazed        bool     `glazed:"glazed"`
	ArchiveFile   string   `glazed:"archive-file"`
	ArchivePrefix string   `glazed:"archive-prefix"`
	Paths         []string `glazed:"paths"`
}

type CatterPrintCommand struct {
	*cmds.CommandDescription
}

func NewCatterPrintCommand() (*CatterPrintCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedSection()
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
				fields.New(
					"max-total-size",
					fields.TypeInteger,
					fields.WithHelp("Maximum total size of all files in bytes (default no limit)"),
					fields.WithDefault(0),
				),
				fields.New(
					"list",
					fields.TypeBool,
					fields.WithHelp("List filenames only without printing content"),
					fields.WithDefault(false),
					fields.WithShortFlag("l"),
				),
				fields.New(
					"delimiter",
					fields.TypeString,
					fields.WithHelp("Type of delimiter to use between files (text output only): default, xml, markdown, simple, begin-end"),
					fields.WithDefault("default"),
					fields.WithShortFlag("d"),
				),
				fields.New(
					"max-lines",
					fields.TypeInteger,
					fields.WithHelp("Maximum number of lines to print per file (0 for no limit, text output only)"),
					fields.WithDefault(0),
				),
				fields.New(
					"max-tokens",
					fields.TypeInteger,
					fields.WithHelp("Maximum number of tokens to print per file (0 for no limit, text output only)"),
					fields.WithDefault(0),
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
					fields.WithHelp("Enable Glazed structured output (ignored if --archive-file is used)"),
					fields.WithDefault(false),
				),
				fields.New(
					"archive-file",
					fields.TypeString,
					fields.WithHelp("Path to the output archive file. Format (zip or tar.gz) inferred from extension."),
					fields.WithDefault(""),
					fields.WithShortFlag("a"),
				),
				fields.New(
					"archive-prefix",
					fields.TypeString,
					fields.WithHelp("Directory prefix to add to files within the archive (e.g., 'myproject/')"),
					fields.WithDefault(""),
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

func (c *CatterPrintCommand) RunIntoGlazeProcessor(ctx context.Context, parsedLayers *values.Values, gp middlewares.Processor) error {
	s := &CatterPrintSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
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

func createFileFilter(parsedLayers *values.Values, filterYAML, filterProfile string) (*filefilter.FileFilter, error) {
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
