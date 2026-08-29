// Package check inspects Markdown documents for integrity problems: invalid
// frontmatter, broken local references and document structure issues. It is
// the engine behind the m2h check subcommand and reports findings exclusively
// through Result diagnostics — a returned error means the check itself could
// not run (bad options, filesystem failure, cancelled context), never that
// documents have problems.
package check

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
)

// Severity classifies a diagnostic. Errors describe documents that break in
// the browser (missing files, unreachable targets, invalid frontmatter);
// warnings describe quality problems that keep the document working.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic is one finding about one Markdown document. Path is the
// user-facing path of the document (prefixed with the input as written), and
// Line/Column locate the offending reference — 1-based, with column 1 meaning
// "line only" when the exact column cannot be recovered from the AST.
type Diagnostic struct {
	Path     string
	Line     int
	Column   int
	Severity Severity
	Rule     string
	Message  string
}

// Options configures one check run.
type Options struct {
	// Input is the Markdown file or directory to check.
	Input string
	// Pattern filters Markdown paths with a doublestar glob, exactly like the
	// serve command's --glob, so both see the same document scope.
	Pattern string
	// Depth bounds directory recursion, exactly like the serve command's
	// --depth. Zero admits only files directly inside the root.
	Depth int
}

// Result summarizes one completed check run. Files counts every Markdown
// document in the scope, including documents whose frontmatter failed to
// parse; Errors and Warnings count diagnostics by severity.
type Result struct {
	Files       int
	Errors      int
	Warnings    int
	Diagnostics []Diagnostic
}

// Run resolves the input scope, walks its Markdown documents and returns the
// collected diagnostics in deterministic order (path, then line, column and
// rule). File and directory inputs mirror the serve command's scope rules: a
// single file is a workspace of one document rooted at its parent directory,
// while a directory admits every Markdown file the depth and glob rules match.
//
// The check itself runs in two phases: every document is first read and
// inspected once into a shared index (frontmatter, anchors, references), and
// only then are references resolved against that index and the filesystem,
// so a target document linked a hundred times is parsed exactly once.
func Run(ctx context.Context, options Options) (Result, error) {
	if err := validateOptions(options); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("check: %w", err)
	}

	input, err := files.Resolve(options.Input)
	if err != nil {
		return Result{}, err
	}

	var scope documentScope
	if input.Kind == files.KindFile {
		if !files.IsMarkdown(input.Path) {
			return Result{}, fmt.Errorf("check requires a Markdown file or directory: %q", options.Input)
		}
		scope = newSingleFileScope(options.Input, input.Path)
	} else {
		scope, err = newDirectoryScope(ctx, options.Input, input.Path, files.DiscoverOptions{
			Depth:   options.Depth,
			Pattern: options.Pattern,
		})
	}
	if err != nil {
		return Result{}, err
	}

	result := Result{Diagnostics: make([]Diagnostic, 0)}

	// Phase A: build the document index.
	index := make(map[string]*indexedDocument, len(scope.documents))
	for _, current := range scope.documents {
		if err := ctx.Err(); err != nil {
			return Result{}, fmt.Errorf("check: %w", err)
		}
		indexed, err := indexDocument(current)
		if err != nil {
			return Result{}, err
		}
		if indexed == nil {
			// The file vanished between discovery and reading; the server's
			// file listing skips it the same way.
			continue
		}
		index[indexed.relative] = indexed
		result.Files++
		result.Diagnostics = append(result.Diagnostics, indexed.diagnostics...)
	}

	// Phase B: resolve every reference against the index and the filesystem.
	resolver := newTargetResolver(scope.root)
	for _, current := range scope.documents {
		if err := ctx.Err(); err != nil {
			return Result{}, fmt.Errorf("check: %w", err)
		}
		indexed, ok := index[current.relative]
		if !ok || !indexed.inspectable {
			continue
		}
		result.Diagnostics = append(result.Diagnostics, checkDocumentReferences(scope, index, resolver, indexed)...)
	}

	sortDiagnostics(result.Diagnostics)
	for _, diagnostic := range result.Diagnostics {
		switch diagnostic.Severity {
		case SeverityError:
			result.Errors++
		case SeverityWarning:
			result.Warnings++
		}
	}
	return result, nil
}

func validateOptions(options Options) error {
	if strings.TrimSpace(options.Input) == "" {
		return fmt.Errorf("input path is required")
	}
	if err := files.ValidateDiscoverOptions(files.DiscoverOptions{
		Depth:   options.Depth,
		Pattern: options.Pattern,
	}); err != nil {
		return fmt.Errorf("validate check options: %w", err)
	}
	return nil
}

// sortDiagnostics orders diagnostics by path, line, column and rule so both
// the text and JSON reports stay deterministic regardless of discovery order.
func sortDiagnostics(diagnostics []Diagnostic) {
	slices.SortStableFunc(diagnostics, func(left, right Diagnostic) int {
		if order := strings.Compare(left.Path, right.Path); order != 0 {
			return order
		}
		if left.Line != right.Line {
			return left.Line - right.Line
		}
		if left.Column != right.Column {
			return left.Column - right.Column
		}
		return strings.Compare(left.Rule, right.Rule)
	})
}
