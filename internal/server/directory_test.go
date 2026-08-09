package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lz-wang/m2h/internal/assets"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

func TestDirectoryFilesAPIUsesSharedDiscoveryAndTitles(t *testing.T) {
	root := directoryFixture(t)
	canonical := canonicalDirectory(t, root)
	options := files.DiscoverOptions{Depth: 2, Pattern: "**/*.md"}
	var logOutput bytes.Buffer
	handler := newDirectoryHandler(canonical, markdown.ModeAuto, false, options, newEventHub(time.Second), &logOutput)

	response := performRequest(handler, http.MethodGet, "/api/files")
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/files status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload fileListResponse
	decodeJSON(t, response, &payload)

	discovered, err := files.Discover(context.Background(), root+string(os.PathSeparator), options)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := make([]string, 0, len(discovered.Markdown))
	for _, entry := range discovered.Markdown {
		wantPaths = append(wantPaths, entry.RelativePath)
	}
	gotPaths := make([]string, 0, len(payload.Files))
	for _, summary := range payload.Files {
		gotPaths = append(gotPaths, summary.Path)
		if summary.Name != filepath.Base(summary.Path) {
			t.Errorf("summary name for %q = %q", summary.Path, summary.Name)
		}
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("API paths = %v, shared discovery paths = %v", gotPaths, wantPaths)
	}
	if payload.DefaultPath != "README.md" {
		t.Fatalf("defaultPath = %q, want README.md", payload.DefaultPath)
	}
	if titleFor(payload.Files, "design/architecture.md") != "Architecture" {
		t.Fatalf("architecture title = %q", titleFor(payload.Files, "design/architecture.md"))
	}
	if titleFor(payload.Files, "notes.md") != "notes.md" {
		t.Fatalf("fallback title = %q", titleFor(payload.Files, "notes.md"))
	}
	if !strings.Contains(logOutput.String(), `method=GET route=/api/files document="" status=200`) {
		t.Fatalf("request log = %q", logOutput.String())
	}
}

func TestDirectoryFilesAPIDepthGlobRefreshAndDefaultSelection(t *testing.T) {
	root := directoryFixture(t)
	canonical := canonicalDirectory(t, root)
	handlerState := &directoryHandler{
		root:      canonical,
		mode:      markdown.ModeAuto,
		discovery: files.DiscoverOptions{Depth: 1, Pattern: "**/*.md"},
		discover:  files.Discover,
	}
	requests := 0
	handlerState.discover = func(ctx context.Context, root string, options files.DiscoverOptions) (files.Discovery, error) {
		requests++
		return files.Discover(ctx, root, options)
	}
	handler := handlerState.routes(newEventHub(time.Second), nil)

	first := performRequest(handler, http.MethodGet, "/api/files")
	var initial fileListResponse
	decodeJSON(t, first, &initial)
	if requests != 1 {
		t.Fatalf("first tree refresh scanned %d times", requests)
	}
	if containsSummary(initial.Files, "deep/topic/details.md") {
		t.Fatal("depth 1 response contains a depth 2 document")
	}

	writeTestFile(t, filepath.Join(root, "added.md"), "# Added")
	second := performRequest(handler, http.MethodGet, "/api/files")
	var refreshed fileListResponse
	decodeJSON(t, second, &refreshed)
	if requests != 2 {
		t.Fatalf("second tree refresh total scans = %d, want 2", requests)
	}
	if !containsSummary(refreshed.Files, "added.md") {
		t.Fatal("refreshed tree is missing added.md")
	}

	document := performRequest(handler, http.MethodGet, "/api/document?path=added.md")
	if document.Code != http.StatusOK || requests != 2 {
		t.Fatalf("document request status=%d scans=%d, want status 200 without tree scan", document.Code, requests)
	}

	if got := defaultDocument([]fileSummary{{Path: "z.md"}, {Path: "index.md"}, {Path: "README.md"}}); got != "README.md" {
		t.Fatalf("README priority = %q", got)
	}
	if got := defaultDocument([]fileSummary{{Path: "z.md"}, {Path: "index.md"}}); got != "index.md" {
		t.Fatalf("index priority = %q", got)
	}
	if got := defaultDocument([]fileSummary{{Path: "a.md"}, {Path: "z.md"}}); got != "a.md" {
		t.Fatalf("sorted fallback = %q", got)
	}
	if got := defaultDocument(nil); got != "" {
		t.Fatalf("empty fallback = %q", got)
	}
}

