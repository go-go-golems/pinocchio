package main

import (
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

func TestRuntimeFingerprint_DoesNotIncludeAPIKeys(t *testing.T) {
	ss, err := settings.NewStepSettings()
	if err != nil {
		t.Fatalf("NewStepSettings: %v", err)
	}
	ss.API.APIKeys["openai"] = "sk-this-should-not-appear"

	fp := runtimeFingerprint("default", "hi", nil, nil, ss)
	if strings.Contains(fp, "sk-this-should-not-appear") {
		t.Fatalf("fingerprint leaked api key: %q", fp)
	}
	if strings.Contains(fp, "\"api_keys\"") || strings.Contains(fp, "\"APIKeys\"") {
		t.Fatalf("fingerprint unexpectedly contains api key fields: %q", fp)
	}
}

func TestWebChatRuntimeComposer_RejectsInvalidOverrideTypes(t *testing.T) {
	composer := newWebChatRuntimeComposer(values.New(), map[string]infruntime.MiddlewareFactory{})

	_, err := composer.Compose(context.Background(), infruntime.RuntimeComposeRequest{
		ConvID:     "c1",
		RuntimeKey: "default",
		Overrides:  map[string]any{"middlewares": "bad"},
	})
	if err == nil {
		t.Fatalf("expected error for invalid middlewares override type")
	}
}
