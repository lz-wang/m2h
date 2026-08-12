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
// preview routes as Markdown links and images. Convert output intentionally
// keeps those URLs relative so copied assets continue to work offline.
func addRawHTMLURLRewriter(engine goldmark.Markdown, options RenderOptions) {
	if options.Target != TargetPreview {
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
				var asset bool
				switch strings.ToLower(attribute.Key) {
				case "href":
					asset = false
				case "src", "poster", "data":
					asset = true
				default:
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