func TestDirectoryFilesAPIEmptyAndFailures(t *testing.T) {
	t.Parallel()

	root := canonicalDirectory(t, t.TempDir())
	handlerState := &directoryHandler{
		root:      root,
		mode:      markdown.ModeAuto,
		discovery: files.DiscoverOptions{Depth: 2},
		discover:  files.Discover,
	}
	handler := handlerState.routes(newEventHub(time.Second), nil)
	response := performRequest(handler, http.MethodGet, "/api/files")
	var payload fileListResponse
	decodeJSON(t, response, &payload)
	if payload.Files == nil || len(payload.Files) != 0 || payload.DefaultPath != "" {
		t.Fatalf("empty payload = %+v", payload)
	}

	response = performRequest(handler, http.MethodGet, "/api/files?unexpected=true")
	assertJSONError(t, response, http.StatusBadRequest)
	response = performRequest(handler, http.MethodPost, "/api/files")
	assertJSONError(t, response, http.StatusMethodNotAllowed)
	if response.Header().Get("Allow") != http.MethodGet {
		t.Errorf("POST /api/files Allow = %q", response.Header().Get("Allow"))
	}

	handlerState.discover = func(context.Context, string, files.DiscoverOptions) (files.Discovery, error) {
		return files.Discovery{}, errors.New("scan failed")
	}
	response = performRequest(handler, http.MethodGet, "/api/files")
	assertJSONError(t, response, http.StatusInternalServerError)

	handlerState.discover = func(context.Context, string, files.DiscoverOptions) (files.Discovery, error) {
		return files.Discovery{Markdown: []files.Entry{{AbsolutePath: filepath.Join(root, "missing.md"), RelativePath: "missing.md"}}}, nil
	}
	response = performRequest(handler, http.MethodGet, "/api/files")
	assertJSONError(t, response, http.StatusInternalServerError)

	valid := filepath.Join(root, "valid.md")
	writeTestFile(t, valid, "# Valid")
	handlerState.discover = func(context.Context, string, files.DiscoverOptions) (files.Discovery, error) {
		return files.Discovery{Markdown: []files.Entry{{AbsolutePath: valid, RelativePath: "../invalid.md"}}}, nil
	}
	response = performRequest(handler, http.MethodGet, "/api/files")
	assertJSONError(t, response, http.StatusInternalServerError)
}

func TestDirectoryDocumentAPIReadsLatestContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "design", "architecture.md")
	writeTestFile(t, source, "# Architecture\n\n[Guide](../guide.md)\n\nold body")
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Guide")
	canonical := canonicalDirectory(t, root)
	handler := newDirectoryHandler(
		canonical,
		markdown.ModeDark,
		false,
		files.DiscoverOptions{Depth: 2},
		newEventHub(time.Second),
		nil,
	)

	response := performRequest(handler, http.MethodGet, "/api/document?path=design%2Farchitecture.md")
	if response.Code != http.StatusOK {
		t.Fatalf("document status = %d, body = %s", response.Code, response.Body.String())
	}
	var document documentResponse
	decodeJSON(t, response, &document)
	if document.Path != "design/architecture.md" || document.Title != "Architecture" {
		t.Fatalf("document metadata = %+v", document)
	}
	for _, want := range []string{"old body", `href="/doc/guide.md"`} {
		if !strings.Contains(document.HTML, want) {
			t.Errorf("document HTML does not contain %q: %s", want, document.HTML)
		}
	}
	if strings.Contains(document.HTML, "<!doctype html>") {
		t.Fatal("document API returned a complete page instead of body HTML")
	}

	writeTestFile(t, source, "# Changed title\n\nnew body")
	response = performRequest(handler, http.MethodGet, "/api/document?path=design/architecture.md")
	decodeJSON(t, response, &document)
	if document.Title != "Changed title" || !strings.Contains(document.HTML, "new body") {
		t.Fatalf("updated document = %+v", document)
	}

	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	response = performRequest(handler, http.MethodGet, "/api/document?path=design/architecture.md")
	assertJSONError(t, response, http.StatusNotFound)
}

