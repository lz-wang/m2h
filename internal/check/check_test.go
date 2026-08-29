package check

import (
	"bytes"
	"context"
	"fmt"
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

// setupCheckRoot writes the given files (relative slash paths) under a fresh
// temporary root and returns the root path.
func setupCheckRoot(t *testing.T, sources map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for relative, contents := range sources {
		writeFile(t, filepath.Join(root, filepath.FromSlash(relative)), contents)
	}
	return root
}

// runCheck runs a directory check over a fresh root and returns the result.
// Discovery options (depth, glob) are passed through unchanged.
func runCheck(t *testing.T, sources map[string]string, options Options) (Result, error) {
	t.Helper()
	options.Input = setupCheckRoot(t, sources)
	return Run(context.Background(), options)
}

// summarize compresses a result into one "path:line:col severity rule"
// entry per diagnostic for compact assertions.
func summarize(t *testing.T, result Result, err error) []string {
	t.Helper()
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	summary := make([]string, 0, len(result.Diagnostics))
	for _, diagnostic := range result.Diagnostics {
		summary = append(summary, fmt.Sprintf("%s:%d:%d %s %s",
			filepath.Base(diagnostic.Path), diagnostic.Line, diagnostic.Column, diagnostic.Severity, diagnostic.Rule))
	}
	return summary
}

func TestCheckCleanDocumentPasses(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"index.md":        "# Index\n\n[Guide](guide.md) and [section](guide.md#install)\n\n![Logo](images/logo.png)\n",
		"guide.md":        "# Guide\n\n## Install\n\nSee [index](index.md) and the [PDF](files/spec.pdf).\n",
		"images/logo.png": "png",
		"files/spec.pdf":  "pdf",
	}, Options{})
	summary := summarize(t, result, err)
	if len(summary) != 0 {
		t.Fatalf("diagnostics = %v, want none", result.Diagnostics)
	}
	if result.Files != 2 || result.Errors != 0 || result.Warnings != 0 {
		t.Fatalf("result = %+v, want 2 clean files", result)
	}
}

func TestCheckDiagnosticPathsAreInputPrefixed(t *testing.T) {
	t.Parallel()

	root := setupCheckRoot(t, map[string]string{
		"guide.md": "# Guide\n\n![missing](nope.png)\n",
	})
	result, err := Run(context.Background(), Options{Input: filepath.Join(root, "sub"), Depth: 4})
	if err == nil || !strings.Contains(err.Error(), "resolve input") && !strings.Contains(err.Error(), "inspect input") {
		t.Fatalf("Run() under a missing directory error = %v, want failure", err)
	}

	result, err = Run(context.Background(), Options{Input: root, Depth: 4})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want one", result.Diagnostics)
	}
	want := filepath.Join(root, "guide.md")
	if result.Diagnostics[0].Path != want {
		t.Fatalf("diagnostic path = %q, want %q", result.Diagnostics[0].Path, want)
	}
}

func TestCheckFrontMatterInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "invalid yaml",
			source: "---\ntitle: [unclosed\n---\n\n[link](guide.md)\n",
			want:   "frontmatter is not valid YAML",
		},
		{
			name:   "not a mapping",
			source: "---\n- one\n- two\n---\n\n[link](guide.md)\n",
			want:   "frontmatter must be a mapping",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := runCheck(t, map[string]string{
				"meta.md":  test.source,
				"guide.md": "# Guide\n",
			}, Options{})
			if err != nil {
				t.Fatalf("Run() returned error: %v", err)
			}
			if result.Files != 2 {
				t.Fatalf("Files = %d, want 2 (invalid frontmatter still counts)", result.Files)
			}
			if len(result.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %+v, want exactly the frontmatter error", result.Diagnostics)
			}
			diagnostic := result.Diagnostics[0]
			if diagnostic.Rule != RuleFrontMatterInvalid || diagnostic.Severity != SeverityError ||
				diagnostic.Line != 1 || diagnostic.Column != 1 || !strings.Contains(diagnostic.Message, test.want) {
				t.Fatalf("diagnostic = %+v, want frontmatter.invalid at 1:1", diagnostic)
			}
		})
	}
}

