package check

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

// targetState classifies the filesystem outcome of one reference target,
// mirroring what the document server's request resolution would do.
type targetState uint8

const (
	targetOK targetState = iota + 1
	targetMissing
	targetNotRegular
	targetOutsideRoot
	targetCrossesSymlink
)

// targetStatus is the resolved state of one unique reference target path.
// target is the percent-decoded root-relative path (set whenever decoding
// succeeded), and err carries the underlying lookup failure for targets that
// exist on disk but cannot be inspected, so the missing message can say why.
type targetStatus struct {
	state  targetState
	target string
	err    error
}

// targetResolver resolves and caches filesystem lookups for reference
// targets beneath one scope root. Resolution applies the same rules as the
// served workspace — percent decoding, exact path components, symlink
// resolution within the root, regular-file checks — so a target the resolver
// accepts is reachable in the browser and one it rejects is not. Each unique
// path is inspected once per run.
type targetResolver struct {
	root   string
	status map[string]targetStatus
}

func newTargetResolver(root string) *targetResolver {
	return &targetResolver{root: root, status: make(map[string]targetStatus)}
}

// resolve inspects one root-relative reference path (percent-encoded as the
// Markdown wrote it), caching the outcome per unique path.
func (resolver *targetResolver) resolve(reference string) targetStatus {
	if cached, ok := resolver.status[reference]; ok {
		return cached
	}
	status := resolver.inspect(reference)
	resolver.status[reference] = status
	return status
}

func (resolver *targetResolver) inspect(reference string) targetStatus {
	target, err := files.DecodeRelativePath(reference)
	if err != nil {
		return targetStatus{state: targetMissing, target: target, err: err}
	}
	if err := files.RequireExactPath(resolver.root, target); err != nil {
		if errors.Is(err, files.ErrExactPathSymlink) {
			return targetStatus{state: targetCrossesSymlink, target: target}
		}
		if errors.Is(err, files.ErrExactPathMissing) {
			return targetStatus{state: targetMissing, target: target}
		}
		return targetStatus{state: targetMissing, target: target, err: err}
	}

	resolved, err := files.CanonicalPath(filepath.Join(resolver.root, filepath.FromSlash(target)))
	if err != nil {
		if os.IsNotExist(err) {
			return targetStatus{state: targetMissing, target: target}
		}
		return targetStatus{state: targetMissing, target: target, err: err}
	}
	if !files.IsWithin(resolver.root, resolved) {
		return targetStatus{state: targetOutsideRoot, target: target}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return targetStatus{state: targetMissing, target: target}
		}
		return targetStatus{state: targetMissing, target: target, err: err}
	}
	if !info.Mode().IsRegular() {
		return targetStatus{state: targetNotRegular, target: target}
	}
	return targetStatus{state: targetOK, target: target}
}

// checkDocumentReferences resolves every reference of one indexed document
// against the scope, the index and the filesystem, returning diagnostics in
// document order.
func checkDocumentReferences(
	scope documentScope,
	index map[string]*indexedDocument,
	resolver *targetResolver,
	current *indexedDocument,
	rules RuleSet,
) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, reference := range current.inspection.References {
		diagnostics = append(diagnostics, checkReference(scope, index, resolver, current, reference, rules)...)
	}
	return diagnostics
}

