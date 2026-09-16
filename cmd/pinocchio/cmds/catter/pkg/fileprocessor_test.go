package pkg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/clay/pkg/filefilter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessPaths_DefaultFilters exercises the default file filter end-to-end
// through the catter FileProcessor. Regression test for the old substring-based
// directory matching: a directory named "builder-api" must NOT be excluded by
// the default "build" pattern, while a directory literally named "build" stays
// excluded. Requires clay >= v0.4.14.
func TestProcessPaths_DefaultFilters(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "builder-api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "build"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "builder-api", "main.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build", "artifact.txt"), []byte("x\n"), 0o644))

	fp := NewFileProcessor(WithFileFilter(filefilter.NewFileFilter()))
	require.NoError(t, fp.ProcessPaths([]string{dir}))

	assert.Contains(t, fp.TokenCounts, filepath.Join(dir, "builder-api", "main.go"),
		"builder-api must not be excluded by the default build pattern")
	assert.NotContains(t, fp.TokenCounts, filepath.Join(dir, "build", "artifact.txt"),
		"build must stay excluded by default")
}

// TestProcessPaths_IncludeDirsWins verifies include-dirs overrides exclude-dirs
// through the catter FileProcessor.
func TestProcessPaths_IncludeDirsWins(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "build"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build", "artifact.txt"), []byte("x\n"), 0o644))

	ff := filefilter.NewFileFilter(
		filefilter.WithExcludeDirs([]string{"build"}),
		filefilter.WithIncludeDirs([]string{"build"}),
	)
	fp := NewFileProcessor(WithFileFilter(ff))
	require.NoError(t, fp.ProcessPaths([]string{dir}))

	assert.Contains(t, fp.TokenCounts, filepath.Join(dir, "build", "artifact.txt"),
		"include-dirs must beat exclude-dirs")
}
