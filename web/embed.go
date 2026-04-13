package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticFS embed.FS

// Static returns the embedded filesystem rooted at the static/ directory.
func Static() fs.FS {
	sub, _ := fs.Sub(staticFS, "static")
	return sub
}
