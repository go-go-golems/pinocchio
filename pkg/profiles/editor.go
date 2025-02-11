package profiles

import (
	"fmt"
	"os"
	"path/filepath"

	yaml_editor "github.com/go-go-golems/clay/pkg/yaml-editor"
	"gopkg.in/yaml.v3"
)

type ProfilesEditor struct {
	editor *yaml_editor.YAMLEditor
	path   string
}

func NewProfilesEditor(path string) (*ProfilesEditor, error) {
	editor, err := yaml_editor.NewYAMLEditorFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not create editor: %w", err)
	}

	return &ProfilesEditor{
		editor: editor,
		path:   path,
	}, nil
}

func (p *ProfilesEditor) Save() error {
	return p.editor.Save(p.path)
}

func (p *ProfilesEditor) SetLayerValue(profile, layer, key, value string) error {
	valueNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: value,
	}
	return p.editor.SetNode(valueNode, profile, layer, key)
}

func (p *ProfilesEditor) GetLayerValue(profile, layer, key string) (string, error) {
	node, err := p.editor.GetNode(profile, layer, key)
	if err != nil {
		return "", fmt.Errorf("could not get value: %w", err)
	}
	return node.Value, nil
}

func (p *ProfilesEditor) DeleteProfile(profile string) error {
	return p.editor.SetNode(nil, profile)
}

func (p *ProfilesEditor) DeleteLayerValue(profile, layer, key string) error {
	return p.editor.SetNode(nil, profile, layer, key)
}

func (p *ProfilesEditor) ListProfiles() ([]string, error) {
	root, err := p.editor.GetNode()
	if err != nil {
		return nil, fmt.Errorf("could not get root node: %w", err)
	}

	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("root node is not a mapping")
	}

	profiles := make([]string, 0)
	for i := 0; i < len(root.Content); i += 2 {
		profiles = append(profiles, root.Content[i].Value)
	}

	return profiles, nil
}

func (p *ProfilesEditor) GetProfileLayers(profile string) (map[string]map[string]string, error) {
	profileNode, err := p.editor.GetNode(profile)
	if err != nil {
		return nil, fmt.Errorf("could not get profile: %w", err)
	}

	if profileNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("profile node is not a mapping")
	}

	layers := make(map[string]map[string]string)
	for i := 0; i < len(profileNode.Content); i += 2 {
		layerName := profileNode.Content[i].Value
		layerNode := profileNode.Content[i+1]

		if layerNode.Kind != yaml.MappingNode {
			continue
		}

		settings := make(map[string]string)
		for j := 0; j < len(layerNode.Content); j += 2 {
			key := layerNode.Content[j].Value
			value := layerNode.Content[j+1].Value
			settings[key] = value
		}

		layers[layerName] = settings
	}

	return layers, nil
}

func GetDefaultProfilesPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not get config dir: %w", err)
	}

	return filepath.Join(configDir, "pinocchio", "profiles.yaml"), nil
}
