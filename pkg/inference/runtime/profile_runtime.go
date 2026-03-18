package runtime

import (
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
)

// MiddlewareUse is Pinocchio-owned runtime middleware configuration.
// It intentionally lives outside Geppetto engine profiles.
type MiddlewareUse struct {
	Name    string         `json:"name" yaml:"name"`
	ID      string         `json:"id,omitempty" yaml:"id,omitempty"`
	Enabled *bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Config  map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
}

// ProfileRuntime is app-owned per-profile runtime policy for Pinocchio surfaces.
// It covers prompt, middlewares, and tool exposure, not engine configuration.
type ProfileRuntime struct {
	SystemPrompt string          `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	Middlewares  []MiddlewareUse `json:"middlewares,omitempty" yaml:"middlewares,omitempty"`
	Tools        []string        `json:"tools,omitempty" yaml:"tools,omitempty"`
}

var WebChatProfileRuntimeExtension = gepprofiles.MustProfileExtensionKey[ProfileRuntime]("pinocchio", "webchat_runtime", 1)

func (r *ProfileRuntime) Clone() *ProfileRuntime {
	if r == nil {
		return nil
	}
	out := &ProfileRuntime{
		SystemPrompt: r.SystemPrompt,
		Middlewares:  make([]MiddlewareUse, 0, len(r.Middlewares)),
		Tools:        append([]string(nil), r.Tools...),
	}
	for _, mw := range r.Middlewares {
		out.Middlewares = append(out.Middlewares, MiddlewareUse{
			Name:    mw.Name,
			ID:      mw.ID,
			Enabled: cloneBoolPtr(mw.Enabled),
			Config:  copyStringAnyMap(mw.Config),
		})
	}
	return out
}

func ProfileRuntimeFromEngineProfile(profile *gepprofiles.EngineProfile) (*ProfileRuntime, bool, error) {
	runtime, ok, err := WebChatProfileRuntimeExtension.Get(profile)
	if err != nil || !ok {
		return nil, ok, err
	}
	return runtime.Clone(), true, nil
}

func SetProfileRuntime(profile *gepprofiles.EngineProfile, runtime *ProfileRuntime) error {
	if profile == nil {
		return nil
	}
	if runtime == nil {
		WebChatProfileRuntimeExtension.Delete(profile)
		return nil
	}
	return WebChatProfileRuntimeExtension.Set(profile, *runtime.Clone())
}

func cloneBoolPtr(v *bool) *bool {
	if v == nil {
		return nil
	}
	ret := new(bool)
	*ret = *v
	return ret
}

func copyStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