func TestDirectoryDocumentAPIRejectsUnsafeAndFilteredPaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "# Readme")
	writeTestFile(t, filepath.Join(root, "design", "architecture.md"), "# Architecture")
	writeTestFile(t, filepath.Join(root, "deep", "topic", "details.md"), "# Deep")
	writeTestFile(t, filepath.Join(root, "notes.txt"), "text")
	canonical := canonicalDirectory(t, root)
	handler := newDirectoryHandler(
		canonical,
		markdown.ModeAuto,
		false,
		files.DiscoverOptions{Depth: 1, Pattern: "**/*.md"},
		newEventHub(time.Second),
		nil,
	)

	badQueries := []string{
		"/api/document",
		"/api/document?path=",
		"/api/document?path=README.md&path=design%2Farchitecture.md",
		"/api/document?path=README.md&extra=true",
		"/api/document?path=..%2Fsecret.md",
		"/api/document?path=%252e%252e%252fsecret.md",
		"/api/document?path=%2Fetc%2Fpasswd",
		"/api/document?path=C:%5CWindows%5Csystem.md",
		"/api/document?path=name%00.md",
	}
	for _, target := range badQueries {
		response := performRequest(handler, http.MethodGet, target)
		assertJSONError(t, response, http.StatusBadRequest)
	}

	for _, target := range []string{
		"/api/document?path=readme.md",
		"/api/document?path=missing.md",
		"/api/document?path=notes.txt",
		"/api/document?path=deep%2Ftopic%2Fdetails.md",
	} {
		response := performRequest(handler, http.MethodGet, target)
		assertJSONError(t, response, http.StatusNotFound)
	}

	if runtime.GOOS != "windows" {
		outside := filepath.Join(t.TempDir(), "outside.md")
		writeTestFile(t, outside, "# Outside")
		if err := os.Symlink(outside, filepath.Join(root, "escape.md")); err != nil {
			t.Fatal(err)
		}
		response := performRequest(handler, http.MethodGet, "/api/document?path=escape.md")
		assertJSONError(t, response, http.StatusNotFound)

		writeTestFile(t, filepath.Join(root, "real", "inside.md"), "# Inside")
		if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "linked")); err != nil {
			t.Fatal(err)
		}
		response = performRequest(handler, http.MethodGet, "/api/document?path=linked%2Finside.md")
		assertJSONError(t, response, http.StatusNotFound)
	}
}

func TestDirectoryAssetsSPAFallbackAndAPINotFound(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "# Readme")
	writeTestFile(t, filepath.Join(root, "styles", "site.css"), "body { color: red; }")
	handler := newDirectoryHandler(
		canonicalDirectory(t, root),
		markdown.ModeAuto,
		false,
		files.DiscoverOptions{Depth: 2},
		newEventHub(time.Second),
		nil,
	)

	asset := performRequest(handler, http.MethodGet, "/assets/styles/site.css")
	if asset.Code != http.StatusOK || asset.Body.String() != "body { color: red; }" {
		t.Fatalf("asset response = %d %q", asset.Code, asset.Body.String())
	}
	if !strings.HasPrefix(asset.Header().Get("Content-Type"), "text/css") || asset.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("asset headers content-type=%q cache=%q", asset.Header().Get("Content-Type"), asset.Header().Get("Cache-Control"))
	}

	rootPage := performRequest(handler, http.MethodGet, "/")
	deepPage := performRequest(handler, http.MethodGet, "/doc/design/architecture.md?mode=auto")
	if rootPage.Code != http.StatusOK || deepPage.Code != http.StatusOK || rootPage.Body.String() != deepPage.Body.String() {
		t.Fatalf("SPA responses root=%d deep=%d", rootPage.Code, deepPage.Code)
	}
	if !strings.Contains(deepPage.Body.String(), `id="root"`) {
		t.Fatal("SPA fallback does not contain the root mount")
	}
	head := performRequest(handler, http.MethodHead, "/doc/design/architecture.md")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("SPA HEAD response = %d %q", head.Code, head.Body.String())
	}
	if response := performRequest(handler, http.MethodPost, "/doc/design/architecture.md"); response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("SPA POST status = %d", response.Code)
	}
	if response := performRequest(handler, http.MethodGet, "/unrelated"); response.Code != http.StatusNotFound {
		t.Fatalf("unrelated route status = %d", response.Code)
	}

	apiMissing := performRequest(handler, http.MethodGet, "/api/missing")
	assertJSONError(t, apiMissing, http.StatusNotFound)
	if !strings.HasPrefix(apiMissing.Header().Get("Content-Type"), "application/json") || strings.Contains(apiMissing.Body.String(), "<!doctype") {
		t.Fatalf("API 404 content type=%q body=%q", apiMissing.Header().Get("Content-Type"), apiMissing.Body.String())
	}
}