// checkReference resolves one local reference. Scheme and protocol-relative
// URLs are deliberately ignored because the web renderer leaves them
// untouched, so check has no workspace target to verify.
func checkReference(
	scope documentScope,
	index map[string]*indexedDocument,
	resolver *targetResolver,
	current *indexedDocument,
	reference markdown.Reference,
	rules RuleSet,
) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	if reference.Kind == markdown.ReferenceImage && rules.Enabled(RuleImageAltEmpty) && strings.TrimSpace(reference.Text) == "" {
		// Reported alongside, not instead of, the target checks: an image can
		// have no alt text and a broken or empty destination at once.
		diagnostics = append(diagnostics, current.diagnostic(RuleImageAltEmpty,
			"image has no alt text", reference))
	}
	if reference.Destination == "" {
		if rules.Enabled(RuleLinkEmptyDestination) {
			diagnostics = append(diagnostics, current.diagnostic(RuleLinkEmptyDestination,
				emptyDestinationMessage(reference.Kind), reference))
		}
		return diagnostics
	}
	if !rules.NeedsTargetResolution() {
		return diagnostics
	}
	if reference.Destination == "#" {
		return diagnostics
	}

	// A bare fragment addresses the referencing document itself; it is not a
	// relative local path, but its anchor is still verifiable.
	if fragment, ok := strings.CutPrefix(reference.Destination, "#"); ok {
		if rules.Enabled(RuleAnchorMissing) && !current.hasAnchor(fragment) {
			diagnostics = append(diagnostics, current.diagnostic(RuleAnchorMissing,
				fmt.Sprintf("heading %q does not exist in %q", "#"+fragment, current.relative), reference))
		}
		return diagnostics
	}

	local, ok := markdown.ParseLocalDestination(reference.Destination)
	if !ok {
		if markdown.InvalidLocalDestination(reference.Destination) && rules.Enabled(RuleLocalTargetMissing) {
			// Malformed percent-encoding can never resolve in the browser.
			diagnostics = append(diagnostics, current.diagnostic(RuleLocalTargetMissing,
				fmt.Sprintf("target %q is not accessible: invalid URL encoding", reference.Destination), reference))
		}
		return diagnostics
	}

	resolved, ok := markdown.ResolveLocalDestination(current.relative, "", local)
	if !ok {
		if rules.Enabled(RuleLocalTargetOutsideRoot) {
			return append(diagnostics, current.diagnostic(RuleLocalTargetOutsideRoot,
				fmt.Sprintf("target %q resolves outside the workspace root", local.Path), reference))
		}
		return diagnostics
	}

	status := resolver.resolve(resolved)
	switch status.state {
	case targetMissing:
		if rules.Enabled(RuleLocalTargetMissing) {
			return append(diagnostics, current.diagnostic(RuleLocalTargetMissing,
				missingMessage(status.target, status.err), reference))
		}
		return diagnostics
	case targetNotRegular:
		if rules.Enabled(RuleLocalTargetNotRegular) {
			return append(diagnostics, current.diagnostic(RuleLocalTargetNotRegular,
				fmt.Sprintf("target %q is not a regular file", status.target), reference))
		}
		return diagnostics
	case targetCrossesSymlink:
		if rules.Enabled(RuleLocalTargetOutsideRoot) {
			return append(diagnostics, current.diagnostic(RuleLocalTargetOutsideRoot,
				fmt.Sprintf("target %q crosses a symlink directory the workspace refuses to follow", status.target), reference))
		}
		return diagnostics
	case targetOutsideRoot:
		if rules.Enabled(RuleLocalTargetOutsideRoot) {
			return append(diagnostics, current.diagnostic(RuleLocalTargetOutsideRoot,
				fmt.Sprintf("target %q resolves outside the workspace root", local.Path), reference))
		}
		return diagnostics
	}

	// Mirror the web renderer's routing, decided from the raw destination:
	// a link whose path part carries a Markdown extension routes to /doc;
	// everything else (images, raw HTML src/poster/data, and links to
	// non-Markdown names — including encoded ones like guide%2Emd) routes to
	// /assets, and the assets route never serves Markdown files. Such a
	// target is unreachable in the browser even though it exists on disk.
	if reference.Route != markdown.ReferenceRouteLink || !markdown.RoutesToDocument(local.Path) {
		if files.IsMarkdown(status.target) && rules.Enabled(RuleLocalTargetMissing) {
			return append(diagnostics, current.diagnostic(RuleLocalTargetMissing,
				fmt.Sprintf("target %q is not accessible: the assets route never serves Markdown files", status.target), reference))
		}
		// Assets only need to exist and be regular; the glob and depth
		// filters never applied to them.
		return diagnostics
	}
	if !scope.allowsDocument(status.target) {
		if rules.Enabled(RuleMarkdownTargetNotServed) {
			return append(diagnostics, current.diagnostic(RuleMarkdownTargetNotServed,
				fmt.Sprintf("Markdown target %q exists but %s", status.target, scope.notServedReason(status.target).message()), reference))
		}
		return diagnostics
	}
	// A target whose frontmatter failed to parse is never inspected, so its
	// headings are unknown; the frontmatter error already explains the break,
	// and deriving an anchor.missing on top of it would be groundless.
	if target, ok := index[status.target]; ok && target.inspectable && rules.Enabled(RuleAnchorMissing) && !target.hasAnchor(local.Fragment) {
		diagnostics = append(diagnostics, current.diagnostic(RuleAnchorMissing,
			fmt.Sprintf("heading %q does not exist in %q", "#"+local.Fragment, status.target), reference))
	}
	return diagnostics
}

// hasAnchor reports whether the document contains a heading with the given
// fragment as its id. Browsers percent-decode fragments before seeking an
// element, so a URL-encoded CJK anchor resolves too. An empty fragment
// addresses the document itself.
func (current *indexedDocument) hasAnchor(fragment string) bool {
	if fragment == "" {
		return true
	}
	if _, ok := current.anchors[fragment]; ok {
		return true
	}
	if decoded, err := url.PathUnescape(fragment); err == nil && decoded != fragment {
		_, ok := current.anchors[decoded]
		return ok
	}
	return false
}

// diagnostic builds one finding at a reference's source position.
func (current *indexedDocument) diagnostic(rule string, message string, reference markdown.Reference) Diagnostic {
	return current.diagnosticForRule(rule, message, markdown.Position{Line: reference.Line, Column: reference.Column})
}

// diagnosticForRule builds one finding at an explicit fact position,
// carrying the severity the rule's registry definition declares — the
// registry is the single source of truth, so no emission site can ever
// disagree with the rule table README documents. Every body fact enters
// this call already carrying file-level lines, so rules never adjust
// frontmatter offsets themselves.
func (current *indexedDocument) diagnosticForRule(rule string, message string, position markdown.Position) Diagnostic {
	definition, ok := ruleDefinition(rule)
	if !ok {
		panic("unknown internal check rule: " + rule)
	}
	return Diagnostic{
		Path:     current.display,
		Line:     position.Line,
		Column:   position.Column,
		Severity: definition.Severity,
		Rule:     rule,
		Message:  message,
	}
}
