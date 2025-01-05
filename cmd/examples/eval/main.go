package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	clay "github.com/go-go-golems/clay/pkg"
	pinocchio_settings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	glazed_settings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
)

type EvalCommand struct {
	*cmds.CommandDescription
}

type EvalSettings struct {
	Dataset string `glazed.parameter:"dataset"`
	Command string `glazed.parameter:"command"`
}

// Add these new structs for the eval dataset
type EvalEntry struct {
	Input        map[string]interface{} `json:"input"`
	GoldenAnswer interface{}            `json:"golden_answer"`
}

type EvalDataset []EvalEntry

func (c *EvalCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &EvalSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	// Read and parse the dataset file
	datasetBytes, err := os.ReadFile(s.Dataset)
	if err != nil {
		return fmt.Errorf("failed to read dataset file: %w", err)
	}

	var dataset EvalDataset
	if err := json.Unmarshal(datasetBytes, &dataset); err != nil {
		return fmt.Errorf("failed to parse dataset JSON: %w", err)
	}

	// Load the command from the YAML file
	commandBytes, err := os.ReadFile(s.Command)
	if err != nil {
		return fmt.Errorf("failed to read command file: %w", err)
	}

	commands, err := pinocchio_cmds.LoadFromYAML(commandBytes)
	if err != nil {
		return fmt.Errorf("failed to parse command YAML: %w", err)
	}

	if len(commands) != 1 {
		return fmt.Errorf("expected exactly one command in YAML, got %d", len(commands))
	}

	command, ok := commands[0].(*pinocchio_cmds.PinocchioCommand)
	if !ok {
		return fmt.Errorf("expected the command to be a PinocchioCommand, got %T", commands[0])
	}

	// Update step settings from parsed layers
	stepSettings := command.StepSettings.Clone()
	err = stepSettings.UpdateFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	// Process each entry in the dataset
	for i, entry := range dataset {
		// Create the conversation manager for this entry
		manager, err := command.CreateConversationManager(entry.Input)
		if err != nil {
			return fmt.Errorf("failed to create conversation manager for entry %d: %w", i+1, err)
		}

		// Run the command with the manager and settings
		resultConversation, err := command.RunWithOptions(ctx,
			run.WithManager(manager),
			run.WithStepSettings(stepSettings),
			run.WithRunMode(run.RunModeBlocking),
			run.WithWriter(os.Stdout),
		)
		if err != nil {
			return fmt.Errorf("failed to run command for entry %d: %w", i+1, err)
		}

		// Get the conversation string
		var conversationString string
		for _, msg := range manager.GetConversation() {
			conversationString += msg.Content.View() + "\n"
		}

		// Get the last message
		lastMessage := resultConversation[len(resultConversation)-1]

		// Create a row with the entry data and AI response
		row := types.NewRow(
			types.MRP("entry_id", i+1),
			types.MRP("input", entry.Input),
			types.MRP("golden_answer", entry.GoldenAnswer),
			types.MRP("conversationString", conversationString),
			types.MRP("conversation", manager.GetConversation()),
			types.MRP("last_message", lastMessage.Content.View()),
			types.MRP("message_metadata", lastMessage.Metadata),
		)

		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

func NewEvalCommand() (*EvalCommand, error) {
	glazedParameterLayer, err := glazed_settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, err
	}

	stepSettings, err := pinocchio_settings.NewStepSettings()
	if err != nil {
		return nil, err
	}

	chatParameterLayer, err := pinocchio_settings.NewChatParameterLayer(
		layers.WithDefaults(stepSettings.Chat),
	)
	if err != nil {
		return nil, err
	}

	clientParameterLayer, err := pinocchio_settings.NewClientParameterLayer(
		layers.WithDefaults(stepSettings.Client),
	)
	if err != nil {
		return nil, err
	}

	claudeParameterLayer, err := claude.NewParameterLayer(
		layers.WithDefaults(stepSettings.Claude),
	)
	if err != nil {
		return nil, err
	}
	openaiParameterLayer, err := openai.NewParameterLayer(
		layers.WithDefaults(stepSettings.OpenAI),
	)
	if err != nil {
		return nil, err
	}

	embeddingsParameterLayer, err := pinocchio_settings.NewEmbeddingsParameterLayer(
		layers.WithDefaults(stepSettings.Embeddings),
	)
	if err != nil {
		return nil, err
	}

	return &EvalCommand{
		CommandDescription: cmds.NewCommandDescription(
			"eval",
			cmds.WithShort("Evaluate prompts against a dataset"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"dataset",
					parameters.ParameterTypeString,
					parameters.WithHelp("Path to the eval dataset JSON file"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"command",
					parameters.ParameterTypeString,
					parameters.WithHelp("Path to the prompt template YAML file"),
					parameters.WithRequired(true),
				),
			),
			cmds.WithLayersList(glazedParameterLayer),
			cmds.WithLayersList(chatParameterLayer, clientParameterLayer, claudeParameterLayer, openaiParameterLayer, embeddingsParameterLayer),
		),
	}, nil
}

// Add these new types for web view
type WebViewCommand struct {
	*cmds.CommandDescription
}

type WebViewSettings struct {
	InputFile string `glazed.parameter:"input"`
	Port      string `glazed.parameter:"port"`
}

type TestOutput struct {
	ConversationString string                 `yaml:"conversationString"`
	EntryID            int                    `yaml:"entry_id"`
	GoldenAnswer       []string               `yaml:"golden_answer"`
	Input              map[string]interface{} `yaml:"input"`
	LastMessage        string                 `yaml:"last_message"`
	MessageMetadata    map[string]interface{} `yaml:"message_metadata"`
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
	var outputs []TestOutput

	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var output TestOutput
		if err := yaml.Unmarshal([]byte(doc), &output); err != nil {
			return fmt.Errorf("failed to parse YAML document: %w", err)
		}
		outputs = append(outputs, output)
	}

	// Create and parse the template
	tmpl := `
<!DOCTYPE html>
<html>
<head>
	<title>Story Generation Results</title>
	<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css" rel="stylesheet">
	<script src="https://unpkg.com/htmx.org@1.9.10"></script>
	<style>
		.story-card { margin-bottom: 2rem; }
		.keywords { color: #666; }
		.story-text { font-size: 1.1rem; line-height: 1.6; }
	</style>
</head>
<body>
	<div class="container mt-4">
		<h1 class="mb-4">Story Generation Results</h1>
		
		<div class="row">
			{{ range . }}
			<div class="col-12 story-card">
				<div class="card">
					<div class="card-body">
						<h5 class="card-title">Story #{{ .EntryID }}</h5>
						<div class="mb-3">
							<strong>Topic:</strong> {{ index .Input "topic" }}<br>
							<strong>Age:</strong> {{ index .Input "age" }}<br>
							<strong>Moral:</strong> {{ index .Input "moral" }}
						</div>
						
						<div class="story-text mb-3">
							{{ .LastMessage }}
						</div>
						
						<div class="keywords mb-3">
							<strong>Expected Keywords:</strong>
							{{ range .GoldenAnswer }}
								<span class="badge bg-secondary me-1">{{ . }}</span>
							{{ end }}
						</div>
						
						<button class="btn btn-sm btn-primary" 
								hx-get="#" 
								hx-target="#conversation-{{ .EntryID }}"
								onclick="toggleConversation({{ .EntryID }})">
							Show Full Conversation
						</button>
						
						<div id="conversation-{{ .EntryID }}" class="mt-3" style="display: none;">
							<pre class="bg-light p-3"><code>{{ .ConversationString }}</code></pre>
						</div>
					</div>
				</div>
			</div>
			{{ end }}
		</div>
	</div>

	<script>
		function toggleConversation(id) {
			const conv = document.getElementById('conversation-' + id);
			if (conv.style.display === 'none') {
				conv.style.display = 'block';
			} else {
				conv.style.display = 'none';
			}
		}
	</script>
</body>
</html>
`

	t, err := template.New("webpage").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Start the web server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := t.Execute(w, outputs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	fmt.Printf("Starting web server on port %s...\n", s.Port)
	return http.ListenAndServe(":"+s.Port, nil)
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "eval",
		Short: "Evaluate prompts against a dataset",
	}

	err := clay.InitViper("pinocchio", rootCmd)
	cobra.CheckErr(err)
	err = clay.InitLogger()
	cobra.CheckErr(err)

	helpSystem := help.NewHelpSystem()

	helpSystem.SetupCobraRootCommand(rootCmd)
	evalCmd, err := NewEvalCommand()
	if err != nil {
		panic(err)
	}

	webViewCmd, err := NewWebViewCommand()
	if err != nil {
		panic(err)
	}

	cli.AddCommandsToRootCommand(
		rootCmd,
		[]cmds.Command{evalCmd, webViewCmd},
		nil,
		cli.WithCobraMiddlewaresFunc(pinocchio_cmds.GetCobraCommandGeppettoMiddlewares),
		cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
