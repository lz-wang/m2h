package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/lz-wang/m2h/internal/assets"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
	webui "github.com/lz-wang/m2h/web"
)

type fileSummary struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

type fileListResponse struct {
	Files       []fileSummary `json:"files"`
	DefaultPath string        `json:"defaultPath"`
}

type documentResponse struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	HTML  string `json:"html"`
}

type directoryHandler struct {
	root       string
	mode       markdown.Mode
	unsafeHTML bool
	discovery  files.DiscoverOptions
	discover   func(context.Context, string, files.DiscoverOptions) (files.Discovery, error)
	ui         fs.FS
}

func newDirectoryHandler(
	root string,
	mode markdown.Mode,
	unsafeHTML bool,
	discovery files.DiscoverOptions,
	events *eventHub,
	logger io.Writer,
) http.Handler {
	handler := &directoryHandler{
		root:       root,
		mode:       mode,
		unsafeHTML: unsafeHTML,
		discovery:  discovery,
		discover:   files.Discover,
		ui:         webui.Content(),
	}
	return handler.routes(events, logger)
}

func (handler *directoryHandler) routes(events *eventHub, logger io.Writer) http.Handler {
	if handler.ui == nil {
		handler.ui = webui.Content()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/files", requireGET(handler.serveFiles))
	mux.HandleFunc("/api/document", requireGET(handler.serveDocument))
	mux.Handle("/api/events", events)
	mux.HandleFunc("/api/", jsonNotFound)
	mux.Handle("/assets/", newAssetHandler(handler.root))
	mux.HandleFunc("/ui/markdown.css", handler.serveMarkdownStyles)
	mux.Handle("/ui/", http.StripPrefix("/ui/", immutableUIAssets(http.FileServer(http.FS(handler.ui)))))
	mux.HandleFunc("/doc/", handler.serveDirectoryIndex)
	mux.HandleFunc("/", handler.serveDirectoryIndex)
	return requestLogger(mux, logger)
}

func (handler *directoryHandler) serveFiles(response http.ResponseWriter, request *http.Request) {
	if request.URL.RawQuery != "" {
		writeJSONError(response, http.StatusBadRequest, "query parameters are not supported")
		return
	}
	discovered, err := handler.discover(request.Context(), handler.root, handler.discovery)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "discover Markdown files")
		return
	}

	summaries := make([]fileSummary, 0, len(discovered.Markdown))
	for _, entry := range discovered.Markdown {
		contents, err := os.ReadFile(entry.AbsolutePath)
		if err != nil {
			writeJSONError(response, http.StatusInternalServerError, "read Markdown file")
			return
		}
		title, err := markdown.Title(contents, entry.RelativePath)
		if err != nil {
			writeJSONError(response, http.StatusInternalServerError, "extract Markdown title")
			return
		}
		summaries = append(summaries, fileSummary{
			Path:  entry.RelativePath,
			Name:  path.Base(entry.RelativePath),
			Title: title,
		})
	}
	writeJSON(response, http.StatusOK, fileListResponse{
		Files:       summaries,
		DefaultPath: defaultDocument(summaries),
	})
}

func (handler *directoryHandler) serveDocument(response http.ResponseWriter, request *http.Request) {
	values, exists := request.URL.Query()["path"]
	if !exists || len(values) != 1 || len(request.URL.Query()) != 1 {
		writeJSONError(response, http.StatusBadRequest, "exactly one path query parameter is required")
		return
	}
	relative, err := safeRelativePath(values[0])
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "invalid document path")
		return
	}
	if !files.IsMarkdown(relative) || !files.Matches(relative, handler.discovery) {
		writeJSONError(response, http.StatusNotFound, "document not found")
		return
	}
	target, err := resolveRequestFile(handler.root, relative)
	if err != nil {
		writeJSONError(response, http.StatusNotFound, "document not found")
		return
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		writeJSONError(response, http.StatusNotFound, "document not found")
		return
	}
	rendered, err := markdown.Render(contents, markdown.RenderOptions{
		Mode:       handler.mode,
		Target:     markdown.TargetPreview,
		SourcePath: relative,
		UnsafeHTML: handler.unsafeHTML,
	})
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "render Markdown document")
		return
	}
	writeJSON(response, http.StatusOK, documentResponse{
		Path:  relative,
		Title: rendered.Title,
		HTML:  rendered.Body,
	})
}

