package profileswitch

import (
	"context"
	"fmt"
	"sort"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/pkg/errors"
)

type Manager struct {
	reg      gepprofiles.Registry
	sources  string
	base     *settings.InferenceSettings
	resolved Resolved
}

func NewManagerFromSources(ctx context.Context, sources string, base *settings.InferenceSettings) (*Manager, error) {
	if strings.TrimSpace(sources) == "" {
		return nil, &gepprofiles.ValidationError{Field: "profile-settings.profile-registries", Reason: "must not be empty"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	entries, err := gepprofiles.ParseProfileRegistrySourceEntries(sources)
	if err != nil {
		return nil, err
	}
	specs, err := gepprofiles.ParseRegistrySourceSpecs(entries)
	if err != nil {
		return nil, err
	}
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, specs)
	if err != nil {
		return nil, err
	}
	m, err := NewManager(chain, sources, base)
	if err != nil {
		_ = chain.Close()
		return nil, err
	}
	return m, nil
}

func NewManager(reg gepprofiles.Registry, sources string, base *settings.InferenceSettings) (*Manager, error) {
	if reg == nil {
		return nil, errors.New("profile manager: registry is nil")
	}
	if base == nil {
		return nil, errors.New("profile manager: base inference settings are nil")
	}
	return &Manager{reg: reg, sources: strings.TrimSpace(sources), base: base}, nil
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	if c, ok := m.reg.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}

func (m *Manager) Sources() string {
	if m == nil {
		return ""
	}
	return m.sources
}

func (m *Manager) Current() Resolved {
	if m == nil {
		return Resolved{}
	}
	return m.resolved
}

func (m *Manager) ListProfiles(ctx context.Context) ([]ProfileListItem, error) {
	if m == nil || m.reg == nil {
		return nil, errors.New("profile manager: not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	regs, err := m.reg.ListRegistries(ctx)
	if err != nil {
		return nil, err
	}
	if len(regs) == 0 {
		return nil, fmt.Errorf("profile manager: no registries loaded from sources")
	}

	items := make([]ProfileListItem, 0, 32)
	for _, rs := range regs {
		regSlug := rs.Slug
		profiles, err := m.reg.ListProfiles(ctx, regSlug)
		if err != nil {
			return nil, err
		}
		for _, p := range profiles {
			if p == nil {
				continue
			}
			items = append(items, ProfileListItem{
				RegistrySlug: regSlug,
				ProfileSlug:  p.Slug,
				DisplayName:  strings.TrimSpace(p.DisplayName),
				Description:  strings.TrimSpace(p.Description),
				IsDefault:    rs.DefaultProfileSlug == p.Slug,
				Version:      p.Metadata.Version,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].RegistrySlug != items[j].RegistrySlug {
			return items[i].RegistrySlug.String() < items[j].RegistrySlug.String()
		}
		return items[i].ProfileSlug.String() < items[j].ProfileSlug.String()
	})
	return items, nil
}

// Resolve resolves either:
// - the stack default profile (when profileSlug is empty), or
// - a specific profile slug (stack lookup rules apply when registry is empty).
func (m *Manager) Resolve(ctx context.Context, profileSlug string) (Resolved, error) {
	if m == nil || m.reg == nil {
		return Resolved{}, errors.New("profile manager: not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var in gepprofiles.ResolveInput

	if strings.TrimSpace(profileSlug) != "" {
		ps, err := gepprofiles.ParseProfileSlug(profileSlug)
		if err != nil {
			return Resolved{}, err
		}
		in.ProfileSlug = ps
	}

	resolved, err := m.reg.ResolveEffectiveProfile(ctx, in)
	if err != nil {
		return Resolved{}, err
	}
	if resolved == nil {
		return Resolved{}, errors.New("profile manager: resolved profile is nil")
	}

	sysPrompt := ""
	version := uint64(0)
	sysPrompt = strings.TrimSpace(resolved.EffectiveRuntime.SystemPrompt)
	if resolved.Metadata != nil {
		if v, ok := resolved.Metadata["profile.version"].(uint64); ok {
			version = v
		}
	}

	out := Resolved{
		RegistrySlug:       resolved.RegistrySlug,
		ProfileSlug:        resolved.ProfileSlug,
		RuntimeKey:         resolved.RuntimeKey,
		RuntimeFingerprint: strings.TrimSpace(resolved.RuntimeFingerprint),
		SystemPrompt:       sysPrompt,
		InferenceSettings:  cloneInferenceSettings(m.base),
		ProfileVersion:     version,
		Metadata:           resolved.Metadata,
	}
	if out.InferenceSettings == nil {
		return Resolved{}, errors.New("profile manager: resolved inference settings are nil")
	}
	return out, nil
}

func (m *Manager) Switch(ctx context.Context, profileSlug string) (Resolved, error) {
	res, err := m.Resolve(ctx, profileSlug)
	if err != nil {
		return Resolved{}, err
	}

	m.resolved = res
	return res, nil
}

func cloneInferenceSettings(in *settings.InferenceSettings) *settings.InferenceSettings {
	if in == nil {
		return nil
	}
	return in.Clone()
}
