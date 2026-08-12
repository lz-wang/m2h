package cli

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lz-wang/m2h/internal/markdown"
	"github.com/lz-wang/m2h/internal/server"
)

func runCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command, err := New("dev-20260809-fe65804", testUI(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	err = command.Run(context.Background(), append([]string{"m2h"}, args...))
	return stdout.String(), stderr.String(), err
}

// testUI returns a minimal embedded WebUI filesystem for CLI tests.
func testUI() fs.FS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><div id="root"></div>`)},
	}
}

func TestVersionCommandsMatch(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"--version"}, {"-v"}} {
		stdout, stderr, err := runCommand(t, args...)
		if err != nil {
			t.Fatalf("m2h %v returned error: %v", args, err)
		}
		if stderr != "" {
			t.Fatalf("m2h %v wrote stderr %q", args, stderr)
		}
		if got, want := stdout, "dev-20260809-fe65804\n"; got != want {
			t.Fatalf("m2h %v output = %q, want %q", args, got, want)
		}
	}
}

func TestRootHelpDocumentsCommands(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := runCommand(t)
	if err != nil {
		t.Fatalf("root help returned error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("root help wrote stderr %q", stderr)
	}
	for _, want := range []string{"web", "--version", "-v", "--output"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("root help does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestHelpDocumentsContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "root",
			args: []string{"--help"},
			want: []string{
				"--output", "-o", "--glob", "--depth", "-d", "(default: 4)",
				"--mode", "(default: \"auto\")", "--width", "(default: \"standard\")",
				"--copy-assets", "(default: true)", "--version", "-v",
			},
		},
		{
			name: "web",
			args: []string{"web", "--help"},
			want: []string{
				"--host", "(default: \"127.0.0.1\")", "--port", "-p", "(default: 8793)",
				"--[no-]open", "(default: true)", "--mode", "(default: \"auto\")",
				"--width", "(default: \"standard\")", "--toc", "(default: true)",
				"--glob", "--depth", "-d", "(default: 4)",
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, err := runCommand(t, test.args...)
			if err != nil {
				t.Fatalf("help returned error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("help wrote stderr %q", stderr)
			}
			for _, want := range test.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("help output does not contain %q:\n%s", want, stdout)
				}
			}
		})
	}
}

func TestUnknownFlagsReturnStableError(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"--unknown"},
		{"README.md", "--unknown"},
		{"README.md", "--unsafe-html"},
		{"README.md", "--toc"},
		{"web", "README.md", "--unsafe-html"},
	} {
		_, _, err := runCommand(t, args...)
		if err == nil {
			t.Fatalf("m2h %v succeeded, want error", args)
		}
		if got, want := err.Error(), "Error: unknown option"; got != want {
			t.Fatalf("m2h %v error = %q, want %q", args, got, want)
		}
	}
}

func TestFlagsAreIsolatedBetweenCommands(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"web", "README.md", "--output", "out.html"},
		{"web", "README.md", "--copy-assets=false"},
		{"README.md", "--port", "9000"},
		{"README.md", "--host", "0.0.0.0"},
		{"README.md", "--toc"},
		{"README.md", "--open"},
	} {
		_, _, err := runCommand(t, args...)
		if err == nil {
			t.Fatalf("m2h %v succeeded, want unknown option", args)
		}
		if got, want := err.Error(), "Error: unknown option"; got != want {
			t.Fatalf("m2h %v error = %q, want %q", args, got, want)
		}
	}
}

func TestWebForwardsOptions(t *testing.T) {
	previous := runPreview
	t.Cleanup(func() { runPreview = previous })

	var captured server.Options
	runPreview = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}
	stdout, stderr, err := runCommand(
		t,
		"web", "guide.md",
		"--host", "0.0.0.0",
		"--port", "9142",
		"--mode", "dark",
	)
	if err != nil || stdout != "" || stderr != "" {
		t.Fatalf("web result stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	if captured.Input != "guide.md" || captured.Host != "0.0.0.0" || captured.Port != 9142 ||
		captured.Mode != markdown.ModeDark || captured.Depth != defaultDepth || captured.DepthSet ||
		!captured.Browser || captured.Log == nil || captured.Version != "dev-20260809-fe65804" ||
		captured.UI == nil {
		t.Fatalf("web options = %+v", captured)
	}
	// --open and --toc default to true but neither flag is set explicitly.
	if !captured.TOC || captured.TOCSet {
		t.Fatalf("web toc = %+v, want TOC=true TOCSet=false", captured)
	}
}

func TestWebOpensBrowserByDefault(t *testing.T) {
	previous := runPreview
	t.Cleanup(func() { runPreview = previous })

	var captured server.Options
	runPreview = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}
	if _, _, err := runCommand(t, "web", "guide.md"); err != nil {
		t.Fatalf("web returned error: %v", err)
	}
	if !captured.Browser {
		t.Fatalf("web Browser = false, want true by default")
	}
}

func TestWebNoOpen(t *testing.T) {
	previous := runPreview
	t.Cleanup(func() { runPreview = previous })

	var captured server.Options
	runPreview = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}
	if _, _, err := runCommand(t, "web", "guide.md", "--no-open"); err != nil {
		t.Fatalf("web --no-open returned error: %v", err)
	}
	if captured.Browser {
		t.Fatalf("web Browser = true, want false with --no-open")
	}
}

func TestWebForwardsTOCFlag(t *testing.T) {
	previous := runPreview
	t.Cleanup(func() { runPreview = previous })

	var captured server.Options
	runPreview = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}
	_, _, err := runCommand(t, "web", "guide.md", "--toc=false")
	if err != nil {
		t.Fatalf("web --toc=false returned error: %v", err)
	}
	if captured.TOC || !captured.TOCSet {
		t.Fatalf("web toc = %+v, want TOC=false TOCSet=true", captured)
	}

	_, _, err = runCommand(t, "web", "guide.md", "--toc=true")
	if err != nil {
		t.Fatalf("web --toc=true returned error: %v", err)
	}
	if !captured.TOC || !captured.TOCSet {
		t.Fatalf("web toc = %+v, want TOC=true TOCSet=true", captured)
	}
}

func TestWebValidatesArgumentsAndPort(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"web"}, want: "Error: web requires exactly one file or directory"},
		{args: []string{"web", "one.md", "two.md"}, want: "Error: web requires exactly one file or directory"},
		{args: []string{"web", "missing.md", "--port", "0"}, want: "Error: --port must be between 1 and 65535"},
		{args: []string{"web", "missing.md", "--port", "65536"}, want: "Error: --port must be between 1 and 65535"},
	} {
		_, _, err := runCommand(t, test.args...)
		if err == nil || err.Error() != test.want {
			t.Errorf("m2h %v error = %v, want %q", test.args, err, test.want)
		}
	}
}

func TestConvertCommandWritesHTML(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide\n\n[Next](next.md)\n\n<details>raw HTML</details>"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "public", "index.html")

	stdout, stderr, err := runCommand(t, source, "--output", output, "--mode", "dark", "--width", "wide")
	if err != nil {
		t.Fatalf("convert returned error: %v", err)
	}
	resolvedOutput, err := filepath.EvalSymlinks(output)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Converted 1 Markdown file.\nOutput HTML files:\n- " + resolvedOutput + "\n"; stdout != want || stderr != "" {
		t.Fatalf("convert output stdout=%q stderr=%q", stdout, stderr)
	}
	html, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<title>Guide</title>`, `href="next.html"`, `class="m2h-mode-dark"`, `data-width="wide"`, `<details>raw HTML</details>`} {
		if !bytes.Contains(html, []byte(want)) {
			t.Errorf("HTML does not contain %q", want)
		}
	}
}

