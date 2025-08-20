package cmds

import (
	"io"
	"io/fs"
	"strings"

	geppettolayers "github.com/go-go-golems/geppetto/pkg/layers"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/alias"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/loaders"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type PinocchioCommandLoader struct {
}

func (g *PinocchioCommandLoader) IsFileSupported(f fs.FS, fileName string) bool {
	return strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml")
}

var _ loaders.CommandLoader = (*PinocchioCommandLoader)(nil)

// LoadFromYAML loads Pinocchio commands from YAML content with additional options
func LoadFromYAML(b []byte, options ...cmds.CommandDescriptionOption) ([]cmds.Command, error) {
	loader := &PinocchioCommandLoader{}
	buf := strings.NewReader(string(b))
	return loader.loadPinocchioCommandFromReader(buf, options, nil)
}

func (g *PinocchioCommandLoader) loadPinocchioCommandFromReader(
	s io.Reader,
	options []cmds.CommandDescriptionOption,
	_ []alias.Option,
) ([]cmds.Command, error) {
	yamlContent, err := io.ReadAll(s)
	if err != nil {
		return nil, err
	}

	buf := strings.NewReader(string(yamlContent))
	scd := &PinocchioCommandDescription{
		Flags:     []*parameters.ParameterDefinition{},
		Arguments: []*parameters.ParameterDefinition{},
	}
	err = yaml.NewDecoder(buf).Decode(scd)
	if err != nil {
		return nil, err
	}

	if scd.Type == "" {
		scd.Type = "pinocchio"
	}

	// TODO(manuel, 2023-01-27): There has to be a better way to parse YAML factories
	// maybe the easiest is just going to be to make them a separate file in the bundle format, really
	// rewind to read the factories...
	buf = strings.NewReader(string(yamlContent))
	stepSettings, err := settings.NewStepSettingsFromYAML(buf)
	if err != nil {
		return nil, err
	}
	ls, err := geppettolayers.CreateGeppettoLayers(geppettolayers.WithDefaultsFromStepSettings(stepSettings))
	if err != nil {
		return nil, err
	}
	// Wrap with pinocchio helper layer
	helpersLayer, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		return nil, err
	}
	ls = append([]layers.ParameterLayer{helpersLayer}, ls...)

	options_ := []cmds.CommandDescriptionOption{
		cmds.WithShort(scd.Short),
		cmds.WithLong(scd.Long),
		cmds.WithFlags(scd.Flags...),
		cmds.WithArguments(scd.Arguments...),
		cmds.WithLayersList(ls...),
		cmds.WithType(scd.Type),
		cmds.WithTags(scd.Tags...),
		cmds.WithMetadata(scd.Metadata),
	}

	description := cmds.NewCommandDescription(
		scd.Name,
		options_...,
	)
	if scd.Prompt != "" && len(scd.Messages) != 0 {
		return nil, errors.Errorf("Prompt and messages are mutually exclusive")
	}

	// Convert simple messages to user blocks (llm content)
	blocks := make([]turns.Block, 0, len(scd.Messages))
	for _, text := range scd.Messages {
		if strings.TrimSpace(text) == "" {
			continue
		}
		blocks = append(blocks, turns.NewUserTextBlock(text))
	}

	sq, err := NewPinocchioCommand(
		description,
		WithPrompt(scd.Prompt),
		WithBlocks(blocks),
		WithSystemPrompt(scd.SystemPrompt),
	)
	if err != nil {
		return nil, err
	}

	// Apply additional options to the command
	for _, option := range options {
		option(sq.Description())
	}

	return []cmds.Command{sq}, nil
}

func (scl *PinocchioCommandLoader) LoadCommands(
	f fs.FS, entryName string,
	options []cmds.CommandDescriptionOption,
	aliasOptions []alias.Option,
) ([]cmds.Command, error) {
	r, err := f.Open(entryName)
	if err != nil {
		return nil, err
	}
	defer func(r fs.File) {
		_ = r.Close()
	}(r)

	// Add source tracking option
	sourceOption := cmds.WithSource("file:" + entryName)
	allOptions := append(options, sourceOption)

	return loaders.LoadCommandOrAliasFromReader(
		r,
		scl.loadPinocchioCommandFromReader,
		allOptions,
		aliasOptions)
}
