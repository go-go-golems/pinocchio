package profilebootstrap

import (
	"testing"

	"github.com/go-go-golems/glazed/pkg/cli"
)

func TestNewCLISelectionValuesBuildsCommandAndProfileSections(t *testing.T) {
	parsed, err := NewCLISelectionValues(CLISelectionInput{
		ConfigFile:        "custom.yaml",
		Profile:           " analyst ",
		ProfileRegistries: []string{" one.yaml ", "", "two.yaml"},
	})
	if err != nil {
		t.Fatalf("NewCLISelectionValues failed: %v", err)
	}

	commandSettings := &cli.CommandSettings{}
	if err := parsed.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings); err != nil {
		t.Fatalf("decode command settings: %v", err)
	}
	if got := commandSettings.ConfigFile; got != "custom.yaml" {
		t.Fatalf("expected config file custom.yaml, got %q", got)
	}

	profileSettings := ResolveProfileSettings(parsed)
	if got := profileSettings.Profile; got != "analyst" {
		t.Fatalf("expected trimmed profile analyst, got %q", got)
	}
	if len(profileSettings.ProfileRegistries) != 2 || profileSettings.ProfileRegistries[0] != "one.yaml" || profileSettings.ProfileRegistries[1] != "two.yaml" {
		t.Fatalf("expected normalized registries, got %#v", profileSettings.ProfileRegistries)
	}
}
