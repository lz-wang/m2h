package markdown

import (
	"bytes"
	"io"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

// addRawHTMLURLRewriter makes local URLs inside raw HTML follow the same
// web routes as Markdown links and images. Passthrough renders intentionally
// keep those URLs relative so the source tree's files keep working.
func addRawHTMLURLRewriter(engine goldmark.Markdown, options RenderOptions) {
	if options.URLMode != URLWeb {
		return
	}
	engine.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&rawHTMLURLRenderer{
			options: options,
			writer:  goldmarkhtml.NewWriter(),
		}, 100),
	))
}

type rawHTMLURLRenderer struct {
	options RenderOptions
	writer  goldmarkhtml.Writer
}

func (renderer *rawHTMLURLRenderer) RegisterFuncs(registerer renderer.NodeRendererFuncRegisterer) {
	registerer.Register(ast.KindHTMLBlock, renderer.renderHTMLBlock)
	registerer.Register(ast.KindRawHTML, renderer.renderRawHTML)
}

func (renderer *rawHTMLURLRenderer) renderHTMLBlock(
	writer util.BufWriter,
	source []byte,
	node ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	block := node.(*ast.HTMLBlock)
	if entering {
		renderer.writer.SecureWrite(writer, rewriteRawHTMLURLs(block.Lines().Value(source), renderer.options))
	} else if block.HasClosure() {
		renderer.writer.SecureWrite(writer, rewriteRawHTMLURLs(block.ClosureLine.Value(source), renderer.options))
	}
	return ast.WalkContinue, nil
}

func (renderer *rawHTMLURLRenderer) renderRawHTML(
	writer util.BufWriter,
	source []byte,
	node ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	raw := node.(*ast.RawHTML)
	_, _ = writer.Write(rewriteRawHTMLURLs(raw.Segments.Value(source), renderer.options))
	return ast.WalkSkipChildren, nil
}

func rewriteRawHTMLURLs(raw []byte, options RenderOptions) []byte {
	tokenizer := html.NewTokenizer(bytes.NewReader(raw))
	var rewritten bytes.Buffer
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				return rewritten.Bytes()
			}
			return raw
		case html.StartTagToken, html.SelfClosingTagToken:
			original := append([]byte(nil), tokenizer.Raw()...)
			token := tokenizer.Token()
			changed := false
			for index := range token.Attr {
				attribute := &token.Attr[index]
				asset, relevant := isRawHTMLURLAttribute(attribute.Key)
				if !relevant {
					continue
				}
				destination := string(rewriteDestination([]byte(attribute.Val), options, asset))
				if destination == attribute.Val {
					continue
				}
				attribute.Val = destination
				changed = true
			}
			if changed {
				rewritten.WriteString(token.String())
			} else {
				rewritten.Write(original)
			}
		default:
			rewritten.Write(tokenizer.Raw())
		}
	}
}

// rawHTMLURLAttributes lists the HTML attributes whose values m2h treats as
// URLs when rendering raw HTML. Rendering rewrites their values onto the web
// routes; Inspect extracts them as document references. The boolean reports
// whether the URL is an asset reference (src/poster/data, routed to /assets)
// rather than a link (href, Markdown targets route to /doc). Both features
// share this one list so they can never drift apart.
var rawHTMLURLAttributes = map[string]bool{
	"href":   false,
	"src":    true,
	"poster": true,
	"data":   true,
}

// isRawHTMLURLAttribute reports whether an attribute name (case-insensitive)
// carries a URL, and whether that URL is an asset reference.
func isRawHTMLURLAttribute(key string) (asset bool, relevant bool) {
	asset, relevant = rawHTMLURLAttributes[strings.ToLower(key)]
	return asset, relevant
}
