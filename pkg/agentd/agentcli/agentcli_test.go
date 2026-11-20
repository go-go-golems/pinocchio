package agentcli

import (
	"testing"

	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func agentSchema(t *testing.T) *schema.Schema {
	t.Helper()
	sections, err := ModelSections("pinocchio")
	require.NoError(t, err)
	def, err := schema.NewSection(schema.DefaultSlug, "Flags", schema.WithFields(fields.New("engine", fields.TypeString)))
	require.NoError(t, err)
	return schema.NewSchema(schema.WithSections(append([]schema.Section{def}, sections...)...))
}

func resolvePiDir(t *testing.T, flags map[string]map[string]interface{}) string {
	t.Helper()
	parsed := values.New()
	require.NoError(t, sources.Execute(agentSchema(t), parsed,
		sources.FromMap(flags), // stands in for Cobra flags (highest precedence)
		PiEnvMiddleware(),
		sources.FromDefaults(),
	))
	lo, err := DecodeModelOptions(parsed)
	require.NoError(t, err)
	return lo.PiDir
}

func TestPiDirFromPiEnvironmentVariable(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "/from-env")
	t.Setenv("ENGINE", "should-not-be-read")
	assert.Equal(t, "/from-env", resolvePiDir(t, nil))

	parsed := values.New()
	require.NoError(t, sources.Execute(agentSchema(t), parsed, PiEnvMiddleware(), sources.FromDefaults()))
	v, _ := parsed.GetField(schema.DefaultSlug, "engine")
	assert.Nil(t, v, "only the Pi section reads unprefixed environment variables")
}

func TestPiDirFlagBeatsEnvironment(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "/from-env")
	assert.Equal(t, "/from-flag", resolvePiDir(t, map[string]map[string]interface{}{PiSlug: {"dir": "/from-flag"}}))
}

func TestPiDirDefaultsToEmpty(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	assert.Equal(t, "", resolvePiDir(t, nil), "empty means geppetto's default ~/.pi/agent")
}
