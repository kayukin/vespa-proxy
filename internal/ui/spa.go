package ui

import (
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// SpaHandler serves a Single Page Application.
//
// Rules:
//   - Paths with a file extension (e.g. /assets/vendor-abc123.js) are served
//     as-is. If the file is missing the client gets a proper 404 — the browser
//     never receives an HTML document for a JS/CSS/wasm module import, which
//     would trigger a strict MIME-type error.
//   - Paths without an extension (e.g. /search, /document/id) fall back to
//     index.html so that client-side routing works correctly.
func SpaHandler(staticFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fs.FS paths must NOT start with '/'; clean and strip it.
		fsPath := strings.TrimLeft(path.Clean(r.URL.Path), "/")
		if fsPath == "" {
			fsPath = "."
		}

		_, err := staticFS.Open(fsPath)
		if err == nil {
			// File exists — let FileServer handle it with the correct MIME type.
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found.
		// If the path carries a file extension it is a genuine missing asset
		// (JS chunk, CSS, font, wasm, …). Return 404 so that the browser gets
		// an accurate error instead of an HTML page with the wrong MIME type.
		if path.Ext(fsPath) != "" {
			http.NotFound(w, r)
			return
		}

		// No extension → treat as an SPA client-side route and serve index.html.
		r2 := r.Clone(r.Context())
		r2.URL = &url.URL{Path: "/"}
		fileServer.ServeHTTP(w, r2)
	})
}
