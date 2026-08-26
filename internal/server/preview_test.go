package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lz-wang/m2h/internal/assets"
	"github.com/lz-wang/m2h/internal/files"
	appversion "github.com/lz-wang/m2h/internal/version"
)

func TestDirectoryFilesAPIUsesSharedDiscoveryAndTitles(t *testing.T) {
	root := directoryFixture(t)
	canonical := canonicalDirectory(t, root)
	options := files.DiscoverOptions{Depth: 2, Pattern: "**/*.md"}
	var logOutput bytes.Buffer
	handler := newPreviewHandler(singleRootWorkspace(previewScope{root: canonical, discovery: options}), newEventHub(time.Second), &logOutput, directoryTestUI())

	response := performRequest(handler, http.MethodGet, "/api/files")
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/files status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload fileListResponse
	decodeJSON(t, response, &payload)
	if payload.Kind != previewDirectory {
		t.Fatalf("directory kind = %q, want %q", payload.Kind, previewDirectory)
	}
	if payload.Version != appversion.Development {
		t.Fatalf("directory version = %q, want %q", payload.Version, appversion.Development)
	}

	discovered, err := files.Discover(context.Background(), root+string(os.PathSeparator), options)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := make([]string, 0, len(discovered.Markdown))
	for _, entry := range discovered.Markdown {
		wantPaths = append(wantPaths, entry.RelativePath)
	}
	if len(payload.Roots) != 1 || payload.Roots[0].ID != "r0" {
		t.Fatalf("roots = %+v, want a single r0 root", payload.Roots)
	}
	files := payload.Roots[0].Files
	gotPaths := make([]string, 0, len(files))
	for _, summary := range files {
		gotPaths = append(gotPaths, summary.Path)
		if summary.Name != filepath.Base(summary.Path) {
			t.Errorf("summary name for %q = %q", summary.Path, summary.Name)
		}
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("API paths = %v, shared discovery paths = %v", gotPaths, wantPaths)
	}
	if payload.DefaultDocument == nil || payload.DefaultDocument.Root != "r0" || payload.DefaultDocument.Path != "README.md" {
		t.Fatalf("defaultDocument = %+v, want r0 README.md", payload.DefaultDocument)
	}
	if titleFor(files, "design/architecture.md") != "Architecture" {
		t.Fatalf("architecture title = %q", titleFor(files, "design/architecture.md"))
	}
	if titleFor(files, "notes.md") != "notes.md" {
		t.Fatalf("fallback title = %q", titleFor(files, "notes.md"))
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \| 192\.0\.2\.1 \[GET\] /api/files \[200\] \d+\.\dms\n$`).MatchString(logOutput.String()) {
		t.Fatalf("request log = %q", logOutput.String())
	}
}

func TestDirectoryFilesAPIDepthGlobRefreshAndDefaultSelection(t *testing.T) {
	root := directoryFixture(t)
	canonical := canonicalDirectory(t, root)
	handlerState := &previewHandler{
		workspace: singleRootWorkspace(previewScope{root: canonical, discovery: files.DiscoverOptions{Depth: 1, Pattern: "**/*.md"}}),
		discover: func(ctx context.Context, scope previewScope) (files.Discovery, error) {
			return scope.discover(ctx)
		},
	}
	requests := 0
	handlerState.discover = func(ctx context.Context, scope previewScope) (files.Discovery, error) {
		requests++
		return scope.discover(ctx)
	}
	handler := handlerState.routes(newEventHub(time.Second), nil)

	first := performRequest(handler, http.MethodGet, "/api/files")
	var initial fileListResponse
	decodeJSON(t, first, &initial)
	if requests != 1 {
		t.Fatalf("first tree refresh scanned %d times", requests)
	}
	if containsSummary(initial.Roots[0].Files, "deep/topic/details.md") {
		t.Fatal("depth 1 response contains a depth 2 document")
	}

	writeTestFile(t, filepath.Join(root, "added.md"), "# Added")
	second := performRequest(handler, http.MethodGet, "/api/files")
	var refreshed fileListResponse
	decodeJSON(t, second, &refreshed)
	if requests != 2 {
		t.Fatalf("second tree refresh total scans = %d, want 2", requests)
	}
	if !containsSummary(refreshed.Roots[0].Files, "added.md") {
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

func TestFilesAPIReportsRootAbsolutePathAndSeparator(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	alpha := filepath.Join(base, "alpha")
	beta := filepath.Join(base, "beta")
	for _, directory := range []string{alpha, beta} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(alpha, "README.md"), "# Alpha Readme")
	writeTestFile(t, filepath.Join(beta, "README.md"), "# Beta Readme")

	// A single directory root reports its canonical absolute path and the
	// server platform's separator; the wire shape carries both keys.
	single, err := newPreviewWorkspace(
		[]files.Input{resolveTestInput(t, alpha)},
		files.DiscoverOptions{Depth: 2},
	)
	if err != nil {
		t.Fatalf("newPreviewWorkspace() error = %v", err)
	}
	response := performRequest(newPreviewHandler(single, newEventHub(time.Second), nil, directoryTestUI()), http.MethodGet, "/api/files")
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/files status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload fileListResponse
	decodeJSON(t, response, &payload)
	canonicalAlpha, err := files.CanonicalPath(alpha)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Roots) != 1 {
		t.Fatalf("single-directory roots = %+v, want one", payload.Roots)
	}
	if payload.Roots[0].Kind != "directory" {
		t.Fatalf("single-directory kind = %q, want %q", payload.Roots[0].Kind, "directory")
	}
	if payload.Roots[0].AbsolutePath != canonicalAlpha {
		t.Fatalf("single-directory absolutePath = %q, want %q", payload.Roots[0].AbsolutePath, canonicalAlpha)
	}
	if payload.Roots[0].PathSeparator != string(filepath.Separator) {
		t.Fatalf("single-directory pathSeparator = %q, want %q", payload.Roots[0].PathSeparator, string(filepath.Separator))
	}

	// A multi-root workspace reports each root's own path in input order.
	workspace, err := newPreviewWorkspace(
		[]files.Input{resolveTestInput(t, alpha), resolveTestInput(t, beta)},
		files.DiscoverOptions{Depth: 2},
	)
	if err != nil {
		t.Fatalf("newPreviewWorkspace() error = %v", err)
	}
	workspaceResponse := performRequest(newPreviewHandler(workspace, newEventHub(time.Second), nil, directoryTestUI()), http.MethodGet, "/api/files")
	if workspaceResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/files status = %d, body = %s", workspaceResponse.Code, workspaceResponse.Body.String())
	}
	var workspacePayload struct {
		Roots []rootSummary `json:"roots"`
	}
	decodeJSON(t, workspaceResponse, &workspacePayload)
	canonicalBeta, err := files.CanonicalPath(beta)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspacePayload.Roots) != 2 {
		t.Fatalf("workspace roots = %+v, want two", workspacePayload.Roots)
	}
	for index, want := range []string{canonicalAlpha, canonicalBeta} {
		if got := workspacePayload.Roots[index].AbsolutePath; got != want {
			t.Fatalf("workspace root %d absolutePath = %q, want %q", index, got, want)
		}
		if got := workspacePayload.Roots[index].Kind; got != "directory" {
			t.Fatalf("workspace root %d kind = %q, want %q", index, got, "directory")
		}
		if got := workspacePayload.Roots[index].PathSeparator; got != string(filepath.Separator) {
			t.Fatalf("workspace root %d pathSeparator = %q, want %q", index, got, string(filepath.Separator))
		}
	}

	// The raw wire format carries the new keys alongside id/name/files.
	var envelope struct {
		Roots []map[string]json.RawMessage `json:"roots"`
	}
	if err := json.Unmarshal(workspaceResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Roots) == 0 {
		t.Fatal("workspace response carried no roots")
	}
	for _, key := range []string{"id", "name", "kind", "absolutePath", "pathSeparator", "files"} {
		if _, exists := envelope.Roots[0][key]; !exists {
			t.Fatalf("root summary is missing the %q key: %s", key, workspaceResponse.Body.String())
		}
	}

	// A single-file root keeps the API shape and names the file itself.
	singleFile, err := newPreviewWorkspace(
		[]files.Input{resolveTestInput(t, filepath.Join(beta, "README.md"))},
		files.DiscoverOptions{Depth: 2},
	)
	if err != nil {
		t.Fatalf("newPreviewWorkspace() error = %v", err)
	}
	fileResponse := performRequest(newPreviewHandler(singleFile, newEventHub(time.Second), nil, directoryTestUI()), http.MethodGet, "/api/files")
	if fileResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/files status = %d, body = %s", fileResponse.Code, fileResponse.Body.String())
	}
	var filePayload fileListResponse
	decodeJSON(t, fileResponse, &filePayload)
	canonicalFile, err := files.CanonicalPath(filepath.Join(beta, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(filePayload.Roots) != 1 || filePayload.Roots[0].AbsolutePath != canonicalFile {
		t.Fatalf("single-file roots = %+v, want absolutePath %q", filePayload.Roots, canonicalFile)
	}
	if filePayload.Roots[0].Kind != "file" {
		t.Fatalf("single-file kind = %q, want %q", filePayload.Roots[0].Kind, "file")
	}
	if filePayload.Roots[0].PathSeparator != string(filepath.Separator) {
		t.Fatalf("single-file pathSeparator = %q, want %q", filePayload.Roots[0].PathSeparator, string(filepath.Separator))
	}

	// A mixed workspace reports each root's own kind in input order.
	mixed, err := newPreviewWorkspace(
		[]files.Input{resolveTestInput(t, alpha), resolveTestInput(t, filepath.Join(beta, "README.md"))},
		files.DiscoverOptions{Depth: 2},
	)
	if err != nil {
		t.Fatalf("newPreviewWorkspace() error = %v", err)
	}
	mixedResponse := performRequest(newPreviewHandler(mixed, newEventHub(time.Second), nil, directoryTestUI()), http.MethodGet, "/api/files")
	if mixedResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/files status = %d, body = %s", mixedResponse.Code, mixedResponse.Body.String())
	}
	var mixedPayload fileListResponse
	decodeJSON(t, mixedResponse, &mixedPayload)
	if len(mixedPayload.Roots) != 2 || mixedPayload.Roots[0].Kind != "directory" || mixedPayload.Roots[1].Kind != "file" {
		t.Fatalf("mixed roots = %+v, want directory then file", mixedPayload.Roots)
	}
}

func TestDirectoryFilesAPIEmptyAndFailures(t *testing.T) {
	t.Parallel()

	root := canonicalDirectory(t, t.TempDir())
	handlerState := &previewHandler{
		workspace: singleRootWorkspace(previewScope{root: root, discovery: files.DiscoverOptions{Depth: 2}}),
		discover: func(ctx context.Context, scope previewScope) (files.Discovery, error) {
			return scope.discover(ctx)
		},
	}
	handler := handlerState.routes(newEventHub(time.Second), nil)
	response := performRequest(handler, http.MethodGet, "/api/files")
	var payload fileListResponse
	decodeJSON(t, response, &payload)
	if len(payload.Roots) != 1 || payload.Roots[0].Files == nil || len(payload.Roots[0].Files) != 0 || payload.DefaultDocument != nil {
		t.Fatalf("empty payload = %+v", payload)
	}

	response = performRequest(handler, http.MethodGet, "/api/files?unexpected=true")
	assertJSONError(t, response, http.StatusBadRequest)
	response = performRequest(handler, http.MethodPost, "/api/files")
	assertJSONError(t, response, http.StatusMethodNotAllowed)
	if response.Header().Get("Allow") != http.MethodGet {
		t.Errorf("POST /api/files Allow = %q", response.Header().Get("Allow"))
	}

	handlerState.discover = func(context.Context, previewScope) (files.Discovery, error) {
		return files.Discovery{}, errors.New("scan failed")
	}
	response = performRequest(handler, http.MethodGet, "/api/files")
	assertJSONError(t, response, http.StatusInternalServerError)

	handlerState.discover = func(context.Context, previewScope) (files.Discovery, error) {
		return files.Discovery{Markdown: []files.Entry{{AbsolutePath: filepath.Join(root, "missing.md"), RelativePath: "missing.md"}}}, nil
	}
	response = performRequest(handler, http.MethodGet, "/api/files")
	assertJSONError(t, response, http.StatusInternalServerError)

	valid := filepath.Join(root, "valid.md")
	writeTestFile(t, valid, "# Valid")
	handlerState.discover = func(context.Context, previewScope) (files.Discovery, error) {
		return files.Discovery{Markdown: []files.Entry{{AbsolutePath: valid, RelativePath: "../invalid.md"}}}, nil
	}
	response = performRequest(handler, http.MethodGet, "/api/files")
	assertJSONError(t, response, http.StatusInternalServerError)
}

func TestDirectoryDocumentAPIReadsLatestContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "design", "architecture.md")
	writeTestFile(t, source, "# Architecture\n\n[Guide](../guide.md)\n\nold body\n\n<details>raw HTML</details>")
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Guide")
	canonical := canonicalDirectory(t, root)
	handler := newPreviewHandler(
		singleRootWorkspace(previewScope{root: canonical, discovery: files.DiscoverOptions{Depth: 2}}),
		newEventHub(time.Second),
		nil,
		directoryTestUI(),
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
	for _, want := range []string{"old body", `href="/doc/guide.md"`, "<details>raw HTML</details>"} {
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
	handler := newPreviewHandler(
		singleRootWorkspace(previewScope{root: canonical, discovery: files.DiscoverOptions{Depth: 1, Pattern: "**/*.md"}}),
		newEventHub(time.Second),
		nil,
		directoryTestUI(),
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

func TestDirectoryDocumentAPIExposesFrontMatter(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(
		t,
		filepath.Join(root, "design.md"),
		"---\ndate: 2026-07-11\ntags:\n  - Go\n  - Markdown\nauthor: lzwang\ndraft: false\n---\n# Design\n\nbody text\n",
	)
	writeTestFile(t, filepath.Join(root, "plain.md"), "# Plain\n\nno metadata\n")
	canonical := canonicalDirectory(t, root)
	handler := newPreviewHandler(
		singleRootWorkspace(previewScope{root: canonical, discovery: files.DiscoverOptions{Depth: 2}}),
		newEventHub(time.Second),
		nil,
		directoryTestUI(),
	)

	response := performRequest(handler, http.MethodGet, "/api/document?path=design.md")
	if response.Code != http.StatusOK {
		t.Fatalf("document status = %d, body = %s", response.Code, response.Body.String())
	}
	var document documentResponse
	decodeJSON(t, response, &document)
	if document.Title != "Design" {
		t.Fatalf("title = %q", document.Title)
	}
	if document.FrontMatter == nil {
		t.Fatal("frontmatter = nil, want metadata")
	}
	if document.FrontMatter.Date != "2026-07-11" {
		t.Errorf("date = %q", document.FrontMatter.Date)
	}
	if !reflect.DeepEqual(document.FrontMatter.Tags, []string{"Go", "Markdown"}) {
		t.Errorf("tags = %v", document.FrontMatter.Tags)
	}
	wantKeys := []string{"date", "tags", "author", "draft"}
	if len(document.FrontMatter.Entries) != len(wantKeys) {
		t.Fatalf("entries = %+v", document.FrontMatter.Entries)
	}
	for i, want := range wantKeys {
		if document.FrontMatter.Entries[i].Key != want {
			t.Errorf("entry %d = %q, want %q", i, document.FrontMatter.Entries[i].Key, want)
		}
	}
	for _, fragment := range []string{"---", "date: 2026-07-11", "author: lzwang", "draft: false"} {
		if strings.Contains(document.HTML, fragment) {
			t.Errorf("HTML should not contain frontmatter YAML %q: %s", fragment, document.HTML)
		}
	}
	if !strings.Contains(document.HTML, "body text") {
		t.Errorf("HTML missing body: %s", document.HTML)
	}
	if strings.Contains(response.Body.String(), `"frontmatter":null`) {
		t.Errorf("response should serialize frontmatter metadata: %s", response.Body.String())
	}

	plain := performRequest(handler, http.MethodGet, "/api/document?path=plain.md")
	if plain.Code != http.StatusOK {
		t.Fatalf("plain document status = %d", plain.Code)
	}
	var plainDocument documentResponse
	decodeJSON(t, plain, &plainDocument)
	if plainDocument.FrontMatter != nil {
		t.Fatalf("plain frontmatter = %+v, want nil", plainDocument.FrontMatter)
	}
	if !strings.Contains(plain.Body.String(), `"frontmatter":null`) {
		t.Errorf("plain response should serialize frontmatter:null: %s", plain.Body.String())
	}
}

func TestDirectoryDocumentAPIExposesTableOfContents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(
		t,
		filepath.Join(root, "guide.md"),
		"# Guide\n\n## 安装\n\n## 安装\n\n### Homebrew\n\n#### C++ API\n\nplain paragraph\n",
	)
	writeTestFile(t, filepath.Join(root, "plain.md"), "No headings here.\n")
	canonical := canonicalDirectory(t, root)
	handler := newPreviewHandler(
		singleRootWorkspace(previewScope{root: canonical, discovery: files.DiscoverOptions{Depth: 2}}),
		newEventHub(time.Second),
		nil,
		directoryTestUI(),
	)

	response := performRequest(handler, http.MethodGet, "/api/document?path=guide.md")
	if response.Code != http.StatusOK {
		t.Fatalf("document status = %d, body = %s", response.Code, response.Body.String())
	}
	var document documentResponse
	decodeJSON(t, response, &document)
	want := []tocEntryResponse{
		{Level: 1, ID: "guide", Text: "Guide"},
		{Level: 2, ID: "安装", Text: "安装"},
		{Level: 2, ID: "安装-1", Text: "安装"},
		{Level: 3, ID: "homebrew", Text: "Homebrew"},
		{Level: 4, ID: "c-api", Text: "C++ API"},
	}
	if !reflect.DeepEqual(document.TOC, want) {
		t.Fatalf("toc = %+v, want %+v", document.TOC, want)
	}
	// Each toc id must point at a real anchor in the rendered HTML.
	for _, entry := range document.TOC {
		if !strings.Contains(document.HTML, fmt.Sprintf(`id=%q`, entry.ID)) {
			t.Errorf("HTML missing anchor for toc entry %+v", entry)
		}
	}

	plain := performRequest(handler, http.MethodGet, "/api/document?path=plain.md")
	if plain.Code != http.StatusOK {
		t.Fatalf("plain document status = %d", plain.Code)
	}
	var plainDocument documentResponse
	decodeJSON(t, plain, &plainDocument)
	if len(plainDocument.TOC) != 0 {
		t.Fatalf("plain toc = %+v, want empty", plainDocument.TOC)
	}
}

func TestDirectoryDocumentAPIInvalidFrontMatter(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "broken.md"), "---\ntags: [\n---\n# Broken\n")
	writeTestFile(t, filepath.Join(root, "scalar.md"), "---\njust text\n---\n# Scalar\n")
	canonical := canonicalDirectory(t, root)
	handler := newPreviewHandler(
		singleRootWorkspace(previewScope{root: canonical, discovery: files.DiscoverOptions{Depth: 2}}),
		newEventHub(time.Second),
		nil,
		directoryTestUI(),
	)

	for _, target := range []string{"broken.md", "scalar.md"} {
		response := performRequest(handler, http.MethodGet, "/api/document?path="+target)
		assertJSONError(t, response, http.StatusUnprocessableEntity)
	}
}

func TestDirectoryAssetsSPAFallbackAndAPINotFound(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "# Readme")
	writeTestFile(t, filepath.Join(root, "styles", "site.css"), "body { color: red; }")
	handler := newPreviewHandler(
		singleRootWorkspace(previewScope{root: canonicalDirectory(t, root), discovery: files.DiscoverOptions{Depth: 2}}),
		newEventHub(time.Second),
		nil,
		directoryTestUI(),
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
	handlerState := &previewHandler{
		workspace: singleRootWorkspace(previewScope{root: root, discovery: files.DiscoverOptions{Depth: 2}}),
		discover: func(ctx context.Context, scope previewScope) (files.Discovery, error) {
			return scope.discover(ctx)
		},
		ui: ui,
	}
	handler := handlerState.routes(newEventHub(time.Second), nil)

	// The Markdown stylesheet is one stable, query-independent resource: its
	// URL never changes on a theme switch, so toggling light/dark issues no new
	// request. Light and dark palettes coexist, selected by html.dark.
	styles := performRequest(handler, http.MethodGet, "/ui/markdown.css")
	if styles.Code != http.StatusOK {
		t.Fatalf("GET Markdown CSS status = %d", styles.Code)
	}
	if styles.Body.String() != assets.PreviewStylesheet() {
		t.Errorf("Markdown CSS did not serve the stable preview stylesheet")
	}
	if !strings.HasPrefix(styles.Header().Get("Content-Type"), "text/css") || styles.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Markdown CSS headers content-type=%q cache=%q", styles.Header().Get("Content-Type"), styles.Header().Get("Cache-Control"))
	}

	headStyles := performRequest(handler, http.MethodHead, "/ui/markdown.css")
	if headStyles.Code != http.StatusOK || headStyles.Body.Len() != 0 {
		t.Fatalf("Markdown CSS HEAD response = %d %q", headStyles.Code, headStyles.Body.String())
	}

	// Any query is rejected so the resource address stays stable and cacheable.
	for _, target := range []string{
		"/ui/markdown.css?mode=light",
		"/ui/markdown.css?mode=dark",
		"/ui/markdown.css?mode=auto",
		"/ui/markdown.css?mode=invalid",
		"/ui/markdown.css?extra=1",
	} {
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
	handler := newPreviewHandler(
		singleRootWorkspace(previewScope{root: canonicalDirectory(t, root), discovery: files.DiscoverOptions{Depth: 2}}),
		newEventHub(time.Second),
		&logOutput,
		directoryTestUI(),
	)
	response := performRequest(handler, http.MethodGet, "/api/document?path=guide.md")
	if response.Code != http.StatusOK {
		t.Fatalf("document status = %d", response.Code)
	}
	logged := logOutput.String()
	for _, want := range []string{` | 192.0.2.1 [GET] `, `/api/document?path=guide.md`, `[200]`} {
		if !strings.Contains(logged, want) {
			t.Errorf("request log does not contain %q: %s", want, logged)
		}
	}
	if strings.Contains(logged, "sensitive-body-text") {
		t.Fatalf("request log contains document body: %s", logged)
	}
	if !regexp.MustCompile(` \d+\.\dms\n$`).MatchString(logged) {
		t.Fatalf("request log duration does not have one decimal place: %s", logged)
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	for remoteAddress, want := range map[string]string{
		"192.0.2.1:1234":   "192.0.2.1",
		"[2001:db8::1]:80": "2001:db8::1",
		"local-client":     "local-client",
	} {
		if got := clientIP(remoteAddress); got != want {
			t.Errorf("clientIP(%q) = %q, want %q", remoteAddress, got, want)
		}
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

// multiRootFixture builds a two-root workspace whose roots deliberately share
// file names: both carry README.md and images/logo.png so routing can prove
// that identical relative paths never cross between roots.
func multiRootFixture(t *testing.T) previewWorkspace {
	t.Helper()

	base := t.TempDir()
	alpha := filepath.Join(base, "alpha")
	beta := filepath.Join(base, "beta")
	for _, directory := range []string{alpha, beta} {
		if err := os.MkdirAll(filepath.Join(directory, "images"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(alpha, "README.md"), "# Alpha Readme")
	writeTestFile(t, filepath.Join(alpha, "notes.md"), "# Notes")
	writeTestFile(t, filepath.Join(alpha, "images", "logo.png"), "alpha-logo")
	writeTestFile(t, filepath.Join(beta, "README.md"), "# Beta Readme")
	writeTestFile(t, filepath.Join(beta, "images", "logo.png"), "beta-logo")

	workspace, err := newPreviewWorkspace(
		[]files.Input{
			resolveTestInput(t, alpha),
			resolveTestInput(t, beta),
		},
		files.DiscoverOptions{Depth: 4},
	)
	if err != nil {
		t.Fatalf("newPreviewWorkspace() error = %v", err)
	}
	return workspace
}

func TestWorkspaceFilesAPILabelsRootsAndPrefersThePrimaryRoot(t *testing.T) {
	t.Parallel()

	// The primary root has neither README.md nor index.md while the second
	// root has a README: the primary's first document still wins — the CLI's
	// first input decides where the preview opens.
	base := t.TempDir()
	alpha := filepath.Join(base, "alpha")
	beta := filepath.Join(base, "beta")
	for _, directory := range []string{alpha, beta} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(alpha, "zeta.md"), "# Zeta")
	writeTestFile(t, filepath.Join(beta, "README.md"), "# Beta Readme")
	workspace, err := newPreviewWorkspace(
		[]files.Input{resolveTestInput(t, alpha), resolveTestInput(t, beta)},
		files.DiscoverOptions{Depth: 4},
	)
	if err != nil {
		t.Fatalf("newPreviewWorkspace() error = %v", err)
	}
	handler := newPreviewHandler(workspace, newEventHub(time.Second), nil, directoryTestUI())

	response := performRequest(handler, http.MethodGet, "/api/files")
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/files status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload fileListResponse
	decodeJSON(t, response, &payload)
	if payload.Kind != previewWorkspaceKind {
		t.Fatalf("workspace kind = %q, want %q", payload.Kind, previewWorkspaceKind)
	}
	if len(payload.Roots) != 2 {
		t.Fatalf("roots = %+v, want two", payload.Roots)
	}
	if payload.Roots[0].ID != "r0" || payload.Roots[0].Name != "alpha" || payload.Roots[1].ID != "r1" || payload.Roots[1].Name != "beta" {
		t.Fatalf("root summaries = %+v", payload.Roots)
	}
	// Files stay root-relative inside their root summary.
	if len(payload.Roots[0].Files) != 1 || payload.Roots[0].Files[0].Path != "zeta.md" {
		t.Fatalf("alpha files = %+v", payload.Roots[0].Files)
	}
	if payload.DefaultDocument == nil || payload.DefaultDocument.Root != "r0" || payload.DefaultDocument.Path != "zeta.md" {
		t.Fatalf("defaultDocument = %+v, want r0 zeta.md", payload.DefaultDocument)
	}
}

func TestWorkspaceDocumentAPIRoutesVirtualPathsPerRoot(t *testing.T) {
	t.Parallel()

	workspace := multiRootFixture(t)
	handler := newPreviewHandler(workspace, newEventHub(time.Second), nil, directoryTestUI())

	// Identical relative paths in two roots resolve to their own root's file.
	alpha := performRequest(handler, http.MethodGet, "/api/document?path=r0/README.md")
	if alpha.Code != http.StatusOK {
		t.Fatalf("alpha document status = %d, body = %s", alpha.Code, alpha.Body.String())
	}
	var alphaDocument documentResponse
	decodeJSON(t, alpha, &alphaDocument)
	if alphaDocument.Title != "Alpha Readme" || alphaDocument.Path != "r0/README.md" {
		t.Fatalf("alpha document = %+v", alphaDocument)
	}

	beta := performRequest(handler, http.MethodGet, "/api/document?path=r1/README.md")
	if beta.Code != http.StatusOK {
		t.Fatalf("beta document status = %d, body = %s", beta.Code, beta.Body.String())
	}
	var betaDocument documentResponse
	decodeJSON(t, beta, &betaDocument)
	if betaDocument.Title != "Beta Readme" || betaDocument.Path != "r1/README.md" {
		t.Fatalf("beta document = %+v", betaDocument)
	}

	// A multi-root workspace only addresses documents through a known root id.
	for _, target := range []string{
		"/api/document?path=README.md",
		"/api/document?path=r2/README.md",
		"/api/document?path=alpha/README.md",
		"/api/document?path=r0",
		"/api/document?path=r0%2Fmissing.md",
	} {
		response := performRequest(handler, http.MethodGet, target)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", target, response.Code)
		}
	}

	// Cross-root traversal is rejected by the shared path validation (400),
	// never resolved against another root.
	if response := performRequest(handler, http.MethodGet, "/api/document?path=r0%2F..%2Fr1%2FREADME.md"); response.Code != http.StatusBadRequest {
		t.Fatalf("cross-root traversal status = %d, want 400", response.Code)
	}
}

func TestWorkspaceAssetHandlerRoutesPerRoot(t *testing.T) {
	t.Parallel()

	workspace := multiRootFixture(t)
	handler := newPreviewHandler(workspace, newEventHub(time.Second), nil, directoryTestUI())

	// Same-named attachments never cross roots.
	alpha := performRequest(handler, http.MethodGet, "/assets/r0/images/logo.png")
	if alpha.Code != http.StatusOK || alpha.Body.String() != "alpha-logo" {
		t.Fatalf("alpha asset = %d %q", alpha.Code, alpha.Body.String())
	}
	beta := performRequest(handler, http.MethodGet, "/assets/r1/images/logo.png")
	if beta.Code != http.StatusOK || beta.Body.String() != "beta-logo" {
		t.Fatalf("beta asset = %d %q", beta.Code, beta.Body.String())
	}

	for _, target := range []string{
		// Multi-root assets require the root prefix.
		"/assets/images/logo.png",
		// Unknown root ids and cross-root traversal resolve nowhere. (The
		// traversal case is sent pre-encoded: the mux answers an unencoded
		// ".." with a redirect before the handler ever sees it.)
		"/assets/r2/images/logo.png",
		"/assets/r0/%2e%2e/r1/images/logo.png",
		// Markdown is never served as an attachment.
		"/assets/r0/README.md",
		"/assets/r0/notes.md",
		"/assets/r0/missing.png",
	} {
		response := performRequest(handler, http.MethodGet, target)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", target, response.Code)
		}
	}
}

func TestWorkspaceDocumentRendersVirtualLinkRouting(t *testing.T) {
	t.Parallel()

	// Both roots carry same-named link targets; the rendered links must name
	// their own root, and a link climbing past the virtual root stays a
	// dead relative href instead of crossing into the other root.
	base := t.TempDir()
	alpha := filepath.Join(base, "alpha")
	beta := filepath.Join(base, "beta")
	for _, directory := range []string{alpha, beta} {
		if err := os.MkdirAll(filepath.Join(directory, "deep", "images"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(directory, "README.md"), "# Readme")
		writeTestFile(t, filepath.Join(directory, "images", "logo.png"), "logo")
		writeTestFile(t, filepath.Join(directory, "deep", "guide.md"),
			"# Guide\n\n[Home](../README.md)\n\n![Logo](../images/logo.png)\n\n[Short escape](../../README.md)\n\n[Long escape](../../../README.md)\n")
	}
	workspace, err := newPreviewWorkspace(
		[]files.Input{
			resolveTestInput(t, alpha),
			resolveTestInput(t, beta),
		},
		files.DiscoverOptions{Depth: 4},
	)
	if err != nil {
		t.Fatalf("newPreviewWorkspace() error = %v", err)
	}
	handler := newPreviewHandler(workspace, newEventHub(time.Second), nil, directoryTestUI())

	response := performRequest(handler, http.MethodGet, "/api/document?path=r1/deep/guide.md")
	if response.Code != http.StatusOK {
		t.Fatalf("document status = %d, body = %s", response.Code, response.Body.String())
	}
	var document documentResponse
	decodeJSON(t, response, &document)

	// Same-root relative links and images resolve under the document's own
	// root prefix.
	for _, want := range []string{
		`href="/doc/r1/README.md"`,
		`src="/assets/r1/images/logo.png"`,
	} {
		if !strings.Contains(document.HTML, want) {
			t.Errorf("document HTML missing %q: %s", want, document.HTML)
		}
	}
	// Climbing past the own document's directory is contained by the virtual
	// root: two ups land on the unprefixed /doc/README.md, which a multi-root
	// workspace does not serve (404), and three ups exceed even the virtual
	// root and stay as authored. Neither can address another root.
	if !strings.Contains(document.HTML, `href="/doc/README.md"`) {
		t.Errorf("short escape link was rewritten past the virtual root: %s", document.HTML)
	}
	if escaped := performRequest(handler, http.MethodGet, "/api/document?path=README.md"); escaped.Code != http.StatusNotFound {
		t.Fatalf("short escape target status = %d, want 404", escaped.Code)
	}
	if !strings.Contains(document.HTML, `href="../../../README.md"`) {
		t.Errorf("long escape link was rewritten: %s", document.HTML)
	}
	if strings.Contains(document.HTML, "/doc/r0/") || strings.Contains(document.HTML, "/assets/r0/") {
		t.Errorf("document leaks the other root's prefix: %s", document.HTML)
	}

	// The rewritten addresses really serve their own root's bytes.
	logo := performRequest(handler, http.MethodGet, "/assets/r1/images/logo.png")
	if logo.Code != http.StatusOK || logo.Body.String() != "logo" {
		t.Fatalf("linked asset = %d %q", logo.Code, logo.Body.String())
	}
	home := performRequest(handler, http.MethodGet, "/api/document?path=r1/README.md")
	if home.Code != http.StatusOK {
		t.Fatalf("linked document status = %d", home.Code)
	}
}

func TestRawMarkdownServesOriginalBytesAndHeaders(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := "---\ntitle: Raw\n---\n# Raw\n\nbody with `code`\n"
	writeTestFile(t, filepath.Join(root, "README.md"), source)
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Guide")
	handler := newPreviewHandler(
		singleRootWorkspace(previewScope{root: canonicalDirectory(t, root), discovery: files.DiscoverOptions{Depth: 2}}),
		newEventHub(time.Second),
		nil,
		directoryTestUI(),
	)

	// The raw route serves the untouched source file, frontmatter included.
	response := performRequest(handler, http.MethodGet, "/raw/README.md")
	if response.Code != http.StatusOK {
		t.Fatalf("GET /raw/README.md status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Body.String() != source {
		t.Fatalf("raw body = %q, want the untouched original source", response.Body.String())
	}
	for header, want := range map[string]string{
		"Content-Type":           "text/markdown; charset=utf-8",
		"Cache-Control":          "no-cache",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := response.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	// Latest on-disk bytes, not a snapshot from startup.
	updated := "# Changed\n"
	writeTestFile(t, filepath.Join(root, "guide.md"), updated)
	refreshed := performRequest(handler, http.MethodGet, "/raw/guide.md")
	if refreshed.Code != http.StatusOK || refreshed.Body.String() != updated {
		t.Fatalf("refreshed raw = %d %q, want %q", refreshed.Code, refreshed.Body.String(), updated)
	}

	head := performRequest(handler, http.MethodHead, "/raw/README.md")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD /raw/README.md = %d %q, want empty body", head.Code, head.Body.String())
	}
	if head.Header().Get("Content-Type") != "text/markdown; charset=utf-8" {
		t.Fatalf("HEAD content-type = %q", head.Header().Get("Content-Type"))
	}

	post := performRequest(handler, http.MethodPost, "/raw/README.md")
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST /raw/README.md = %d allow=%q, want 405 with Allow", post.Code, post.Header().Get("Allow"))
	}
	if response := performRequest(handler, http.MethodGet, "/raw/README.md?extra=1"); response.Code != http.StatusBadRequest {
		t.Fatalf("GET /raw/README.md?extra=1 status = %d, want 400", response.Code)
	}
}

func TestRawMarkdownRejectsUnsafeAndFilteredPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "# Readme")
	writeTestFile(t, filepath.Join(root, "design", "architecture.md"), "# Architecture")
	writeTestFile(t, filepath.Join(root, "deep", "topic", "details.md"), "# Deep")
	writeTestFile(t, filepath.Join(root, "notes.txt"), "text")
	canonical := canonicalDirectory(t, root)
	handler := newPreviewHandler(
		singleRootWorkspace(previewScope{root: canonical, discovery: files.DiscoverOptions{Depth: 1, Pattern: "**/*.md"}}),
		newEventHub(time.Second),
		nil,
		directoryTestUI(),
	)

	// Malformed addressable paths are refused exactly like /api/document.
	// Traversal is sent pre-encoded: an unencoded ".." is canonicalized away
	// by the router (a redirect) before the handler ever sees it.
	for _, target := range []string{
		"/raw/",
		"/raw/..%2Fsecret.md",
		"/raw/%2e%2e%2fsecret.md",
		"/raw/%252e%252e%252fsecret.md",
		"/raw/%2Fetc%2Fpasswd",
		"/raw/C:%5CWindows%5Csystem.md",
		"/raw/name%00.md",
	} {
		if response := performRequest(handler, http.MethodGet, target); response.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want 400", target, response.Code)
		}
	}

	// Well-formed addresses to invisible documents resolve nowhere: missing
	// files, non-Markdown files, case mismatches and depth-filtered trees.
	for _, target := range []string{
		"/raw/missing.md",
		"/raw/notes.txt",
		"/raw/readme.md",
		"/raw/deep%2Ftopic%2Fdetails.md",
	} {
		if response := performRequest(handler, http.MethodGet, target); response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", target, response.Code)
		}
	}

	if runtime.GOOS != "windows" {
		outside := filepath.Join(t.TempDir(), "outside.md")
		writeTestFile(t, outside, "# Outside")
		if err := os.Symlink(outside, filepath.Join(root, "escape.md")); err != nil {
			t.Fatal(err)
		}
		if response := performRequest(handler, http.MethodGet, "/raw/escape.md"); response.Code != http.StatusNotFound {
			t.Fatalf("symlink escape status = %d, want 404", response.Code)
		}
	}
}

func TestRawMarkdownServesSingleFileRoot(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	writeTestFile(t, filepath.Join(base, "solo.md"), "# Solo\n\nonly document\n")
	writeTestFile(t, filepath.Join(base, "sibling.md"), "# Sibling")
	singleFile, err := newPreviewWorkspace(
		[]files.Input{resolveTestInput(t, filepath.Join(base, "solo.md"))},
		files.DiscoverOptions{Depth: 2},
	)
	if err != nil {
		t.Fatalf("newPreviewWorkspace() error = %v", err)
	}
	handler := newPreviewHandler(singleFile, newEventHub(time.Second), nil, directoryTestUI())

	response := performRequest(handler, http.MethodGet, "/raw/solo.md")
	if response.Code != http.StatusOK || response.Body.String() != "# Solo\n\nonly document\n" {
		t.Fatalf("single-file raw = %d %q", response.Code, response.Body.String())
	}
	// The scope admits only the named file; siblings stay unreachable.
	if response := performRequest(handler, http.MethodGet, "/raw/sibling.md"); response.Code != http.StatusNotFound {
		t.Fatalf("sibling status = %d, want 404", response.Code)
	}
}

func TestRawMarkdownMultiRootNeverCrossesRoots(t *testing.T) {
	t.Parallel()

	workspace := multiRootFixture(t)
	handler := newPreviewHandler(workspace, newEventHub(time.Second), nil, directoryTestUI())

	// Identical relative paths in two roots serve their own root's bytes.
	alpha := performRequest(handler, http.MethodGet, "/raw/r0/README.md")
	if alpha.Code != http.StatusOK || alpha.Body.String() != "# Alpha Readme" {
		t.Fatalf("alpha raw = %d %q", alpha.Code, alpha.Body.String())
	}
	beta := performRequest(handler, http.MethodGet, "/raw/r1/README.md")
	if beta.Code != http.StatusOK || beta.Body.String() != "# Beta Readme" {
		t.Fatalf("beta raw = %d %q", beta.Code, beta.Body.String())
	}

	// A multi-root workspace only serves documents through a known root id.
	for _, target := range []string{
		"/raw/README.md",
		"/raw/r2/README.md",
		"/raw/alpha/README.md",
		"/raw/r0",
		"/raw/r0%2Fmissing.md",
	} {
		if response := performRequest(handler, http.MethodGet, target); response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", target, response.Code)
		}
	}

	// Cross-root traversal is rejected by the shared path validation (400),
	// never resolved against another root.
	if response := performRequest(handler, http.MethodGet, "/raw/r0%2F..%2Fr1%2FREADME.md"); response.Code != http.StatusBadRequest {
		t.Fatalf("cross-root traversal status = %d, want 400", response.Code)
	}
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

// directoryTestUI is the embedded WebUI filesystem used by directory handler tests.
func directoryTestUI() fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><div id="root"></div>`)},
	}
}
