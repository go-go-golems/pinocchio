// Package presets reads agent presets: YAML files that name a model, a
// system prompt file and run limits. A preset is data for the application
// that builds a run's runtime; chatapp never reads presets.
//
//	name: coder
//	description: Writes and edits code in the page
//	model: anthropic/claude-sonnet-4-5     # a modelsource reference
//	thinking: medium
//	maxTokens: 8000
//	systemPrompt: coder.md                 # relative to this file
//	maxRounds: 40
//	toolTimeout: 120s
//	resultCut: 9000
//	tools: [read_file, write_file]         # in-process tools the application offers
//	budget:
//	  tokensPerSession: 2000000
//	room: main                             # unknown keys land in Extra
//
// Zero values mean "not set": the application applies its own defaults.
package presets

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration read from strings such as "120s".
type Duration time.Duration

// UnmarshalYAML parses a Go duration string.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	v, err := time.ParseDuration(strings.TrimSpace(s))
	if err != nil {
		return fmt.Errorf("duration %q: %w", s, err)
	}
	*d = Duration(v)
	return nil
}

// Preset is one agent preset.
type Preset struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Model is a modelsource reference, e.g. "anthropic/claude-sonnet-4-5".
	Model    string `yaml:"model,omitempty" json:"model,omitempty"`
	Thinking string `yaml:"thinking,omitempty" json:"thinking,omitempty"`
	// MaxTokens caps one model response.
	MaxTokens int `yaml:"maxTokens,omitempty" json:"maxTokens,omitempty"`
	// SystemPrompt is a file path, relative to the preset file; see SystemText.
	SystemPrompt string `yaml:"systemPrompt,omitempty" json:"-"`
	// MaxRounds is the tool loop's round limit.
	MaxRounds int `yaml:"maxRounds,omitempty" json:"maxRounds,omitempty"`
	// ToolTimeout is a per-call limit (toolexec.Deadline).
	ToolTimeout Duration `yaml:"toolTimeout,omitempty" json:"-"`
	// ResultCut caps one tool result in characters (toolexec.CutResults).
	ResultCut int `yaml:"resultCut,omitempty" json:"resultCut,omitempty"`
	// Tools names the in-process tools the application offers this preset.
	Tools  []string `yaml:"tools,omitempty" json:"tools,omitempty"`
	Budget struct {
		// TokensPerSession is a session's token budget (budget.SessionTokens).
		TokensPerSession int `yaml:"tokensPerSession,omitempty" json:"tokensPerSession,omitempty"`
	} `yaml:"budget,omitempty" json:"budget"`
	// Extra holds the keys this type does not know, for the application.
	Extra map[string]any `yaml:",inline" json:"-"`

	systemText string
	path       string
}

// SystemText is the content of the SystemPrompt file ("" without one).
func (p *Preset) SystemText() string { return p.systemText }

// Path is the file the preset was read from.
func (p *Preset) Path() string { return p.path }

// ToolTimeoutDuration is ToolTimeout as a time.Duration.
func (p *Preset) ToolTimeoutDuration() time.Duration { return time.Duration(p.ToolTimeout) }

// LoadFile reads one preset and its system prompt. A preset without a name
// is named after its file.
func LoadFile(path string) (*Preset, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Preset
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	p.path = path
	if strings.TrimSpace(p.Name) == "" {
		p.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if p.SystemPrompt != "" {
		sp := p.SystemPrompt
		if !filepath.IsAbs(sp) {
			sp = filepath.Join(filepath.Dir(path), sp)
		}
		text, err := os.ReadFile(sp)
		if err != nil {
			return nil, fmt.Errorf("%s: system prompt: %w", path, err)
		}
		p.systemText = string(text)
	}
	return &p, nil
}

// Load reads every *.yaml and *.yml file in dir. Two presets with one name
// are an error, and so is a directory without presets.
func Load(dir string) (map[string]*Preset, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]*Preset{}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		p, err := LoadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if prev, dup := out[p.Name]; dup {
			return nil, fmt.Errorf("preset %q is defined in %s and %s", p.Name, prev.path, p.path)
		}
		out[p.Name] = p
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no presets (*.yaml) in %s", dir)
	}
	return out, nil
}

// Names returns the preset names, sorted.
func Names(presets map[string]*Preset) []string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
