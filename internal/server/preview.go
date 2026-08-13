package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
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
	appversion "github.com/lz-wang/m2h/internal/version"
)

type fileSummary struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

type fileListResponse struct {
	Kind        previewKind   `json:"kind"`
	Files       []fileSummary `json:"files"`
	DefaultPath string        `json:"defaultPath"`
	Version     string        `json:"version"`
}

type frontMatterEntryResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type frontMatterResponse struct {
	Entries []frontMatterEntryResponse `json:"entries"`
	Date    string                     `json:"date,omitempty"`
	Tags    []string                   `json:"tags,omitempty"`
}

type tocEntryResponse struct {
	Level int    `json:"level"`
	ID    string `json:"id"`
	Text  string `json:"text"`
}

type documentResponse struct {
	Path        string               `json:"path"`
	Title       string               `json:"title"`
	HTML        string               `json:"html"`
	FrontMatter *frontMatterResponse `json:"frontmatter"`
	TOC         []tocEntryResponse   `json:"toc"`
}

// previewHandler serves the unified React WebUI and its JSON API for both
// single-file and directory preview. The only difference between the two is the
// previewScope, which decides which Markdown files exist and are addressable.
type previewHandler struct {
	scope   previewScope
	mode    markdown.Mode
	width   markdown.Width
	ui      fs.FS
	version string

	discover func(context.Context, previewScope) (files.Discovery, error)
}

func newPreviewHandler(
	scope previewScope,
	mode markdown.Mode,
	width markdown.Width,
	events *eventHub,
	logger io.Writer,
	ui fs.FS,
) http.Handler {
	return newPreviewHandlerWithVersion(scope, mode, width, events, logger, ui, appversion.Development)
}

func newPreviewHandlerWithVersion(
	scope previewScope,
	mode markdown.Mode,
	width markdown.Width,
	events *eventHub,
	logger io.Writer,
	ui fs.FS,
	buildVersion string,
) http.Handler {
	handler := &previewHandler{
		scope:   scope,
		mode:    mode,
		width:   width,
		ui:      ui,
		version: buildVersion,
		discover: func(ctx context.Context, scope previewScope) (files.Discovery, error) {
			return scope.discover(ctx)
		},
	}
	return handler.routes(events, logger)
}

func (handler *previewHandler) routes(events *eventHub, logger io.Writer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/files", requireGET(handler.serveFiles))
	mux.HandleFunc("/api/document", requireGET(handler.serveDocument))
	mux.Handle("/api/events", events)
	mux.HandleFunc("/api/", jsonNotFound)
	mux.Handle("/assets/", newAssetHandler(handler.scope.root))
	mux.Handle("/runtime/", newRuntimeHandler())
	mux.HandleFunc("/ui/markdown.css", handler.serveMarkdownStyles)
	mux.Handle("/ui/", http.StripPrefix("/ui/", immutableUIAssets(http.FileServer(http.FS(handler.ui)))))
	mux.HandleFunc("/doc/", handler.serveDirectoryIndex)
	mux.HandleFunc("/", handler.serveDirectoryIndex)
	return requestLogger(mux, logger)
}

func (handler *previewHandler) serveFiles(response http.ResponseWriter, request *http.Request) {
	if request.URL.RawQuery != "" {
		writeJSONError(response, http.StatusBadRequest, "query parameters are not supported")
		return
	}
	discovered, err := handler.discover(request.Context(), handler.scope)
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
		Kind:        handler.scope.kind(),
		Files:       summaries,
		DefaultPath: defaultDocument(summaries),
		Version:     handler.version,
	})
}

func (handler *previewHandler) serveDocument(response http.ResponseWriter, request *http.Request) {
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
	if !handler.scope.allowsDocument(relative) {
		writeJSONError(response, http.StatusNotFound, "document not found")
		return
	}
	target, err := resolveRequestFile(handler.scope.root, relative)
	if err != nil {
		writeJSONError(response, http.StatusNotFound, "document not found")
		return
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		writeJSONError(response, http.StatusNotFound, "document not found")
		return
	}
	body, frontMatter, err := markdown.ParseFrontMatter(contents)
	if err != nil {
		writeJSONError(
			response,
			http.StatusUnprocessableEntity,
			fmt.Sprintf("invalid frontmatter: %v", err),
		)
		return
	}
	rendered, err := markdown.Render(body, markdown.RenderOptions{
		Mode:       handler.mode,
		Width:      handler.width,
		Target:     markdown.TargetPreview,
		SourcePath: relative,
	})
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "render Markdown document")
		return
	}
	writeJSON(response, http.StatusOK, documentResponse{
		Path:        relative,
		Title:       rendered.Title,
		HTML:        rendered.Body,
		FrontMatter: frontMatterResponseFrom(frontMatter),
		TOC:         tocEntriesFrom(rendered.Headings),
	})
}

func (handler *previewHandler) serveDirectoryIndex(response http.ResponseWriter, request *http.Request) {
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

func (handler *previewHandler) serveMarkdownStyles(response http.ResponseWriter, request *http.Request) {
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

func tocEntriesFrom(headings []markdown.Heading) []tocEntryResponse {
	entries := make([]tocEntryResponse, 0, len(headings))
	for _, heading := range headings {
		entries = append(entries, tocEntryResponse{
			Level: heading.Level,
			ID:    heading.ID,
			Text:  heading.Text,
		})
	}
	return entries
}

func frontMatterResponseFrom(frontMatter *markdown.FrontMatter) *frontMatterResponse {
	if frontMatter == nil {
		return nil
	}
	entries := make([]frontMatterEntryResponse, 0, len(frontMatter.Entries))
	for _, entry := range frontMatter.Entries {
		entries = append(entries, frontMatterEntryResponse{
			Key:   entry.Key,
			Value: entry.Value,
		})
	}
	return &frontMatterResponse{
		Entries: entries,
		Date:    frontMatter.Date,
		Tags:    frontMatter.Tags,
	}
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
		_, _ = fmt.Fprintf(logger, "%s | %s [%s] %s [%d] %.1fms\n",
			started.Format("2006-01-02 15:04:05"),
			sanitizeLogValue(clientIP(request.RemoteAddr)),
			sanitizeLogValue(request.Method),
			sanitizeLogValue(request.URL.RequestURI()),
			status,
			float64(time.Since(started).Microseconds())/1000,
		)
	})
}

func clientIP(remoteAddress string) string {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err == nil {
		return host
	}
	return remoteAddress
}

func sanitizeLogValue(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return ' '
		}
		return character
	}, value)
}