func TestConvertCommandWritesDirectoryResult(t *testing.T) {
	source := t.TempDir()
	output := filepath.Join(t.TempDir(), "public")
	if err := os.WriteFile(filepath.Join(source, "guide.md"), []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "logo.svg"), []byte("svg"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCommand(t, source, "--output", output)
	if err != nil {
		t.Fatalf("convert returned error: %v", err)
	}
	resolvedOutput, err := filepath.EvalSymlinks(output)
	if err != nil {
		t.Fatal(err)
	}
	want := "Converted 1 Markdown file; copied 1 asset.\nOutput HTML files:\n- " + filepath.Join(resolvedOutput, "guide.html") + "\n"
	if stdout != want || stderr != "" {
		t.Fatalf("convert output stdout=%q stderr=%q, want stdout=%q stderr=\"\"", stdout, stderr, want)
	}
}

func TestConvertCommandValidatesArgumentsAndDirectoryOnlyFlags(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		args []string
		want string
	}{
		{args: []string{source, source}, want: "Error: requires exactly one file or directory"},
		{args: []string{source, "--glob", "*.md"}, want: "Error: --glob can only be used when converting a directory"},
		{args: []string{source, "--depth", "2"}, want: "Error: --depth can only be used when converting a directory"},
		{args: []string{source, "--copy-assets=false"}, want: "Error: --copy-assets can only be used when converting a directory"},
	}
	for _, test := range tests {
		_, _, err := runCommand(t, test.args...)
		if err == nil || err.Error() != test.want {
			t.Errorf("m2h %v error = %v, want %q", test.args, err, test.want)
		}
	}
}

func TestConvertCommandValidatesGlobBeforeInput(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, _, err := runCommand(t, missing, "--glob", "[")
	if err == nil || !strings.Contains(err.Error(), "invalid glob") || strings.Contains(err.Error(), "missing") {
		t.Fatalf("convert error = %v, want invalid glob before input error", err)
	}
}

func TestModeValidationRunsBeforeFeatureHandlers(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"README.md", "--mode", "sepia"},
		{"web", "README.md", "--mode", "sepia"},
	} {
		_, _, err := runCommand(t, args...)
		if err == nil {
			t.Fatalf("m2h %v accepted an invalid mode", args)
		}
		if got, want := err.Error(), "Error: --mode must be one of light, dark, or auto"; got != want {
			t.Fatalf("m2h %v error = %q, want %q", args, got, want)
		}
	}
}

func TestWidthValidationRunsBeforeFeatureHandlers(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"README.md", "--width", "narrow"},
		{"web", "README.md", "--width", "narrow"},
	} {
		_, _, err := runCommand(t, args...)
		if err == nil {
			t.Fatalf("m2h %v accepted an invalid width", args)
		}
		if got, want := err.Error(), "Error: --width must be one of standard, wide, or full"; got != want {
			t.Fatalf("m2h %v error = %q, want %q", args, got, want)
		}
	}
}

func TestInvalidFlagValueGetsStablePrefix(t *testing.T) {
	t.Parallel()

	_, _, err := runCommand(t, "web", "README.md", "--port", "many")
	if err == nil {
		t.Fatal("preview accepted a non-integer port")
	}
	if !strings.HasPrefix(err.Error(), "Error: ") {
		t.Fatalf("error = %q, want stable Error prefix", err)
	}
}

func TestInvalidVersionFailsConstruction(t *testing.T) {
	t.Parallel()

	if _, err := New("invalid", nil, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("New() succeeded with an invalid version")
	}
}
