package view

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/markdown"
)

func TestRunRendersStandardGFMGoldenWithoutColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var output bytes.Buffer
	err := Run(context.Background(), Options{
		Input:  filepath.Join("testdata", "gfm.md"),
		Mode:   markdown.ModeDark,
		Output: &output,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("NO_COLOR output contains ANSI escapes: %q", output.String())
	}
	want, err := os.ReadFile(filepath.Join("testdata", "gfm.golden"))
	if err != nil {
		t.Fatal(err)
	}
	gotNormalized := normalizeGolden(output.String())
	wantNormalized := normalizeGolden(string(want))
	if gotNormalized != wantNormalized {
		t.Fatalf("terminal output differs from golden\ngot:  %q\nwant: %q", gotNormalized, wantNormalized)
	}
}

func TestRunStripsColorForNonTTYOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	source := filepath.Join(t.TempDir(), "guide.md")
	writeViewFile(t, source, "# Guide\n\n`terminal`")

	var output bytes.Buffer
	if err := Run(context.Background(), Options{Input: source, Mode: markdown.ModeDark, Output: &output}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(output.String(), "\x1b[") || !strings.Contains(output.String(), "Guide") {
		t.Fatalf("non-TTY output = %q", output.String())
	}
}

func TestRenderMarkdownSanitizesDangerousHyperlinks(t *testing.T) {
	rendered, err := renderMarkdown([]byte("[safe](https://example.com) [unsafe](javascript:alert(1))"), "dark")
	if err != nil {
		t.Fatalf("renderMarkdown() error = %v", err)
	}
	if !bytes.Contains(rendered, []byte("https://example.com")) || bytes.Contains(rendered, []byte("javascript:")) {
		t.Fatalf("rendered links = %q", rendered)
	}
}

func TestRunMapsModesAndAutoDetection(t *testing.T) {
	source := filepath.Join(t.TempDir(), "guide.md")
	writeViewFile(t, source, "# Guide")

	tests := []struct {
		name       string
		mode       markdown.Mode
		dark       bool
		noColor    bool
		wantStyle  string
		wantDetect bool
	}{
		{name: "light", mode: markdown.ModeLight, wantStyle: "light"},
		{name: "dark", mode: markdown.ModeDark, wantStyle: "dark"},
		{name: "auto dark", mode: markdown.ModeAuto, dark: true, wantStyle: "dark", wantDetect: true},
		{name: "auto light", mode: markdown.ModeAuto, wantStyle: "light", wantDetect: true},
		{name: "auto no color", mode: markdown.ModeAuto, noColor: true, wantStyle: "dark"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detected := false
			style := ""
			var output bytes.Buffer
			err := run(context.Background(), Options{
				Input:  source,
				Mode:   test.mode,
				Stdin:  os.Stdin,
				Output: os.Stdout,
			}, dependencies{
				read: func(context.Context, string) ([]byte, error) { return []byte("# Guide"), nil },
				render: func(_ []byte, selected string) ([]byte, error) {
					style = selected
					return []byte("rendered"), nil
				},
				detectDark: func(*os.File, *os.File) bool {
					detected = true
					return test.dark
				},
				noColor: func() bool { return test.noColor },
				write: func(_ io.Writer, contents []byte) error {
					_, err := output.Write(contents)
					return err
				},
			})
			if err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if style != test.wantStyle || detected != test.wantDetect || output.String() != "rendered" {
				t.Fatalf("style=%q detected=%v output=%q", style, detected, output.String())
			}
		})
	}
}

func TestRunAutoUsesDarkFallbackWithoutTerminal(t *testing.T) {
	source := filepath.Join(t.TempDir(), "guide.md")
	writeViewFile(t, source, "# Guide")
	selected := ""
	deps := testDependencies(
		func(context.Context, string) ([]byte, error) { return []byte("# Guide"), nil },
		func(_ []byte, style string) ([]byte, error) {
			selected = style
			return []byte("rendered"), nil
		},
	)
	deps.detectDark = func(*os.File, *os.File) bool {
		t.Fatal("background detection ran without a terminal output")
		return false
	}

	if err := run(context.Background(), Options{
		Input:  source,
		Mode:   markdown.ModeAuto,
		Output: &bytes.Buffer{},
	}, deps); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if selected != "dark" {
		t.Fatalf("auto fallback style = %q, want dark", selected)
	}
}

