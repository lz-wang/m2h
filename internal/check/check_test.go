package check

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options Options
		want    string
	}{
		{
			name:    "empty input",
			options: Options{Input: "  "},
			want:    "input path is required",
		},
		{
			name:    "negative depth",
			options: Options{Input: "docs", Depth: -1},
			want:    "validate check options: invalid depth -1",
		},
		{
			name:    "invalid glob",
			options: Options{Input: "docs", Pattern: "["},
			want:    `validate check options: invalid glob "["`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Run(context.Background(), test.options); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Run() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunPropagatesCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, Options{Input: "docs"}); err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("Run() error = %v, want context cancellation", err)
	}
}

func TestRunRejectsMissingAndNonMarkdownInputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "notes.txt"), "plain text")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "missing", input: filepath.Join(root, "missing.md"), want: "inspect input"},
		{name: "non-markdown file", input: filepath.Join(root, "notes.txt"), want: "check requires a Markdown file or directory"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Run(context.Background(), Options{Input: test.input})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Run(%q) error = %v, want %q", test.input, err, test.want)
			}
		})
	}
}

func TestRunCountsDirectoryMarkdownFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "index.md"), "# Index")
	writeFile(t, filepath.Join(root, "guide.md"), "# Guide")
	writeFile(t, filepath.Join(root, "notes.txt"), "plain")
	writeFile(t, filepath.Join(root, "sub", "deep.md"), "# Deep")

	result, err := Run(context.Background(), Options{Input: root, Depth: 4})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if result.Files != 3 {
		t.Fatalf("Files = %d, want 3", result.Files)
	}
	if result.Errors != 0 || result.Warnings != 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("result = %+v, want no diagnostics", result)
	}

	// The glob filter must mirror the serve command's document scope.
	filtered, err := Run(context.Background(), Options{Input: root, Depth: 4, Pattern: "*.md"})
	if err != nil {
		t.Fatalf("Run() with glob returned error: %v", err)
	}
	if filtered.Files != 2 {
		t.Fatalf("Files with glob = %d, want 2 (index.md, guide.md)", filtered.Files)
	}

	// Depth bounds recursion the same way the serve command applies it.
	shallow, err := Run(context.Background(), Options{Input: root, Depth: 0})
	if err != nil {
		t.Fatalf("Run() with depth 0 returned error: %v", err)
	}
	if shallow.Files != 2 {
		t.Fatalf("Files with depth 0 = %d, want 2", shallow.Files)
	}
}

func TestRunCountsSingleFileScope(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Readme")
	writeFile(t, filepath.Join(root, "guide.md"), "# Guide")

	result, err := Run(context.Background(), Options{Input: filepath.Join(root, "README.md")})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if result.Files != 1 {
		t.Fatalf("Files = %d, want 1: single-file scope admits only the input itself", result.Files)
	}
	if result.Errors != 0 || result.Warnings != 0 {
		t.Fatalf("result = %+v, want no diagnostics", result)
	}
}

func TestRunRejectsDirectoryInputForFileOptions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "guide.md"), "# Guide")

	// A directory passed where a single-file scope would be built must not
	// be mistaken for one: files.Resolve reports the kind and Run routes it
	// through directory discovery instead.
	result, err := Run(context.Background(), Options{Input: root})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if result.Files != 1 {
		t.Fatalf("Files = %d, want 1", result.Files)
	}
}

func TestSortDiagnosticsOrdersDeterministically(t *testing.T) {
	t.Parallel()

	diagnostics := []Diagnostic{
		{Path: "b.md", Line: 1, Column: 1, Rule: "a-rule"},
		{Path: "a.md", Line: 10, Column: 1, Rule: "z-rule"},
		{Path: "a.md", Line: 2, Column: 9, Rule: "a-rule"},
		{Path: "a.md", Line: 2, Column: 3, Rule: "b-rule"},
		{Path: "a.md", Line: 2, Column: 3, Rule: "a-rule"},
	}
	sortDiagnostics(diagnostics)
	ordered := []Diagnostic{
		{Path: "a.md", Line: 2, Column: 3, Rule: "a-rule"},
		{Path: "a.md", Line: 2, Column: 3, Rule: "b-rule"},
		{Path: "a.md", Line: 2, Column: 9, Rule: "a-rule"},
		{Path: "a.md", Line: 10, Column: 1, Rule: "z-rule"},
		{Path: "b.md", Line: 1, Column: 1, Rule: "a-rule"},
	}
	if !slices.Equal(diagnostics, ordered) {
		t.Fatalf("sortDiagnostics() = %+v, want %+v", diagnostics, ordered)
	}
}

