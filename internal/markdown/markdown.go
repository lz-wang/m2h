// Package markdown is the sole GFM parsing and HTML rendering core for m2h.
// It renders a Markdown source into an HTML fragment (body, title, headings);
// assembling a complete page around the fragment is the caller's job.
package markdown

import (
	"bytes"
	stdhtml "html"
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
)

// Mode controls the rendered color theme of a surrounding page.
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

// URLMode controls how local links and images in the Markdown source are
// rewritten for the surrounding document.
type URLMode uint8

const (
	// URLPassthrough keeps every local destination exactly as written, so an
	// exported HTML file continues to reference the source tree's files.
	URLPassthrough URLMode = iota
	// URLWeb rewrites local Markdown links to /doc/<path> and other local
	// references to /assets/<path>, the address space of the document server.
	URLWeb
)

// String names the URL mode for option errors.
func (mode URLMode) String() string {
	switch mode {
	case URLPassthrough:
		return "passthrough"
	case URLWeb:
		return "web"
	default:
		return "URLMode(%d)"
	}
}

// RenderOptions configures a single Markdown render.
type RenderOptions struct {
	SourcePath string
	URLMode    URLMode
}

// Heading is one entry of the document's table of contents, extracted from the
// same Goldmark AST that produced the HTML so heading ids always match the
// rendered anchors. Line is the 1-based source line the heading starts on.
type Heading struct {
	Level int
	ID    string
	Text  string
	Line  int
}

// Result contains the rendered fragment and its metadata.
type Result struct {
	Body     string
	Title    string
	Headings []Heading
}

