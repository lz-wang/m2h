package server

import (
	"context"
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
