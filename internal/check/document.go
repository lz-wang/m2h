package check

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

// document is one Markdown file inside the checked scope. relative is the
// normalized slash path the serve command would address the document by,
// absolute is its filesystem location, and display is the path shown in
// diagnostics: the input as the user wrote it, joined with the relative path,
// so path:line:column stays resolvable from the working directory.
type document struct {
	relative string
	absolute string
	display  string
}

// documentScope mirrors the serve command's rootScope: a single Markdown file
// becomes a scope rooted at its parent directory that admits only that one
// document (sibling assets stay reachable), while a directory scope admits
// every Markdown file the discovery query matches beneath the root. Keeping
// the scope rules identical means check and the WebUI can never disagree
// about which documents exist.
type documentScope struct {
	root      string
	single    bool
	file      string // single-file scope: the only admitted document
	discovery files.DiscoverOptions
	documents []document
}

// newSingleFileScope builds the scope for a resolved Markdown file input. The
// file's name is kept literally and never reinterpreted as a glob, so files
// named with glob metacharacters remain checkable. Filesystem identity
// (relative, absolute) comes from the resolved path, while the diagnostic
// display path keeps the input as the user wrote it: a symlinked input such
// as alias.md -> docs/real-name.md must report against alias.md, which stays
// resolvable from the working directory even though the resolved basename
// differs.
func newSingleFileScope(input string, resolved string) documentScope {
	root := filepath.Dir(resolved)
	relative := files.NormalizeRelativePath(filepath.Base(resolved))
	return documentScope{
		root:   root,
		single: true,
		file:   relative,
		documents: []document{{
			relative: relative,
			absolute: filepath.Join(root, filepath.FromSlash(relative)),
			display:  filepath.Clean(input),
		}},
	}
}

// newDirectoryScope builds the scope for a resolved directory input by
// reusing the serve command's discovery walk, so depth, glob and symlink
// safety behave identically in both commands.
func newDirectoryScope(ctx context.Context, input string, resolved string, discovery files.DiscoverOptions) (documentScope, error) {
	found, err := files.Discover(ctx, resolved, discovery)
	if err != nil {
		return documentScope{}, err
	}
	scope := documentScope{
		root:      resolved,
		discovery: discovery,
		documents: make([]document, 0, len(found.Markdown)),
	}
	for _, entry := range found.Markdown {
		scope.documents = append(scope.documents, newDocument(resolved, input, entry.RelativePath))
	}
	return scope, nil
}

func newDocument(root string, inputDir string, relative string) document {
	return document{
		relative: relative,
		absolute: filepath.Join(root, filepath.FromSlash(relative)),
		display:  filepath.Join(inputDir, filepath.FromSlash(relative)),
	}
}

// allowsDocument reports whether a normalized relative path is reachable
// through the scope, mirroring rootScope.allowsDocument on the server: a
// single-file scope admits only itself; a directory scope admits Markdown
// files that pass the depth and glob rules.
func (scope documentScope) allowsDocument(relative string) bool {
	if scope.single {
		return relative == scope.file
	}
	return files.IsMarkdown(relative) && files.Matches(relative, scope.discovery)
}

// notServedReason explains why an existing Markdown target is unreachable in
// the scope, distinguishing the single-file boundary from the depth and glob
// filters so the diagnostic can say which rule excluded it.
func (scope documentScope) notServedReason(relative string) notServedReason {
	if scope.single {
		return notServedSingleFile
	}
	if relative == "." || files.FileDepth(relative) > scope.discovery.Depth {
		return notServedDepth
	}
	return notServedGlob
}

// indexedDocument is one parsed document of a check run. inspectable is
// false when its frontmatter failed to parse — the WebUI refuses such a
// document with a 422, so its references can never be followed and are not
// checked — and diagnostics carries that document-level finding.
type indexedDocument struct {
	document
	inspectable bool
	diagnostics []Diagnostic
	frontMatter *markdown.FrontMatter
	inspection  markdown.Inspection
	anchors     map[string]struct{}
}

// indexDocument reads and inspects one document for the check index,
// mirroring the serve command's document pipeline: frontmatter is split
// first, then the body is inspected with the shared Markdown engine. A
// frontmatter that fails to parse makes the document non-inspectable — the
// WebUI refuses it with a 422, so its references can never be followed —
// and yields the frontmatter.invalid diagnostic instead. A file that
// vanished between discovery and reading returns (nil, nil).
func indexDocument(current document) (*indexedDocument, error) {
	source, err := os.ReadFile(current.absolute)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read Markdown %q: %w", current.absolute, err)
	}

	body, frontMatter, err := markdown.ParseFrontMatter(source)
	if err != nil {
		return &indexedDocument{
			document:    current,
			inspectable: false,
			diagnostics: []Diagnostic{{
				Path:     current.display,
				Line:     1,
				Column:   1,
				Severity: SeverityError,
				Rule:     RuleFrontMatterInvalid,
				Message:  err.Error(),
			}},
		}, nil
	}

	inspection := markdown.Inspect(body)
	// Reference positions are body-relative; shift them past the frontmatter
	// block so reported lines match the source file.
	lineOffset := markdown.FrontMatterLineOffset(source) - 1
	for index := range inspection.References {
		inspection.References[index].Line += lineOffset
	}
	anchors := make(map[string]struct{}, len(inspection.Headings))
	for _, heading := range inspection.Headings {
		anchors[heading.ID] = struct{}{}
	}

	diagnostics := make([]Diagnostic, 0)
	// Heading lines are body-relative too.
	if inspection.H1Count > 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     current.display,
			Line:     secondH1Line(inspection.Headings) + lineOffset,
			Column:   1,
			Severity: SeverityWarning,
			Rule:     RuleDocumentMultipleH1,
			Message:  fmt.Sprintf("document contains %d H1 headings", inspection.H1Count),
		})
	}
	for _, entry := range frontMatterDateEntries(frontMatter) {
		if value := strings.TrimSpace(entry.Value); value != "" && !markdown.IsISODate(value) {
			// The entry position is relative to the YAML block; in the file
			// the opening `---` delimiter sits one line above it.
			line, column := 1, 1
			if entry.Line > 0 {
				line, column = entry.Line+1, max(entry.Column, 1)
			}
			diagnostics = append(diagnostics, Diagnostic{
				Path:     current.display,
				Line:     line,
				Column:   column,
				Severity: SeverityWarning,
				Rule:     RuleFrontMatterDateInvalid,
				Message:  fmt.Sprintf("%s is not a valid ISO date", entry.Key),
			})
		}
	}

	return &indexedDocument{
		document:    current,
		inspectable: true,
		diagnostics: diagnostics,
		frontMatter: frontMatter,
		inspection:  inspection,
		anchors:     anchors,
	}, nil
}

// secondH1Line returns the source line of a document's second H1 heading —
// the first surplus one — for the multiple-H1 warning.
func secondH1Line(headings []markdown.Heading) int {
	seen := 0
	for _, heading := range headings {
		if heading.Level != 1 {
			continue
		}
		seen++
		if seen == 2 {
			return heading.Line
		}
	}
	return 1
}

// frontMatterDateEntries returns the key/value pairs of the frontmatter date
// fields m2h recognizes, in source order.
func frontMatterDateEntries(frontMatter *markdown.FrontMatter) []markdown.FrontMatterEntry {
	if frontMatter == nil {
		return nil
	}
	entries := make([]markdown.FrontMatterEntry, 0, len(frontMatter.Entries))
	for _, entry := range frontMatter.Entries {
		if frontMatterDateKeys[entry.Key] {
			entries = append(entries, entry)
		}
	}
	return entries
}
