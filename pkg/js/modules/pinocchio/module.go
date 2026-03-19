package pinocchio

import (
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	aitypes "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
)

const ModuleName = "pinocchio"

type Options struct {
	DefaultInferenceSettings *aisettings.InferenceSettings
}

func Register(reg *require.Registry, opts Options) {
	if reg == nil {
		return
	}
	reg.RegisterNativeModule(ModuleName, (&module{opts: opts}).Loader)
}

type module struct {
	opts Options
}

func (m *module) Loader(vm *goja.Runtime, moduleObj *goja.Object) {
	exports := moduleObj.Get("exports").(*goja.Object)
	enginesObj := vm.NewObject()
	if err := enginesObj.Set("fromDefaults", func(call goja.FunctionCall) goja.Value {
		ref, err := m.engineFromDefaults(call)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(ref)
	}); err != nil {
		panic(vm.NewGoError(err))
	}
	if err := enginesObj.Set("inspectDefaults", func(call goja.FunctionCall) goja.Value {
		info, err := m.inspectEngineDefaults(call)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(info)
	}); err != nil {
		panic(vm.NewGoError(err))
	}
	if err := exports.Set("engines", enginesObj); err != nil {
		panic(vm.NewGoError(err))
	}
}

func (m *module) engineFromDefaults(call goja.FunctionCall) (any, error) {
	ss, err := m.cloneInferenceSettingsWithOverrides(call)
	if err != nil {
		return nil, err
	}
	eng, err := factory.NewEngineFromSettings(ss)
	if err != nil {
		return nil, err
	}
	return eng, nil
}

func (m *module) inspectEngineDefaults(call goja.FunctionCall) (map[string]any, error) {
	ss, err := m.cloneInferenceSettingsWithOverrides(call)
	if err != nil {
		return nil, err
	}
	return describeInferenceSettings(ss), nil
}

func (m *module) cloneInferenceSettingsWithOverrides(call goja.FunctionCall) (*aisettings.InferenceSettings, error) {
	if m.opts.DefaultInferenceSettings == nil {
		return nil, fmt.Errorf("pinocchio default inference settings are not configured")
	}
	ss := m.opts.DefaultInferenceSettings.Clone()
	if ss == nil {
		return nil, fmt.Errorf("pinocchio default inference settings are not available")
	}
	if len(call.Arguments) > 0 && call.Arguments[0] != nil && !goja.IsUndefined(call.Arguments[0]) && !goja.IsNull(call.Arguments[0]) {
		opts, ok := call.Arguments[0].Export().(map[string]any)
		if !ok {
			return nil, fmt.Errorf("pinocchio.engines.fromDefaults expects an options object")
		}
		applyEngineOverrides(ss, opts)
	}
	return ss, nil
}

func applyEngineOverrides(ss *aisettings.InferenceSettings, opts map[string]any) {
	if ss == nil || opts == nil {
		return
	}
	model := strings.TrimSpace(asString(opts["model"]))
	if model := strings.TrimSpace(asString(opts["model"])); model != "" {
		ss.Chat.Engine = &model
	}
	currentAPIType := ""
	if ss.Chat.ApiType != nil {
		currentAPIType = strings.TrimSpace(string(*ss.Chat.ApiType))
	}
	if apiTypeRaw := strings.TrimSpace(strings.ToLower(asString(opts["apiType"]))); apiTypeRaw != "" {
		apiType := aitypes.ApiType(apiTypeRaw)
		ss.Chat.ApiType = &apiType
	} else if model != "" && currentAPIType == "" {
		apiType := inferAPIType(model)
		ss.Chat.ApiType = &apiType
	}
	if baseURL := strings.TrimSpace(asString(opts["baseURL"])); baseURL != "" && ss.Chat.ApiType != nil {
		if ss.API.BaseUrls == nil {
			ss.API.BaseUrls = map[string]string{}
		}
		ss.API.BaseUrls[string(*ss.Chat.ApiType)+"-base-url"] = baseURL
		if *ss.Chat.ApiType == aitypes.ApiTypeOpenAIResponses {
			ss.API.BaseUrls["openai-base-url"] = baseURL
		}
	}
	if timeoutMS, ok := asPositiveInt(opts["timeoutMs"]); ok {
		d := time.Duration(timeoutMS) * time.Millisecond
		sec := int(d.Seconds())
		ss.Client.Timeout = &d
		ss.Client.TimeoutSeconds = &sec
	}
	if apiKey := strings.TrimSpace(asString(opts["apiKey"])); apiKey != "" && ss.Chat.ApiType != nil {
		if ss.API.APIKeys == nil {
			ss.API.APIKeys = map[string]string{}
		}
		ss.API.APIKeys[string(*ss.Chat.ApiType)+"-api-key"] = apiKey
		if *ss.Chat.ApiType == aitypes.ApiTypeOpenAI || *ss.Chat.ApiType == aitypes.ApiTypeOpenAIResponses {
			ss.API.APIKeys["openai-api-key"] = apiKey
		}
		if *ss.Chat.ApiType == aitypes.ApiTypeOpenAIResponses {
			ss.API.APIKeys["openai-responses-api-key"] = apiKey
		}
	}
}

