package profiles

import (
	"fmt"
	"os"
	"path/filepath"

	yaml_editor "github.com/go-go-golems/clay/pkg/yaml-editor"
	"github.com/rs/zerolog/log"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"gopkg.in/yaml.v3"
)

type ProfileName = string
type LayerName = string
type SettingName = string
type SettingValue = string

type LayerSettings = *orderedmap.OrderedMap[SettingName, SettingValue]
type ProfileLayers = *orderedmap.OrderedMap[LayerName, LayerSettings]
type Profiles = *orderedmap.OrderedMap[ProfileName, ProfileLayers]

type ProfilesEditor struct {
	editor *yaml_editor.YAMLEditor
	path   string
}

func NewProfilesEditor(path string) (*ProfilesEditor, error) {
	log.Debug().Msgf("Creating profiles editor for path: %s", path)
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

func (p *ProfilesEditor) ListProfiles() ([]ProfileName, map[ProfileName]map[LayerName]map[SettingName]SettingValue, error) {
	root, err := p.editor.GetNode()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get root node: %w", err)
	}

	if root.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("root node is not a mapping")
	}

	profiles := make([]ProfileName, 0)
	profileContents := make(map[ProfileName]map[LayerName]map[SettingName]SettingValue)

	for i := 0; i < len(root.Content); i += 2 {
		profileName := root.Content[i].Value
		profiles = append(profiles, profileName)

		// Get the full content for each profile
		layers, err := p.GetProfileLayers(profileName)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get layers for profile %s: %w", profileName, err)
		}

		// Convert the ordered maps to regular maps for backwards compatibility
		profileContents[profileName] = make(map[LayerName]map[SettingName]SettingValue)
		for pair := layers.Oldest(); pair != nil; pair = pair.Next() {
			layerName := pair.Key
			settings := pair.Value

			profileContents[profileName][layerName] = make(map[SettingName]SettingValue)
			for settingPair := settings.Oldest(); settingPair != nil; settingPair = settingPair.Next() {
				profileContents[profileName][layerName][settingPair.Key] = settingPair.Value
			}
		}
	}

	return profiles, profileContents, nil
}

func (p *ProfilesEditor) GetProfileLayers(profile ProfileName) (ProfileLayers, error) {
	profileNode, err := p.editor.GetNode(profile)
	if err != nil {
		return nil, fmt.Errorf("could not get profile: %w", err)
	}

	if profileNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("profile node is not a mapping")
	}

	layers := orderedmap.New[LayerName, LayerSettings]()
	for i := 0; i < len(profileNode.Content); i += 2 {
		layerName := profileNode.Content[i].Value
		layerNode := profileNode.Content[i+1]

		if layerNode.Kind != yaml.MappingNode {
			continue
		}

		settings := orderedmap.New[SettingName, SettingValue]()
		for j := 0; j < len(layerNode.Content); j += 2 {
			key := layerNode.Content[j].Value
			value := layerNode.Content[j+1].Value
			settings.Set(key, value)
		}

		layers.Set(layerName, settings)
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
