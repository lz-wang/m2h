package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lz-wang/m2h/internal/assets"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

// singleFileHandler renders one Markdown file as a standalone HTML document.
//
// TODO: preview is being unified onto the React WebUI (previewHandler). Once
// that migration lands, this standalone handler and its full-HTML render path
// will be removed.
type singleFileHandler struct {
	source string
	root   string
	mode   markdown.Mode
	width  markdown.Width
}

func newSingleFileHandler(source string, mode markdown.Mode, events *eventHub) http.Handler {
	return newSingleFileHandlerWithWidth(source, mode, markdown.WidthStandard, events)
}

func newSingleFileHandlerWithWidth(source string, mode markdown.Mode, width markdown.Width, events *eventHub) http.Handler {
	if canonical, err := files.CanonicalPath(source); err == nil {
		source = canonical
	}
	handler := &singleFileHandler{
		source: source,
		root:   filepath.Dir(source),
		mode:   mode,
		width:  width,
	}
	mux := http.NewServeMux()
	mux.Handle("/api/events", events)
	mux.Handle("/assets/", newAssetHandler(handler.root))
	mux.Handle(assets.RichServePrefix, http.StripPrefix(assets.RichServePrefix, http.FileServer(http.FS(assets.RichFS()))))
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
		AssetBase:  assets.RichServePrefix,
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
