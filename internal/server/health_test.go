package server

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	// An empty workspace still reports healthy: the endpoint answers whether
	// the HTTP process serves, never whether any document exists.
	handler := newDocumentHandler(singleRootWorkspace(rootScope{
		root:      canonicalDirectory(t, t.TempDir()),
		discovery: files.DiscoverOptions{Depth: 1, SkipHidden: true},
	}), nil, directoryTestUI())

	health := performRequest(handler, http.MethodGet, "/healthz")
	if health.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", health.Code)
	}
	if got := health.Body.String(); got != "ok\n" {
		t.Fatalf("GET /healthz body = %q, want %q", got, "ok\n")
	}
	if got := health.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := health.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	head := performRequest(handler, http.MethodHead, "/healthz")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD /healthz = %d %q, want 200 with empty body", head.Code, head.Body.String())
	}

	post := performRequest(handler, http.MethodPost, "/healthz")
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /healthz status = %d, want 405", post.Code)
	}
	if got := post.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want GET, HEAD", got)
	}
}

func TestHealthEndpointDoesNotScanDocuments(t *testing.T) {
	t.Parallel()

	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Guide")
	handlerState := &documentHandler{
		workspace: singleRootWorkspace(rootScope{root: root}),
		discover: func(context.Context, rootScope) (files.Discovery, error) {
			t.Fatal("health check must not scan documents")
			return files.Discovery{}, nil
		},
	}

	health := performRequest(handlerState.routes(nil), http.MethodGet, "/healthz")
	if health.Code != http.StatusOK || health.Body.String() != "ok\n" {
		t.Fatalf("GET /healthz = %d %q", health.Code, health.Body.String())
	}
}
