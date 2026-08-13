package convert

import (
	"encoding/base64"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
)

// imageMIMEByExtension maps local image suffixes to data URI media types.
// Unknown extensions are left untouched so the HTML keeps a working reference
// whenever the browser might still understand it.
var imageMIMEByExtension = map[string]string{
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".gif":  "image/gif",
	".ico":  "image/x-icon",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
}

// newInlineImageResolver returns a markdown.InlineImage resolver that reads
// images under root and returns data URIs for self-contained HTML. Every
// failure resolves to ok=false so the original relative path survives and the
// document degrades to a normal file reference instead of failing conversion.
func newInlineImageResolver(root string) func(string) (string, bool) {
	return func(relative string) (string, bool) {
		mimeType, supported := imageMIMEByExtension[strings.ToLower(path.Ext(relative))]
		if !supported {
			return "", false
		}
		target := filepath.Join(root, filepath.FromSlash(relative))
		// Resolve symlinks before the boundary check so a link pointing out of
		// the input root never leaks file contents into the HTML.
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil || !files.IsWithin(root, resolved) {
			return "", false
		}
		contents, err := os.ReadFile(resolved)
		if err != nil {
			return "", false
		}
		return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(contents), true
	}
}
