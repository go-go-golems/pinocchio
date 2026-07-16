package configdoc

import (
	"testing"

	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/pinocchio/pkg/oauthprofiles"
	"github.com/stretchr/testify/require"
)

func TestDocumentExplainRedactsOAuthProfileExtension(t *testing.T) {
	doc, err := DecodeDocument([]byte(`
profiles:
  assistant:
    extensions:
      "pinocchio.oauth@v1":
        kind: oauth_bearer
        authorization_url: https://issuer.example.test/authorize
        token_url: https://issuer.example.test/token
        client_id: public-client
        access_token: access-must-not-appear
        refresh_token: refresh-must-not-appear
`))
	require.NoError(t, err)
	explain := NewDocumentExplain()
	recordDocumentExplain(explain, nil, doc, glazedconfig.ResolvedConfigFile{})

	entries := explain.Entries("profiles.assistant.extensions")
	require.Len(t, entries, 1)
	extensions, ok := entries[0].Value.(map[string]any)
	require.True(t, ok)
	oauth, ok := extensions[oauthprofiles.ExtensionKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "<redacted>", oauth["access_token"])
	require.Equal(t, "<redacted>", oauth["refresh_token"])
	require.NotContains(t, entries[0].Value, "access-must-not-appear")
	require.NotContains(t, entries[0].Value, "refresh-must-not-appear")
}
