// Package markdown is the sole GFM parsing and HTML rendering core for m2h.
package markdown

import (
	"bytes"
	stdhtml "html"
	"html/template"
	"net/url"
	pathpkg "path"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"

	"github.com/lz-wang/m2h/internal/assets"
)

// Mode controls the rendered color theme.
type Mode string

const (
	ModeLight Mode = "light"
	ModeDark  Mode = "dark"
	ModeAuto  Mode = "auto"
)

// Width controls the maximum width of rendered browser documents.
type Width string

const (
	WidthStandard Width = "standard"
	WidthWide     Width = "wide"
	WidthFull     Width = "full"
)

// Target controls local link rewriting for generated files or live previews.
type Target string

const (
	TargetConvert Target = "convert"
	TargetPreview Target = "preview"
)

// RenderOptions configures a single Markdown render.
type RenderOptions struct {
	Mode       Mode
	Width      Width
	Target     Target
	SourcePath string
	UnsafeHTML bool
	AssetBase  string
}

// Result contains both the reusable rendered body and a complete HTML page.
type Result struct {
	HTML  string
	Body  string
	Title string
}

var pageTemplate = template.Must(template.New("document").Parse(`<!doctype html>
<html lang="zh-CN" class="m2h-mode-{{.Mode}}" data-target="{{.Target}}" data-width="{{.Width}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
{{.Styles}}
  </style>
{{if .AssetBase}}  <link rel="stylesheet" href="{{.AssetBase}}katex.min.css">{{end}}
</head>
<body class="m2h-page">
  <article class="markdown-body">
{{.Body}}  </article>
{{if .AssetBase}}  <script src="{{.AssetBase}}katex.min.js"></script>
  <script src="{{.AssetBase}}auto-render.min.js"></script>
  <script src="{{.AssetBase}}mermaid.min.js"></script>
  <script src="{{.AssetBase}}rich-content.js"></script>{{end}}
{{if .LiveReload}}  <script>
    const events = new EventSource("/api/events");
    events.addEventListener("document-changed", () => window.location.reload());
  </script>
{{end}}
</body>
</html>
`))

// Render parses standard GFM once, rewrites its AST, and renders a complete page.
func Render(source []byte, options RenderOptions) (Result, error) {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return Result{}, err
	}

	engine := newEngine(normalized.UnsafeHTML)
	document := engine.Parser().Parse(text.NewReader(source))
	if err := rewriteDocument(document, normalized); err != nil {
		return Result{}, err
	}

	title := extractTitle(document, source, normalized.SourcePath)
	var body bytes.Buffer
	if err := engine.Renderer().Render(&body, source, document); err != nil {
		return Result{}, err
	}

	stylesheet, err := assets.Stylesheet(string(normalized.Mode))
	if err != nil {
		return Result{}, err
	}

	var page bytes.Buffer
	data := struct {
		Mode       Mode
		Width      Width
		Target     Target
		Title      string
		Styles     template.CSS
		Body       template.HTML
		LiveReload bool
		AssetBase  string
	}{
		Mode:       normalized.Mode,
		Width:      normalized.Width,
		Target:     normalized.Target,
		Title:      title,
		Styles:     template.CSS(stylesheet),
		Body:       template.HTML(body.String()),
		LiveReload: normalized.Target == TargetPreview,
		AssetBase:  normalized.AssetBase,
	}
	if err := pageTemplate.Execute(&page, data); err != nil {
		return Result{}, err
	}

	return Result{HTML: page.String(), Body: body.String(), Title: title}, nil
}

// Title extracts the first H1 as plain text and falls back to the filename.
func Title(source []byte, sourcePath string) (string, error) {
	normalized, err := normalizeSourcePath(sourcePath)
	if err != nil {
		return "", err
	}
	engine := newEngine(false)
	document := engine.Parser().Parse(text.NewReader(source))
	return extractTitle(document, source, normalized), nil
}

// NewGFM creates a Markdown engine with m2h's standard GFM extensions plus any
// renderer-specific extensions supplied by the caller.
func NewGFM(extraExtensions ...goldmark.Extender) goldmark.Markdown {
	extensions := append([]goldmark.Extender{extension.GFM}, extraExtensions...)
	return goldmark.New(goldmark.WithExtensions(extensions...))
}

// SanitizeDangerousURLs removes link and image destinations that Goldmark's
// HTML renderer would reject, allowing alternate renderers to share the same
// default-safe URL contract.
func SanitizeDangerousURLs(document ast.Node) error {
	return ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Link:
			if goldmarkhtml.IsDangerousURL(bytes.TrimSpace(typed.Destination)) {
				typed.Destination = nil
			}
		case *ast.Image:
			if goldmarkhtml.IsDangerousURL(bytes.TrimSpace(typed.Destination)) {
				typed.Destination = nil
			}
		}
		return ast.WalkContinue, nil
	})
}