func TestCheckFrontMatterShiftsReferenceLines(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"guide.md": "---\ntitle: Guide\n---\n\nBody with a [broken](missing.png) link.\n",
	}, Options{})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Line != 5 {
		t.Fatalf("diagnostics = %+v, want the reference reported on source line 5", result.Diagnostics)
	}
}

func TestCheckMissingTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		markup  string
		column  int
		rule    string
		message string
	}{
		{
			name:    "missing image",
			markup:  "![Logo](images/missing.png)\n",
			column:  3,
			rule:    RuleLocalTargetMissing,
			message: `target "images/missing.png" does not exist`,
		},
		{
			name:    "missing markdown",
			markup:  "[Guide](missing-guide.md)\n",
			column:  2,
			rule:    RuleLocalTargetMissing,
			message: `target "missing-guide.md" does not exist`,
		},
		{
			name:    "missing attachment",
			markup:  "[PDF](files/missing.pdf)\n",
			column:  2,
			rule:    RuleLocalTargetMissing,
			message: `target "files/missing.pdf" does not exist`,
		},
		{
			name:    "directory target is not regular",
			markup:  "[Assets](images)\n",
			column:  2,
			rule:    RuleLocalTargetNotRegular,
			message: `target "images" is not a regular file`,
		},
		{
			name:    "invalid percent encoding",
			markup:  "![Logo](missing%zz.png)\n",
			column:  3,
			rule:    RuleLocalTargetMissing,
			message: `is not accessible: invalid URL encoding`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := runCheck(t, map[string]string{
				"guide.md":        "# Guide\n\n" + test.markup,
				"images/logo.png": "png",
				"files/spec.pdf":  "pdf",
			}, Options{})
			summary := summarize(t, result, err)
			want := []string{fmt.Sprintf("guide.md:3:%d error %s", test.column, test.rule)}
			if !slices.Equal(summary, want) {
				t.Fatalf("diagnostics = %v, want %v", summary, want)
			}
			if !strings.Contains(result.Diagnostics[0].Message, test.message) {
				t.Fatalf("message = %q, want it to contain %q", result.Diagnostics[0].Message, test.message)
			}
		})
	}
}

func TestCheckOutsideRootTargets(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"sub/guide.md": "# Guide\n\n[Up one](../logo.png) and [too far](../../logo.png)\n",
		"logo.png":     "png",
	}, Options{Depth: 4})
	summary := summarize(t, result, err)
	// ../logo.png from sub/ resolves to the root-level file: allowed.
	want := []string{"guide.md:3:28 error " + RuleLocalTargetOutsideRoot}
	if !slices.Equal(summary, want) {
		t.Fatalf("diagnostics = %v, want only the root escape %v", summary, want)
	}
	if !strings.Contains(result.Diagnostics[0].Message, "resolves outside the workspace root") {
		t.Fatalf("message = %q", result.Diagnostics[0].Message)
	}
}

func TestCheckPercentDecodingAndQueries(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"guide.md":    "# Guide\n\n![Logo](my%20logo.png)\n[Query](guide.md?lang=zh)\n[Escaped](missing%zz)\n",
		"my logo.png": "png",
	}, Options{})
	summary := summarize(t, result, err)
	want := []string{fmt.Sprintf("guide.md:5:2 error %s", RuleLocalTargetMissing)}
	if !slices.Equal(summary, want) {
		t.Fatalf("diagnostics = %v, want only the encoded-missing entry %v", summary, want)
	}
}

func TestCheckSchemeAndAbsoluteDestinationsAreIgnored(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"guide.md": "# Guide\n\n[Site](https://example.com/x.png) and [Mail](mailto:a@b.c)\n\n[Teal](tel:123) and [CDN](//cdn.example.com/a.png) and [Root](/absolute.png) and [Anchor](#install)\n\n## Install\n",
	}, Options{})
	summary := summarize(t, result, err)
	if len(summary) != 0 {
		t.Fatalf("diagnostics = %v, want none for non-relative destinations", result.Diagnostics)
	}
}

func TestCheckAnchorRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "local anchor present",
			source: "# Guide\n\n[Install](#install)\n\n## Install\n",
			want:   nil,
		},
		{
			name:   "local anchor missing",
			source: "# Guide\n\n[Install](#setup)\n",
			want:   []string{"guide.md:3:2 error " + RuleAnchorMissing},
		},
		{
			name:   "cross document anchor",
			source: "# Guide\n\n[Install](target.md#install)\n",
			want:   []string{"guide.md:3:2 error " + RuleAnchorMissing},
		},
		{
			name:   "anchor ids are case sensitive",
			source: "# Guide\n\n[Install](#Install)\n\n## install\n",
			want:   []string{"guide.md:3:2 error " + RuleAnchorMissing},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := runCheck(t, map[string]string{
				"guide.md":  test.source,
				"target.md": "# Target\n\n## Overview\n",
			}, Options{})
			summary := summarize(t, result, err)
			want := test.want
			if want == nil {
				want = []string{}
			}
			if !slices.Equal(summary, want) {
				t.Fatalf("diagnostics = %v, want %v", summary, want)
			}
		})
	}
}

func TestCheckDuplicateAndCJKAnchors(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"guide.md": "# Guide\n\n[First](#中文-标题)\n[First again](#中文-标题)\n[Duplicate](#title-1)\n[Missing](#title-2)\n\n## 中文 标题\n\n# Title\n\n# Title\n",
	}, Options{})
	summary := summarize(t, result, err)
	// Duplicate H1s get -1 suffixes; #title-2 never exists.
	want := []string{fmt.Sprintf("guide.md:6:2 error %s", RuleAnchorMissing)}
	if !slices.Equal(summary, want) {
		t.Fatalf("diagnostics = %v, want only the missing duplicate anchor %v", summary, want)
	}
}

func TestCheckFragmentWithQueryIsSplit(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"guide.md":  "# Guide\n\n[Install](target.md?lang=zh#install)\n",
		"target.md": "# Target\n\n## Overview\n",
	}, Options{})
	summary := summarize(t, result, err)
	want := []string{"guide.md:3:2 error " + RuleAnchorMissing}
	if !slices.Equal(summary, want) {
		t.Fatalf("diagnostics = %v, want the fragment checked (and missing) %v", summary, want)
	}
}

func TestCheckRawHTMLReferences(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"guide.md": "# Guide\n\n<a href=\"missing-a.md\">A</a>\n<img src=\"missing-b.png\">\n<video poster=\"missing-c.jpg\"></video>\n<object data=\"missing-d.pdf\"></object>\n",
	}, Options{})
	summary := summarize(t, result, err)
	want := []string{
		fmt.Sprintf("guide.md:3:1 error %s", RuleLocalTargetMissing),
		fmt.Sprintf("guide.md:4:1 error %s", RuleLocalTargetMissing),
		fmt.Sprintf("guide.md:5:1 error %s", RuleLocalTargetMissing),
		fmt.Sprintf("guide.md:6:1 error %s", RuleLocalTargetMissing),
	}
	if !slices.Equal(summary, want) {
		t.Fatalf("diagnostics = %v, want %v", summary, want)
	}
}

func TestCheckReferenceStyleLinks(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"guide.md": "# Guide\n\n[Guide][guide]\n\n[guide]: missing-guide.md\n",
	}, Options{})
	summary := summarize(t, result, err)
	want := []string{fmt.Sprintf("guide.md:3:2 error %s", RuleLocalTargetMissing)}
	if !slices.Equal(summary, want) {
		t.Fatalf("diagnostics = %v, want %v", summary, want)
	}
}

func TestCheckCodeContentNeverReports(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"guide.md": "# Guide\n\nInline `![not image](missing.png)` and `[link](missing.md)`.\n\n```markdown\n![not image](missing.png)\n<a href=\"missing.md\">A</a>\n```\n",
	}, Options{})
	summary := summarize(t, result, err)
	if len(summary) != 0 {
		t.Fatalf("diagnostics = %v, want none for code content", result.Diagnostics)
	}
}

