package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	webui "github.com/lz-wang/m2h/web"
)

func TestRunReturnsExitCodeAndRoutesOutput(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "version",
			args:       []string{"m2h", "--version"},
			wantCode:   0,
			wantStdout: M2HVersion + "\n",
		},
		{
			name:       "unknown option",
			args:       []string{"m2h", "--unknown"},
			wantCode:   1,
			wantStderr: "Error: unknown option\n",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if got := run(test.args, webui.Content(), &stdout, &stderr); got != test.wantCode {
				t.Fatalf("run() exit code = %d, want %d", got, test.wantCode)
			}
			if got := stdout.String(); got != test.wantStdout {
				t.Fatalf("stdout = %q, want %q", got, test.wantStdout)
			}
			if got := stderr.String(); got != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, test.wantStderr)
			}
		})
	}
}

// TestRunCheckExitCodes pins the check subcommand's process-level contract:
// errors and strict warnings fail silently after the stdout report, while a
// clean scope and non-strict warnings exit 0.
func TestRunCheckExitCodes(t *testing.T) {
	root := t.TempDir()
	broken := filepath.Join(root, "broken.md")
	if err := os.WriteFile(broken, []byte("# Broken\n\n![missing](nope.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	clean := filepath.Join(root, "clean.md")
	if err := os.WriteFile(clean, []byte("# Clean\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	warning := filepath.Join(root, "warning.md")
	if err := os.WriteFile(warning, []byte("# Warning\n\n![](logo.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		args           []string
		wantCode       int
		wantDiagnostic string
		wantSummary    string
	}{
		{
			name:           "error",
			args:           []string{"m2h", "check", broken},
			wantCode:       1,
			wantDiagnostic: "error [local-target.missing]",
			wantSummary:    "Checked 1 Markdown file: 1 error\n",
		},
		{
			name:        "clean",
			args:        []string{"m2h", "check", clean},
			wantCode:    0,
			wantSummary: "Checked 1 Markdown file: no issues found\n",
		},
		{
			name:           "warning without strict",
			args:           []string{"m2h", "check", warning},
			wantCode:       0,
			wantDiagnostic: "warning [image.alt-empty]",
			wantSummary:    "Checked 1 Markdown file: 1 warning\n",
		},
		{
			name:           "warning with strict",
			args:           []string{"m2h", "check", warning, "--strict"},
			wantCode:       1,
			wantDiagnostic: "warning [image.alt-empty]",
			wantSummary:    "Checked 1 Markdown file: 1 warning\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if got := run(test.args, webui.Content(), &stdout, &stderr); got != test.wantCode {
				t.Fatalf("run() exit code = %d, want %d", got, test.wantCode)
			}
			if test.wantDiagnostic != "" && !strings.Contains(stdout.String(), test.wantDiagnostic) {
				t.Fatalf("stdout = %q, want diagnostic %q", stdout.String(), test.wantDiagnostic)
			}
			if !strings.HasSuffix(stdout.String(), test.wantSummary) {
				t.Fatalf("stdout = %q, want summary %q", stdout.String(), test.wantSummary)
			}
			if stderr.String() != "" {
				t.Fatalf("stderr = %q, want no redundant failure description", stderr.String())
			}
		})
	}
}

// TestRunCheckDocumentIntegrityRulesCLI exercises the four document-integrity
// rules through the process surface: each rule name works with --disable,
// the comma-joined form silences all four, --disable all empties the run,
// --strict only changes the exit code, and the JSON report keeps its
// field-per-diagnostic contract for the new rule ids.
func TestRunCheckDocumentIntegrityRulesCLI(t *testing.T) {
	root := t.TempDir()
	fence := filepath.Join(root, "fence.md")
	if err := os.WriteFile(fence, []byte("# Fence\n\n```go\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	html := filepath.Join(root, "html.md")
	if err := os.WriteFile(html, []byte("# Html\n\n<div>\nx\n</div>\ntext\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	allFour := "html.block-blank-after,code-fence.unclosed,document.merge-conflict,document.invalid-utf8"

	tests := []struct {
		name           string
		args           []string
		wantCode       int
		wantDiagnostic string
		wantSummary    string
	}{
		{
			name:           "defaults report both findings",
			args:           []string{"m2h", "check", root},
			wantCode:       1,
			wantDiagnostic: "error [code-fence.unclosed]",
			wantSummary:    "Checked 2 Markdown files: 1 error, 1 warning\n",
		},
		{
			name:           "single disable removes one rule",
			args:           []string{"m2h", "check", root, "--disable", "html.block-blank-after"},
			wantCode:       1,
			wantDiagnostic: "error [code-fence.unclosed]",
			wantSummary:    "Checked 2 Markdown files: 1 error\n",
		},
		{
			name:        "comma joined disable silences all four",
			args:        []string{"m2h", "check", root, "--disable", allFour},
			wantCode:    0,
			wantSummary: "Checked 2 Markdown files: no issues found\n",
		},
		{
			name:        "disable all empties the run",
			args:        []string{"m2h", "check", root, "--disable", "all"},
			wantCode:    0,
			wantSummary: "Checked 2 Markdown files: no issues found\n",
		},
		{
			name:           "strict keeps severity and fails on warning",
			args:           []string{"m2h", "check", html, "--strict"},
			wantCode:       1,
			wantDiagnostic: "warning [html.block-blank-after]",
			wantSummary:    "Checked 1 Markdown file: 1 warning\n",
		},
		{
			name:           "json output keeps the diagnostic contract",
			args:           []string{"m2h", "check", fence, "--format", "json"},
			wantCode:       1,
			wantDiagnostic: `"rule": "code-fence.unclosed"`,
			wantSummary:    "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if got := run(test.args, webui.Content(), &stdout, &stderr); got != test.wantCode {
				t.Fatalf("run() exit code = %d, want %d", got, test.wantCode)
			}
			if test.wantDiagnostic != "" && !strings.Contains(stdout.String(), test.wantDiagnostic) {
				t.Fatalf("stdout = %q, want diagnostic %q", stdout.String(), test.wantDiagnostic)
			}
			if test.wantSummary != "" && !strings.HasSuffix(stdout.String(), test.wantSummary) {
				t.Fatalf("stdout = %q, want summary %q", stdout.String(), test.wantSummary)
			}
			if stderr.String() != "" {
				t.Fatalf("stderr = %q, want silent failure reporting", stderr.String())
			}
		})
	}
}

// TestRunCheckJSONDecodesNewRules pins the full JSON shape — files, errors,
// warnings and every diagnostic field — for the new rule ids, so tooling
// consuming the report can rely on the contract.
func TestRunCheckJSONDecodesNewRules(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide\n\n<div>\nx\n</div>\ntext\n\n```go\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := run([]string{"m2h", "check", root, "--format", "json"}, webui.Content(), &stdout, &stderr); got != 1 {
		t.Fatalf("run() exit code = %d, want 1", got)
	}
	var report struct {
		Files       int `json:"files"`
		Errors      int `json:"errors"`
		Warnings    int `json:"warnings"`
		Diagnostics []struct {
			Path     string `json:"path"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
			Severity string `json:"severity"`
			Rule     string `json:"rule"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("json report does not decode: %v\n%s", err, stdout.String())
	}
	if report.Files != 1 || report.Errors != 1 || report.Warnings != 1 {
		t.Fatalf("counts = files %d, errors %d, warnings %d, want 1/1/1", report.Files, report.Errors, report.Warnings)
	}
	if len(report.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %+v, want two", report.Diagnostics)
	}
	first, second := report.Diagnostics[0], report.Diagnostics[1]
	if first.Rule != "html.block-blank-after" || first.Severity != "warning" || first.Line != 6 || first.Column != 1 || first.Message == "" {
		t.Fatalf("first diagnostic = %+v, want the html blank warning at 6:1", first)
	}
	if second.Rule != "code-fence.unclosed" || second.Severity != "error" || second.Line != 8 || second.Column != 1 || second.Message == "" {
		t.Fatalf("second diagnostic = %+v, want the unclosed fence error at 8:1", second)
	}
}

func TestRunRejectsInvalidInjectedVersion(t *testing.T) {
	previous := M2HVersion
	M2HVersion = "invalid"
	t.Cleanup(func() {
		M2HVersion = previous
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := run([]string{"m2h", "--version"}, nil, &stdout, &stderr); got != 1 {
		t.Fatalf("run() exit code = %d, want 1", got)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("invalid m2h version")) {
		t.Fatalf("stderr = %q, want invalid version error", stderr.String())
	}
}
