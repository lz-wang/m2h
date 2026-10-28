package markdown

import (
	"strings"
	"testing"
)

func TestRewriteSrcsetCandidates(t *testing.T) {
	options := RenderOptions{URLMode: URLWeb, SourcePath: "r0/sub/index.md", RootPath: "r0", WebBasePath: "/apps/docs"}
	for _, test := range []struct{ source, want string }{
		{"", ""},
		{" , \t,", " , \t,"},
		{"one.png 1x, two.png 2x", "/apps/docs/assets/r0/sub/one.png 1x, /apps/docs/assets/r0/sub/two.png 2x"},
		{"one.png, two.png,", "/apps/docs/assets/r0/sub/one.png, /apps/docs/assets/r0/sub/two.png,"},
		{"one.png,,,\n two.png\t640w", "/apps/docs/assets/r0/sub/one.png,,,\n /apps/docs/assets/r0/sub/two.png\t640w"},
		{"data:image/png;base64,AAAA,BBBB 1x, two.png 2x", "data:image/png;base64,AAAA,BBBB 1x, /apps/docs/assets/r0/sub/two.png 2x"},
		{"data:image/png;base64,AAAA, two.png 2x", "data:image/png;base64,AAAA, /apps/docs/assets/r0/sub/two.png 2x"},
		{"https://example.com/a,b.png 1x, //example.com/x.png 2x", "https://example.com/a,b.png 1x, //example.com/x.png 2x"},
		{"/logo%20%25%E4%B8%AD.png?q=x#hash 640w", "/apps/docs/assets/r0/logo%20%25%E4%B8%AD.png?q=x#hash 640w"},
		{"one.png future(1, 2) 1x, two.png 2x", "/apps/docs/assets/r0/sub/one.png future(1, 2) 1x, /apps/docs/assets/r0/sub/two.png 2x"},
		{"one.png future(unclosed, two.png 2x", "/apps/docs/assets/r0/sub/one.png future(unclosed, two.png 2x"},
		{"one.png\f1x,\r\n two.png 2x", "/apps/docs/assets/r0/sub/one.png\f1x,\r\n /apps/docs/assets/r0/sub/two.png 2x"},
		{"../../secret.png 2x", "/apps/docs/__m2h_invalid_local_reference__?target=..%2F..%2Fsecret.png 2x"},
	} {
		if got := rewriteSrcset(test.source, options); got != test.want {
			t.Errorf("%q: got %q; want %q", test.source, got, test.want)
		}
		passthrough := options
		passthrough.URLMode = URLPassthrough
		if got := rewriteSrcset(test.source, passthrough); got != test.source {
			t.Errorf("passthrough changed %q to %q", test.source, got)
		}
	}
}

func TestRawResponsiveImagesRenderAndInspect(t *testing.T) {
	source := `<picture><source sizes="100vw" srcset="small.png 640w, /large.png 1280w"><img src="fallback.png" SRCSET="data:image/png;base64,AAAA,BBBB 1x, two.png?x=1&amp;y=2 2x" alt="Responsive"></picture>`
	rendered, err := Render([]byte(source), RenderOptions{URLMode: URLWeb, SourcePath: "guide.md", WebBasePath: "/docs"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`srcset="/docs/assets/small.png 640w, /docs/assets/large.png 1280w"`, `srcset="data:image/png;base64,AAAA,BBBB 1x, /docs/assets/two.png?x=1&amp;y=2 2x"`, `sizes="100vw"`} {
		if !strings.Contains(rendered.Body, want) {
			t.Errorf("missing %s in %s", want, rendered.Body)
		}
	}
	inspection := Inspect([]byte(source))
	want := []string{"small.png", "/large.png", "fallback.png", "data:image/png;base64,AAAA,BBBB", "two.png?x=1&y=2"}
	if len(inspection.References) != len(want) {
		t.Fatalf("references: %+v", inspection.References)
	}
	for i, reference := range inspection.References {
		if reference.Destination != want[i] || reference.Route != ReferenceRouteAsset || reference.Line != 1 || reference.Column < 1 {
			t.Errorf("reference %d: %+v", i, reference)
		}
	}
	exported, err := Render([]byte(source), RenderOptions{SourcePath: "guide.md"})
	if err != nil || !strings.Contains(exported.Body, source) {
		t.Errorf("export changed raw references: %+v %v", exported, err)
	}
}
