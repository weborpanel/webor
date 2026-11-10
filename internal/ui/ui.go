// Package ui serves the embedded SvelteKit static panel UI.
package ui

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler returns an HTTP handler that serves embedded static assets with
// SPA fallback to 200.html (SvelteKit adapter-static fallback).
func Handler() http.Handler {
	root, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("ui embed: " + err.Error())
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "." || name == "" {
			if serveFile(w, r, root, "index.html") || serveFile(w, r, root, "200.html") {
				return
			}
			http.NotFound(w, r)
			return
		}
		if serveFile(w, r, root, name) {
			return
		}
		if !strings.Contains(path.Base(name), ".") {
			if serveFile(w, r, root, path.Join(name, "index.html")) {
				return
			}
		}
		if !serveFile(w, r, root, "200.html") {
			http.NotFound(w, r)
		}
	})
}

func serveFile(w http.ResponseWriter, r *http.Request, root fs.FS, name string) bool {
	f, err := root.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return true
	}
	http.ServeContent(w, r, path.Base(name), info.ModTime(), rs)
	return true
}
