package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
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
	Path        string `json:"path"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// rootSummary groups one workspace root's documents. Files carry root-relative
// paths; the id prefixes the addressable (virtual) path in a multi-root
// workspace (see workspace.publicPath). The summary deliberately carries only
// what the WebUI needs to navigate: the server's filesystem layout (absolute
// paths, platform separators) never crosses this boundary, so nothing about
// the serving machine leaks to any listener.
type rootSummary struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Files []fileSummary `json:"files"`
}

type fileListResponse struct {
	Kind    workspaceKind `json:"kind"`
	Roots   []rootSummary `json:"roots"`
	Version string        `json:"version"`
}

type frontMatterEntryResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type frontMatterResponse struct {
	Entries []frontMatterEntryResponse `json:"entries"`
	Created string                     `json:"created,omitempty"`
	Updated string                     `json:"updated,omitempty"`
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

// documentHandler serves the unified React WebUI and its JSON API for every
// workspace shape. The workspace decides which Markdown files exist and
// are addressable: one root behaves exactly like the historical single-input// serving, several roots address documents through virtual root-prefixed
// paths while each root keeps its own access boundary.
type documentHandler struct {
	workspace workspace
	ui        fs.FS
	version   string

	discover func(context.Context, rootScope) (files.Discovery, error)
}

func newDocumentHandler(workspace workspace, logger io.Writer, ui fs.FS) http.Handler {
	return newDocumentHandlerWithVersion(workspace, logger, ui, appversion.Development)
}

func newDocumentHandlerWithVersion(
	workspace workspace,
	logger io.Writer,
	ui fs.FS,
	buildVersion string,
) http.Handler {
	handler := &documentHandler{
		workspace: workspace,
		ui:        ui,
		version:   buildVersion,
		discover: func(ctx context.Context, scope rootScope) (files.Discovery, error) {
			return scope.discover(ctx)
		},
	}
	return handler.routes(logger)
}

func (handler *documentHandler) routes(logger io.Writer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.serveHealth)
	mux.HandleFunc("/api/files", requireGET(handler.serveFiles))
	mux.HandleFunc("/api/document", requireGET(handler.serveDocument))
	mux.HandleFunc("/api/", jsonNotFound)
	mux.Handle("/assets/", newAssetHandler(handler.workspace))
	mux.Handle("/runtime/", newRuntimeHandler())
	mux.HandleFunc("/ui/markdown.css", handler.serveMarkdownStyles)
	mux.Handle("/ui/", http.StripPrefix("/ui/", immutableUIAssets(http.FileServer(http.FS(handler.ui)))))
	mux.HandleFunc(markdown.InvalidLocalReferencePath, http.NotFound)
	mux.HandleFunc("/doc/", handler.serveDirectoryIndex)
	mux.HandleFunc("/raw/", handler.serveRawMarkdown)
	mux.HandleFunc("/", handler.serveDirectoryIndex)
	// requestLogger → securityHeaders → mux: the log sees the final status,
	// and securityHeaders sits directly above the mux so every response —
	// pages, APIs, assets, runtime, 404s — gets the same hardening baseline
	// while handlers keep the ability to override individual headers.
	return requestLogger(securityHeaders(mux), logger)
}

func (handler *documentHandler) serveFiles(response http.ResponseWriter, request *http.Request) {
	if request.URL.RawQuery != "" {
		writeJSONError(response, http.StatusBadRequest, "query parameters are not supported")
		return
	}
	roots := make([]rootSummary, 0, handler.workspace.rootCount())
	for _, root := range handler.workspace.roots {
		discovered, err := handler.discover(request.Context(), root.scope)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				discovered = files.Discovery{}
			} else {
				writeJSONError(response, http.StatusInternalServerError, "discover Markdown files")
				return
			}
		}

		summaries := make([]fileSummary, 0, len(discovered.Markdown))
		for _, entry := range discovered.Markdown {
			contents, err := os.ReadFile(entry.AbsolutePath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				writeJSONError(response, http.StatusInternalServerError, "read Markdown file")
				return
			}
			metadata, err := fileDisplayMetadata(contents, entry.RelativePath)
			if err != nil {
				writeJSONError(response, http.StatusInternalServerError, "extract Markdown title")
				return
			}
			summaries = append(summaries, fileSummary{
				Path:        entry.RelativePath,
				Name:        path.Base(entry.RelativePath),
				Title:       metadata.Title,
				Description: metadata.Description,
			})
		}
		roots = append(roots, rootSummary{
			ID:    root.id,
			Name:  root.label,
			Files: summaries,
		})
	}
	writeJSON(response, http.StatusOK, fileListResponse{
		Kind:    handler.workspace.kind(),
		Roots:   roots,
		Version: handler.version,
	})
}

func (handler *documentHandler) serveDocument(response http.ResponseWriter, request *http.Request) {
	values, exists := request.URL.Query()["path"]
	if !exists || len(values) != 1 || len(request.URL.Query()) != 1 {
		writeJSONError(response, http.StatusBadRequest, "exactly one path query parameter is required")
		return
	}
	virtual, err := files.DecodeRelativePath(values[0])
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "invalid document path")
		return
	}
	root, relative, target, err := handler.resolveVisibleDocument(virtual)
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
		URLMode:  markdown.URLWeb,
		RootPath: handler.workspace.publicRoot(root.id),
		// Rendering resolves relative Markdown links and attachments against
		// the virtual (public) path. RootPath anchors both document-relative
		// and workspace-root-relative destinations inside the current root,
		// routing them through /doc/<root>/<...> or /assets/<root>/<...>
		// without allowing a sibling root to become the resolution base.
		SourcePath: virtual,
	})
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "render Markdown document")
		return
	}
	writeJSON(response, http.StatusOK, documentResponse{
		Path:        handler.workspace.publicPath(root.id, relative),
		Title:       markdown.PreferredTitle(frontMatter, rendered.Title),
		HTML:        rendered.Body,
		FrontMatter: frontMatterResponseFrom(frontMatter),
		TOC:         tocEntriesFrom(rendered.Headings),
	})
}

// fileDisplayInfo is the file-list entry's display metadata, resolved in one
// frontmatter parse: the title (a valid frontmatter title outranks the first
// H1) and the description tooltip text. An invalid frontmatter block must not
// fail the whole listing — opening that document surfaces the 422, the
// sidebar should still list it — so the invalid block falls back to plain
// first-H1/filename extraction over the full source, with no description.
type fileDisplayInfo struct {
	Title       string
	Description string
}

func fileDisplayMetadata(contents []byte, relativePath string) (fileDisplayInfo, error) {
	body, frontMatter, err := markdown.ParseFrontMatter(contents)
	if err != nil {
		title, titleErr := markdown.Title(contents, relativePath)
		return fileDisplayInfo{Title: title}, titleErr
	}
	fallback, err := markdown.Title(body, relativePath)
	if err != nil {
		return fileDisplayInfo{}, err
	}
	metadata := fileDisplayInfo{
		Title: markdown.PreferredTitle(frontMatter, fallback),
	}
	if frontMatter != nil {
		metadata.Description = frontMatter.Description
	}
	return metadata, nil
}

// resolveVisibleDocument maps an addressable (virtual) document path onto its
// root, root-relative path and absolute file target. It is the one filesystem
// boundary shared by /api/document and /raw/ so both entrances can never drift
// apart: unknown roots, filtered or non-Markdown files, traversal and symlink
// escapes all fail here. A file symlink whose canonical target is hidden is
// refused here too — the publishing policy runs on the requested identity and
// again on the canonical one. Callers keep their own HTTP error shape and
// rendering.
func (handler *documentHandler) resolveVisibleDocument(virtual string) (workspaceRoot, string, string, error) {
	root, relative, err := handler.workspace.locate(virtual)
	if err != nil {
		return workspaceRoot{}, "", "", err
	}
	if !root.scope.allowsDocument(relative) {
		return workspaceRoot{}, "", "", fmt.Errorf("document %q is not served by its root", virtual)
	}
	resolved, err := resolveRequestFile(root.scope.root, relative)
	if err != nil {
		return workspaceRoot{}, "", "", err
	}
	if !root.scope.allowsResolvedDocument(resolved.relative) {
		return workspaceRoot{}, "", "", fmt.Errorf("document %q resolves to a hidden target", virtual)
	}
	return root, relative, resolved.target, nil
}

// serveRawMarkdown streams the original Markdown source of one addressable
// document: GET /raw/<virtual-path> is a stable, browser-addressable URL for
// sharing and future integrations (external editors, downloads), not just an
// internal fetch helper. Resolution goes through resolveVisibleDocument, so
// the whole /api/document access boundary applies unchanged.
func (handler *documentHandler) serveRawMarkdown(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request.URL.RawQuery != "" {
		http.Error(response, "query parameters are not supported", http.StatusBadRequest)
		return
	}
	virtual, err := files.DecodeRelativePath(strings.TrimPrefix(request.URL.Path, "/raw/"))
	if err != nil {
		http.Error(response, "invalid document path", http.StatusBadRequest)
		return
	}
	_, _, target, err := handler.resolveVisibleDocument(virtual)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	if request.Method == http.MethodHead {
		return
	}
	_, _ = response.Write(contents)
}

func (handler *documentHandler) serveDirectoryIndex(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request.URL.Path != "/" && !strings.HasPrefix(request.URL.Path, "/doc/") {
		http.NotFound(response, request)
		return
	}
	index, err := fs.ReadFile(handler.ui, "index.html")
	if err != nil {
		http.Error(response, "read embedded WebUI", http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	if strings.HasPrefix(request.URL.Path, "/doc/") {
		virtual, err := files.DecodeRelativePath(strings.TrimPrefix(request.URL.Path, "/doc/"))
		if err != nil {
			http.NotFound(response, request)
			return
		}
		if _, _, _, err := handler.resolveVisibleDocument(virtual); err != nil {
			status = http.StatusNotFound
		}
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-cache")
	response.WriteHeader(status)
	if request.Method == http.MethodGet {
		_, _ = response.Write(index)
	}
}

func (handler *documentHandler) serveMarkdownStyles(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// The WebUI stylesheet is a single mode-independent resource: light and
	// dark palettes coexist, selected by the html.dark class the client already
	// toggles. The URL never changes, so a theme switch triggers no new request.
	// Reject any query to keep the resource address stable and cache-friendly.
	if request.URL.RawQuery != "" {
		http.Error(response, "query parameters are not supported", http.StatusBadRequest)
		return
	}
	stylesheet := assets.WebStylesheet()
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
		Created: frontMatter.CreatedDate,
		Updated: frontMatter.UpdatedDate,
		Date:    frontMatter.Date,
		Tags:    frontMatter.Tags,
	}
}

// resolveRequestFile maps a decoded root-relative request path onto the exact
// resolvedRequestFile pairs the absolute file the workspace would serve with
// its canonical target path relative to the root. The two diverge exactly
// when the request named a file symlink: the requested identity keeps the
// alias path, the canonical identity names what is actually read. Publishing
// policy is applied to both, while this resolution itself understands none.
type resolvedRequestFile struct {
	target   string
	relative string
}

// resolveRequestFile applies the shared filesystem boundary: exact-case
// components, no symlink directories, canonical resolution inside the root
// and a regular-file requirement.
func resolveRequestFile(root, relative string) (resolvedRequestFile, error) {
	if err := files.RequireExactPath(root, relative); err != nil {
		return resolvedRequestFile{}, err
	}
	target, err := files.CanonicalPath(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return resolvedRequestFile{}, err
	}
	if !files.IsWithin(root, target) {
		return resolvedRequestFile{}, fmt.Errorf("resolved path escapes root")
	}
	canonicalRelative, err := filepath.Rel(root, target)
	if err != nil {
		return resolvedRequestFile{}, fmt.Errorf("make %q relative to %q: %w", target, root, err)
	}
	info, err := os.Stat(target)
	if err != nil || !info.Mode().IsRegular() {
		return resolvedRequestFile{}, fmt.Errorf("path is not a regular file")
	}
	return resolvedRequestFile{
		target:   target,
		relative: files.NormalizeRelativePath(canonicalRelative),
	}, nil
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