func (handler *directoryHandler) serveDirectoryIndex(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request.URL.Path != "/" && !strings.HasPrefix(request.URL.Path, "/doc/") {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-cache")
	if request.Method == http.MethodGet {
		index, err := fs.ReadFile(handler.ui, "index.html")
		if err != nil {
			http.Error(response, "read embedded WebUI", http.StatusInternalServerError)
			return
		}
		_, _ = response.Write(index)
	}
}

func (handler *directoryHandler) serveMarkdownStyles(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mode := string(handler.mode)
	if request.URL.RawQuery != "" {
		values := request.URL.Query()
		provided, exists := values["mode"]
		if !exists || len(values) != 1 || len(provided) != 1 {
			http.Error(response, "invalid mode query", http.StatusBadRequest)
			return
		}
		mode = provided[0]
	}
	stylesheet, err := assets.Stylesheet(mode)
	if err != nil {
		http.Error(response, "invalid mode query", http.StatusBadRequest)
		return
	}
	response.Header().Set("Content-Type", "text/css; charset=utf-8")
	response.Header().Set("Cache-Control", "no-cache")
	if request.Method == http.MethodGet {
		_, _ = io.WriteString(response, stylesheet)
	}
}

func immutableUIAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(response, request)
	})
}

func jsonNotFound(response http.ResponseWriter, _ *http.Request) {
	writeJSONError(response, http.StatusNotFound, "API route not found")
}

func requireGET(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		next(response, request)
	}
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeJSONError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, struct {
		Error string `json:"error"`
	}{Error: message})
}

func defaultDocument(summaries []fileSummary) string {
	for _, preferred := range []string{"README.md", "index.md"} {
		for _, summary := range summaries {
			if summary.Path == preferred {
				return preferred
			}
		}
	}
	if len(summaries) == 0 {
		return ""
	}
	return summaries[0].Path
}

func safeRelativePath(value string) (string, error) {
	for iteration := 0; iteration < 8; iteration++ {
		decoded, err := url.PathUnescape(value)
		if err != nil {
			return "", fmt.Errorf("decode path: %w", err)
		}
		if decoded == value {
			break
		}
		value = decoded
		if iteration == 7 {
			return "", fmt.Errorf("path exceeds decoding limit")
		}
	}
	value = strings.ReplaceAll(value, "\\", "/")
	if value == "" || strings.ContainsRune(value, '\x00') || path.IsAbs(value) || hasWindowsVolume(value) {
		return "", fmt.Errorf("path must be relative")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return "", fmt.Errorf("path escapes root")
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid relative path")
	}
	return cleaned, nil
}

func hasWindowsVolume(value string) bool {
	return len(value) >= 2 && unicode.IsLetter(rune(value[0])) && value[1] == ':'
}

func resolveRequestFile(root, relative string) (string, error) {
	if err := requireExactPath(root, relative); err != nil {
		return "", err
	}
	target, err := files.CanonicalPath(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	if !files.IsWithin(root, target) {
		return "", fmt.Errorf("resolved path escapes root")
	}
	info, err := os.Stat(target)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("path is not a regular file")
	}
	return target, nil
}

func requireExactPath(root, relative string) error {
	current := root
	segments := strings.Split(relative, "/")
	for index, segment := range segments {
		entries, err := os.ReadDir(current)
		if err != nil {
			return err
		}
		found := false
		for _, entry := range entries {
			if entry.Name() == segment {
				if index < len(segments)-1 && entry.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("path traverses a symlink directory")
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("path component %q does not exist with exact case", segment)
		}
		current = filepath.Join(current, segment)
	}
	return nil
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusResponseWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusResponseWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

type flushingStatusResponseWriter struct {
	*statusResponseWriter
	flusher http.Flusher
}

func (writer *flushingStatusResponseWriter) Flush() {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	writer.flusher.Flush()
}

func requestLogger(next http.Handler, logger io.Writer) http.Handler {
	if logger == nil {
		logger = io.Discard
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		writer := &statusResponseWriter{ResponseWriter: response}
		var wrapped http.ResponseWriter = writer
		if flusher, ok := response.(http.Flusher); ok {
			wrapped = &flushingStatusResponseWriter{statusResponseWriter: writer, flusher: flusher}
		}
		next.ServeHTTP(wrapped, request)
		status := writer.status
		if status == 0 {
			status = http.StatusOK
		}
		_, _ = fmt.Fprintf(
			logger,
			"m2h: request method=%s route=%s document=%q status=%d duration=%s\n",
			request.Method,
			requestRoute(request.URL.Path),
			sanitizeLogValue(requestDocumentPath(request)),
			status,
			time.Since(started).Round(time.Microsecond),
		)
	})
}

func requestRoute(requestPath string) string {
	switch {
	case strings.HasPrefix(requestPath, "/doc/"):
		return "/doc/*"
	case strings.HasPrefix(requestPath, "/assets/"):
		return "/assets/*"
	default:
		return requestPath
	}
}

func requestDocumentPath(request *http.Request) string {
	if request.URL.Path == "/api/document" {
		return request.URL.Query().Get("path")
	}
	if strings.HasPrefix(request.URL.Path, "/doc/") {
		return strings.TrimPrefix(request.URL.Path, "/doc/")
	}
	return ""
}

func sanitizeLogValue(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return ' '
		}
		return character
	}, value)
}
