package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfirmAcceptsExplicitYesOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "lowercase y", input: "y\n", want: true},
		{name: "uppercase y", input: "Y\n", want: true},
		{name: "yes", input: "yes\n", want: true},
		{name: "YES uppercase", input: "YES\n", want: true},
		{name: "padded y", input: "  y  \n", want: true},
		{name: "y without trailing newline", input: "y", want: true},
		{name: "n declines", input: "n\n", want: false},
		{name: "empty line declines", input: "\n", want: false},
		{name: "eof declines", input: "", want: false},
		{name: "garbage declines", input: "maybe\n", want: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var prompt bytes.Buffer
			got := confirm(strings.NewReader(test.input), &prompt, "Proceed? [y/N] ")
			if got != test.want {
				t.Fatalf("confirm(%q) = %v, want %v", test.input, got, test.want)
			}
			if got := prompt.String(); got != "Proceed? [y/N] " {
				t.Fatalf("prompt output = %q, want prompt echoed verbatim", got)
			}
		})
	}
}

func TestInteractiveStdinRejectsNonFileReaders(t *testing.T) {
	t.Parallel()

	if interactiveStdin(strings.NewReader("y\n")) {
		t.Fatal("interactiveStdin(strings.Reader) = true, want false for non-file readers")
	}

	file, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if interactiveStdin(file) {
		t.Fatal("interactiveStdin(regular *os.File) = true, want false for non-terminal files")
	}
}

func TestConvertPromptDescribesWriteTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()

	tests := []struct {
		name   string
		input  string
		output string
		want   string
	}{
		{
			name:   "file without output derives html target",
			input:  source,
			output: "",
			want:   fmt.Sprintf("Convert %s to %s? [y/N] ", source, filepath.Join(root, "guide.html")),
		},
		{
			name:   "file with output echoes it",
			input:  source,
			output: "public/index.html",
			want:   "Convert " + source + " to public/index.html? [y/N] ",
		},
		{
			name:   "directory without output warns in place",
			input:  directory,
			output: "",
			want:   "Convert " + directory + " in place (may overwrite existing HTML)? [y/N] ",
		},
		{
			name:   "directory with output echoes it",
			input:  directory,
			output: "public/docs",
			want:   "Convert " + directory + " into public/docs? [y/N] ",
		},
		{
			name:   "missing input returns empty prompt",
			input:  filepath.Join(root, "missing.md"),
			output: "",
			want:   "",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := convertPrompt(test.input, test.output); got != test.want {
				t.Fatalf("convertPrompt(%q, %q) = %q, want %q", test.input, test.output, got, test.want)
			}
		})
	}
}