func TestCheckMarkdownNotServed(t *testing.T) {
	t.Parallel()

	t.Run("glob excluded", func(t *testing.T) {
		t.Parallel()

		result, err := runCheck(t, map[string]string{
			"index.md": "# Index\n\n[Guide](guide.md)\n",
			"guide.md": "# Guide\n",
		}, Options{Pattern: "index.md", Depth: 4})
		summary := summarize(t, result, err)
		want := []string{"index.md:3:2 error " + RuleMarkdownTargetNotServed}
		if !slices.Equal(summary, want) {
			t.Fatalf("diagnostics = %v, want %v", summary, want)
		}
		if !strings.Contains(result.Diagnostics[0].Message, "excluded by the glob filter") {
			t.Fatalf("message = %q, want glob reason", result.Diagnostics[0].Message)
		}
	})

	t.Run("depth excluded", func(t *testing.T) {
		t.Parallel()

		result, err := runCheck(t, map[string]string{
			"index.md":      "# Index\n\n[Deep](deep/guide.md)\n",
			"deep/guide.md": "# Deep\n",
		}, Options{Depth: 0})
		summary := summarize(t, result, err)
		want := []string{"index.md:3:2 error " + RuleMarkdownTargetNotServed}
		if !slices.Equal(summary, want) {
			t.Fatalf("diagnostics = %v, want %v", summary, want)
		}
		if !strings.Contains(result.Diagnostics[0].Message, "excluded by the depth limit") {
			t.Fatalf("message = %q, want depth reason", result.Diagnostics[0].Message)
		}
	})

	t.Run("single file mode", func(t *testing.T) {
		t.Parallel()

		root := setupCheckRoot(t, map[string]string{
			"README.md": "# Readme\n\n[Guide](guide.md) and ![Logo](logo.png)\n",
			"guide.md":  "# Guide\n",
			"logo.png":  "png",
		})
		result, err := Run(context.Background(), Options{Input: filepath.Join(root, "README.md")})
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
		summary := summarize(t, result, err)
		want := []string{"README.md:3:2 error " + RuleMarkdownTargetNotServed}
		if !slices.Equal(summary, want) {
			t.Fatalf("diagnostics = %v, want only the sibling Markdown rejection %v", summary, want)
		}
		if !strings.Contains(result.Diagnostics[0].Message, "not available in single-file mode") {
			t.Fatalf("message = %q, want single-file reason", result.Diagnostics[0].Message)
		}
	})
}

func TestCheckSymlinkTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "guide.md"), "# Guide\n\n[Inside](inside.png), [Outside](outside.png), [Broken](broken.png), [Through dir](linked/file.png)\n")
	writeFile(t, filepath.Join(root, "real.png"), "png")
	writeFile(t, filepath.Join(root, "realdir", "file.png"), "png")

	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.png"), "png")

	if err := os.Symlink(filepath.Join(root, "real.png"), filepath.Join(root, "inside.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.png"), filepath.Join(root, "outside.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "nowhere.png"), filepath.Join(root, "broken.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "realdir"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}

	result, err := Run(context.Background(), Options{Input: root, Depth: 4})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	summary := summarize(t, result, err)
	want := []string{
		fmt.Sprintf("guide.md:3:24 error %s", RuleLocalTargetOutsideRoot),
		fmt.Sprintf("guide.md:3:48 error %s", RuleLocalTargetMissing),
		fmt.Sprintf("guide.md:3:70 error %s", RuleLocalTargetOutsideRoot),
	}
	if !slices.Equal(summary, want) {
		t.Fatalf("diagnostics = %v\nwant %v\nfull: %+v", summary, want, result.Diagnostics)
	}
	// The inside-root symlink file itself must pass, and the outside one
	// must be reported as a root escape, not a missing file.
	if !strings.Contains(result.Diagnostics[0].Message, "resolves outside the workspace root") {
		t.Fatalf("outside message = %q", result.Diagnostics[0].Message)
	}
	if !strings.Contains(result.Diagnostics[2].Message, "crosses a symlink directory") {
		t.Fatalf("symlink directory message = %q", result.Diagnostics[2].Message)
	}
}

