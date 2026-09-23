package files

import (
	"path"
	"strings"
)

// IsActiveWebAsset reports whether a root-relative path names an "active"
// web document: a file type a browser executes as code or renders as a
// same-origin page when navigated to directly (HTML variants, scripts,
// stylesheets). Serving such files through an attachment route would let
// anything inside the published root become same-origin active content on
// the m2h origin, so the route refuses them; ordinary passive attachments
// (images, PDFs, archives, media) stay downloadable. The rule is an
// extension allowlist by negation on purpose — an ever-growing MIME
// allowlist would keep rejecting new document formats users legitimately
// want to publish. Both the server's asset route and the check command's
// reference verdicts share this predicate so a link check can never call
// an attachment the browser will be refused.
func IsActiveWebAsset(relative string) bool {
	switch strings.ToLower(path.Ext(relative)) {
	case ".html", ".htm", ".xhtml", ".js", ".mjs", ".cjs", ".css":
		return true
	default:
		return false
	}
}
