// Package agentcli holds the command-line pieces shared by programs that run
// an agent service: the model-source flag sections, the Pi environment source
// and model loading. `pinocchio agent serve` uses it, and so can a program
// that embeds agentd.Server.
package agentcli

import (
	"context"
	"fmt"

	"github.com/go-go-golems/geppetto/pkg/inference/modelsource"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

// Section slugs. The Pi section's prefix makes its one field read Pi's own
// environment variable, PI_CODING_AGENT_DIR (see PiEnvMiddleware).
const (
	ModelsSlug = "agent-models"
	PiSlug     = "pi"
	piPrefix   = "pi-coding-agent-"
)

type modelSettings struct {
	ProfileRegistries []string `glazed:"profile-registries"`
	AppName           string   `glazed:"app-name"`
	NoPi              bool     `glazed:"no-pi"`
	NoProfiles        bool     `glazed:"no-profiles"`
}

type piSettings struct {
	Dir string `glazed:"dir"`
}

// ModelSections returns the "Model sources" and "Pi coding agent" sections.
// appName is the default for --app-name (where profiles.yaml is looked up).
func ModelSections(appName string) ([]schema.Section, error) {
	models, err := schema.NewSection(ModelsSlug, "Model sources",
		schema.WithFields(
			fields.New("profile-registries", fields.TypeStringList,
				fields.WithHelp("Engine profile registries (yaml:PATH, sqlite:PATH); default <config dir>/<app-name>/profiles.yaml")),
			fields.New("app-name", fields.TypeString, fields.WithDefault(appName),
				fields.WithHelp("Application name for the default profiles.yaml location")),
			fields.New("no-pi", fields.TypeBool, fields.WithDefault(false),
				fields.WithHelp("Do not load models from Pi's agent directory")),
			fields.New("no-profiles", fields.TypeBool, fields.WithDefault(false),
				fields.WithHelp("Do not load engine profiles")),
		))
	if err != nil {
		return nil, err
	}
	pi, err := schema.NewSection(PiSlug, "Pi coding agent",
		schema.WithPrefix(piPrefix),
		schema.WithFields(
			fields.New("dir", fields.TypeString,
				fields.WithHelp("Pi agent directory (env PI_CODING_AGENT_DIR; default ~/.pi/agent)")),
		))
	if err != nil {
		return nil, err
	}
	return []schema.Section{models, pi}, nil
}

// WithPiEnv inserts PiEnvMiddleware after the first two sources of a standard
// chain (flags, arguments), so flags still win over PI_CODING_AGENT_DIR.
func WithPiEnv(base []sources.Middleware) []sources.Middleware {
	if len(base) < 2 {
		return append(base, PiEnvMiddleware())
	}
	out := append([]sources.Middleware{}, base[:2]...)
	out = append(out, PiEnvMiddleware())
	return append(out, base[2:]...)
}

// PiEnvMiddleware applies Glazed's env source without an app prefix to the Pi
// section alone: its field `dir` with prefix `pi-coding-agent-` reads
// PI_CODING_AGENT_DIR, and no other section reads unprefixed variables.
func PiEnvMiddleware() sources.Middleware {
	return func(next sources.HandlerFunc) sources.HandlerFunc {
		return func(s *schema.Schema, parsed *values.Values) error {
			if err := next(s, parsed); err != nil {
				return err
			}
			sec, ok := s.Get(PiSlug)
			if !ok {
				return nil
			}
			only := schema.NewSchema(schema.WithSections(sec))
			identity := func(*schema.Schema, *values.Values) error { return nil }
			return sources.FromEnv("", fields.WithSource("env"))(identity)(only, parsed)
		}
	}
}

// DecodeModelOptions reads the two sections into modelsource load options.
func DecodeModelOptions(parsed *values.Values) (modelsource.LoadOptions, error) {
	m := &modelSettings{}
	if err := parsed.DecodeSectionInto(ModelsSlug, m); err != nil {
		return modelsource.LoadOptions{}, fmt.Errorf("decode model source settings: %w", err)
	}
	p := &piSettings{}
	if err := parsed.DecodeSectionInto(PiSlug, p); err != nil {
		return modelsource.LoadOptions{}, fmt.Errorf("decode pi settings: %w", err)
	}
	return modelsource.LoadOptions{PiDir: p.Dir, SkipPi: m.NoPi, ProfileSources: m.ProfileRegistries, AppName: m.AppName, SkipProfiles: m.NoProfiles}, nil
}

// LoadModels loads the model set; notes say which sources were skipped and why.
func LoadModels(ctx context.Context, o modelsource.LoadOptions) (*modelsource.Set, []string, error) {
	return modelsource.Load(ctx, o)
}
