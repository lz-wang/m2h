package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lz-wang/m2h/internal/assets"
)

func TestRuntimeHandlerServesEmbeddedAssets(t *testing.T) {
	t.Parallel()

	handler := newRuntimeHandler()
	for _, item := range []struct {
		path        string
		contentType string
	}{
		{path: "/runtime/mermaid.min.js", contentType: "text/javascript; charset=utf-8"},
		{path: "/runtime/katex.min.js", contentType: "text/javascript; charset=utf-8"},
		{path: "/runtime/auto-render.min.js", contentType: "text/javascript; charset=utf-8"},
		{path: "/runtime/tablesort.min.js", contentType: "text/javascript; charset=utf-8"},
		{path: "/runtime/tablesort.number.js", contentType: "text/javascript; charset=utf-8"},
		{path: "/runtime/katex.min.css", contentType: "text/css; charset=utf-8"},
		{path: "/runtime/fonts/KaTeX_Main-Regular.woff2", contentType: "font/woff2"},
	} {
		item := item
		t.Run(item.path, func(t *testing.T) {
			t.Parallel()

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, item.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", item.path, response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Content-Type"); got != item.contentType {
				t.Errorf("GET %s Content-Type = %q, want %q", item.path, got, item.contentType)
			}
			if got := response.Header().Get("Cache-Control"); got != "no-cache" {
				t.Errorf("GET %s Cache-Control = %q, want %q", item.path, got, "no-cache")
			}
			if response.Body.Len() == 0 {
				t.Errorf("GET %s returned an empty body", item.path)
			}
		})
	}
}

func TestRuntimeHandlerRefusesNonGETAndDirectories(t *testing.T) {
	t.Parallel()

	handler := newRuntimeHandler()
	for _, item := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodPost, path: "/runtime/mermaid.min.js", status: http.StatusMethodNotAllowed},
		{method: http.MethodGet, path: "/runtime/", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime/fonts", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime/fonts/", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime/missing.js", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime/../../main.go", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime/", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime/fonts", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime/fonts/", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime/missing.js", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/runtime/../../main.go", status: http.StatusNotFound},
	} {
		item := item
		t.Run(item.method+" "+item.path, func(t *testing.T) {
			t.Parallel()

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(item.method, item.path, nil))
			if response.Code != item.status {
				t.Fatalf("%s %s status = %d, want %d", item.method, item.path, response.Code, item.status)
			}
		})
	}
}

// TestRuntimeHandlerSupportsHead covers the HEAD path the WebUI and export
// bootstrap may use to probe an asset: same status and headers as GET, but no
// body.
func TestRuntimeHandlerSupportsHead(t *testing.T) {
	t.Parallel()

	handler := newRuntimeHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/runtime/katex.min.js", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("HEAD /runtime/katex.min.js status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Errorf("HEAD Content-Type = %q, want %q", got, "text/javascript; charset=utf-8")
	}
	if response.Body.Len() != 0 {
		t.Errorf("HEAD returned %d body bytes, want none", response.Body.Len())
	}
}

// TestRuntimeAssetsCoverExportOutput guards the sharing contract: every
// non-font asset embedded for the WebUI must also be reachable under
// /runtime/, so the WebUI and exported HTML run the same pinned releases.
func TestRuntimeAssetsCoverExportOutput(t *testing.T) {
	t.Parallel()

	handler := newRuntimeHandler()
	err := fs.WalkDir(assets.RichFS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		request := httptest.NewRequest(http.MethodGet, "/runtime/"+path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("GET /runtime/%s status = %d, want %d", path, response.Code, http.StatusOK)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeHandlerRemovedRenderersReturnNotFound(t *testing.T) {
	t.Parallel()
	handler := newRuntimeHandler()
	for _, path := range []string{
		"/runtime/vega.min.js", "/runtime/vega-lite.min.js", "/runtime/vega-embed.min.js",
		"/runtime/mermaid-zenuml/mermaid-zenuml.esm.min.mjs",
		"/runtime/mermaid-zenuml/chunks/mermaid-zenuml.esm.min/zenuml-definition-EPHX7WPJ.mjs",
		"/runtime/mermaid-zenuml/chunks/mermaid-zenuml.esm.min/chunk-PPGA74DV.mjs",
		"/runtime/mermaid-zenuml/chunks/mermaid-zenuml.esm.min/chunk-VDOFMJQL.mjs",
		"/runtime/mermaid-zenuml/chunks/mermaid-zenuml.esm.min/cloud-icons-eHuugVSv-MANGUYXV.mjs",
		"/runtime/LICENSE.vega", "/runtime/LICENSE.vega-lite", "/runtime/LICENSE.vega-embed",
		"/runtime/LICENSE.mermaid-zenuml", "/runtime/LICENSE.zenuml-core",
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(method, path, nil))
			if response.Code != http.StatusNotFound {
				t.Errorf("%s %s status = %d, want 404", method, path, response.Code)
			}
		}
	}
}
