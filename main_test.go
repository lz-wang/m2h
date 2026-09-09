package main

import (
	"bytes"
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
