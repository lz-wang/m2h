package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lz-wang/m2h/internal/files"
)

func TestCDNConfigurationInShellAndSecurityHeaders(t *testing.T) {
	t.Parallel()
	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Guide")
	writeTestFile(t, filepath.Join(root, "image.svg"), "<svg></svg>")
	// Exercise the real shell, including the <head> configuration insertion.
	index, err := os.ReadFile("../../web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	ui := fstest.MapFS{"index.html": {Data: index}}
	workspace := singleRootWorkspace(rootScope{
		root: root, discovery: files.DiscoverOptions{Depth: 2, SkipHidden: true},
	})
	for _, cdn := range []bool{false, true} {
		name := "local"
		if cdn {
			name = "cdn"
		}
		t.Run(name, func(t *testing.T) {
			handler := newDocumentHandlerWithConfig(workspace, nil, ui, "1.2.3", cdn)
			for _, target := range []string{"/", "/?cdn=true", "/doc/guide.md", "/doc/guide.md?cdn=false", "/doc/missing.md"} {
				response := performRequest(handler, http.MethodGet, target)
				wantStatus := http.StatusOK
				if target == "/doc/missing.md" {
					wantStatus = http.StatusNotFound
				}
				if response.Code != wantStatus {
					t.Fatalf("GET %s = %d, want %d", target, response.Code, wantStatus)
				}
				body := response.Body.String()
				if got := strings.Contains(body, `<head><meta name="m2h-cdn" content="true">`); got != cdn {
					t.Errorf("GET %s CDN metadata = %v, want %v", target, got, cdn)
				}
				if response.Header().Get("Cache-Control") != "no-cache" {
					t.Error("shell must revalidate when the server configuration changes")
				}
				policy := response.Header().Get("Content-Security-Policy")
				if !cdn {
					assertSecurityHeaders(t, response)
					continue
				}
				for _, directive := range []string{
					"script-src 'self' https://cdn.jsdelivr.net;",
					"style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline';",
					"font-src 'self' https://cdn.jsdelivr.net data:;",
					"connect-src 'self';", "base-uri 'none';", "object-src 'none';",
				} {
					if !strings.Contains(policy, directive) {
						t.Errorf("CDN policy missing %q: %s", directive, policy)
					}
				}
				if strings.Contains(policy, "unsafe-eval") || strings.Contains(policy, "script-src 'self' 'unsafe-inline'") {
					t.Errorf("CDN policy permits inline script or eval: %s", policy)
				}
			}
			if response := performRequest(handler, http.MethodHead, "/doc/guide.md"); response.Body.Len() != 0 || response.Code != http.StatusOK {
				t.Errorf("HEAD response = %d %q", response.Code, response.Body.String())
			}
			asset := performRequest(handler, http.MethodGet, "/assets/image.svg")
			if asset.Code != http.StatusOK || asset.Header().Get("Content-Security-Policy") != "sandbox; default-src 'none'" {
				t.Error("CDN option must preserve the attachment sandbox")
			}
		})
	}
}
