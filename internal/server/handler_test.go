package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/markdown"
)

func TestSingleFileHandlerRendersLatestDocument(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	writeTestFile(t, source, "# Before\n\n![image](images/demo.png)\n\n[download](files/demo.txt)")
	handler := newSingleFileHandler(source, markdown.ModeDark, false, newEventHub(time.Second))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d", response.Code)
	}
	for _, want := range []string{
		"<title>Before</title>",
		`class="markdown-body"`,
		`class="m2h-mode-dark"`,
		`src="/assets/images/demo.png"`,
		`href="/assets/files/demo.txt"`,
		`new EventSource("/api/events")`,
	} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("document does not contain %q", want)
		}
	}
	for _, absent := range []string{"<header", "<nav", "<button"} {
		if strings.Contains(response.Body.String(), absent) {
			t.Errorf("single-file document unexpectedly contains %q", absent)
		}
	}

	writeTestFile(t, source, "# After")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(response.Body.String(), "<title>After</title>") {
		t.Fatalf("second request did not read latest Markdown: %s", response.Body.String())
	}
}

func TestSingleFileHandlerMethodsAndFailures(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	writeTestFile(t, source, "# Guide")
	handler := newSingleFileHandler(source, markdown.ModeAuto, false, newEventHub(time.Second))

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{method: http.MethodHead, path: "/", want: http.StatusOK},
		{method: http.MethodPost, path: "/", want: http.StatusMethodNotAllowed},
		{method: http.MethodGet, path: "/missing", want: http.StatusNotFound},
		{method: http.MethodPost, path: "/assets/image.png", want: http.StatusMethodNotAllowed},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.want {
			t.Errorf("%s %s status = %d, want %d", test.method, test.path, response.Code, test.want)
		}
		if test.method == http.MethodHead && response.Body.Len() != 0 {
			t.Errorf("HEAD response body = %q", response.Body.String())
		}
	}

	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("missing source status = %d", response.Code)
	}
}

func TestSingleFileHandlerServesOnlySafeAssets(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	asset := filepath.Join(root, "images", "demo.txt")
	writeTestFile(t, source, "# Guide")
	writeTestFile(t, asset, "asset body")
	writeTestFile(t, filepath.Join(root, "private.md"), "secret")
	handler := newSingleFileHandler(source, markdown.ModeAuto, false, newEventHub(time.Second))

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, "/assets/images/demo.txt", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s safe asset status = %d", method, response.Code)
		}
		if method == http.MethodGet && response.Body.String() != "asset body" {
			t.Fatalf("safe asset body = %q", response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("safe asset cache control = %q", response.Header().Get("Cache-Control"))
		}
	}

	unsafePaths := []string{
		"/assets/../private.md",
		"/assets/%2e%2e/private.md",
		"/assets/%252e%252e/private.md",
		"/assets/%25252e%25252e/private.md",
		"/assets/%2Fetc/passwd",
		"/assets/%5c..%5cprivate.md",
		"/assets/%00name",
		"/assets/private.md",
		"/assets/",
		"/assets/images",
		"/assets/missing.png",
	}
	for _, requestPath := range unsafePaths {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if response.Code == http.StatusOK {
			t.Errorf("unsafe asset %q returned 200", requestPath)
		}
	}

	if runtime.GOOS != "windows" {
		outside := filepath.Join(t.TempDir(), "outside.txt")
		writeTestFile(t, outside, "outside")
		if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/escape.txt", nil))
		if response.Code == http.StatusOK {
			t.Fatal("symlink escape returned 200")
		}
	}
}

func TestAssetPathDecodingAndValidation(t *testing.T) {
	t.Parallel()

	for _, requestPath := range []string{"/assets/image.png", "/assets/folder/a%20b.png", "/assets/folder/./image.png"} {
		request := httptest.NewRequest(http.MethodGet, requestPath, nil)
		if _, err := assetPath(request.URL); err != nil {
			t.Errorf("assetPath(%q) error = %v", requestPath, err)
		}
	}
	for _, requestPath := range []string{"/wrong/image.png", "/assets/../../image.png", "/assets//absolute.png"} {
		if _, err := assetPath(&url.URL{Path: requestPath}); err == nil {
			t.Errorf("assetPath(%q) succeeded", requestPath)
		}
	}
}

func writeTestFile(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
