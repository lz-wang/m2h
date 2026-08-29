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
// a clean scope exits 0, diagnostics on stdout fail the process with 1.
func TestRunCheckExitCodes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide\n\n![missing](nope.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := run([]string{"m2h", "check", filepath.Join(root, "guide.md")}, webui.Content(), &stdout, &stderr); got != 1 {
		t.Fatalf("run() exit code = %d, want 1 for broken references", got)
	}
	if !strings.Contains(stdout.String(), "error [local-target.missing]") ||
		!strings.HasSuffix(stdout.String(), "Checked 1 Markdown file: 1 error\n") {
		t.Fatalf("stdout = %q, want the diagnostic report", stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "Error: check found 1 error\n") {
		t.Fatalf("stderr = %q, want the failure reason", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	clean := filepath.Join(root, "clean.md")
	if err := os.WriteFile(clean, []byte("# Clean\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"m2h", "check", clean}, webui.Content(), &stdout, &stderr); got != 0 {
		t.Fatalf("run() exit code = %d, want 0 for a clean document", got)
	}
	if stdout.String() != "Checked 1 Markdown file: no issues found\n" || stderr.String() != "" {
		t.Fatalf("stdout = %q stderr = %q, want a clean summary", stdout.String(), stderr.String())
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
