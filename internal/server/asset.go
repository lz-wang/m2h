package server

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
)

// assetHandler serves non-Markdown files for a served workspace. A single
// root keeps the unprefixed /assets/<path> routes; a multi-root workspace
// requires the root id as the first segment (/assets/r0/<path>) so every
// attachment resolves inside its own root's boundary and can never cross into
// another root's tree. Markdown files are deliberately refused.
type assetHandler struct {
	workspace workspace
}

func newAssetHandler(workspace workspace) http.Handler {
	return &assetHandler{workspace: workspace}
}

func (handler *assetHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	virtual, err := assetPath(request.URL)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	root, relative, err := handler.workspace.locate(virtual)
	if err != nil || files.IsMarkdown(relative) {
		http.NotFound(response, request)
		return
	}
	target, err := resolveRequestFile(root.scope.root, relative)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	asset, err := os.Open(target)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	defer asset.Close()
	info, err := asset.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(response, request, info.Name(), info.ModTime(), asset)
}

func assetPath(requestURL *url.URL) (string, error) {
	escaped := requestURL.EscapedPath()
	if !strings.HasPrefix(escaped, "/assets/") {
		return "", fmt.Errorf("invalid asset route")
	}
	return safeRelativePath(strings.TrimPrefix(escaped, "/assets/"))
}
