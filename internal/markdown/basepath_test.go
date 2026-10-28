package markdown

import (
	"strings"
	"testing"
)

func TestWebBasePathDestinations(t *testing.T) {
	for _, base := range []string{"", "/docs", "/apps/docs"} {
		for _, root := range []string{"", "r0"} {
			prefix := ""
			if root != "" {
				prefix = root + "/"
			}
			options := RenderOptions{
				URLMode: URLWeb, SourcePath: prefix + "README.md", RootPath: root, WebBasePath: base,
				DirectoryDocument: func(target string) (string, bool) {
					if target == prefix+"folder" {
						return prefix + "folder/index.md", true
					}
					return "", target == prefix+"empty"
				},
			}
			source := `[Guide](guide.md?q=x#intro)
![Logo](logo.png)
[Index](/README.md)
[Section](#intro)
[External](https://example.com/a)
[Protocol](//example.com/a)
[Folder](folder/)
[Empty](empty/)
[Outside](../secret.md)
<img src="raw.png" srcset="one.png 1x, two.png 2x">
<a href="guide.md">Raw</a>`
			rendered, err := Render([]byte(source), options)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				`href="` + base + `/doc/` + prefix + `guide.md?q=x#intro"`,
				`src="` + base + `/assets/` + prefix + `logo.png"`,
				`href="` + base + `/doc/` + prefix + `README.md"`,
				`href="#intro"`, `href="https://example.com/a"`, `href="//example.com/a"`,
				`href="` + base + `/doc/` + prefix + `folder/index.md"`,
				`href="` + base + InvalidLocalReferencePath + `?target=empty%2F"`,
				`href="` + base + InvalidLocalReferencePath + `?target=..%2Fsecret.md"`,
				`src="` + base + `/assets/` + prefix + `raw.png"`,
				`srcset="` + base + `/assets/` + prefix + `one.png 1x, ` + base + `/assets/` + prefix + `two.png 2x"`,
			} {
				if !strings.Contains(rendered.Body, want) {
					t.Errorf("base=%q root=%q missing %s in %s", base, root, want, rendered.Body)
				}
			}
			options.URLMode = URLPassthrough
			rendered, err = Render([]byte(source), options)
			if err != nil || !strings.Contains(rendered.Body, `href="guide.md?q=x#intro"`) || !strings.Contains(rendered.Body, `src="logo.png"`) {
				t.Errorf("passthrough changed: %+v, %v", rendered, err)
			}
		}
	}
}
