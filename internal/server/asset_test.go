package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

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

func TestIsActiveWebAsset(t *testing.T) {
	t.Parallel()

	for _, active := range []string{
		"page.html", "page.htm", "page.xhtml", "app.js", "app.mjs", "app.cjs",
		"style.css", "nested/PAGE.HTML", "nested/App.JS",
	} {
		if !isActiveWebAsset(active) {
			t.Errorf("isActiveWebAsset(%q) = false, want true", active)
		}
	}
	for _, passive := range []string{
		"image.png", "diagram.svg", "manual.pdf", "archive.zip", "movie.mp4",
		"data.json", "notes.txt", "noext", "htmlish.md.txt",
	} {
		if isActiveWebAsset(passive) {
			t.Errorf("isActiveWebAsset(%q) = true, want false", passive)
		}
	}
}

func TestAssetAdmissionMatrix(t *testing.T) {
	root := canonicalDirectory(t, t.TempDir())
	for name, contents := range map[string]string{
		"guide.md":              "# Guide",
		"image.png":             "png",
		"manual.pdf":            "pdf",
		"archive.zip":           "zip",
		"diagram.svg":           "<svg xmlns=\"http://www.w3.org/2000/svg\"/>",
		"page.html":             "<html></html>",
		"page.htm":              "<html></html>",
		"page.xhtml":            "<html></html>",
		"app.js":                "console.log(1)",
		"app.mjs":               "console.log(1)",
		"app.cjs":               "console.log(1)",
		"style.css":             "body{}",
		".env":                  "SECRET=1",
		".git/config":           "[core]",
		"foo/.secret/data.json": "{}",
		"assets/manual.pdf":     "nested pdf",
		"docs/design.md":        "# Design",
		"download/archive.zip":  "nested zip",
		"images/logo.png":       "nested png",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), contents)
	}

	handler := newAssetHandler(singleRootWorkspace(rootScope{root: root}))

	tests := []struct {
		target string
		status int
	}{
		{"/assets/image.png", http.StatusOK},
		{"/assets/manual.pdf", http.StatusOK},
		{"/assets/archive.zip", http.StatusOK},
		{"/assets/diagram.svg", http.StatusOK},
		{"/assets/images/logo.png", http.StatusOK},
		{"/assets/assets/manual.pdf", http.StatusOK},
		{"/assets/download/archive.zip", http.StatusOK},
		{"/assets/guide.md", http.StatusNotFound},
		{"/assets/docs/design.md", http.StatusNotFound},
		{"/assets/page.html", http.StatusNotFound},
		{"/assets/page.htm", http.StatusNotFound},
		{"/assets/page.xhtml", http.StatusNotFound},
		{"/assets/app.js", http.StatusNotFound},
		{"/assets/app.mjs", http.StatusNotFound},
		{"/assets/app.cjs", http.StatusNotFound},
		{"/assets/style.css", http.StatusNotFound},
		{"/assets/.env", http.StatusNotFound},
		{"/assets/.git/config", http.StatusNotFound},
		{"/assets/foo/.secret/data.json", http.StatusNotFound},
		{"/assets/missing.png", http.StatusNotFound},
		{"/assets/../secret", http.StatusNotFound},
	}
	for _, test := range tests {
		if response := performRequest(handler, http.MethodGet, test.target); response.Code != test.status {
			t.Errorf("GET %s status = %d, want %d", test.target, response.Code, test.status)
		}
	}

	// HEAD behaves like GET; other methods are refused.
	if response := performRequest(handler, http.MethodHead, "/assets/image.png"); response.Code != http.StatusOK {
		t.Errorf("HEAD /assets/image.png status = %d, want 200", response.Code)
	}
	if response := performRequest(handler, http.MethodPost, "/assets/image.png"); response.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /assets/image.png status = %d, want 405", response.Code)
	}

	// Served attachments carry the sandbox CSP so a direct navigation to an
	// SVG never runs embedded script as document script.
	svg := performRequest(handler, http.MethodGet, "/assets/diagram.svg")
	if got := svg.Header().Get("Content-Security-Policy"); got != "sandbox; default-src 'none'" {
		t.Errorf("asset CSP = %q, want sandbox policy", got)
	}
}

