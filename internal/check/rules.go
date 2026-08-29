package check

import "fmt"

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
)

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
