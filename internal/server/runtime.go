package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/lz-wang/m2h/internal/assets"
)

// newRuntimeHandler serves the Mermaid and KaTeX runtime embedded in the Go
// binary, so the WebUI renders rich content offline at the same pinned
// releases convert loads from the CDN instead of bundling a second copy
// through Vite. Directory requests are refused so the embedded file server
// never renders a listing; path traversal is rejected by the embedded
// filesystem itself.
func newRuntimeHandler() http.Handler {
	files := http.FileServer(http.FS(assets.RichFS()))
	return http.StripPrefix("/runtime/", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		relative := request.URL.Path
		if relative == "" || strings.HasSuffix(relative, "/") {
			http.NotFound(response, request)
			return
		}
		if info, err := fs.Stat(assets.RichFS(), relative); err == nil && info.IsDir() {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(response, request)
	}))
}
