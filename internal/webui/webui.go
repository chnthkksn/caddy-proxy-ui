// Package webui serves the embedded Svelte SPA, falling back to index.html
// for client-side routes so the app's own router can handle them.
package webui

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"caddy-proxy-ui/web"
)

func Handler() (http.Handler, error) {
	dist, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath != "" && cleanPath != "." {
			if _, err := fs.Stat(dist, cleanPath); err != nil {
				r = cloneWithPath(r, "/")
				cleanPath = ""
			}
		}

		// Vite's asset filenames are content-hashed (assets/foo-<hash>.js) —
		// safe to cache forever, since a content change means a new
		// filename. Everything else (index.html, the SPA fallback) must
		// always be revalidated, or a browser could keep serving a stale
		// index.html pointing at asset hashes from a previous build.
		if strings.HasPrefix(cleanPath, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}

		fileServer.ServeHTTP(w, r)
	}), nil
}

func cloneWithPath(r *http.Request, urlPath string) *http.Request {
	clone := r.Clone(r.Context())
	clone.URL.Path = urlPath
	return clone
}
