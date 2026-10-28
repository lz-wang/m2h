package server

import (
	"bytes"
	"fmt"
	"net/http"

	"golang.org/x/net/html"
	"strings"
)

// NormalizeBasePath validates an HTTP deployment prefix. The empty result means
// origin root; filesystem and workspace paths never pass through this function.
func NormalizeBasePath(value string) (string, error) {
	if value == "" || value == "/" {
		return "", nil
	}
	invalid := func() (string, error) {
		return "", fmt.Errorf("invalid --base-path %q: expected / or an absolute path of ASCII URL segments (letters, digits, -, _, ., ~), without empty, . or .. segments", value)
	}
	if !strings.HasPrefix(value, "/") {
		return invalid()
	}
	normalized := strings.TrimSuffix(value, "/")
	for _, segment := range strings.Split(normalized[1:], "/") {
		if segment == "" || segment == "." || segment == ".." {
			return invalid()
		}
		for _, char := range segment {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-_.~", char)) {
				return invalid()
			}
		}
	}
	return normalized, nil
}

// mountBasePath leaves the internal router and virtual file paths unchanged.
// Security headers and logging wrap this handler, including redirects and 404s.
func mountBasePath(basePath string, next http.Handler) http.Handler {
	if basePath == "" {
		return next
	}
	stripped := http.StripPrefix(basePath, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == basePath {
			target := basePath + "/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusPermanentRedirect)
			return
		}
		if !strings.HasPrefix(r.URL.Path, basePath+"/") {
			http.NotFound(w, r)
			return
		}
		stripped.ServeHTTP(basePathResponseWriter{ResponseWriter: w, basePath: basePath}, r)
	})
}

// ServeMux can issue absolute-path redirects while cleaning duplicate slashes
// or dot segments. Keep those redirects inside the public deployment prefix.
type basePathResponseWriter struct {
	http.ResponseWriter
	basePath string
}

func (w basePathResponseWriter) WriteHeader(status int) {
	if location := w.Header().Get("Location"); strings.HasPrefix(location, "/") && !strings.HasPrefix(location, "//") {
		w.Header().Set("Location", w.basePath+location)
	}
	w.ResponseWriter.WriteHeader(status)
}

// renderIndexHTML rewrites only resource attributes in the trusted UI shell.
// Relative Vite entry URLs must resolve under /ui/ even on document deep links.
func renderIndexHTML(index []byte, basePath string, cdn bool) []byte {
	var output bytes.Buffer
	tokens := html.NewTokenizer(bytes.NewReader(index))
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			return output.Bytes()
		}
		raw := append([]byte(nil), tokens.Raw()...)
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			output.Write(raw)
			continue
		}
		token := tokens.Token()
		changed := false
		if token.Data == "script" || token.Data == "link" {
			for i := range token.Attr {
				attr := &token.Attr[i]
				if attr.Key != "src" && attr.Key != "href" {
					continue
				}
				if strings.HasPrefix(attr.Val, "/ui/") {
					attr.Val = basePath + attr.Val
					changed = true
				} else if strings.HasPrefix(attr.Val, "./") {
					attr.Val = basePath + "/ui/" + strings.TrimPrefix(attr.Val, "./")
					changed = true
				}
			}
		}
		if changed {
			output.WriteString(token.String())
		} else {
			output.Write(raw)
		}
		if token.Data == "head" {
			if cdn {
				output.WriteString(`<meta name="m2h-cdn" content="true">`)
			}
			output.WriteString(`<meta name="m2h-base-path" content="` + basePath + `">`)
		}
	}
}
