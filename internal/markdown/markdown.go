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
	emoji "github.com/yuin/goldmark-emoji"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
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
}

// Heading is one entry of the document's table of contents, extracted from the
// same Goldmark AST that produced the HTML so heading ids always match the
// rendered anchors.
type Heading struct {
	Level int
	ID    string
	Text  string
}

// Result contains both the reusable rendered body and a complete HTML page.
type Result struct {
	HTML     string
	Body     string
	Title    string
	Headings []Heading
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
{{.ExtraHead}}</head>
<body class="m2h-page">
  <article class="markdown-body">
{{.Body}}  </article>
{{.ExtraBody}}</body>
</html>
`))

// Render parses standard GFM once, rewrites its AST, and renders a complete page.
func Render(source []byte, options RenderOptions) (Result, error) {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return Result{}, err
	}

	engine := newEngine()
	addRawHTMLURLRewriter(engine, normalized)
	context := parser.NewContext(parser.WithIDs(newGitHubIDs()))
	document := engine.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if err := rewriteDocument(document, normalized); err != nil {
		return Result{}, err
	}

	title := extractTitle(document, source, normalized.SourcePath)
	headings := extractHeadings(document, source)
	var body bytes.Buffer
	if err := engine.Renderer().Render(&body, source, document); err != nil {
		return Result{}, err
	}

	stylesheet, err := assets.Stylesheet(string(normalized.Mode))
	if err != nil {
		return Result{}, err
	}
	extraHead, extraBody, err := runtimeFragments(normalized, body.String())
	if err != nil {
		return Result{}, err
	}

	var page bytes.Buffer
	data := struct {
		Mode      Mode
		Width     Width
		Target    Target
		Title     string
		Styles    template.CSS
		Body      template.HTML
		ExtraHead template.HTML
		ExtraBody template.HTML
	}{
		Mode:      normalized.Mode,
		Width:     normalized.Width,
		Target:    normalized.Target,
		Title:     title,
		Styles:    template.CSS(stylesheet),
		Body:      template.HTML(body.String()),
		ExtraHead: extraHead,
		ExtraBody: extraBody,
	}
	if err := pageTemplate.Execute(&page, data); err != nil {
		return Result{}, err
	}

	return Result{HTML: page.String(), Body: body.String(), Title: title, Headings: headings}, nil
}

// Title extracts the first H1 as plain text and falls back to the filename.
func Title(source []byte, sourcePath string) (string, error) {
	normalized, err := normalizeSourcePath(sourcePath)
	if err != nil {
		return "", err
	}
	engine := newEngine()
	context := parser.NewContext(parser.WithIDs(newGitHubIDs()))
	document := engine.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	return extractTitle(document, source, normalized), nil
}

// newGFM creates a Markdown engine with m2h's standard GFM extensions plus any
// renderer-specific extensions supplied by the caller.
func newGFM(extraExtensions ...goldmark.Extender) goldmark.Markdown {
	extensions := append([]goldmark.Extender{extension.GFM}, extraExtensions...)
	return goldmark.New(goldmark.WithExtensions(extensions...))
}

// sanitizeDangerousURLs removes link and image destinations that Goldmark's
// HTML renderer would reject, allowing alternate renderers to share the same
// default-safe URL contract.
func sanitizeDangerousURLs(document ast.Node) error {
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

func newEngine() goldmark.Markdown {
	engine := newGFM(
		extension.Footnote,
		emoji.New(emoji.WithRenderingMethod(emoji.Unicode)),
		AlertExtension,
		ExtendedMarkupExtension,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(html.WithClasses(true)),
		),
	)
	engine.Renderer().AddOptions(goldmarkhtml.WithUnsafe())
	engine.Parser().AddOptions(parser.WithAutoHeadingID())
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
	if err := sanitizeDangerousURLs(document); err != nil {
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

// extractHeadings collects every heading that received an id, in document
// order. The ids come from the same Goldmark parser context that produced the
// HTML anchors (GitHub-compatible slugs with CJK support and duplicate
// suffixes), so the table of contents can never drift from the rendered ids.
func extractHeadings(document ast.Node, source []byte) []Heading {
	headings := make([]Heading, 0)
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := node.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		rawID, ok := heading.AttributeString("id")
		if !ok {
			return ast.WalkContinue, nil
		}
		id, ok := rawID.([]byte)
		if !ok {
			return ast.WalkContinue, nil
		}
		headings = append(headings, Heading{
			Level: heading.Level,
			ID:    string(id),
			Text:  normalizeTitle(string(heading.Text(source))),
		})
		return ast.WalkContinue, nil
	})
	return headings
}

func normalizeTitle(value string) string {
	return strings.Join(strings.Fields(stdhtml.UnescapeString(value)), " ")
}
