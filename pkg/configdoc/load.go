package configdoc

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

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
	return DecodeDocumentWithSource(path, data)
}

func DecodeDocument(data []byte) (*Document, error) {
	return DecodeDocumentWithSource("", data)
}

func DecodeDocumentWithSource(source string, data []byte) (*Document, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, wrapConfigDocumentError(source, errors.Wrap(err, "decode config document structure"))
	}
	if err := validateTopLevelKeys(source, &root); err != nil {
		return nil, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	doc := &Document{}
	if err := decoder.Decode(doc); err != nil {
		return nil, wrapConfigDocumentError(source, errors.Wrap(err, "decode config document"))
	}
	annotatePresence(doc, &root)

	if err := doc.NormalizeAndValidate(); err != nil {
		return nil, wrapConfigDocumentError(source, err)
	}
	return doc, nil
}

func wrapConfigDocumentError(source string, err error) error {
	if err == nil {
		return nil
	}
	if strings.TrimSpace(source) == "" {
		return err
	}
	return errors.Wrapf(err, "config document %s", source)
}

func validateTopLevelKeys(source string, root *yaml.Node) error {
	if root == nil || len(root.Content) == 0 {
		return nil
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return wrapConfigDocumentError(source, errors.New("top-level YAML document must be a mapping with supported keys app, profile, or profiles"))
	}

	type keyFinding struct {
		line   int
		key    string
		reason string
		legacy bool
	}

	supported := map[string]struct{}{
		"app":      {},
		"profile":  {},
		"profiles": {},
	}
	legacyReasons := map[string]string{
		"profile-settings": "rewrite to profile.active and profile.registries",
		"repositories":     "move repository entries to app.repositories",
		"ai-chat":          "move runtime settings into profiles.<slug>.inference_settings or an engine-only profiles.yaml",
		"openai-chat":      "move runtime settings into profiles.<slug>.inference_settings or an engine-only profiles.yaml",
		"claude-chat":      "move runtime settings into profiles.<slug>.inference_settings or an engine-only profiles.yaml",
		"gemini-chat":      "move runtime settings into profiles.<slug>.inference_settings or an engine-only profiles.yaml",
		"ai-client":        "keep shared provider/client settings in environment variables or move runtime defaults into profiles.<slug>.inference_settings where appropriate",
	}

	findings := []keyFinding{}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		key := strings.TrimSpace(keyNode.Value)
		if key == "" {
			continue
		}
		if _, ok := supported[key]; ok {
			continue
		}
		if reason, ok := legacyReasons[key]; ok {
			findings = append(findings, keyFinding{line: keyNode.Line, key: key, reason: reason, legacy: true})
			continue
		}
		findings = append(findings, keyFinding{line: keyNode.Line, key: key, reason: "unsupported top-level key; supported keys are app, profile, and profiles"})
	}
	if len(findings) == 0 {
		return nil
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].line == findings[j].line {
			return findings[i].key < findings[j].key
		}
		return findings[i].line < findings[j].line
	})

	var b strings.Builder
	b.WriteString("this file was discovered by Pinocchio config resolution and is being decoded as a unified config document (standard config location, local .pinocchio.yml, or --config-file), but it contains unsupported legacy top-level keys:\n")
	for _, finding := range findings {
		if finding.legacy {
			fmt.Fprintf(&b, "  - line %d: %s (legacy key; %s)\n", finding.line, finding.key, finding.reason)
		} else {
			fmt.Fprintf(&b, "  - line %d: %s (%s)\n", finding.line, finding.key, finding.reason)
		}
	}
	b.WriteString("supported top-level keys are: app, profile, profiles\n")
	b.WriteString("Pinocchio is not trying to find or require these legacy keys; it is rejecting them because they belong to the removed pre-unified config shape\n")
	b.WriteString("these keys are not treated as optional or ignored: Pinocchio decodes this document strictly so stale config files fail loudly instead of being partially ignored")
	return wrapConfigDocumentError(source, errors.New(strings.TrimSpace(b.String())))
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