func inferAPIType(model string) aitypes.ApiType {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(m, "gemini"):
		return aitypes.ApiTypeGemini
	case strings.Contains(m, "claude"):
		return aitypes.ApiTypeClaude
	case strings.HasPrefix(m, "o1"), strings.HasPrefix(m, "o3"), strings.HasPrefix(m, "o4"), strings.HasPrefix(m, "gpt-5"):
		return aitypes.ApiTypeOpenAIResponses
	default:
		return aitypes.ApiTypeOpenAI
	}
}

func describeInferenceSettings(ss *aisettings.InferenceSettings) map[string]any {
	out := map[string]any{}
	if ss == nil {
		return out
	}
	apiType := ""
	if ss.Chat.ApiType != nil {
		apiType = strings.TrimSpace(string(*ss.Chat.ApiType))
	}
	model := ""
	if ss.Chat.Engine != nil {
		model = strings.TrimSpace(*ss.Chat.Engine)
	}
	out["apiType"] = apiType
	out["model"] = model
	out["baseURL"] = resolveBaseURL(ss, apiType)
	out["hasAPIKey"] = resolveHasAPIKey(ss, apiType)
	if ss.Client.Timeout != nil {
		out["timeoutMs"] = ss.Client.Timeout.Milliseconds()
	} else if ss.Client.TimeoutSeconds != nil {
		out["timeoutMs"] = int64(*ss.Client.TimeoutSeconds) * 1000
	}
	return out
}

func resolveBaseURL(ss *aisettings.InferenceSettings, apiType string) string {
	if ss == nil || ss.API.BaseUrls == nil || apiType == "" {
		return ""
	}
	keys := []string{apiType + "-base-url"}
	if apiType == string(aitypes.ApiTypeOpenAIResponses) {
		keys = append(keys, "openai-base-url")
	}
	for _, key := range keys {
		if v := strings.TrimSpace(ss.API.BaseUrls[key]); v != "" {
			return v
		}
	}
	return ""
}

func resolveHasAPIKey(ss *aisettings.InferenceSettings, apiType string) bool {
	if ss == nil || ss.API.APIKeys == nil || apiType == "" {
		return false
	}
	keys := []string{apiType + "-api-key"}
	switch apiType {
	case string(aitypes.ApiTypeOpenAI):
		keys = append(keys, "openai-api-key")
	case string(aitypes.ApiTypeOpenAIResponses):
		keys = append(keys, "openai-api-key", "openai-responses-api-key")
	}
	for _, key := range keys {
		if strings.TrimSpace(ss.API.APIKeys[key]) != "" {
			return true
		}
	}
	return false
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprint(v)
	}
}

func asPositiveInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, x > 0
	case int64:
		i := int(x)
		return i, i > 0
	case int32:
		i := int(x)
		return i, i > 0
	case float64:
		i := int(x)
		return i, i > 0
	case float32:
		i := int(x)
		return i, i > 0
	default:
		return 0, false
	}
}
