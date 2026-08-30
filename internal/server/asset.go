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
// another root's tree. Admission goes through the scope's asset policy —
// Markdown, hidden paths and active web documents are refused — before the
// shared filesystem sandbox resolves the file.
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
	if err != nil || !root.scope.allowsAsset(relative) {
		http.NotFound(response, request)
		return
	}
	resolved, err := resolveRequestFile(root.scope.root, relative)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	// The publishing policy runs again on the canonical identity: a
	// harmless-looking alias (safe.txt) whose target is hidden or an active
	// web document (app.js, page.html) must not slip through the first,
	// alias-based check. Assets carry no glob/depth semantics, so the same
	// rule simply applies twice.
	if !root.scope.allowsAsset(resolved.relative) {
		http.NotFound(response, request)
		return
	}
	asset, err := os.Open(resolved.target)
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
	// Attachments can be navigated to directly. A standalone SVG may carry
	// embedded script, and the sandbox CSP keeps it from running as document
	// script when that happens — while <img src="/assets/..."> embedding is
	// unaffected, because a subresource load ignores the response's own CSP.
	response.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	http.ServeContent(response, request, info.Name(), info.ModTime(), asset)
}

func assetPath(requestURL *url.URL) (string, error) {
	escaped := requestURL.EscapedPath()
	if !strings.HasPrefix(escaped, "/assets/") {
		return "", fmt.Errorf("invalid asset route")
	}
	return files.DecodeRelativePath(strings.TrimPrefix(escaped, "/assets/"))
}
