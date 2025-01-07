package serve

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/pinocchio/cmd/examples/eval/eval"
	"gopkg.in/yaml.v3"
)

type WebViewCommand struct {
	*cmds.CommandDescription
}

type WebViewSettings struct {
	InputFile string `glazed.parameter:"input"`
	Port      string `glazed.parameter:"port"`
}

func (c *WebViewCommand) Run(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
) error {
	s := &WebViewSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	// Read and parse the YAML file
	data, err := os.ReadFile(s.InputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Split the YAML documents
	docs := strings.Split(string(data), "---\n")
	var outputs []eval.TestOutput

	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var output eval.TestOutput
		if err := yaml.Unmarshal([]byte(doc), &output); err != nil {
			return fmt.Errorf("failed to parse YAML document: %w", err)
		}
		outputs = append(outputs, output)
	}

	// Convert outputs to renderable format
	renderableOutputs := ConvertOutputs(outputs)

	// Start the web server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := page(renderableOutputs).Render(ctx, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	fmt.Printf("Starting web server on port %s...\n", s.Port)
	return http.ListenAndServe(":"+s.Port, nil)
}

func NewWebViewCommand() (*WebViewCommand, error) {
	return &WebViewCommand{
		CommandDescription: cmds.NewCommandDescription(
			"web-view",
			cmds.WithShort("Start a web server to view test results"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"input",
					parameters.ParameterTypeString,
					parameters.WithHelp("Path to the test output YAML file"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"port",
					parameters.ParameterTypeString,
					parameters.WithHelp("Port to run the web server on"),
					parameters.WithDefault("8080"),
				),
			),
		),
	}, nil
}
