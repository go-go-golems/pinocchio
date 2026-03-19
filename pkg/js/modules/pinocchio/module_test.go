package pinocchio

import (
	"testing"
	"time"

	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	aitypes "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
)

func TestApplyEngineOverridesIgnoresMissingKeys(t *testing.T) {
	base, err := aisettings.NewInferenceSettings()
	if err != nil {
		t.Fatalf("NewInferenceSettings failed: %v", err)
	}

	apiType := aitypes.ApiTypeOpenAI
	model := "base-model"
	timeout := 30 * time.Second
	timeoutSeconds := int(timeout.Seconds())
	base.Chat.ApiType = &apiType
	base.Chat.Engine = &model
	base.Client.Timeout = &timeout
	base.Client.TimeoutSeconds = &timeoutSeconds
	base.API.BaseUrls["openai-base-url"] = "https://base.example"
	base.API.APIKeys["openai-api-key"] = "base-key"

	applyEngineOverrides(base, map[string]any{})

	if base.Chat.Engine == nil || *base.Chat.Engine != "base-model" {
		t.Fatalf("missing model override should preserve base engine, got %#v", base.Chat.Engine)
	}
	if base.Chat.ApiType == nil || *base.Chat.ApiType != aitypes.ApiTypeOpenAI {
		t.Fatalf("missing apiType override should preserve base api type, got %#v", base.Chat.ApiType)
	}
	if got := base.API.BaseUrls["openai-base-url"]; got != "https://base.example" {
		t.Fatalf("missing baseURL override should preserve base baseURL, got %q", got)
	}
	if got := base.API.APIKeys["openai-api-key"]; got != "base-key" {
		t.Fatalf("missing apiKey override should preserve base api key, got %q", got)
	}
	if base.Client.Timeout == nil || base.Client.Timeout.Milliseconds() != 30000 {
		t.Fatalf("missing timeout override should preserve base timeout, got %#v", base.Client.Timeout)
	}
}

func TestAsStringReturnsEmptyStringForNil(t *testing.T) {
	if got := asString(nil); got != "" {
		t.Fatalf("expected empty string for nil, got %q", got)
	}
}
