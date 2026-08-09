//go:build webui

// Package webui exposes the embedded directory preview application.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedDistribution embed.FS

// Content returns the Vite production distribution embedded at build time.
func Content() fs.FS {
	content, err := fs.Sub(embeddedDistribution, "dist")
	if err != nil {
		panic(err)
	}
	return content
}
