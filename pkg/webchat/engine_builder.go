package webchat

import (
	"fmt"

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

	sysPrompt := p.DefaultPrompt
	mws := append([]MiddlewareUse{}, p.DefaultMws...)

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
	}

	stepSettings, err := settings.NewStepSettingsFromParsedLayers(r.parsed)
	if err != nil {
		return EngineConfig{}, err
	}

	return EngineConfig{
		ProfileSlug:  profileSlug,
		SystemPrompt: sysPrompt,
		Middlewares:  mws,
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

	sink := middleware.NewWatermillSink(r.router.Publisher, topicForConv(convID))
	eng, err := composeEngineFromSettings(config.StepSettings.Clone(), config.SystemPrompt, config.Middlewares, r.mwFactories)
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
