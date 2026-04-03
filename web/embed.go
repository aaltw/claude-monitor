package webfs

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var embedFS embed.FS

// FS returns the static files filesystem rooted at the static/ directory.
func FS() (fs.FS, error) {
	return fs.Sub(embedFS, "static")
}
