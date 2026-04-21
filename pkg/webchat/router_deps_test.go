package webchat

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
	"github.com/stretchr/testify/require"
)

func mustNewInMemoryStreamBackend(t *testing.T) StreamBackend {
	t.Helper()
	backend, err := NewStreamBackend(context.Background(), rediscfg.Settings{})
	require.NoError(t, err)
	return backend
}

func stubRuntimeComposer() infruntime.RuntimeBuilder {
	return infruntime.RuntimeBuilderFunc(func(context.Context, infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		return infruntime.ComposedRuntime{Engine: noopEngine{}}, nil
	})
}

func TestBuildRouterDepsFromValues_Defaults(t *testing.T) {
	deps, err := BuildRouterDepsFromValues(context.Background(), values.New(), fstest.MapFS{})
	require.NoError(t, err)
	require.NotNil(t, deps.StreamBackend)
	require.NotNil(t, deps.TimelineStore)
	require.Nil(t, deps.TurnStore)
}

func TestNewRouter_BuildHTTPServer_UsesSettingsWithoutRetainingParsedValues(t *testing.T) {
	parsed := values.New()
	defaults := parsed.DefaultSectionValues()
	defaults.Fields.Update("addr", &fields.FieldValue{Value: ":4242"})
	defaults.Fields.Update("idle-timeout-seconds", &fields.FieldValue{Value: 17})
	defaults.Fields.Update("evict-idle-seconds", &fields.FieldValue{Value: 19})
	defaults.Fields.Update("evict-interval-seconds", &fields.FieldValue{Value: 23})

	r, err := NewRouter(
		context.Background(),
		parsed,
		fstest.MapFS{},
		WithRuntimeComposer(stubRuntimeComposer()),
	)
	require.NoError(t, err)

	httpSrv, err := r.BuildHTTPServer()
	require.NoError(t, err)
	require.Equal(t, ":4242", httpSrv.Addr)
	require.Equal(t, 17, r.idleTimeoutSec)
}

func TestNewServerFromDeps_BuildsHTTPServer(t *testing.T) {
	srv, err := NewServerFromDeps(context.Background(), RouterDeps{
		StaticFS:      fstest.MapFS{},
		Settings:      RouterSettings{Addr: ":9000"},
		StreamBackend: mustNewInMemoryStreamBackend(t),
	}, WithRuntimeComposer(stubRuntimeComposer()))
	require.NoError(t, err)
	require.NotNil(t, srv.HTTPServer())
	require.Equal(t, ":9000", srv.HTTPServer().Addr)
}
