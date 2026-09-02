package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

// assertSecurityHeaders verifies the shared hardening baseline: every
// response carries the default CSP and the four companion headers.
func assertSecurityHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()

	headers := response.Header()
	if got := headers.Get("Content-Security-Policy"); got != defaultContentSecurityPolicy {
		t.Errorf("Content-Security-Policy = %q, want the default policy", got)
	}
	if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := headers.Get("Referrer-Policy"); got != "same-origin" {
		t.Errorf("Referrer-Policy = %q, want same-origin", got)
	}
	if got := headers.Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q, want SAMEORIGIN", got)
	}
	if got := headers.Get("Permissions-Policy"); got != "camera=(), microphone=(), geolocation=(), payment=(), usb=()" {
		t.Errorf("Permissions-Policy = %q", got)
	}
}

func TestSecurityHeadersCoverEveryRoute(t *testing.T) {
	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Guide")
	writeTestFile(t, filepath.Join(root, "image.png"), "png")

	handler := newDocumentHandler(singleRootWorkspace(rootScope{
		root:      root,
		discovery: files.DiscoverOptions{Depth: 2, SkipHidden: true},
	}), nil, directoryTestUI())

	for _, target := range []string{
		"/",
		"/doc/guide.md",
		"/doc/missing.md",
		"/raw/guide.md",
		"/api/files",
		"/api/document?path=guide.md",
		"/api/unknown",
		"/ui/markdown.css",
	} {
		assertSecurityHeaders(t, performRequest(handler, http.MethodGet, target))
	}

	// Error responses are hardened too: a 405 must not lose the baseline.
	response := performRequest(handler, http.MethodPost, "/api/files")
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/files status = %d, want 405", response.Code)
	}
	assertSecurityHeaders(t, response)
}

func TestAssetResponsesOverrideCSPWithSandbox(t *testing.T) {
	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "image.png"), "png")
	handler := newDocumentHandler(singleRootWorkspace(rootScope{root: root}), nil, directoryTestUI())

	response := performRequest(handler, http.MethodGet, "/assets/image.png")
	if response.Code != http.StatusOK {
		t.Fatalf("GET /assets/image.png status = %d", response.Code)
	}
	if got := response.Header().Get("Content-Security-Policy"); got != "sandbox; default-src 'none'" {
		t.Fatalf("asset CSP = %q, want the sandbox policy", got)
	}
	// Only the CSP is overridden; the rest of the baseline survives.
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("asset X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := response.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Fatalf("asset Referrer-Policy = %q, want same-origin", got)
	}
}

func TestDefaultContentSecurityPolicyDirectives(t *testing.T) {
	t.Parallel()

	for _, directive := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob: http: https:",
		"media-src 'self' blob: http: https:",
		"font-src 'self' data:",
		"connect-src 'self'",
		"worker-src 'self' blob:",
		"base-uri 'none'",
		"object-src 'none'",
		"frame-ancestors 'self'",
		"form-action 'none'",
		"frame-src 'none'",
	} {
		if !containsDirective(defaultContentSecurityPolicy, directive) {
			t.Errorf("default CSP is missing %q: %s", directive, defaultContentSecurityPolicy)
		}
	}
	// Code evaluation stays forbidden: Vega charts run through the AST
	// interpreter precisely so no 'unsafe-eval' is needed, and inline scripts
	// stay blocked everywhere — 'unsafe-inline' must not reach script-src.
	for _, forbidden := range []string{"script-src 'unsafe-eval'", "default-src 'unsafe-eval'", "script-src 'unsafe-inline'"} {
		if containsDirective(defaultContentSecurityPolicy, forbidden) {
			t.Errorf("default CSP must not contain %q", forbidden)
		}
	}
	if strings.Contains(defaultContentSecurityPolicy, "unsafe-eval") {
		t.Errorf("default CSP must not contain 'unsafe-eval' anywhere: %s", defaultContentSecurityPolicy)
	}
}

func containsDirective(policy, directive string) bool {
	return slices.Contains(strings.Split(policy, "; "), directive)
}