func TestWriteTextReport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{
			name:   "clean",
			result: Result{Files: 27},
			want:   "Checked 27 Markdown files: no issues found\n",
		},
		{
			name:   "single file clean",
			result: Result{Files: 1},
			want:   "Checked 1 Markdown file: no issues found\n",
		},
		{
			name: "issues",
			result: Result{
				Files:    27,
				Errors:   3,
				Warnings: 1,
				Diagnostics: []Diagnostic{
					{Path: "docs/guide.md", Line: 42, Column: 17, Severity: SeverityError, Rule: "local-target.missing", Message: `target "images/topology.png" does not exist`},
					{Path: "docs/logo.md", Line: 12, Column: 1, Severity: SeverityWarning, Rule: "image.alt-empty", Message: "image has no alt text"},
				},
			},
			want: "docs/guide.md:42:17: error [local-target.missing]: target \"images/topology.png\" does not exist\n" +
				"docs/logo.md:12:1: warning [image.alt-empty]: image has no alt text\n" +
				"Checked 27 Markdown files: 3 errors, 1 warning\n",
		},
		{
			name:   "warnings only",
			result: Result{Files: 2, Warnings: 1},
			want:   "Checked 2 Markdown files: 1 warning\n",
		},
		{
			name:   "errors only",
			result: Result{Files: 2, Errors: 1},
			want:   "Checked 2 Markdown files: 1 error\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			if err := WriteReport(&output, test.result, FormatText); err != nil {
				t.Fatalf("WriteReport() returned error: %v", err)
			}
			if output.String() != test.want {
				t.Fatalf("WriteReport() = %q, want %q", output.String(), test.want)
			}
		})
	}
}

func TestWriteJSONReportIsDeterministic(t *testing.T) {
	t.Parallel()

	result := Result{
		Files:    27,
		Errors:   1,
		Warnings: 1,
		Diagnostics: []Diagnostic{
			{Path: "guide.md", Line: 42, Column: 17, Severity: SeverityError, Rule: "local-target.missing", Message: `target "images/topology.png" does not exist`},
		},
	}
	want := `{
  "files": 27,
  "errors": 1,
  "warnings": 1,
  "diagnostics": [
    {
      "path": "guide.md",
      "line": 42,
      "column": 17,
      "severity": "error",
      "rule": "local-target.missing",
      "message": "target \"images/topology.png\" does not exist"
    }
  ]
}
`

	var first bytes.Buffer
	if err := WriteReport(&first, result, FormatJSON); err != nil {
		t.Fatalf("WriteReport() returned error: %v", err)
	}
	if first.String() != want {
		t.Fatalf("WriteReport() = %q, want %q", first.String(), want)
	}

	var empty bytes.Buffer
	if err := WriteReport(&empty, Result{}, FormatJSON); err != nil {
		t.Fatalf("WriteReport() returned error: %v", err)
	}
	if !strings.Contains(empty.String(), `"diagnostics": []`) {
		t.Fatalf("empty report should carry an empty diagnostics array, got %q", empty.String())
	}
}

func TestWriteReportRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := WriteReport(&output, Result{}, Format("table"))
	if err == nil || !strings.Contains(err.Error(), `unknown format "table"`) {
		t.Fatalf("WriteReport() error = %v, want unknown format", err)
	}
}

func TestWriteReportPropagatesWriteErrors(t *testing.T) {
	t.Parallel()

	writer := failingWriter{}
	if err := WriteReport(writer, Result{Files: 1}, FormatText); err == nil {
		t.Fatal("WriteReport() succeeded with a failing writer")
	}
	if err := WriteReport(writer, Result{Files: 1}, FormatJSON); err == nil {
		t.Fatal("WriteReport() JSON succeeded with a failing writer")
	}
}

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, os.ErrClosed
}
