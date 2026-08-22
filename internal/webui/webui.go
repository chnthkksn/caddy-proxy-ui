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
			}
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}

func cloneWithPath(r *http.Request, urlPath string) *http.Request {
	clone := r.Clone(r.Context())
	clone.URL.Path = urlPath
	return clone
}