func TestCheckExactCaseTargets(t *testing.T) {
	t.Parallel()

	// On case-insensitive filesystems os.Stat happily accepts LOGO.png for
	// logo.png, but the served workspace refuses the differently-cased name;
	// check reports it missing so both agree.
	result, err := runCheck(t, map[string]string{
		"guide.md": "# Guide\n\n![Logo](LOGO.png)\n",
		"logo.png": "png",
	}, Options{})
	summary := summarize(t, result, err)
	want := []string{fmt.Sprintf("guide.md:3:3 error %s", RuleLocalTargetMissing)}
	if !slices.Equal(summary, want) {
		t.Fatalf("diagnostics = %v, want the case mismatch reported %v", summary, want)
	}
}

func TestCheckTargetStatusIsCachedPerPath(t *testing.T) {
	t.Parallel()

	// Many references to one target exercise the resolver cache; the
	// outcome must not depend on the number of references.
	source := "# Guide\n\n"
	for index := range 50 {
		source += fmt.Sprintf("![Logo %d](missing.png)\n", index)
	}
	result, err := runCheck(t, map[string]string{"guide.md": source}, Options{})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if len(result.Diagnostics) != 50 || result.Errors != 50 {
		t.Fatalf("diagnostics = %d, errors = %d, want 50", len(result.Diagnostics), result.Errors)
	}
}

func TestCheckDecodedRootEscapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		markup string
	}{
		{
			name:   "encoded parent escape",
			markup: "[Secret](..%2Fsecret.png)\n",
		},
		{
			name:   "encoded mid-path parent",
			markup: "[Up](sub%2F..%2Fsecret.png)\n",
		},
		{
			name:   "absolute path after decoding",
			markup: "[Abs](%2Fetc%2Fpasswd)\n",
		},
		{
			name:   "decoding never stabilizes",
			markup: "[Loop](%25252525252525252525252525x)\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := runCheck(t, map[string]string{
				"guide.md": "# Guide\n\n" + test.markup,
			}, Options{})
			summary := summarize(t, result, err)
			want := []string{fmt.Sprintf("guide.md:3:2 error %s", RuleLocalTargetMissing)}
			if !slices.Equal(summary, want) {
				t.Fatalf("diagnostics = %v, want the encoded escape reported missing %v", summary, want)
			}
			message := result.Diagnostics[0].Message
			if !strings.Contains(message, "not accessible") {
				t.Fatalf("message = %q, want an accessibility failure", message)
			}
		})
	}
}

func TestCheckPercentEncodedCJKAnchor(t *testing.T) {
	t.Parallel()

	// Browsers percent-decode fragments before seeking the element id, so a
	// URL-encoded CJK heading anchor must resolve.
	result, err := runCheck(t, map[string]string{
		"guide.md": "# Guide\n\n[中文](#%E4%B8%AD%E6%96%87-%E6%A0%87%E9%A2%98)\n\n## 中文 标题\n",
	}, Options{})
	summary := summarize(t, result, err)
	if len(summary) != 0 {
		t.Fatalf("diagnostics = %v, want the encoded anchor to resolve", result.Diagnostics)
	}
}

func TestCheckDiagnosticsOrderAcrossFiles(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"b.md": "# B\n\n![x](missing-b.png)\n",
		"a.md": "# A\n\n![x](missing-a.png)\n![y](missing-a2.png)\n",
	}, Options{})
	summary := summarize(t, result, err)
	want := []string{
		fmt.Sprintf("a.md:3:3 error %s", RuleLocalTargetMissing),
		fmt.Sprintf("a.md:4:3 error %s", RuleLocalTargetMissing),
		fmt.Sprintf("b.md:3:3 error %s", RuleLocalTargetMissing),
	}
	if !slices.Equal(summary, want) {
		t.Fatalf("diagnostics = %v, want path-then-line order %v", summary, want)
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
