package markdown

import (
	"bytes"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// RenderShare uses the ordinary Markdown renderer and applies the public
// HTML and URL policy to its output. The callback maps admitted local assets
// to share URLs. Only references in retained elements reach the callback.
func RenderShare(source []byte, sourcePath string, assetURL func(string) string) (Result, error) {
	result, err := Render(source, RenderOptions{SourcePath: sourcePath})
	if err != nil {
		return Result{}, err
	}
	nodes, err := html.ParseFragment(strings.NewReader(result.Body), &html.Node{
		Type: html.ElementNode, Data: "div", DataAtom: atom.Div,
	})
	if err != nil {
		return Result{}, fmt.Errorf("parse public HTML: %w", err)
	}
	policy := shareHTMLPolicy{sourcePath: sourcePath, assetURL: assetURL}
	var body bytes.Buffer
	for _, node := range nodes {
		if policy.clean(node) {
			if err := html.Render(&body, node); err != nil {
				return Result{}, fmt.Errorf("render public HTML: %w", err)
			}
		}
	}
	result.Body = body.String()
	return result, nil
}

var shareElements = strings.Fields("a abbr b blockquote br caption cite code col colgroup dd del details div dl dt em figcaption figure h1 h2 h3 h4 h5 h6 hr i img input ins kbd label li mark ol p picture pre q rp rt ruby s samp section small source span strong sub summary sup table tbody td th thead tfoot tr u ul var video audio")

type shareHTMLPolicy struct {
	sourcePath string
	assetURL   func(string) string
}

func (policy shareHTMLPolicy) clean(node *html.Node) bool {
	if node.Type == html.TextNode {
		return true
	}
	if node.Type != html.ElementNode || node.Namespace != "" {
		return false
	}
	if !slices.Contains(shareElements, node.Data) {
		return false
	}
	// Goldmark emits disabled checkboxes and m2h emits tab radios.
	if node.Data == "input" && !shareInput(node) {
		return false
	}

	attrs := node.Attr[:0]
	for _, attr := range node.Attr {
		if attr.Namespace != "" {
			continue
		}
		if value, keep := policy.attribute(node.Data, attr); keep {
			attr.Val = value
			attrs = append(attrs, attr)
		}
	}
	node.Attr = attrs
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		if !policy.clean(child) {
			node.RemoveChild(child)
		}
		child = next
	}
	return true
}

func shareInput(node *html.Node) bool {
	var kind, class string
	var disabled bool
	for _, attr := range node.Attr {
		switch attr.Key {
		case "type":
			kind = attr.Val
		case "class":
			class = attr.Val
		case "disabled":
			disabled = true
		}
	}
	return kind == "checkbox" && disabled || kind == "radio" && class == "m2h-tabbed-radio"
}

func (policy shareHTMLPolicy) attribute(tag string, attr html.Attribute) (string, bool) {
	value := attr.Val
	switch attr.Key {
	case "href":
		if tag == "a" {
			value = policy.destination(value, true)
			return value, value != ""
		}
	case "src", "poster":
		if tag == "img" || tag == "video" || tag == "audio" || tag == "source" {
			value = policy.destination(value, false)
			return value, value != ""
		}
	case "srcset":
		if tag == "img" || tag == "source" {
			value = policy.srcset(value)
			return value, value != ""
		}
	case "id", "class", "title", "alt", "align", "width", "height", "start", "reversed", "open",
		"colspan", "rowspan", "scope", "role", "aria-label", "aria-hidden", "aria-describedby",
		"data-m2h-tab-id", "data-footnote-ref", "data-footnote-backref", "data-footnotes",
		"media", "sizes", "controls", "loading", "decoding", "type", "name", "for", "checked", "disabled":
		return value, true
	case "style":
		compacted := strings.ReplaceAll(strings.TrimSpace(value), " ", "")
		switch strings.TrimSuffix(compacted, ";") {
		case "text-align:left", "text-align:center", "text-align:right":
			return compacted, true
		}
	}
	return "", false
}

func (policy shareHTMLPolicy) destination(value string, link bool) string {
	value = strings.TrimSpace(value)
	switch ClassifyDestination(value) {
	case DestinationFragment:
		if link {
			return value
		}
	case DestinationExternal, DestinationProtocolRelative:
		parsed, err := url.Parse(value)
		if err == nil && parsed.Hostname() != "" && parsed.User == nil &&
			(parsed.Scheme == "" || parsed.Scheme == "https" || parsed.Scheme == "http") {
			return value
		}
	case DestinationRelative, DestinationRootRelative:
		local, ok := ParseLocalDestination(value)
		if !ok {
			return ""
		}
		decoded, err := url.PathUnescape(local.Path)
		if err != nil {
			return ""
		}
		local.Path = decoded
		resolved, ok := ResolveLocalDestination(policy.sourcePath, "", local)
		if !ok {
			return ""
		}
		asset := policy.assetURL(resolved)
		if asset != "" && link && local.Fragment != "" {
			asset += "#" + local.Fragment
		}
		return asset
	}
	return ""
}

func (policy shareHTMLPolicy) srcset(value string) string {
	spans := srcsetURLRanges(value)
	candidates := make([]string, 0, len(spans))
	for index, span := range spans {
		end := len(value)
		if index+1 < len(spans) {
			end = spans[index+1].start
		}
		descriptor, _, _ := strings.Cut(value[span.end:end], ",")
		if rewritten := policy.destination(value[span.start:span.end], false); rewritten != "" {
			candidates = append(candidates, strings.TrimSpace(rewritten+" "+strings.TrimSpace(descriptor)))
		}
	}
	return strings.Join(candidates, ", ")
}
