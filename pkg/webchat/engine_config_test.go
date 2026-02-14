package webchat

import (
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

func TestEngineConfigSignature_DoesNotIncludeAPIKeys(t *testing.T) {
	ss, err := settings.NewStepSettings()
	if err != nil {
		t.Fatalf("NewStepSettings: %v", err)
	}
	ss.API.APIKeys["openai"] = "sk-this-should-not-appear"

	ec := EngineConfig{
		RuntimeKey:  "default",
		SystemPrompt: "hi",
		StepSettings: ss,
	}

	sig := ec.Signature()
	if strings.Contains(sig, "sk-this-should-not-appear") {
		t.Fatalf("signature leaked api key: %q", sig)
	}
	if strings.Contains(sig, "\"api_keys\"") || strings.Contains(sig, "\"APIKeys\"") {
		t.Fatalf("signature unexpectedly contains api key fields: %q", sig)
	}
}

func TestEngineConfigSignature_IsDeterministicAndSensitiveToRelevantChanges(t *testing.T) {
	ss, err := settings.NewStepSettings()
	if err != nil {
		t.Fatalf("NewStepSettings: %v", err)
	}

	ec := EngineConfig{
		RuntimeKey:  "default",
		SystemPrompt: "hi",
		Middlewares:  []MiddlewareUse{{Name: "mw1"}},
		Tools:        []string{"tool1"},
		StepSettings: ss,
	}

	s1 := ec.Signature()
	s2 := ec.Signature()
	if s1 != s2 {
		t.Fatalf("signature not deterministic:\n%s\n%s", s1, s2)
	}

	ec.SystemPrompt = "changed"
	s3 := ec.Signature()
	if s3 == s1 {
		t.Fatalf("signature did not change when SystemPrompt changed")
	}

	ec.SystemPrompt = "hi"
	ec.Tools = []string{"tool2"}
	s4 := ec.Signature()
	if s4 == s1 {
		t.Fatalf("signature did not change when Tools changed")
	}
}
