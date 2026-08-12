package server

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
)

// assetHandler serves non-Markdown files rooted at a preview scope's root, so
// local images and attachments referenced by rendered Markdown resolve in both
// single-file and directory preview. Markdown files are deliberately refused.
type assetHandler struct {
	root string
}

func newAssetHandler(root string) http.Handler {
	if canonical, err := files.CanonicalPath(root); err == nil {
		root = canonical
	}
	return &assetHandler{root: root}
}

func (handler *assetHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relative, err := assetPath(request.URL)
	if err != nil || files.IsMarkdown(relative) {
		http.NotFound(response, request)
		return
	}
	target, err := resolveRequestFile(handler.root, relative)
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
