package chatstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeBlockContentHash_DeterministicAcrossMapOrder(t *testing.T) {
	payloadA := map[string]any{
		"text": "hello",
		"meta": map[string]any{
			"b": 2,
			"a": 1,
		},
	}
	payloadB := map[string]any{
		"meta": map[string]any{
			"a": 1,
			"b": 2,
		},
		"text": "hello",
	}
	metadataA := map[string]any{
		"z": "tail",
		"a": "head",
	}
	metadataB := map[string]any{
		"a": "head",
		"z": "tail",
	}

	hashA, err := ComputeBlockContentHash("llm_text", "assistant", payloadA, metadataA)
	require.NoError(t, err)

	hashB, err := ComputeBlockContentHash("llm_text", "assistant", payloadB, metadataB)
	require.NoError(t, err)

	require.Equal(t, hashA, hashB)
}

func TestComputeBlockContentHash_NilAndEmptyPayloadMetadataMatch(t *testing.T) {
	hashNil, err := ComputeBlockContentHash("user", "user", nil, nil)
	require.NoError(t, err)

	hashEmpty, err := ComputeBlockContentHash("user", "user", map[string]any{}, map[string]any{})
	require.NoError(t, err)

	require.Equal(t, hashNil, hashEmpty)
}

func TestComputeBlockContentHash_ContentChangesProduceDifferentHashes(t *testing.T) {
	base, err := ComputeBlockContentHash(
		"tool_call",
		"",
		map[string]any{"name": "weather", "args": map[string]any{"city": "Paris"}},
		map[string]any{"source": "mw"},
	)
	require.NoError(t, err)

	changedPayload, err := ComputeBlockContentHash(
		"tool_call",
		"",
		map[string]any{"name": "weather", "args": map[string]any{"city": "Berlin"}},
		map[string]any{"source": "mw"},
	)
	require.NoError(t, err)
	require.NotEqual(t, base, changedPayload)

	changedMetadata, err := ComputeBlockContentHash(
		"tool_call",
		"",
		map[string]any{"name": "weather", "args": map[string]any{"city": "Paris"}},
		map[string]any{"source": "other"},
	)
	require.NoError(t, err)
	require.NotEqual(t, base, changedMetadata)
}

func TestCanonicalBlockMaterialJSON_NormalizesNonStringMapKeys(t *testing.T) {
	payload := map[string]any{
		"map_any": map[any]any{
			1:   "one",
			"b": true,
		},
	}

	b, err := CanonicalBlockMaterialJSON("other", "", payload, nil)
	require.NoError(t, err)
	require.Contains(t, string(b), `"1":"one"`)
	require.Contains(t, string(b), `"b":true`)
}
