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
	target, err := decodeLocalPath(reference)
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
) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, reference := range current.inspection.References {
		diagnostics = append(diagnostics, checkReference(scope, index, resolver, current, reference)...)
	}
	return diagnostics
}

// checkReference resolves one reference. Non-relative destinations (scheme
// URLs, absolute paths) are deliberately ignored: the web renderer leaves
// them untouched, so check has nothing to verify.
func checkReference(
	scope documentScope,
	index map[string]*indexedDocument,
	resolver *targetResolver,
	current *indexedDocument,
	reference markdown.Reference,
) []Diagnostic {
	if reference.Destination == "" || reference.Destination == "#" {
		return nil
	}

	// A bare fragment addresses the referencing document itself; it is not a
	// relative local path, but its anchor is still verifiable.
	if fragment, ok := strings.CutPrefix(reference.Destination, "#"); ok {
		if current.hasAnchor(fragment) {
			return nil
		}
		return []Diagnostic{current.diagnostic(SeverityError, RuleAnchorMissing,
			fmt.Sprintf("heading %q does not exist in %q", "#"+fragment, current.relative), reference)}
	}

	local, ok := markdown.ParseLocalDestination(reference.Destination)
	if !ok {
		if !markdown.InvalidLocalDestination(reference.Destination) {
			return nil
		}
		// Malformed percent-encoding can never resolve in the browser.
		return []Diagnostic{current.diagnostic(SeverityError, RuleLocalTargetMissing,
			fmt.Sprintf("target %q is not accessible: invalid URL encoding", reference.Destination), reference)}
	}

	resolved, ok := markdown.ResolveLocalDestination(current.relative, local.Path)
	if !ok {
		return []Diagnostic{current.diagnostic(SeverityError, RuleLocalTargetOutsideRoot,
			fmt.Sprintf("target %q resolves outside the workspace root", local.Path), reference)}
	}

	status := resolver.resolve(resolved)
	switch status.state {
	case targetMissing:
		return []Diagnostic{current.diagnostic(SeverityError, RuleLocalTargetMissing,
			missingMessage(status.target, status.err), reference)}
	case targetNotRegular:
		return []Diagnostic{current.diagnostic(SeverityError, RuleLocalTargetNotRegular,
			fmt.Sprintf("target %q is not a regular file", status.target), reference)}
	case targetCrossesSymlink:
		return []Diagnostic{current.diagnostic(SeverityError, RuleLocalTargetOutsideRoot,
			fmt.Sprintf("target %q crosses a symlink directory the workspace refuses to follow", status.target), reference)}
	case targetOutsideRoot:
		return []Diagnostic{current.diagnostic(SeverityError, RuleLocalTargetOutsideRoot,
			fmt.Sprintf("target %q resolves outside the workspace root", local.Path), reference)}
	}

	if !files.IsMarkdown(status.target) {
		// Assets only need to exist and be regular; the glob and depth
		// filters never applied to them.
		return nil
	}
	if !scope.allowsDocument(status.target) {
		return []Diagnostic{current.diagnostic(SeverityError, RuleMarkdownTargetNotServed,
			fmt.Sprintf("Markdown target %q exists but %s", status.target, scope.notServedReason(status.target).message()), reference)}
	}
	if target, ok := index[status.target]; ok && !target.hasAnchor(local.Fragment) {
		return []Diagnostic{current.diagnostic(SeverityError, RuleAnchorMissing,
			fmt.Sprintf("heading %q does not exist in %q", "#"+local.Fragment, status.target), reference)}
	}
	return nil
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
func (current *indexedDocument) diagnostic(severity Severity, rule string, message string, reference markdown.Reference) Diagnostic {
	return Diagnostic{
		Path:     current.display,
		Line:     reference.Line,
		Column:   reference.Column,
		Severity: severity,
		Rule:     rule,
		Message:  message,
	}
}

// decodeLocalPath repeatedly percent-decodes a path exactly the way the
// server decodes a request path (bounded), and rejects encodings that decode
// into a root escape or an absolute path.
func decodeLocalPath(value string) (string, error) {
	for iteration := range 8 {
		decoded, err := url.PathUnescape(value)
		if err != nil {
			return "", fmt.Errorf("invalid URL encoding: %w", err)
		}
		if decoded == value {
			break
		}
		value = decoded
		if iteration == 7 {
			return "", fmt.Errorf("path exceeds decoding limit")
		}
	}
	if escapesRootPath(value) {
		return value, fmt.Errorf("decoded path escapes the workspace root")
	}
	return value, nil
}

// escapesRootPath reports whether a slash path cannot stay inside a workspace
// root: an absolute path, or any ".." segment — mirroring how the served
// workspace refuses request paths.
func escapesRootPath(value string) bool {
	value = strings.ReplaceAll(value, "\\", "/")
	if strings.HasPrefix(value, "/") {
		return true
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}