func TestDirectoryWebUIAssetsAndSharedMarkdownStyles(t *testing.T) {
	t.Parallel()

	root := canonicalDirectory(t, t.TempDir())
	ui := fstest.MapFS{
		"index.html":       &fstest.MapFile{Data: []byte(`<!doctype html><div id="root">production UI</div>`)},
		"assets/index.js":  &fstest.MapFile{Data: []byte(`console.log("m2h")`)},
		"assets/index.css": &fstest.MapFile{Data: []byte(`.app { color: blue; }`)},
	}
	handlerState := &directoryHandler{
		root:      root,
		mode:      markdown.ModeAuto,
		discovery: files.DiscoverOptions{Depth: 2},
		discover:  files.Discover,
		ui:        ui,
	}
	handler := handlerState.routes(newEventHub(time.Second), nil)

	for _, mode := range []markdown.Mode{markdown.ModeLight, markdown.ModeDark, markdown.ModeAuto} {
		response := performRequest(handler, http.MethodGet, "/ui/markdown.css?mode="+string(mode))
		if response.Code != http.StatusOK {
			t.Fatalf("GET Markdown CSS mode %s status = %d", mode, response.Code)
		}
		want, err := assets.Stylesheet(string(mode))
		if err != nil {
			t.Fatal(err)
		}
		if response.Body.String() != want {
			t.Errorf("Markdown CSS mode %s did not use shared stylesheet", mode)
		}
		if !strings.HasPrefix(response.Header().Get("Content-Type"), "text/css") || response.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("Markdown CSS mode %s headers content-type=%q cache=%q", mode, response.Header().Get("Content-Type"), response.Header().Get("Cache-Control"))
		}
	}

	defaultStyles := performRequest(handler, http.MethodGet, "/ui/markdown.css")
	wantDefault, err := assets.Stylesheet(string(markdown.ModeAuto))
	if err != nil {
		t.Fatal(err)
	}
	if defaultStyles.Body.String() != wantDefault {
		t.Fatal("Markdown CSS without query did not use the preview mode")
	}
	headStyles := performRequest(handler, http.MethodHead, "/ui/markdown.css?mode=light")
	if headStyles.Code != http.StatusOK || headStyles.Body.Len() != 0 {
		t.Fatalf("Markdown CSS HEAD response = %d %q", headStyles.Code, headStyles.Body.String())
	}
	for _, target := range []string{"/ui/markdown.css?mode=invalid", "/ui/markdown.css?mode=light&extra=1", "/ui/markdown.css?mode=light&mode=dark"} {
		if response := performRequest(handler, http.MethodGet, target); response.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want 400", target, response.Code)
		}
	}
	if response := performRequest(handler, http.MethodPost, "/ui/markdown.css"); response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Markdown CSS POST status=%d allow=%q", response.Code, response.Header().Get("Allow"))
	}

	for target, want := range map[string]string{
		"/ui/assets/index.js":  `console.log("m2h")`,
		"/ui/assets/index.css": `.app { color: blue; }`,
	} {
		response := performRequest(handler, http.MethodGet, target)
		if response.Code != http.StatusOK || response.Body.String() != want {
			t.Errorf("GET %s response = %d %q", target, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Errorf("GET %s cache = %q", target, response.Header().Get("Cache-Control"))
		}
	}
	if response := performRequest(handler, http.MethodGet, "/ui/assets/missing.js"); response.Code != http.StatusNotFound {
		t.Fatalf("missing WebUI asset status = %d", response.Code)
	}

	rootPage := performRequest(handler, http.MethodGet, "/")
	deepPage := performRequest(handler, http.MethodGet, "/doc/guides/setup.md?mode=dark")
	if rootPage.Code != http.StatusOK || rootPage.Body.String() != deepPage.Body.String() || !strings.Contains(rootPage.Body.String(), "production UI") {
		t.Fatalf("embedded UI root=%d deep=%d body=%q", rootPage.Code, deepPage.Code, rootPage.Body.String())
	}
}

