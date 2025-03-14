package cmds

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/weaviate/tiktoken-go"
)

type ClipSettings struct {
	Stats   bool     `glazed.parameter:"stats"`
	Preview bool     `glazed.parameter:"preview"`
	Args    []string `glazed.parameter:"__args"`
}

type ClipCommand struct {
	*cmds.CommandDescription
}

var _ cmds.BareCommand = &ClipCommand{}

func NewClipCommand() (*ClipCommand, error) {
	return &ClipCommand{
		CommandDescription: cmds.NewCommandDescription(
			"clip",
			cmds.WithShort("Copy command output to clipboard"),
			cmds.WithLong("Execute a command and copy its output to the clipboard, with optional statistics and preview"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"stats",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Show statistics about the output"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"preview",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Preview the content in $PAGER"),
					parameters.WithDefault(false),
				),
			),
		),
	}, nil
}

func (c *ClipCommand) Run(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
) error {
	s := &ClipSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return fmt.Errorf("error initializing settings: %w", err)
	}

	var outputStr string
	if len(s.Args) == 0 {
		// Read from stdin when no arguments provided
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("error reading from stdin: %v", err)
		}
		outputStr = string(input)
	} else {
		// Create command from arguments
		cmd := exec.Command(s.Args[0], s.Args[1:]...)

		// Capture both stdout and stderr
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output

		// Run the command
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("error executing command: %v", err)
		}

		outputStr = output.String()
	}

	// Copy to clipboard
	err = clipboard.WriteAll(outputStr)
	if err != nil {
		return fmt.Errorf("error copying to clipboard: %v", err)
	}

	// Show stats if requested
	if s.Stats {
		printStats(outputStr)
	}

	// Preview if requested
	if s.Preview {
		previewInPager(outputStr)
	}

	return nil
}

func printStats(content string) {
	// Initialize token counter
	tokenCounter, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing token counter: %v\n", err)
		return
	}

	// Count tokens
	tokens := tokenCounter.Encode(content, nil, nil)
	tokenCount := len(tokens)

	// Count lines
	lineCount := strings.Count(content, "\n") + 1

	// Get size in bytes
	size := len(content)

	fmt.Printf("Statistics:\n")
	fmt.Printf("  Tokens: %d\n", tokenCount)
	fmt.Printf("  Lines:  %d\n", lineCount)
	fmt.Printf("  Size:   %d bytes\n", size)
}

func previewInPager(content string) {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less" // Default to less if $PAGER is not set
	}

	cmd := exec.Command(pager)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running pager: %v\n", err)
	}
}
