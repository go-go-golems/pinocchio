package auth

import (
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/stretchr/testify/require"
)

func TestAuthLifecycleCommandsAreGlazedAndRegistered(t *testing.T) {
	status, err := NewStatusCommand()
	require.NoError(t, err)
	logout, err := NewLogoutCommand()
	require.NoError(t, err)
	var _ cmds.GlazeCommand = status
	var _ cmds.GlazeCommand = logout

	root, err := NewAuthCommand()
	require.NoError(t, err)
	for _, name := range []string{"login", "status", "logout"} {
		command, _, findErr := root.Find([]string{name})
		require.NoError(t, findErr)
		require.NotNil(t, command.Flags().Lookup("profile"))
		require.NotNil(t, command.Flags().Lookup("profile-registries"))
	}
}

func TestCredentialStateIsSecretFreeReadinessOnly(t *testing.T) {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	require.Equal(t, "missing_or_expired", credentialState(credentials.Credential{}, now))
	require.Equal(t, "missing_or_expired", credentialState(credentials.Credential{
		AccessToken: "synthetic-access",
		ExpiresAt:   now.Add(-time.Second),
	}, now))
	require.Equal(t, "expiring", credentialState(credentials.Credential{
		AccessToken: "synthetic-access",
		ExpiresAt:   now.Add(time.Minute),
	}, now))
	require.Equal(t, "usable", credentialState(credentials.Credential{
		AccessToken: "synthetic-access",
		ExpiresAt:   now.Add(time.Hour),
	}, now))
}
