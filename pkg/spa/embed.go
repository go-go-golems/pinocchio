//go:build embed

package spa

import (
	"embed"
	"io/fs"
)

// embeddedAssets contains the Glazed help browser SPA frontend files under
// the embedded dist/ directory. Assets exposes dist/ as the filesystem root so
// the HTTP handler can read index.html directly.
//
//go:embed dist
var embeddedAssets embed.FS

// Assets serves the fetched Glazed help browser SPA frontend files.
var Assets fs.FS = mustSub(embeddedAssets, "dist")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