func TestRunRejectsInvalidInputsBeforeOutput(t *testing.T) {
	root := t.TempDir()
	textFile := filepath.Join(root, "notes.txt")
	writeViewFile(t, textFile, "plain text")

	tests := []struct {
		name  string
		ctx   context.Context
		input string
		mode  markdown.Mode
		want  string
	}{
		{name: "missing", ctx: context.Background(), input: filepath.Join(root, "missing.md"), mode: markdown.ModeAuto, want: "resolve input"},
		{name: "directory", ctx: context.Background(), input: root, mode: markdown.ModeAuto, want: "expected a Markdown file"},
		{name: "non Markdown", ctx: context.Background(), input: textFile, mode: markdown.ModeAuto, want: "expected a Markdown file"},
		{name: "invalid mode", ctx: context.Background(), input: textFile, mode: "sepia", want: "invalid mode"},
		{name: "canceled", ctx: canceledContext(), input: filepath.Join(root, "missing.md"), mode: markdown.ModeAuto, want: "context canceled"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := Run(test.ctx, Options{Input: test.input, Mode: test.mode, Output: &output})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Run() error = %v, want %q", err, test.want)
			}
			if output.Len() != 0 {
				t.Fatalf("failed view wrote output %q", output.String())
			}
		})
	}
}

func TestRunAcceptsMarkdownFileSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target.md")
	link := filepath.Join(root, "linked.md")
	writeViewFile(t, target, "# Linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Run(context.Background(), Options{Input: link, Mode: markdown.ModeLight, Output: &output}); err != nil {
		t.Fatalf("Run() symlink error = %v", err)
	}
	if !strings.Contains(output.String(), "Linked") {
		t.Fatalf("symlink output = %q", output.String())
	}
}

func TestRunPropagatesReadRenderAndWriteErrorsWithoutSuccessOutput(t *testing.T) {
	source := filepath.Join(t.TempDir(), "guide.md")
	writeViewFile(t, source, "# Guide")
	sentinel := errors.New("sentinel failure")

	tests := []struct {
		name string
		deps dependencies
		want string
	}{
		{
			name: "read",
			deps: testDependencies(
				func(context.Context, string) ([]byte, error) { return nil, sentinel },
				func([]byte, string) ([]byte, error) { return []byte("unused"), nil },
			),
			want: "read Markdown",
		},
		{
			name: "render",
			deps: testDependencies(
				func(context.Context, string) ([]byte, error) { return []byte("# Guide"), nil },
				func([]byte, string) ([]byte, error) { return nil, sentinel },
			),
			want: "render Markdown",
		},
		{
			name: "write",
			deps: func() dependencies {
				deps := testDependencies(
					func(context.Context, string) ([]byte, error) { return []byte("# Guide"), nil },
					func([]byte, string) ([]byte, error) { return []byte("rendered"), nil },
				)
				deps.write = func(io.Writer, []byte) error { return sentinel }
				return deps
			}(),
			want: "write terminal output",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := run(context.Background(), Options{Input: source, Mode: markdown.ModeDark, Output: &output}, test.deps)
			if err == nil || !errors.Is(err, sentinel) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("run() error = %v, want wrapped %q", err, test.want)
			}
			if output.Len() != 0 {
				t.Fatalf("failed view wrote output %q", output.String())
			}
		})
	}
}

func TestRunCancelsRenderingWithoutWritingPartialOutput(t *testing.T) {
	source := filepath.Join(t.TempDir(), "large.md")
	writeViewFile(t, source, "# Large")
	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	deps := testDependencies(
		func(context.Context, string) ([]byte, error) { return []byte(strings.Repeat("large ", 1_000)), nil },
		func([]byte, string) ([]byte, error) {
			close(started)
			<-release
			return []byte("partial success"), nil
		},
	)
	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, Options{Input: source, Mode: markdown.ModeDark, Output: &output}, deps)
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled run error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled rendering did not return promptly")
	}
	if output.Len() != 0 {
		t.Fatalf("canceled rendering wrote output %q", output.String())
	}
}

func testDependencies(
	read func(context.Context, string) ([]byte, error),
	render func([]byte, string) ([]byte, error),
) dependencies {
	return dependencies{
		read:       read,
		render:     render,
		detectDark: func(*os.File, *os.File) bool { return true },
		noColor:    func() bool { return false },
		write:      func(io.Writer, []byte) error { return nil },
	}
}

func writeViewFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func normalizeGolden(contents string) string {
	lines := strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}