func newEngine(unsafeHTML bool) goldmark.Markdown {
	rendererOptions := []renderer.Option{}
	if unsafeHTML {
		rendererOptions = append(rendererOptions, goldmarkhtml.WithUnsafe())
	}

	engine := NewGFM(
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(html.WithClasses(true)),
		),
	)
	engine.Renderer().AddOptions(rendererOptions...)
	return engine
}

func normalizeOptions(options RenderOptions) (RenderOptions, error) {
	switch options.Mode {
	case ModeLight, ModeDark, ModeAuto:
	default:
		return RenderOptions{}, &OptionError{Name: "mode", Value: string(options.Mode)}
	}
	if options.Width == "" {
		options.Width = WidthStandard
	}
	switch options.Width {
	case WidthStandard, WidthWide, WidthFull:
	default:
		return RenderOptions{}, &OptionError{Name: "width", Value: string(options.Width)}
	}
	switch options.Target {
	case TargetConvert, TargetPreview:
	default:
		return RenderOptions{}, &OptionError{Name: "target", Value: string(options.Target)}
	}

	normalizedSource, err := normalizeSourcePath(options.SourcePath)
	if err != nil {
		return RenderOptions{}, err
	}
	options.SourcePath = normalizedSource
	return options, nil
}

func normalizeSourcePath(sourcePath string) (string, error) {
	sourcePath = strings.ReplaceAll(sourcePath, "\\", "/")
	sourcePath = pathpkg.Clean(sourcePath)
	if sourcePath == "." || pathpkg.IsAbs(sourcePath) || escapesRoot(sourcePath) {
		return "", &OptionError{Name: "source path", Value: sourcePath}
	}
	return sourcePath, nil
}

func rewriteDocument(document ast.Node, options RenderOptions) error {
	if err := SanitizeDangerousURLs(document); err != nil {
		return err
	}
	return ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Link:
			typed.Destination = rewriteDestination(typed.Destination, options, false)
		case *ast.Image:
			typed.Destination = rewriteDestination(typed.Destination, options, true)
		}
		return ast.WalkContinue, nil
	})
}

func rewriteDestination(destination []byte, options RenderOptions, image bool) []byte {
	original := string(destination)
	pathPart, suffix := splitDestination(original)
	if !isRelativeLocalPath(pathPart) {
		return destination
	}

	if image {
		if options.Target == TargetConvert {
			return destination
		}
		resolved, ok := resolveWithinRoot(options.SourcePath, pathPart)
		if !ok {
			return destination
		}
		return []byte("/assets/" + resolved + suffix)
	}

	extension := pathpkg.Ext(pathPart)
	if !strings.EqualFold(extension, ".md") && !strings.EqualFold(extension, ".markdown") {
		if options.Target == TargetPreview {
			resolved, ok := resolveWithinRoot(options.SourcePath, pathPart)
			if !ok {
				return destination
			}
			return []byte("/assets/" + resolved + suffix)
		}
		return destination
	}
	if options.Target == TargetConvert {
		return []byte(strings.TrimSuffix(pathPart, extension) + ".html" + suffix)
	}

	resolved, ok := resolveWithinRoot(options.SourcePath, pathPart)
	if !ok {
		return destination
	}
	return []byte("/doc/" + resolved + suffix)
}

func splitDestination(destination string) (string, string) {
	if index := strings.IndexAny(destination, "?#"); index >= 0 {
		return destination[:index], destination[index:]
	}
	return destination, ""
}

func isRelativeLocalPath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "" && parsed.Host == ""
}

func resolveWithinRoot(sourcePath, destination string) (string, bool) {
	destination = strings.ReplaceAll(destination, "\\", "/")
	resolved := pathpkg.Clean(pathpkg.Join(pathpkg.Dir(sourcePath), destination))
	if resolved == "." || escapesRoot(resolved) || pathpkg.IsAbs(resolved) {
		return "", false
	}
	return resolved, true
}

func escapesRoot(value string) bool {
	return value == ".." || strings.HasPrefix(value, "../")
}

func extractTitle(document ast.Node, source []byte, sourcePath string) string {
	var title string
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || title != "" {
			return ast.WalkContinue, nil
		}
		heading, ok := node.(*ast.Heading)
		if !ok || heading.Level != 1 {
			return ast.WalkContinue, nil
		}
		title = normalizeTitle(string(heading.Text(source)))
		return ast.WalkStop, nil
	})
	if title != "" {
		return title
	}
	return pathpkg.Base(sourcePath)
}

func normalizeTitle(value string) string {
	return strings.Join(strings.Fields(stdhtml.UnescapeString(value)), " ")
}
