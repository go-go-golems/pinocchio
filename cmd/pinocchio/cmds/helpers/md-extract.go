package helpers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/helpers/markdown"
	"gopkg.in/yaml.v3"
)

type ExtractMdCommand struct {
	*cmds.CommandDescription
}

type ExtractMdSettings struct {
	Output           string   `glazed:"output"`
	WithQuotes       bool     `glazed:"with-quotes"`
	File             string   `glazed:"file"`
	AllowedLanguages []string `glazed:"allowed-languages"`
	BlockType        string   `glazed:"blocks"`
}

func NewExtractMdCommand() (*ExtractMdCommand, error) {
	return &ExtractMdCommand{
		CommandDescription: cmds.NewCommandDescription(
			"md-extract",
			cmds.WithShort("Extract code blocks from markdown"),
			cmds.WithFlags(
				fields.New(
					"output",
					fields.TypeChoice,
					fields.WithHelp("Output format"),
					fields.WithDefault("concatenated"),
					fields.WithChoices("concatenated", "list", "yaml"),
				),
				fields.New(
					"with-quotes",
					fields.TypeBool,
					fields.WithHelp("Include code block quotes"),
					fields.WithDefault(false),
				),
				fields.New(
					"allowed-languages",
					fields.TypeStringList,
					fields.WithHelp("List of allowed languages to extract"),
				),
				fields.New(
					"blocks",
					fields.TypeChoice,
					fields.WithHelp("Type of blocks to extract"),
					fields.WithDefault("code"),
					fields.WithChoices("all", "normal", "code"),
				),
			),
			cmds.WithArguments(
				fields.New(
					"file",
					fields.TypeString,
					fields.WithHelp("Input file (use - for stdin)"),
					fields.WithDefault("-"),
				),
			),
		),
	}, nil
}

func (c *ExtractMdCommand) RunIntoWriter(ctx context.Context, parsedLayers *values.Values, w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer func() { _ = bw.Flush() }()

	s := &ExtractMdSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
	if err != nil {
		return err
	}

	var input strings.Builder
	var lastOutput string

	processAndWrite := func(data string) error {
		input.WriteString(data)
		blocks := markdown.ExtractAllBlocks(input.String())
		output, err := generateOutput(blocks, s)
		if err != nil {
			return err
		}

		if s.File == "-" {
			newOutput := strings.TrimPrefix(output, lastOutput)
			_, err = fmt.Fprint(w, newOutput)
			lastOutput = output
		} else {
			_, err = fmt.Fprint(w, output)
		}
		return err
	}

	if s.File == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if err := processAndWrite(scanner.Text() + "\n"); err != nil {
				return err
			}
		}
		return scanner.Err()
	}

	data, err := os.ReadFile(s.File)
	if err != nil {
		return err
	}
	return processAndWrite(string(data))
}

func generateOutput(blocks []markdown.MarkdownBlock, s *ExtractMdSettings) (string, error) {
	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	isLanguageAllowed := func(lang string) bool {
		if len(s.AllowedLanguages) == 0 {
			return true
		}
		for _, allowed := range s.AllowedLanguages {
			if allowed == lang {
				return true
			}
		}
		return false
	}

	isBlockTypeAllowed := func(block markdown.MarkdownBlock) bool {
		switch s.BlockType {
		case "all":
			return true
		case "normal":
			return block.Type == markdown.Normal
		case "code":
			return block.Type == markdown.Code && isLanguageAllowed(block.Language)
		default:
			return false
		}
	}

	switch s.Output {
	case "concatenated":
		for _, block := range blocks {
			if !isBlockTypeAllowed(block) {
				continue
			}

			if block.Type == markdown.Code && s.WithQuotes {
				_, _ = fmt.Fprintf(bw, "```%s\n%s\n```\n", block.Language, block.Content)
			} else {
				_, _ = fmt.Fprintln(bw, block.Content)
			}
			_ = bw.Flush() // Flush after each write
		}
	case "list":
		for _, block := range blocks {
			if !isBlockTypeAllowed(block) {
				continue
			}

			if block.Type == markdown.Code {
				_, _ = fmt.Fprintf(bw, "Language: %s\n", block.Language)
				if s.WithQuotes {
					_, _ = fmt.Fprintf(bw, "```%s\n%s\n```\n", block.Language, block.Content)
				} else {
					_, _ = fmt.Fprintln(bw, block.Content)
				}
			} else {
				_, _ = fmt.Fprintf(bw, "Type: normal\n%s\n", block.Content)
			}
			_, _ = fmt.Fprintln(bw, "---")
			_ = bw.Flush() // Flush after each write
		}
	case "yaml":
		filteredBlocks := make([]markdown.MarkdownBlock, 0)
		for _, block := range blocks {
			if isBlockTypeAllowed(block) {
				filteredBlocks = append(filteredBlocks, block)
			}
		}
		yamlEncoder := yaml.NewEncoder(bw)
		err := yamlEncoder.Encode(filteredBlocks)
		if err != nil {
			return "", err
		}
		_ = bw.Flush() // Flush after YAML encoding
	}

	_ = bw.Flush()
	return buf.String(), nil
}
