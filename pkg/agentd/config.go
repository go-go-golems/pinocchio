// Package agentd is a session-based agent service whose tools run in the
// browser. The service owns the model loop and the conversation (geppetto
// sessions and tool loop); a page registers a tool manifest, sends turns,
// reads events over SSE and posts tool results. Agents are configuration
// files owned by the client application.
//
// The protocol is described in pkg/doc/topics (pinocchio-agentd) and in the
// porting playbook it was designed with (homolog-bench HB-002).
package agentd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that reads "120s" style strings from YAML.
type Duration time.Duration

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

// AgentConfig describes one agent: its instructions, model and limits.
type AgentConfig struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// SystemPrompt is a file path relative to the config file.
	SystemPrompt string `yaml:"systemPrompt" json:"-"`
	// PageInstructions allows the page to append instructions (e.g. an API
	// reference) to the system prompt when it creates a session.
	PageInstructions         bool `yaml:"pageInstructions,omitempty" json:"pageInstructions"`
	MaxPageInstructionsBytes int  `yaml:"maxPageInstructionsBytes,omitempty" json:"maxPageInstructionsBytes,omitempty"`
	// Model is the default model reference (modelsource syntax).
	Model    string `yaml:"model" json:"model"`
	Thinking string `yaml:"thinking,omitempty" json:"thinking,omitempty"`
	// MaxTokens caps one model response; 0 uses the model's limit.
	MaxTokens   int      `yaml:"maxTokens,omitempty" json:"maxTokens,omitempty"`
	MaxRounds   int      `yaml:"maxRounds,omitempty" json:"maxRounds"`
	ToolTimeout Duration `yaml:"toolTimeout,omitempty" json:"-"`
	// PersonTimeout limits a tool call the page marked awaiting-person
	// (an approval the person has to decide).
	PersonTimeout Duration `yaml:"personTimeout,omitempty" json:"-"`
	// ResultCut is the maximum characters of one tool result sent to the model.
	ResultCut int `yaml:"resultCut,omitempty" json:"resultCut"`
	// Tools sets the approval mode per tool name: "auto" (default) or "human".
	Tools map[string]string `yaml:"tools,omitempty" json:"tools,omitempty"`
	// HostTools names the in-process tools of the embedding program this agent offers.
	HostTools []string `yaml:"hostTools,omitempty" json:"hostTools,omitempty"`
	// Budget limits tokens per session (input + output, all rounds); 0 = none.
	Budget struct {
		TokensPerSession int `yaml:"tokensPerSession,omitempty" json:"tokensPerSession,omitempty"`
	} `yaml:"budget,omitempty" json:"budget"`

	systemText string
	path       string
}

// Defaults applied to a loaded config.
const (
	DefaultMaxRounds                = 40
	DefaultToolTimeout              = 120 * time.Second
	DefaultPersonTimeout            = 10 * time.Minute
	DefaultResultCut                = 9000
	DefaultMaxPageInstructionsBytes = 64 << 10
)

// SystemText is the loaded system prompt.
func (c *AgentConfig) SystemText() string { return c.systemText }

// ToolTimeoutDuration returns the per-call tool timeout.
func (c *AgentConfig) ToolTimeoutDuration() time.Duration { return time.Duration(c.ToolTimeout) }

// PersonTimeoutDuration returns the limit of a call awaiting a person's decision.
func (c *AgentConfig) PersonTimeoutDuration() time.Duration { return time.Duration(c.PersonTimeout) }

func (c *AgentConfig) hasHostTool(name string) bool {
	for _, n := range c.HostTools {
		if n == name {
			return true
		}
	}
	return false
}

// ToolMode returns "auto" or "human" for a tool name.
func (c *AgentConfig) ToolMode(name string) string {
	if m := strings.ToLower(strings.TrimSpace(c.Tools[name])); m == "human" {
		return m
	}
	return "auto"
}

// LoadAgentFile reads one agent YAML file and its system prompt.
func LoadAgentFile(path string) (*AgentConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c AgentConfig
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.path = path
	if strings.TrimSpace(c.Name) == "" {
		c.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if c.SystemPrompt != "" {
		p := c.SystemPrompt
		if !filepath.IsAbs(p) {
			p = filepath.Join(filepath.Dir(path), p)
		}
		t, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("%s: system prompt: %w", path, err)
		}
		c.systemText = string(t)
	}
	if c.MaxRounds <= 0 {
		c.MaxRounds = DefaultMaxRounds
	}
	if c.ToolTimeout <= 0 {
		c.ToolTimeout = Duration(DefaultToolTimeout)
	}
	if c.PersonTimeout <= 0 {
		c.PersonTimeout = Duration(DefaultPersonTimeout)
	}
	if c.ResultCut <= 0 {
		c.ResultCut = DefaultResultCut
	}
	if c.MaxPageInstructionsBytes <= 0 {
		c.MaxPageInstructionsBytes = DefaultMaxPageInstructionsBytes
	}
	return &c, nil
}

// LoadAgents reads every *.yaml / *.yml agent file in dir.
func LoadAgents(dir string) (map[string]*AgentConfig, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]*AgentConfig{}
	names := []string{}
	for _, e := range ents {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		c, err := LoadAgentFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if _, dup := out[c.Name]; dup {
			return nil, fmt.Errorf("agent %q is defined twice in %s", c.Name, dir)
		}
		out[c.Name] = c
		names = append(names, c.Name)
	}
	sort.Strings(names)
	if len(out) == 0 {
		return nil, fmt.Errorf("no agent configs (*.yaml) in %s", dir)
	}
	return out, nil
}
