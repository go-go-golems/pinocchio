//go:build !embed

package spa

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing/fstest"
)

// Assets serves the SPA frontend when the binary was built with -tags embed.
// Without the tag, it falls back to a disk search and then a placeholder.
var Assets fs.FS = findAssets()

func findAssets() fs.FS {
	// Try to find assets on disk (dev builds where make fetch-spa was run).
	wd, _ := os.Getwd()
	for dir := wd; dir != ""; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "pkg", "spa", "dist", "index.html")
		if _, err := os.Stat(candidate); err == nil {
			return os.DirFS(filepath.Join(dir, "pkg", "spa", "dist"))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	// Placeholder for builds without SPA assets.
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<!doctype html>
<html lang="en">
  <head><meta charset="UTF-8" /><title>Pinocchio Help Browser</title></head>
  <body><div id="root">
    Pinocchio help browser assets not found.
    Run <code>make fetch-spa</code> and rebuild with <code>-tags embed</code>
    for the full browser UI.
  </div></body>
</html>`)},
	}
}

func init() {
	// Provide a helpful message if assets are a placeholder.
	if _, ok := Assets.(*fstest.MapFS); ok {
		fmt.Fprintf(os.Stderr, "NOTE: SPA assets not found. Serving placeholder. Run 'make fetch-spa' and rebuild with -tags embed.\n")
	}
}
