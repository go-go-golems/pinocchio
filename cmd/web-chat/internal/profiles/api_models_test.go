package profiles

import (
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/pinocchio/pkg/oauthprofiles"
	"github.com/stretchr/testify/require"
)

func TestProfileAPIDocRedactsOAuthCredentialExtension(t *testing.T) {
	profileSlug := gepprofiles.MustEngineProfileSlug("assistant")
	profile := &gepprofiles.EngineProfile{
		Slug: profileSlug,
		Extensions: map[string]any{
			oauthprofiles.ExtensionKey: map[string]any{
				"kind":          oauthprofiles.OAuthBearerKind,
				"access_token":  "access-must-not-appear",
				"refresh_token": "refresh-must-not-appear",
			},
		},
	}
	doc := profileDocFromModel(gepprofiles.MustRegistrySlug("workspace"), nil, profile)
	oauth, ok := doc.Extensions[oauthprofiles.ExtensionKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "<redacted>", oauth["access_token"])
	require.Equal(t, "<redacted>", oauth["refresh_token"])
}