func TestAssetServesRangeRequests(t *testing.T) {
	root := canonicalDirectory(t, t.TempDir())
	contents := "0123456789abcdef"
	writeTestFile(t, filepath.Join(root, "manual.pdf"), contents)
	handler := newAssetHandler(singleRootWorkspace(rootScope{root: root}))

	request := httptest.NewRequest(http.MethodGet, "/assets/manual.pdf", nil)
	request.Header.Set("Range", "bytes=0-9")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent {
		t.Fatalf("Range status = %d, want 206", response.Code)
	}
	if got := response.Header().Get("Content-Range"); got != "bytes 0-9/"+strconv.Itoa(len(contents)) {
		t.Fatalf("Content-Range = %q", got)
	}
	if response.Body.String() != contents[:10] {
		t.Fatalf("Range body = %q, want first 10 bytes", response.Body.String())
	}

	// A full GET still returns the whole file — the security changes must not
	// disturb http.ServeContent's default behavior.
	if response := performRequest(handler, http.MethodGet, "/assets/manual.pdf"); response.Code != http.StatusOK || response.Body.String() != contents {
		t.Fatalf("full GET = %d %q, want 200 with full body", response.Code, response.Body.String())
	}
}

func TestAssetSymlinkBoundaries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}

	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "real.png"), "real")
	outside := filepath.Join(t.TempDir(), "outside.png")
	writeTestFile(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(root, "escape.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real.png"), filepath.Join(root, "alias.png")); err != nil {
		t.Fatal(err)
	}

	handler := newAssetHandler(singleRootWorkspace(rootScope{root: root}))
	if response := performRequest(handler, http.MethodGet, "/assets/escape.png"); response.Code != http.StatusNotFound {
		t.Errorf("symlink escaping root status = %d, want 404", response.Code)
	}
	if response := performRequest(handler, http.MethodGet, "/assets/alias.png"); response.Code != http.StatusOK {
		t.Errorf("symlink to in-root regular file status = %d, want 200", response.Code)
	}
}

func TestAssetSymlinksCannotBypassPublishingPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}

	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, ".private", "report.pdf"), "report")
	writeTestFile(t, filepath.Join(root, "app.js"), "console.log(1)")
	writeTestFile(t, filepath.Join(root, "page.html"), "<html></html>")
	writeTestFile(t, filepath.Join(root, "images", "logo-real.png"), "logo")
	// Harmless-looking names whose canonical targets are hidden or active
	// web content: the asset policy must hold for the canonical target too.
	for link, target := range map[string]string{
		"public.pdf":    ".private/report.pdf",
		"safe.js.txt":   "app.js",
		"safe.html.txt": "page.html",
		"logo.png":      "images/logo-real.png",
	} {
		if err := os.Symlink(filepath.Join(root, filepath.FromSlash(target)), filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}

	handler := newAssetHandler(singleRootWorkspace(rootScope{root: root}))
	for _, refused := range []string{
		"/assets/public.pdf",
		"/assets/safe.js.txt",
		"/assets/safe.html.txt",
	} {
		if response := performRequest(handler, http.MethodGet, refused); response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404 (canonical target must satisfy the asset policy)", refused, response.Code)
		}
	}
	// A visible alias to a visible passive target keeps working.
	if response := performRequest(handler, http.MethodGet, "/assets/logo.png"); response.Code != http.StatusOK {
		t.Errorf("GET /assets/logo.png status = %d, want 200", response.Code)
	}
}

// writeTestFile is the shared fixture helper for the server package tests.
func writeTestFile(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