// Render parses standard GFM once, rewrites its AST, and renders the body
// fragment.
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

	return Result{Body: body.String(), Title: title, Headings: headings}, nil
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
	switch options.URLMode {
	case URLPassthrough, URLWeb:
	default:
		return RenderOptions{}, &OptionError{Name: "url mode", Value: options.URLMode.String()}
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

// rewriteDestination maps a local destination onto the address space selected
// by URLMode. External, protocol-relative and fragment destinations are always
// kept as written; URLPassthrough also keeps local destinations unchanged so
// an exported file continues to reference the source tree.
func rewriteDestination(destination []byte, options RenderOptions, image bool) []byte {
	if options.URLMode != URLWeb {
		return destination
	}

	original := string(destination)
	local, ok := ParseLocalDestination(original)
	if !ok {
		return destination
	}
	resolved, ok := ResolveLocalDestination(options.SourcePath, "", local)
	if !ok {
		return destination
	}
	_, suffix := splitDestination(original)
	if image || !RoutesToDocument(local.Path) {
		return []byte("/assets/" + resolved + suffix)
	}
	return []byte("/doc/" + resolved + suffix)
}

// RoutesToDocument reports whether a local destination — identified by its
// path part exactly as the Markdown source wrote it, before any percent
// decoding — is a Markdown document the web renderer routes to /doc. Every
// other local destination routes to /assets. The check command reuses this
// predicate so routing decisions can never disagree with the renderer: an
// encoded extension (guide%2Emd) is not a Markdown destination here, and the
// assets route that receives it never serves Markdown files.
func RoutesToDocument(pathPart string) bool {
	extension := pathpkg.Ext(pathPart)
	return strings.EqualFold(extension, ".md") || strings.EqualFold(extension, ".markdown")
}

func splitDestination(destination string) (string, string) {
	if index := strings.IndexAny(destination, "?#"); index >= 0 {
		return destination[:index], destination[index:]
	}
	return destination, ""
}

// DestinationKind classifies the addressing model of one Markdown URL.
// The zero value represents an empty or syntactically invalid destination.
type DestinationKind uint8

const (
	DestinationRelative DestinationKind = iota + 1
	DestinationRootRelative
	DestinationFragment
	DestinationExternal
	DestinationProtocolRelative
)

// LocalDestinationBase identifies which logical directory anchors a local
// destination: the referencing document's directory or its workspace root.
type LocalDestinationBase uint8

const (
	DestinationBaseDocument LocalDestinationBase = iota + 1
	DestinationBaseRoot
)

// LocalDestination is one parsed local reference: the path part with query
// and fragment split off, each kept without its leading "?"/"#" exactly as
// the Markdown source wrote it. Root-relative paths omit their one leading
// slash because it selects Base; it never denotes the host filesystem root.
type LocalDestination struct {
	Path     string
	Query    string
	Fragment string
	Base     LocalDestinationBase
}

// ClassifyDestination reports how a Markdown destination is addressed.
// A single leading slash means workspace-root-relative; two leading slashes
// remain a protocol-relative network URL. Empty and malformed URLs return the
// zero value.
func ClassifyDestination(destination string) DestinationKind {
	pathPart, _ := splitDestination(destination)
	if strings.HasPrefix(destination, "#") {
		return DestinationFragment
	}
	if pathPart == "" {
		return 0
	}
	if strings.HasPrefix(pathPart, "//") {
		return DestinationProtocolRelative
	}
	parsed, err := url.Parse(pathPart)
	if err != nil {
		return 0
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return DestinationExternal
	}
	if strings.HasPrefix(pathPart, "/") {
		return DestinationRootRelative
	}
	return DestinationRelative
}

// ParseLocalDestination splits a document-relative or workspace-root-relative
// destination into its logical path, query, fragment and base. Scheme URLs,
// protocol-relative URLs and path-less fragments return false.
func ParseLocalDestination(destination string) (LocalDestination, bool) {
	pathPart, suffix := splitDestination(destination)
	kind := ClassifyDestination(destination)
	if kind != DestinationRelative && kind != DestinationRootRelative {
		return LocalDestination{}, false
	}
	parsed := LocalDestination{Path: pathPart, Base: DestinationBaseDocument}
	if kind == DestinationRootRelative {
		parsed.Path = strings.TrimPrefix(pathPart, "/")
		parsed.Base = DestinationBaseRoot
	}
	if parsed.Path == "" {
		return LocalDestination{}, false
	}
	if suffix == "" {
		return parsed, true
	}
	query := suffix
	if before, fragment, found := strings.Cut(suffix, "#"); found {
		query = before
		parsed.Fragment = fragment
	}
	parsed.Query = strings.TrimPrefix(query, "?")
	return parsed, true
}

// ResolveLocalDestination resolves a parsed local destination inside one
// logical workspace root. sourcePath and rootPath use the public slash paths
// exposed by the server; rootPath is empty for a single-root workspace and is
// the root id (for example "r1") in a multi-root workspace.
func ResolveLocalDestination(sourcePath, rootPath string, destination LocalDestination) (string, bool) {
	normalizedRoot, ok := normalizeLogicalRootPath(rootPath)
	if !ok || !withinLogicalRoot(sourcePath, normalizedRoot) {
		return "", false
	}

	var basePath string
	switch destination.Base {
	case DestinationBaseDocument:
		basePath = pathpkg.Dir(sourcePath)
	case DestinationBaseRoot:
		basePath = normalizedRoot
	default:
		return "", false
	}
	return resolveWithinLogicalRoot(basePath, normalizedRoot, destination.Path)
}

// InvalidLocalDestination reports whether destination is a relative-looking
// reference whose percent-encoding is malformed. Such a destination is not a
// valid relative local path, but it is also not a working URL — the browser
// can never resolve it — so the check command reports it as unreachable.
func InvalidLocalDestination(destination string) bool {
	pathPart, _ := splitDestination(destination)
	if pathPart == "" || strings.HasPrefix(pathPart, "//") {
		return false
	}
	_, err := url.Parse(pathPart)
	return err != nil
}

func resolveWithinLogicalRoot(basePath, rootPath, destination string) (string, bool) {
	destination = strings.ReplaceAll(destination, "\\", "/")
	resolved := pathpkg.Clean(pathpkg.Join(basePath, destination))
	if resolved == "." || escapesRoot(resolved) || pathpkg.IsAbs(resolved) || !withinLogicalRoot(resolved, rootPath) {
		return "", false
	}
	return resolved, true
}

func normalizeLogicalRootPath(rootPath string) (string, bool) {
	if rootPath == "" {
		return "", true
	}
	rootPath = strings.ReplaceAll(rootPath, "\\", "/")
	rootPath = pathpkg.Clean(rootPath)
	if rootPath == "." || pathpkg.IsAbs(rootPath) || escapesRoot(rootPath) {
		return "", false
	}
	return rootPath, true
}

func withinLogicalRoot(value, rootPath string) bool {
	if rootPath == "" {
		return !pathpkg.IsAbs(value) && !escapesRoot(value)
	}
	return value == rootPath || strings.HasPrefix(value, rootPath+"/")
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
	locator := newSourceLocator(source)
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
		line := 1
		if heading.Lines().Len() > 0 {
			line, _ = locator.locate(heading.Lines().At(0).Start)
		}
		headings = append(headings, Heading{
			Level: heading.Level,
			ID:    string(id),
			Text:  normalizeTitle(string(heading.Text(source))),
			Line:  line,
		})
		return ast.WalkContinue, nil
	})
	return headings
}

func normalizeTitle(value string) string {
	return strings.Join(strings.Fields(stdhtml.UnescapeString(value)), " ")
}
