package check

import (
	"fmt"

	"github.com/lz-wang/m2h/internal/markdown"
)

// Rule identifiers name diagnostic categories. They appear in both report
// formats and are part of the stable contract CI and tooling consume, so
// they must never be renamed.
const (
	// RuleFrontMatterInvalid: frontmatter YAML cannot be parsed or its root
	// node is not a mapping. The document cannot be rendered at all.
	RuleFrontMatterInvalid = "frontmatter.invalid"
	// RuleLocalTargetMissing: a local link, image or attachment points at a
	// target that does not exist (or a name the workspace would refuse).
	RuleLocalTargetMissing = "local-target.missing"
	// RuleLocalTargetNotRegular: the referenced target exists but is not a
	// regular file (a directory, device, …).
	RuleLocalTargetNotRegular = "local-target.not-regular"
	// RuleLocalTargetOutsideRoot: a `../` segment or a symlink resolves the
	// target beyond the workspace root, or the path crosses a symlink
	// directory the workspace refuses to follow.
	RuleLocalTargetOutsideRoot = "local-target.outside-root"
	// RuleMarkdownTargetNotServed: the Markdown file exists but the current
	// scope does not serve it (single-file mode, or excluded by the glob and
	// depth filters).
	RuleMarkdownTargetNotServed = "markdown-target.not-served"
	// RuleAnchorMissing: a `#anchor` or `target.md#anchor` fragment points at
	// a heading id the target document does not contain.
	RuleAnchorMissing = "anchor.missing"

	// RuleImageAltEmpty: an image reference carries no alt text. It stays a
	// warning — not an error — because decorative images legitimately use an
	// empty alt.
	RuleImageAltEmpty = "image.alt-empty"
	// RuleDocumentMultipleH1: a document contains more than one H1 while the
	// sidebar and document title use only the first.
	RuleDocumentMultipleH1 = "document.multiple-h1"
	// RuleFrontMatterDateInvalid: a frontmatter date field m2h recognizes
	// holds a value that is not a valid ISO date, so it never reaches the
	// toolbar summary.
	RuleFrontMatterDateInvalid = "frontmatter.date-invalid"
	// RuleLinkEmptyDestination: a link, image or raw HTML URL attribute has
	// an empty destination.
	RuleLinkEmptyDestination = "link.empty-destination"

	// RuleHeadingLevelSkip: a heading jumps down more than one level. A
	// warning — not an error — because the document still renders and
	// navigates; closing a section by any number of levels upwards stays
	// legal.
	RuleHeadingLevelSkip = "heading.level-skip"
	// RuleReferenceUndefined: an explicit reference-style use — [text][label]
	// or [text][] — names a label no definition provides, so the renderer
	// outputs literal brackets instead of a link.
	RuleReferenceUndefined = "reference.undefined"
	// RuleReferenceUnused: a reference definition nothing ever references.
	RuleReferenceUnused = "reference.unused"

	// RuleFootnoteUndefined: a [^label] use with no matching definition, so
	// the footnote marker renders as literal text instead of a link.
	RuleFootnoteUndefined = "footnote.undefined"
	// RuleFootnoteUnused: a footnote definition no marker ever references.
	RuleFootnoteUnused = "footnote.unused"
	// RuleFootnoteEmpty: a footnote definition without any content, which
	// renders an empty footnote block at the end of the document.
	RuleFootnoteEmpty = "footnote.empty"

	// RuleTableColumnMismatch: a table row — or a header/delimiter pair —
	// whose column count disagrees with the table. The renderer pads or
	// truncates accepted rows, and rejects the whole table when the
	// delimiter disagrees with the header.
	RuleTableColumnMismatch = "table.column-mismatch"
	// RuleHTMLCommentUnclosed: a `<!--` HTML comment block that never saw
	// its closing `-->`; everything after it renders as comment content.
	RuleHTMLCommentUnclosed = "html.comment-unclosed"
	// RuleLinkReversed: a high-confidence `(text)[destination]` with a
	// URL- or path-like destination — Markdown link syntax written backwards.
	RuleLinkReversed = "link.reversed"
)

// frontMatterDateKeys are the frontmatter fields m2h normalizes into date
// summaries; any other key may hold arbitrary metadata without a warning.
var frontMatterDateKeys = map[string]bool{
	"date":        true,
	"create_date": true,
	"create_at":   true,
	"create_time": true,
	"update_date": true,
	"update_at":   true,
	"update_time": true,
}

// emptyDestinationMessage describes an empty destination per reference kind.
func emptyDestinationMessage(kind markdown.ReferenceKind) string {
	switch kind {
	case markdown.ReferenceImage:
		return "image destination is empty"
	case markdown.ReferenceRawHTML:
		return "raw HTML URL attribute is empty"
	default:
		return "link destination is empty"
	}
}

// notServedReason explains why an existing Markdown target is unreachable in
// the current scope.
type notServedReason string

const (
	notServedSingleFile notServedReason = "single-file"
	notServedDepth      notServedReason = "depth"
	notServedGlob       notServedReason = "glob"
)

func (reason notServedReason) message() string {
	switch reason {
	case notServedSingleFile:
		return "is not available in single-file mode"
	case notServedDepth:
		return "is excluded by the depth limit"
	case notServedGlob:
		return "is excluded by the glob filter"
	}
	return "is not served by this workspace"
}

func missingMessage(target string, err error) string {
	if err != nil {
		return fmt.Sprintf("target %q is not accessible: %v", target, err)
	}
	return fmt.Sprintf("target %q does not exist", target)
}
