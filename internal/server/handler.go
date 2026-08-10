package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

type singleFileHandler struct {
	source     string
	root       string
	mode       markdown.Mode
	width      markdown.Width
	unsafeHTML bool
}

func newSingleFileHandler(source string, mode markdown.Mode, unsafeHTML bool, events *eventHub) http.Handler {
	return newSingleFileHandlerWithWidth(source, mode, markdown.WidthStandard, unsafeHTML, events)
}

func newSingleFileHandlerWithWidth(source string, mode markdown.Mode, width markdown.Width, unsafeHTML bool, events *eventHub) http.Handler {
	if canonical, err := files.CanonicalPath(source); err == nil {
		source = canonical
	}
	handler := &singleFileHandler{
		source:     source,
		root:       filepath.Dir(source),
		mode:       mode,
		width:      width,
		unsafeHTML: unsafeHTML,
	}
	mux := http.NewServeMux()
	mux.Handle("/api/events", events)
	mux.Handle("/assets/", newAssetHandler(handler.root))
	mux.HandleFunc("/", handler.serveDocument)
	return mux
}

func (handler *singleFileHandler) serveDocument(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contents, err := os.ReadFile(handler.source)
	if err != nil {
		http.Error(response, fmt.Sprintf("read Markdown: %v", err), http.StatusInternalServerError)
		return
	}
	rendered, err := markdown.Render(contents, markdown.RenderOptions{
		Mode:       handler.mode,
		Width:      handler.width,
		Target:     markdown.TargetPreview,
		SourcePath: filepath.Base(handler.source),
		UnsafeHTML: handler.unsafeHTML,
	})
	if err != nil {
		http.Error(response, fmt.Sprintf("render Markdown: %v", err), http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if request.Method == http.MethodGet {
		_, _ = io.WriteString(response, rendered.HTML)
	}
}

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
