package server

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

func TestNormalizeBasePath(t *testing.T) {
	for _, value := range []string{"", "/", "/docs", "/docs/", "/apps/docs", "/a-B_0.~"} {
		want := strings.TrimSuffix(value, "/")
		got, err := NormalizeBasePath(value)
		if err != nil || got != want {
			t.Errorf("%q = %q, %v; want %q", value, got, err, want)
		}
		options, err := normalizeOptions(Options{Inputs: []string{"missing"}, Mode: markdown.ModeAuto, BasePath: value})
		if err != nil || options.BasePath != want {
			t.Errorf("normalize options %q = %+v, %v", value, options, err)
		}
	}
	for _, value := range []string{"docs", "//docs", "/docs//", "/docs/../api", "/./docs", "/docs//api", "/docs?x=1", "/docs#x", "/docs%2Fapi", "/a\\b", "/文档", "/a b", "/a\nb", "/a:b"} {
		if _, err := NormalizeBasePath(value); err == nil {
			t.Errorf("accepted %q", value)
		}
		err := run(context.Background(), Options{Inputs: []string{"missing"}, BasePath: value}, dependencies{})
		if err == nil || !strings.Contains(err.Error(), "--base-path") {
			t.Errorf("run %q: %v", value, err)
		}
	}
}

func TestMountBasePath(t *testing.T) {
	for _, base := range []string{"", "/docs", "/apps/docs"} {
		handler := securityHeaders(mountBasePath(base, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Path", r.URL.EscapedPath())
			w.Header().Set("X-Query", r.URL.RawQuery)
			w.WriteHeader(http.StatusNoContent)
		})), false)
		response := performRequest(handler, "GET", base+"/doc/a%20b.md?mode=dark")
		if response.Code != 204 || response.Header().Get("X-Path") != "/doc/a%20b.md" || response.Header().Get("X-Query") != "mode=dark" {
			t.Errorf("%q: %+v", base, response)
		}
		assertSecurityHeaders(t, response)
		if base == "" {
			continue
		}
		for _, target := range []string{"/", "/api/files", base + "-other/", "/doc/a.md"} {
			response := performRequest(handler, "GET", target)
			if response.Code != 404 {
				t.Errorf("%s: %d", target, response.Code)
			}
			assertSecurityHeaders(t, response)
		}
		response = performRequest(handler, "GET", base+"?mode=dark&toc=false")
		if response.Code != 308 || response.Header().Get("Location") != base+"/?mode=dark&toc=false" {
			t.Errorf("redirect: %+v", response)
		}
		assertSecurityHeaders(t, response)
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
		response = performRequest(mountBasePath(base, mux), "GET", base+"/doc//a.md?q=x")
		if response.Header().Get("Location") != base+"/doc/a.md?q=x" {
			t.Errorf("clean redirect escaped prefix: %s", response.Header().Get("Location"))
		}
	}
}

func TestRenderIndexHTML(t *testing.T) {
	index := []byte(`<!doctype html><html><head><link href="/ui/favicon.svg"><link href='./assets/main.css'><script type="module" src="./assets/main.js"></script><link href="https://example.com/a.css"></head><body><p>./assets/untouched</p></body></html>`)
	for _, base := range []string{"", "/docs", "/apps/docs"} {
		for _, cdn := range []bool{false, true} {
			got := string(renderIndexHTML(index, base, cdn))
			for _, want := range []string{`name="m2h-base-path" content="` + base + `"`, `href="` + base + `/ui/favicon.svg"`, `href="` + base + `/ui/assets/main.css"`, `src="` + base + `/ui/assets/main.js"`, `href="https://example.com/a.css"`, `<p>./assets/untouched</p>`} {
				if !strings.Contains(got, want) {
					t.Errorf("missing %s in %s", want, got)
				}
			}
			if strings.Contains(got, `name="m2h-cdn"`) != cdn {
				t.Errorf("CDN metadata: %s", got)
			}
		}
	}
}

