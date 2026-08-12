package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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