func TestDirectoryRequestLogContainsMetadataNotBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Secret title\n\nsensitive-body-text")
	var logOutput bytes.Buffer
	handler := newDirectoryHandler(
		canonicalDirectory(t, root),
		markdown.ModeAuto,
		false,
		files.DiscoverOptions{Depth: 2},
		newEventHub(time.Second),
		&logOutput,
	)
	response := performRequest(handler, http.MethodGet, "/api/document?path=guide.md")
	if response.Code != http.StatusOK {
		t.Fatalf("document status = %d", response.Code)
	}
	logged := logOutput.String()
	for _, want := range []string{`method=GET`, `route=/api/document`, `document="guide.md"`, `status=200`, `duration=`} {
		if !strings.Contains(logged, want) {
			t.Errorf("request log does not contain %q: %s", want, logged)
		}
	}
	if strings.Contains(logged, "sensitive-body-text") {
		t.Fatalf("request log contains document body: %s", logged)
	}
}

func TestSafeRelativePathAndExactResolution(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"guide.md", "design/architecture.md", "space name.md", "计划.md"} {
		if got, err := safeRelativePath(value); err != nil || got != value {
			t.Errorf("safeRelativePath(%q) = %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"", ".", "../guide.md", "/guide.md", `C:\guide.md`, "name\x00.md", "%", "%2525252525252525252e"} {
		if _, err := safeRelativePath(value); err == nil {
			t.Errorf("safeRelativePath(%q) succeeded", value)
		}
	}

	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "Folder", "Guide.md"), "# Guide")
	if _, err := resolveRequestFile(root, "Folder/Guide.md"); err != nil {
		t.Fatalf("resolveRequestFile() error = %v", err)
	}
	if _, err := resolveRequestFile(root, "folder/Guide.md"); err == nil {
		t.Fatal("resolveRequestFile() accepted a case-mismatched component")
	}
	if _, err := resolveRequestFile(root, "Folder"); err == nil {
		t.Fatal("resolveRequestFile() accepted a directory")
	}
}

func TestDirectoryFilesAPIRenderFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Guide")
	handler := newDirectoryHandler(
		canonicalDirectory(t, root),
		"invalid",
		false,
		files.DiscoverOptions{Depth: 2},
		newEventHub(time.Second),
		nil,
	)
	response := performRequest(handler, http.MethodGet, "/api/document?path=guide.md")
	assertJSONError(t, response, http.StatusInternalServerError)
}

func directoryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "Overview without H1")
	writeTestFile(t, filepath.Join(root, "index.md"), "# Index")
	writeTestFile(t, filepath.Join(root, "notes.md"), "Notes")
	writeTestFile(t, filepath.Join(root, "design", "architecture.md"), "# Architecture")
	writeTestFile(t, filepath.Join(root, "deep", "topic", "details.md"), "# Deep")
	writeTestFile(t, filepath.Join(root, "asset.txt"), "asset")
	return root
}

func canonicalDirectory(t *testing.T, root string) string {
	t.Helper()
	input, err := files.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	return input.Path
}

func performRequest(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, nil))
	return response
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode JSON %q: %v", response.Body.String(), err)
	}
}

func assertJSONError(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("response status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
	var payload struct {
		Error string `json:"error"`
	}
	decodeJSON(t, response, &payload)
	if payload.Error == "" {
		t.Fatalf("JSON error is empty: %s", response.Body.String())
	}
}

func containsSummary(summaries []fileSummary, relative string) bool {
	return titleFor(summaries, relative) != ""
}

func titleFor(summaries []fileSummary, relative string) string {
	for _, summary := range summaries {
		if summary.Path == relative {
			return summary.Title
		}
	}
	return ""
}