func TestBasePathRoutesAndWorkspaceBoundaries(t *testing.T) {
	root := canonicalDirectory(t, t.TempDir())
	other := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "README.md"), "# Home\n\n[Guide](guide.md)\n\n![Logo](logo.svg)\n\n[Outside](../secret.md)")
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Guide\n\nsearchmarker")
	writeTestFile(t, filepath.Join(root, "logo.svg"), "<svg></svg>")
	writeTestFile(t, filepath.Join(root, "bad.html"), "<script>bad</script>")
	writeTestFile(t, filepath.Join(root, ".private.md"), "# Private")
	writeTestFile(t, filepath.Join(other, "README.md"), "# Other")
	ui := fstest.MapFS{
		"index.html":    {Data: []byte(`<html><head><script src="./assets/app.js"></script><link href="/ui/markdown.css"><link href="/ui/favicon.svg"></head><body></body></html>`)},
		"assets/app.js": {Data: []byte("export {}")},
		"favicon.svg":   {Data: []byte("<svg></svg>")},
	}
	for _, multi := range []bool{false, true} {
		inputs := []files.Input{resolveTestInput(t, root)}
		key := ""
		if multi {
			inputs = append(inputs, resolveTestInput(t, other))
			key = "r0/"
		}
		workspace, err := newWorkspace(inputs, files.DiscoverOptions{Depth: 4, SkipHidden: true}, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, base := range []string{"", "/docs", "/apps/docs"} {
			for _, cdn := range []bool{false, true} {
				handler := newDocumentHandlerWithConfig(workspace, nil, ui, "1.2.3", cdn, base)
				for _, route := range []string{"/", "/doc/" + key + "README.md", "/healthz", "/api/files", "/api/document?path=" + key + "README.md", "/api/file-metadata?path=" + key + "guide.md", "/api/search?q=searchmarker", "/raw/" + key + "README.md", "/assets/" + key + "logo.svg", "/runtime/katex.min.js", "/ui/assets/app.js", "/ui/markdown.css", "/ui/favicon.svg"} {
					response := performRequest(handler, "GET", base+route)
					if response.Code != 200 {
						t.Errorf("multi=%v cdn=%v GET %s%s: %d %s", multi, cdn, base, route, response.Code, response.Body.String())
					}
					if strings.HasPrefix(route, "/assets/") {
						if response.Header().Get("Content-Security-Policy") != "sandbox; default-src 'none'" {
							t.Error("asset sandbox lost")
						}
					} else if !cdn {
						assertSecurityHeaders(t, response)
					}
					if strings.HasPrefix(route, "/api/document?") {
						var doc documentResponse
						if err := json.Unmarshal(response.Body.Bytes(), &doc); err != nil {
							t.Fatal(err)
						}
						for _, want := range []string{base + "/doc/" + key + "guide.md", base + "/assets/" + key + "logo.svg", base + markdown.InvalidLocalReferencePath} {
							if !strings.Contains(doc.HTML, want) {
								t.Errorf("document missing %s: %s", want, doc.HTML)
							}
						}
					}
				}
				for _, route := range []string{"/doc/" + key + ".private.md", "/assets/" + key + "bad.html", "/assets/" + key + "../secret.md", "/api/unknown", markdown.InvalidLocalReferencePath} {
					response := performRequest(handler, "GET", base+route)
					if response.Code != 404 && response.Code != 301 && response.Code != 307 {
						t.Errorf("boundary %s%s: %d", base, route, response.Code)
					}
					if (response.Code == 301 || response.Code == 307) && !strings.HasPrefix(response.Header().Get("Location"), base+"/") {
						t.Errorf("redirect escaped prefix: %s", response.Header().Get("Location"))
					}
				}
				if multi {
					response := performRequest(handler, "GET", base+"/api/document?path=r1/README.md")
					if response.Code != 200 || !strings.Contains(response.Body.String(), "Other") {
						t.Error("second root inaccessible")
					}
				}
			}
		}
	}
}

func TestBasePathHTTPBoundaryDetails(t *testing.T) {
	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "README.md"), "# Home")
	writeTestFile(t, filepath.Join(root, "中 %.png"), "encoded attachment")
	writeTestFile(t, filepath.Join(root, "bad.html"), "<script>bad</script>")
	workspace := singleRootWorkspace(rootScope{root: root, discovery: files.DiscoverOptions{Depth: 2, SkipHidden: true}})
	for _, base := range []string{"", "/docs", "/apps/docs"} {
		handler := newDocumentHandlerWithConfig(workspace, nil, directoryTestUI(), "1.2.3", false, base)
		for _, route := range []string{"/doc/README.md", "/doc/missing.md"} {
			get := performRequest(handler, "GET", base+route)
			head := performRequest(handler, "HEAD", base+route)
			if get.Code != head.Code || head.Body.Len() != 0 {
				t.Errorf("HEAD %s%s = %d %q; GET %d", base, route, head.Code, head.Body.String(), get.Code)
			}
			for _, header := range []string{"Content-Type", "Cache-Control", "Content-Security-Policy", "X-Content-Type-Options"} {
				if get.Header().Get(header) != head.Header().Get(header) {
					t.Errorf("HEAD header %s differs", header)
				}
			}
		}
		for _, test := range []struct{ route, target string }{
			{"/ui?q=x", "/ui/?q=x"}, {"/runtime", "/runtime/"},
			{"/doc/../assets/bad.html", "/assets/bad.html"},
		} {
			response := performRequest(handler, "GET", base+test.route)
			if response.Code < 300 || response.Code >= 400 || response.Header().Get("Location") != base+test.target {
				t.Errorf("GET %s%s = %d location %q", base, test.route, response.Code, response.Header().Get("Location"))
			}
			assertSecurityHeaders(t, response)
		}
		response := performRequest(handler, "GET", base+"/assets/%E4%B8%AD%20%25.png")
		if response.Code != 200 || response.Body.String() != "encoded attachment" {
			t.Errorf("encoded asset %q: %d %q", base, response.Code, response.Body.String())
		}
		response = performRequest(handler, "GET", base+"/assets/bad.html")
		if response.Code != 404 {
			t.Errorf("cleaned route bypassed asset policy: %d", response.Code)
		}
	}
}
