// Package web embeds the built Svelte frontend (dist/, produced by
// `npm run build` in this directory). It's kept separate from
// internal/webui because a go:embed pattern can't reach outside its own
// package directory.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
