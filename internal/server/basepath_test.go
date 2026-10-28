package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

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
