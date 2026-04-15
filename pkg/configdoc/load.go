package configdoc

import (
	"bytes"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func LoadDocument(path string) (*Document, error) {
	if err := ValidateLocalOverrideFileName(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read config document")
	}
	return DecodeDocument(data)
}

func DecodeDocument(data []byte) (*Document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	doc := &Document{}
	if err := decoder.Decode(doc); err != nil {
		return nil, errors.Wrap(err, "decode config document")
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, errors.Wrap(err, "decode config document structure")
	}
	annotatePresence(doc, &root)

	if err := doc.NormalizeAndValidate(); err != nil {
		return nil, err
	}
	return doc, nil
}

func annotatePresence(doc *Document, root *yaml.Node) {
	if doc == nil || root == nil || len(root.Content) == 0 {
		return
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		value := mapping.Content[i+1]
		switch key.Value {
		case "app":
			annotateAppPresence(&doc.App, value)
		case "profile":
			annotateProfilePresence(&doc.Profile, value)
		case "profiles":
			annotateProfilesPresence(doc.Profiles, value)
		}
	}
}

func annotateAppPresence(app *AppBlock, node *yaml.Node) {
	if app == nil || node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "repositories" {
			app.hasRepositories = true
		}
	}
}

func annotateProfilePresence(profile *ProfileBlock, node *yaml.Node) {
	if profile == nil || node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "active":
			profile.hasActive = true
		case "registries":
			profile.hasRegistries = true
		}
	}
}

func annotateProfilesPresence(profiles map[string]*InlineProfile, node *yaml.Node) {
	if len(profiles) == 0 || node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		rawSlug := node.Content[i].Value
		profileNode := node.Content[i+1]
		profile := profiles[rawSlug]
		if profile == nil {
			continue
		}
		annotateInlineProfilePresence(profile, profileNode)
	}
}

func annotateInlineProfilePresence(profile *InlineProfile, node *yaml.Node) {
	if profile == nil || node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "display_name":
			profile.hasDisplayName = true
		case "description":
			profile.hasDescription = true
		case "stack":
			profile.hasStack = true
		case "inference_settings":
			profile.hasInferenceSettings = true
		case "extensions":
			profile.hasExtensions = true
		}
	}
}
