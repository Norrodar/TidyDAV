// Package ui embeds the built SvelteKit single-page app and serves it.
//
// The frontend is built into ./dist by `npm run build` (see web/svelte.config.js)
// and embedded here with the `all:` prefix so that underscore-prefixed files such
// as _app/ are included. A placeholder dist/index.html is kept in version control
// so the package builds without a prior frontend build.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// FS returns the embedded frontend rooted at the dist directory.
func FS() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}

// Handler serves the embedded SPA: an existing static asset is served directly,
// any other path falls back to index.html for client-side routing.
func Handler() (http.Handler, error) {
	sub, err := FS()
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "." {
			if f, err := sub.Open(name); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	}), nil
}
