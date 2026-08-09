//go:build !webui

// Package webui exposes the embedded directory preview application.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed fallback
var embeddedFallback embed.FS

// Content returns a source-tree fallback for ordinary Go tests.
func Content() fs.FS {
	content, err := fs.Sub(embeddedFallback, "fallback")
	if err != nil {
		panic(err)
	}
	return content
}
