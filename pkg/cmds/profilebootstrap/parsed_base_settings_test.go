package profilebootstrap

import (
	"testing"

	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/stretchr/testify/require"
)

func TestResolveParsedBaseInferenceSettingsWithBase_AppliesNonProfileValuesAndStripsProfileValues(t *testing.T) {
	chatSection, err := aisettings.NewChatValueSection()
	require.NoError(t, err)
	clientSection, err := aisettings.NewClientValueSection()
	require.NoError(t, err)

	chatValues, err := values.NewSectionValues(
		chatSection,
		values.WithFieldValue("ai-engine", "profile-engine", fields.WithSource("profiles")),
	)
	require.NoError(t, err)
	clientValues, err := values.NewSectionValues(
		clientSection,
		values.WithFieldValue("timeout", 123, fields.WithSource("cobra")),
	)
	require.NoError(t, err)

	parsed := values.New(
		values.WithSectionValues(aisettings.AiChatSlug, chatValues),
		values.WithSectionValues(aisettings.AiClientSlug, clientValues),
	)

	initial, err := aisettings.NewInferenceSettings()
	require.NoError(t, err)
	engine := "hidden-engine"
	initial.Chat.Engine = &engine

	base, err := ResolveParsedBaseInferenceSettingsWithBase(parsed, initial)
	require.NoError(t, err)
	require.NotNil(t, base.Chat)
	require.NotNil(t, base.Chat.Engine)
	require.Equal(t, "hidden-engine", *base.Chat.Engine)
	require.NotNil(t, base.Client)
	require.NotNil(t, base.Client.TimeoutSeconds)
	require.Equal(t, 123, *base.Client.TimeoutSeconds)
}
