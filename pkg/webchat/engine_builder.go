package webchat

import (
	"fmt"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

// EngineBuilder centralizes engine + sink composition so Router handlers stay lean and recomposition
// happens deterministically.
//
// This mirrors the go-go-mento pattern closely so Moments can later adopt the same shape.
type EngineBuilder interface {
	BuildConfig(profileSlug string, overrides map[string]any) (EngineConfig, error)
	BuildFromConfig(convID string, config EngineConfig) (engine.Engine, events.EventSink, error)
}

// SubscriberFactory builds a subscriber for a conversation and indicates whether it should be closed
// when the conversation is rebuilt or evicted.
type SubscriberFactory func(convID string) (sub message.Subscriber, closeOnReplace bool, err error)

func (r *Router) BuildConfig(profileSlug string, overrides map[string]any) (EngineConfig, error) {
	if r == nil {
		return EngineConfig{}, errors.New("router is nil")
	}
	p, ok := r.profiles.Get(profileSlug)
	if !ok {
		return EngineConfig{}, errors.Errorf("profile not found: %s", profileSlug)
	}

	if err := validateOverrides(p, overrides); err != nil {
		return EngineConfig{}, err
	}

	sysPrompt := p.DefaultPrompt
	mws := append([]MiddlewareUse{}, p.DefaultMws...)
	tools := append([]string{}, p.DefaultTools...)

	if overrides != nil {
		if v, ok := overrides["system_prompt"].(string); ok && v != "" {
			sysPrompt = v
		}
		if arr, ok := overrides["middlewares"].([]any); ok {
			parsed, err := parseMiddlewareOverrides(arr)
			if err != nil {
				return EngineConfig{}, err
			}
			mws = parsed
		}
		if arr, ok := overrides["tools"].([]any); ok {
			parsed, err := parseToolOverrides(arr)
			if err != nil {
				return EngineConfig{}, err
			}
			tools = parsed
		}
	}
	if strings.TrimSpace(sysPrompt) == "" {
		sysPrompt = "You are an assistant"
	}

	stepSettings, err := settings.NewStepSettingsFromParsedValues(r.parsed)
	if err != nil {
		return EngineConfig{}, err
	}

	return EngineConfig{
		ProfileSlug:  profileSlug,
		SystemPrompt: sysPrompt,
		Middlewares:  mws,
		Tools:        tools,
		StepSettings: stepSettings,
	}, nil
}

func (r *Router) BuildFromConfig(convID string, config EngineConfig) (engine.Engine, events.EventSink, error) {
	if r == nil {
		return nil, nil, errors.New("router is nil")
	}
	if config.StepSettings == nil {
		return nil, nil, errors.New("engine config missing step settings")
	}
	if convID == "" {
		return nil, nil, errors.New("convID is empty")
	}
	if r.baseCtx == nil {
		return nil, nil, errors.New("router context is nil")
	}

	var sink events.EventSink = middleware.NewWatermillSink(r.router.Publisher, topicForConv(convID))
	if r.eventSinkWrapper != nil {
		wrapped, err := r.eventSinkWrapper(convID, config, sink)
		if err != nil {
			return nil, nil, err
		}
		sink = wrapped
	}
	eng, err := composeEngineFromSettings(
		r.baseCtx,
		config.StepSettings.Clone(),
		config.SystemPrompt,
		config.Middlewares,
		r.mwFactories,
	)
	if err != nil {
		return nil, nil, err
	}
	return eng, sink, nil
}

func parseMiddlewareOverrides(arr []any) ([]MiddlewareUse, error) {
	mws := make([]MiddlewareUse, 0, len(arr))
	for _, raw := range arr {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("middleware override entries must be objects")
		}
		name, _ := m["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("middleware override missing name")
		}
		mws = append(mws, MiddlewareUse{Name: name, Config: m["config"]})
	}
	return mws, nil
}

func validateOverrides(p *Profile, overrides map[string]any) error {
	if overrides == nil {
		return nil
	}

	hasEngineOverride := false
	if _, ok := overrides["system_prompt"]; ok {
		hasEngineOverride = true
	}
	if _, ok := overrides["middlewares"]; ok {
		hasEngineOverride = true
	}
	if _, ok := overrides["tools"]; ok {
		hasEngineOverride = true
	}
	if hasEngineOverride && p != nil && !p.AllowOverrides {
		return errors.Errorf("profile %q does not allow overrides", strings.TrimSpace(p.Slug))
	}

	if v, ok := overrides["system_prompt"]; ok {
		if _, ok2 := v.(string); !ok2 {
			return fmt.Errorf("system_prompt override must be a string")
		}
	}
	if v, ok := overrides["middlewares"]; ok {
		if _, ok2 := v.([]any); !ok2 {
			return fmt.Errorf("middlewares override must be an array")
		}
	}
	if v, ok := overrides["tools"]; ok {
		if _, ok2 := v.([]any); !ok2 {
			return fmt.Errorf("tools override must be an array")
		}
	}
	return nil
}

func parseToolOverrides(arr []any) ([]string, error) {
	tools := make([]string, 0, len(arr))
	for _, raw := range arr {
		switch v := raw.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return nil, fmt.Errorf("tool override contains empty name")
			}
			tools = append(tools, v)
		default:
			return nil, fmt.Errorf("tool override entries must be strings")
		}
	}
	return tools, nil
}
