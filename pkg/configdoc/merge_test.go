package configdoc

import "testing"

func mustDecodeDocument(t *testing.T, body string) *Document {
	t.Helper()
	doc, err := DecodeDocument([]byte(body))
	if err != nil {
		t.Fatalf("DecodeDocument failed: %v", err)
	}
	return doc
}

func TestMergeDocuments_MergesRepositoriesWithDedupeAndStableOrder(t *testing.T) {
	low := mustDecodeDocument(t, `
app:
  repositories:
    - ~/prompts/base
    - ~/prompts/shared
`)
	high := mustDecodeDocument(t, `
app:
  repositories:
    - ./prompts
    - ~/prompts/shared
`)

	merged, err := MergeDocuments(low, high)
	if err != nil {
		t.Fatalf("MergeDocuments failed: %v", err)
	}

	want := []string{"~/prompts/base", "~/prompts/shared", "./prompts"}
	if len(merged.App.Repositories) != len(want) {
		t.Fatalf("unexpected repository count: got=%#v want=%#v", merged.App.Repositories, want)
	}
	for i := range want {
		if merged.App.Repositories[i] != want[i] {
			t.Fatalf("repository[%d] mismatch: got=%q want=%q", i, merged.App.Repositories[i], want[i])
		}
	}
}

func TestMergeDocuments_ProfileControlPlanePreservesAbsenceAndAllowsReplacement(t *testing.T) {
	low := mustDecodeDocument(t, `
profile:
  active: default
  registries:
    - ~/.pinocchio/base.yaml
`)
	absentHigh := mustDecodeDocument(t, `
profiles:
  default:
    display_name: Default
`)

	merged, err := MergeDocuments(low, absentHigh)
	if err != nil {
		t.Fatalf("MergeDocuments with absent high profile block failed: %v", err)
	}
	if got := merged.Profile.Active; got != "default" {
		t.Fatalf("expected profile.active to be preserved, got %q", got)
	}
	if len(merged.Profile.Registries) != 1 || merged.Profile.Registries[0] != "~/.pinocchio/base.yaml" {
		t.Fatalf("expected registries to be preserved, got %#v", merged.Profile.Registries)
	}

	replacingHigh := mustDecodeDocument(t, `
profile:
  active: assistant
  registries: []
`)
	merged, err = MergeDocuments(low, replacingHigh)
	if err != nil {
		t.Fatalf("MergeDocuments with replacing high profile block failed: %v", err)
	}
	if got := merged.Profile.Active; got != "assistant" {
		t.Fatalf("expected profile.active replacement, got %q", got)
	}
	if merged.Profile.Registries != nil {
		t.Fatalf("expected registries to be cleared by explicit empty list, got %#v", merged.Profile.Registries)
	}
}

func TestMergeDocuments_MergesSameSlugProfilesFieldByField(t *testing.T) {
	low := mustDecodeDocument(t, `
profiles:
  assistant:
    display_name: Base Assistant
    description: Base description
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5
    extensions:
      ui:
        color: blue
        size: medium
`)
	high := mustDecodeDocument(t, `
profiles:
  assistant:
    display_name: Team Assistant
    stack:
      - profile_slug: team-default
    inference_settings:
      chat:
        engine: gpt-5-mini
    extensions:
      ui:
        size: compact
      extra:
        enabled: true
`)

	merged, err := MergeDocuments(low, high)
	if err != nil {
		t.Fatalf("MergeDocuments failed: %v", err)
	}
	assistant := merged.Profiles["assistant"]
	if assistant == nil {
		t.Fatal("expected merged assistant profile")
	}
	if got := assistant.DisplayName; got != "Team Assistant" {
		t.Fatalf("expected display name override, got %q", got)
	}
	if got := assistant.Description; got != "Base description" {
		t.Fatalf("expected description to be preserved, got %q", got)
	}
	if len(assistant.Stack) != 1 || assistant.Stack[0].EngineProfileSlug.String() != "team-default" {
		t.Fatalf("expected stack replacement, got %#v", assistant.Stack)
	}
	if assistant.InferenceSettings == nil || assistant.InferenceSettings.Chat == nil || assistant.InferenceSettings.Chat.Engine == nil {
		t.Fatal("expected merged inference settings with chat engine")
	}
	if got := *assistant.InferenceSettings.Chat.Engine; got != "gpt-5-mini" {
		t.Fatalf("expected engine override, got %q", got)
	}
	if assistant.InferenceSettings.Chat.ApiType == nil || *assistant.InferenceSettings.Chat.ApiType != "openai" {
		t.Fatalf("expected api_type to be preserved, got %#v", assistant.InferenceSettings.Chat.ApiType)
	}
	ui, ok := assistant.Extensions["ui"].(map[string]any)
	if !ok {
		t.Fatalf("expected merged ui extension map, got %#v", assistant.Extensions)
	}
	if ui["color"] != "blue" || ui["size"] != "compact" {
		t.Fatalf("unexpected merged ui extension: %#v", ui)
	}
	extra, ok := assistant.Extensions["extra"].(map[string]any)
	if !ok || extra["enabled"] != true {
		t.Fatalf("unexpected extra extension: %#v", assistant.Extensions["extra"])
	}
}
