package filefilter

import (
	"fmt"
	"os"

	"github.com/denormal/go-gitignore"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

type FileFilterSettings struct {
	MaxFileSize           int64    `glazed:"max-file-size"`
	DisableGitIgnore      bool     `glazed:"disable-gitignore"`
	DisableDefaultFilters bool     `glazed:"disable-default-filters"`
	Include               []string `glazed:"include"`
	Exclude               []string `glazed:"exclude"`
	MatchFilename         []string `glazed:"match-filename"`
	MatchPath             []string `glazed:"match-path"`
	ExcludeDirs           []string `glazed:"exclude-dirs"`
	ExcludeMatchFilename  []string `glazed:"exclude-match-filename"`
	ExcludeMatchPath      []string `glazed:"exclude-match-path"`
	FilterBinary          bool     `glazed:"filter-binary"`
	Verbose               bool     `glazed:"verbose"`
}

const FileFilterSlug = "file-filter"

func NewFileFilterParameterLayer() (schema.Section, error) {
	return schema.NewSection(
		FileFilterSlug,
		"File Filter Options",
		schema.WithFields(
			fields.New(
				"max-file-size",
				fields.TypeInteger,
				fields.WithHelp("Maximum size of individual files in bytes"),
				fields.WithDefault(int64(1024*1024)),
			),
			fields.New(
				"disable-gitignore",
				fields.TypeBool,
				fields.WithHelp("Disable .gitignore filter"),
				fields.WithDefault(false),
			),
			fields.New(
				"disable-default-filters",
				fields.TypeBool,
				fields.WithHelp("Disable default file and directory filters"),
				fields.WithDefault(false),
			),
			fields.New(
				"include",
				fields.TypeStringList,
				fields.WithHelp("List of file extensions to include (e.g., .go,.js)"),
				fields.WithShortFlag("i"),
			),
			fields.New(
				"exclude",
				fields.TypeStringList,
				fields.WithHelp("List of file extensions to exclude (e.g., .exe,.dll)"),
				fields.WithShortFlag("e"),
			),
			fields.New(
				"match-filename",
				fields.TypeStringList,
				fields.WithHelp("List of regular expressions to match filenames"),
				fields.WithShortFlag("f"),
			),
			fields.New(
				"match-path",
				fields.TypeStringList,
				fields.WithHelp("List of regular expressions to match full paths"),
				fields.WithShortFlag("p"),
			),
			fields.New(
				"exclude-dirs",
				fields.TypeStringList,
				fields.WithHelp("List of directories to exclude"),
				fields.WithShortFlag("x"),
			),
			fields.New(
				"exclude-match-filename",
				fields.TypeStringList,
				fields.WithHelp("List of regular expressions to exclude matching filenames"),
				fields.WithShortFlag("F"),
			),
			fields.New(
				"exclude-match-path",
				fields.TypeStringList,
				fields.WithHelp("List of regular expressions to exclude matching full paths"),
				fields.WithShortFlag("P"),
			),
			fields.New(
				"filter-binary",
				fields.TypeBool,
				fields.WithHelp("Filter out binary files"),
				fields.WithDefault(true),
			),
			fields.New(
				"verbose",
				fields.TypeBool,
				fields.WithHelp("Enable verbose logging of filtered/unfiltered paths"),
				fields.WithDefault(false),
				fields.WithShortFlag("v"),
			),
		),
	)
}

func CreateFileFilterFromSettings(parsedSection *values.SectionValues) (*FileFilter, error) {
	s := &FileFilterSettings{}
	err := parsedSection.DecodeInto(s)
	if err != nil {
		return nil, err
	}

	ff := NewFileFilter()

	ff.MaxFileSize = s.MaxFileSize
	ff.IncludeExts = s.Include
	ff.ExcludeExts = s.Exclude
	ff.MatchFilenames = compileRegexps(s.MatchFilename)
	ff.MatchPaths = compileRegexps(s.MatchPath)
	ff.ExcludeDirs = s.ExcludeDirs
	ff.ExcludeMatchFilenames = compileRegexps(s.ExcludeMatchFilename)
	ff.ExcludeMatchPaths = compileRegexps(s.ExcludeMatchPath)
	ff.DisableGitIgnore = s.DisableGitIgnore
	ff.DisableDefaultFilters = s.DisableDefaultFilters
	ff.Verbose = s.Verbose
	ff.FilterBinaryFiles = s.FilterBinary

	if !ff.DisableGitIgnore {
		gitIgnoreFilter, err := initGitIgnoreFilter()
		if err != nil {
			return nil, fmt.Errorf("error initializing gitignore filter: %w", err)
		}
		ff.GitIgnoreFilter = gitIgnoreFilter
	}

	return ff, nil
}

func initGitIgnoreFilter() (gitignore.GitIgnore, error) {
	if _, err := os.Stat(".gitignore"); err == nil {
		gitIgnoreFilter, err := gitignore.NewFromFile(".gitignore")
		if err != nil {
			return nil, fmt.Errorf("error initializing gitignore filter from file: %w", err)
		}
		return gitIgnoreFilter, nil
	}

	gitIgnoreFilter, err := gitignore.NewRepository(".")
	if err != nil {
		return nil, fmt.Errorf("error initializing gitignore filter: %w", err)
	}
	return gitIgnoreFilter, nil
}
