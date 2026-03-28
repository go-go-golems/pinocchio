package tokens

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	claudesettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	openaisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	types2 "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

type rewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = rt.target.Scheme
	req2.URL.Host = rt.target.Host
	req2.Host = rt.target.Host
	return rt.base.RoundTrip(req2)
}

func TestCountCommandEstimateMode(t *testing.T) {
	cmd, err := NewCountCommand()
	if err != nil {
		t.Fatalf("NewCountCommand: %v", err)
	}

	parsed := mustParsedValues(t, cmd, map[string]map[string]any{
		schema.DefaultSlug: {
			"count-mode": countModeEstimate,
			"model":      "gpt-4",
			"input":      "hello world",
		},
	})

	var buf bytes.Buffer
	if err := cmd.RunIntoWriter(context.Background(), parsed, &buf); err != nil {
		t.Fatalf("RunIntoWriter: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Requested mode: estimate") {
		t.Fatalf("missing estimate mode output: %s", out)
	}
	if !strings.Contains(out, "Count source: estimate") {
		t.Fatalf("missing estimate source output: %s", out)
	}
	if !strings.Contains(out, "Codec: cl100k_base") {
		t.Fatalf("missing codec output: %s", out)
	}
	if !strings.Contains(out, "Total tokens:") {
		t.Fatalf("missing total token output: %s", out)
	}
}

func TestCountCommandAPIModeOpenAI(t *testing.T) {
	cmd, err := NewCountCommand()
	if err != nil {
		t.Fatalf("NewCountCommand: %v", err)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses/input_tokens" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		payload := string(body)
		if !strings.Contains(payload, `"model":"gpt-4o-mini"`) {
			t.Fatalf("request missing model: %s", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"input_tokens":42}`))
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{
		base:   server.Client().Transport,
		target: targetURL,
	}
	defer func() {
		http.DefaultTransport = originalTransport
	}()

	parsed := mustParsedValues(t, cmd, map[string]map[string]any{
		schema.DefaultSlug: {
			"count-mode": countModeAPI,
			"model":      "gpt-4o-mini",
			"input":      "hello world",
		},
		aisettings.AiChatSlug: {
			"ai-api-type": string(types2.ApiTypeOpenAIResponses),
		},
		openaisettings.OpenAiChatSlug: {
			"openai-api-key":  "test-key",
			"openai-base-url": "https://api.openai.com/v1",
		},
	})

	var buf bytes.Buffer
	if err := cmd.RunIntoWriter(context.Background(), parsed, &buf); err != nil {
		t.Fatalf("RunIntoWriter: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Requested mode: api") {
		t.Fatalf("missing api mode output: %s", out)
	}
	if !strings.Contains(out, "Count source: provider_api") {
		t.Fatalf("missing provider source output: %s", out)
	}
	if !strings.Contains(out, "Provider: open-responses") {
		t.Fatalf("missing provider output: %s", out)
	}
	if !strings.Contains(out, "Total tokens: 42") {
		t.Fatalf("missing token count output: %s", out)
	}
}

func TestCountCommandAutoFallsBackToEstimate(t *testing.T) {
	cmd, err := NewCountCommand()
	if err != nil {
		t.Fatalf("NewCountCommand: %v", err)
	}

	parsed := mustParsedValues(t, cmd, map[string]map[string]any{
		schema.DefaultSlug: {
			"count-mode": countModeAuto,
			"model":      "gpt-4",
			"input":      "hello world",
		},
		aisettings.AiChatSlug: {
			"ai-api-type": string(types2.ApiTypeClaude),
		},
		claudesettings.ClaudeChatSlug: {
			"claude-base-url": "https://api.anthropic.com",
		},
	})

	var buf bytes.Buffer
	if err := cmd.RunIntoWriter(context.Background(), parsed, &buf); err != nil {
		t.Fatalf("RunIntoWriter: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Requested mode: auto") {
		t.Fatalf("missing auto mode output: %s", out)
	}
	if !strings.Contains(out, "Count source: estimate") {
		t.Fatalf("missing fallback estimate output: %s", out)
	}
	if !strings.Contains(out, "Fallback reason: claude token count: no claude api key configured") {
		t.Fatalf("missing fallback reason output: %s", out)
	}
}

func mustParsedValues(t *testing.T, cmd *CountCommand, fieldsBySection map[string]map[string]any) *values.Values {
	t.Helper()

	parsed := values.New()
	cmd.Schema.ForEach(func(slug string, section schema.Section) {
		sectionValues, err := values.NewSectionValues(section)
		if err != nil {
			t.Fatalf("NewSectionValues(%s): %v", slug, err)
		}
		parsed.Set(slug, sectionValues)
	})

	for slug, fieldValues := range fieldsBySection {
		sectionValues, ok := parsed.Get(slug)
		if !ok {
			t.Fatalf("section %s not found", slug)
		}

		for name, value := range fieldValues {
			if err := values.WithFieldValue(name, value)(sectionValues); err != nil {
				t.Fatalf("WithFieldValue(%s.%s): %v", slug, name, err)
			}
		}
	}

	return parsed
}
