// Package ui exposes the pre-built Vespa UI as an embedded filesystem.
//
// The static files are placed into the "static" directory at build time by
// the Dockerfile, which runs "yarn build" inside the cloned Vespa repository
// and copies the output here before "go build" runs.
package ui

import (
	"embed"
	"io/fs"
)

// Static holds the compiled Vespa client application.
// The "static" directory is populated by the Dockerfile before the Go binary
// is compiled, so it will be empty during plain "go build" outside Docker
// unless you manually populate it first.
//
//go:embed static
var Static embed.FS

// StaticFS returns the sub-filesystem rooted at "static/",
// so paths like "index.html" work without the "static/" prefix.
func StaticFS() (fs.FS, error) {
	return fs.Sub(Static, "static")
}
