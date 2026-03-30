package agentmode

import (
	"fmt"
	"strings"
)

const (
	ModeSwitchTagPackage = "pinocchio"
	ModeSwitchTagType    = "agent_mode_switch"
	ModeSwitchTagVersion = "v1"
)

var (
	modeSwitchOpenTag  = fmt.Sprintf("<%s:%s:%s>", ModeSwitchTagPackage, ModeSwitchTagType, ModeSwitchTagVersion)
	modeSwitchCloseTag = fmt.Sprintf("</%s:%s:%s>", ModeSwitchTagPackage, ModeSwitchTagType, ModeSwitchTagVersion)
)

type ParseOptions struct {
	SanitizeYAML *bool `json:"sanitize_yaml,omitempty" yaml:"sanitize_yaml,omitempty"`
}

func DefaultParseOptions() ParseOptions {
	return ParseOptions{}.WithSanitizeYAML(true)
}

func (o ParseOptions) withDefaults() ParseOptions {
	if o.SanitizeYAML != nil {
		return o
	}
	return DefaultParseOptions()
}

func (o ParseOptions) SanitizeEnabled() bool {
	o = o.withDefaults()
	return o.SanitizeYAML != nil && *o.SanitizeYAML
}

func (o ParseOptions) WithSanitizeYAML(v bool) ParseOptions {
	ret := o
	ret.SanitizeYAML = new(bool)
	*ret.SanitizeYAML = v
	return ret
}

type ModeSwitchPayload struct {
	ModeSwitch struct {
		Analysis string `yaml:"analysis"`
		NewMode  string `yaml:"new_mode,omitempty"`
	} `yaml:"mode_switch"`
}

type ParsedModeSwitch struct {
	Analysis   string
	NewMode    string
	RawYAML    string
	ParsedYAML string
	Sanitized  bool
	ParseClean bool
}

func BuildModeSwitchInstructions(current string, available []string) string {
	var b strings.Builder
	b.WriteString("<modeSwitchGuidelines>")
	b.WriteString("Analyze the current conversation and determine if a mode switch would be beneficial. ")
	b.WriteString("If a mode switch would help, emit exactly one structured block using the XML-like tag format shown below. ")
	b.WriteString("The structured tag is required. The YAML payload goes inside the tag. ")
	b.WriteString("If the current mode is appropriate, omit the new_mode field.")
	b.WriteString("</modeSwitchGuidelines>\n\n")
	b.WriteString(modeSwitchOpenTag)
	b.WriteString("\n```yaml\n")
	b.WriteString("mode_switch:\n")
	b.WriteString("  analysis: |\n")
	b.WriteString("    • What is the user trying to accomplish?\n")
	b.WriteString("    • What capabilities are needed?\n")
	b.WriteString("    • Is the current mode optimal for this task?\n")
	b.WriteString("    • If switching, what specific benefits would the new mode provide?\n")
	b.WriteString("  new_mode: MODE_NAME  # Only include this if you recommend switching modes\n")
	b.WriteString("```\n")
	b.WriteString(modeSwitchCloseTag)
	b.WriteString("\n\nCurrent mode: ")
	b.WriteString(current)
	if len(available) > 0 {
		b.WriteString("\nAvailable modes: ")
		b.WriteString(strings.Join(available, ", "))
	}
	b.WriteString("\n\nRemember: staying in the current mode is often the right choice.")
	return b.String()
}

// Deprecated: use BuildModeSwitchInstructions.
func BuildYamlModeSwitchInstructions(current string, available []string) string {
	return BuildModeSwitchInstructions(current, available)
}
