package cmds

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImagePathsToTurnImages_LocalFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := filepath.Join(dir, "img.png")
	require.NoError(t, os.WriteFile(p, []byte("not-a-real-png-but-extension-is-enough"), 0o644))

	imgs, err := imagePathsToTurnImages([]string{p})
	require.NoError(t, err)
	require.Len(t, imgs, 1)

	require.Equal(t, "image/png", imgs[0]["media_type"])
	require.Equal(t, []byte("not-a-real-png-but-extension-is-enough"), imgs[0]["content"])
}

func TestImagePathsToTurnImages_UnsupportedExtension(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := filepath.Join(dir, "img.txt")
	require.NoError(t, os.WriteFile(p, []byte("hello"), 0o644))

	_, err := imagePathsToTurnImages([]string{p})
	require.Error(t, err)
}

func TestImagePathsToTurnImages_URL(t *testing.T) {
	t.Parallel()

	imgs, err := imagePathsToTurnImages([]string{"https://example.com/image.png"})
	require.NoError(t, err)
	require.Len(t, imgs, 1)

	// Best-effort: media_type inferred from extension.
	require.Equal(t, "image/png", imgs[0]["media_type"])
	require.Equal(t, "https://example.com/image.png", imgs[0]["url"])
}
