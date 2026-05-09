package chatapp

import (
	"math"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/stretchr/testify/require"
)

func TestUsageInfoFromGeppettoSaturatesTokenCounts(t *testing.T) {
	usage := &gepevents.Usage{
		InputTokens:              math.MaxInt32 + 1,
		OutputTokens:             42,
		CachedTokens:             math.MaxInt32,
		CacheCreationInputTokens: math.MaxInt32 + 100,
		CacheReadInputTokens:     -1,
	}

	info := UsageInfoFromGeppetto(usage)
	require.NotNil(t, info)
	require.Equal(t, int32(math.MaxInt32), info.GetInputTokens())
	require.Equal(t, int32(42), info.GetOutputTokens())
	require.Equal(t, int32(math.MaxInt32), info.GetCachedTokens())
	require.Equal(t, int32(math.MaxInt32), info.GetCacheCreationInputTokens())
	require.Equal(t, int32(-1), info.GetCacheReadInputTokens())
}
