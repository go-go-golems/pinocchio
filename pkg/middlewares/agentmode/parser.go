package agentmode

import (
	"errors"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/events/structuredsink/parsehelpers"
	"github.com/go-go-golems/geppetto/pkg/turns"
	yamlsanitize "github.com/go-go-golems/sanitize/pkg/yaml"
	"gopkg.in/yaml.v3"
)

var (
	ErrEmptyModeSwitchPayload = errors.New("empty agentmode payload")
	ErrNoModeSwitchData       = errors.New("agentmode payload contains no analysis or new_mode")
)

func ParseModeSwitchPayload(raw []byte, opts ParseOptions) (*ParsedModeSwitch, error) {
	opts = opts.withDefaults()

	_, body := parsehelpers.StripCodeFenceBytes(raw)
	src := strings.TrimSpace(string(body))
	if src == "" {
		return nil, ErrEmptyModeSwitchPayload
	}

	parsedYAML := src
	sanitized := false
	parseClean := true
	if opts.SanitizeEnabled() {
		result := yamlsanitize.Sanitize(src)
		if trimmed := strings.TrimSpace(result.Sanitized); trimmed != "" {
			parsedYAML = trimmed
		}
		sanitized = parsedYAML != src
		parseClean = result.ParseClean && result.LintClean
	}

	var payload ModeSwitchPayload
	if err := yaml.Unmarshal([]byte(parsedYAML), &payload); err != nil {
		return nil, err
	}

	analysis := strings.TrimSpace(payload.ModeSwitch.Analysis)
	newMode := strings.TrimSpace(payload.ModeSwitch.NewMode)
	if analysis == "" && newMode == "" {
		return nil, ErrNoModeSwitchData
	}

	return &ParsedModeSwitch{
		Analysis:   analysis,
		NewMode:    newMode,
		RawYAML:    src,
		ParsedYAML: parsedYAML,
		Sanitized:  sanitized,
		ParseClean: parseClean,
	}, nil
}

func FindModeSwitchPayloadInText(text string) ([]byte, bool) {
	src := strings.TrimSpace(text)
	if src == "" {
		return nil, false
	}

	closeIdx := strings.LastIndex(src, modeSwitchCloseTag)
	if closeIdx < 0 {
		return nil, false
	}
	openIdx := strings.LastIndex(src[:closeIdx], modeSwitchOpenTag)
	if openIdx < 0 {
		return nil, false
	}

	start := openIdx + len(modeSwitchOpenTag)
	if start > closeIdx {
		return nil, false
	}
	return []byte(src[start:closeIdx]), true
}

func DetectModeSwitch(t *turns.Turn, opts ParseOptions) (*ParsedModeSwitch, bool) {
	if t == nil {
		return nil, false
	}
	return DetectModeSwitchInBlocks(t.Blocks, opts)
}

func DetectModeSwitchInBlocks(blocks []turns.Block, opts ParseOptions) (*ParsedModeSwitch, bool) {
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		if b.Kind != turns.BlockKindLLMText {
			continue
		}
		txt, _ := b.Payload[turns.PayloadKeyText].(string)
		if strings.TrimSpace(txt) == "" {
			continue
		}
		raw, ok := FindModeSwitchPayloadInText(txt)
		if !ok {
			continue
		}
		parsed, err := ParseModeSwitchPayload(raw, opts)
		if err != nil {
			continue
		}
		return parsed, true
	}
	return nil, false
}

// Deprecated: use DetectModeSwitch.
func DetectYamlModeSwitch(t *turns.Turn) (string, string) {
	parsed, ok := DetectModeSwitch(t, DefaultParseOptions())
	if !ok {
		return "", ""
	}
	return parsed.NewMode, parsed.Analysis
}

// Deprecated: use DetectModeSwitchInBlocks.
func DetectYamlModeSwitchInBlocks(blocks []turns.Block) (string, string) {
	parsed, ok := DetectModeSwitchInBlocks(blocks, DefaultParseOptions())
	if !ok {
		return "", ""
	}
	return parsed.NewMode, parsed.Analysis
}
