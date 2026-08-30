package cli

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lz-wang/m2h/internal/check"
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
	for _, want := range []string{"export", "check", "--version", "-v", "--host", "--port"} {
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
				"--host", "(default: \"127.0.0.1\")", "--port", "-p", "(default: 8793)",
				"--[no-]open", "(default: true)", "--mode", "(default: \"auto\")",
				"--width", "(default: \"standard\")", "--toc", "(default: true)",
				"--glob", "--depth", "-d", "(default: 4)",
				"--version", "-v",
			},
		},
		{
			name: "export",
			args: []string{"export", "--help"},
			want: []string{
				"--output", "-o", "--mode", "(default: \"auto\")",
				"--width", "(default: \"standard\")", "--force",
			},
		},
		{
			name: "check",
			args: []string{"check", "--help"},
			want: []string{
				"--glob", "--depth", "-d", "(default: 4)",
				"--format", "(default: \"text\")", "--strict",
				"--enable", "--disable",
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
		{"export", "README.md", "--toc"},
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
		{"README.md", "--output", "out.html"},
		{"README.md", "--force"},
		{"export", "README.md", "--port", "9000"},
		{"export", "README.md", "--host", "0.0.0.0"},
		{"export", "README.md", "--toc"},
		{"export", "README.md", "--open"},
		{"export", "README.md", "--glob", "*.md"},
		{"export", "README.md", "--depth", "2"},
		{"export", "README.md", "--no-local-paths"},
		{"export", "README.md", "--standalone"},
		{"export", "README.md", "--copy-assets=false"},
		{"export", "README.md", "--yes"},
		{"check", "README.md", "--host", "0.0.0.0"},
		{"check", "README.md", "--port", "9000"},
		{"check", "README.md", "--mode", "dark"},
		{"check", "README.md", "--width", "wide"},
		{"check", "README.md", "--toc"},
		{"check", "README.md", "--open"},
		{"check", "README.md", "--output", "out.html"},
		{"check", "README.md", "--force"},
		{"README.md", "--strict"},
		{"README.md", "--format", "json"},
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

// "web" stopped being a subcommand when the root command became the server
// itself, so it must now reach the server as an ordinary input path — like any
// directory a project happens to call "web" — instead of being rejected as a
// reserved word.
func TestServeAcceptsWebAsInput(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		input []string
	}{
		{name: "single", args: []string{"web"}, input: []string{"web"}},
		{name: "among others", args: []string{"web", "docs"}, input: []string{"web", "docs"}},
		{name: "after options", args: []string{"--no-open", "web"}, input: []string{"web"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previous := runServer
			t.Cleanup(func() { runServer = previous })

			var captured server.Options
			runServer = func(_ context.Context, options server.Options) error {
				captured = options
				return nil
			}
			if _, _, err := runCommand(t, test.args...); err != nil {
				t.Fatalf("m2h %v returned error: %v", test.args, err)
			}
			if !slices.Equal(captured.Inputs, test.input) {
				t.Fatalf("m2h %v inputs = %v, want %v", test.args, captured.Inputs, test.input)
			}
		})
	}
}

// The HTML export subcommand is spelled "export", so the retired "convert"
// name must reach the server as an ordinary input path — like any file or
// directory a project happens to call "convert" — instead of being rejected
// as a reserved word.
func TestServeAcceptsConvertAsInput(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		input []string
	}{
		{name: "single", args: []string{"convert"}, input: []string{"convert"}},
		{name: "among others", args: []string{"convert", "docs"}, input: []string{"convert", "docs"}},
		{name: "after options", args: []string{"--no-open", "convert"}, input: []string{"convert"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previous := runServer
			t.Cleanup(func() { runServer = previous })

			var captured server.Options
			runServer = func(_ context.Context, options server.Options) error {
				captured = options
				return nil
			}
			if _, _, err := runCommand(t, test.args...); err != nil {
				t.Fatalf("m2h %v returned error: %v", test.args, err)
			}
			if !slices.Equal(captured.Inputs, test.input) {
				t.Fatalf("m2h %v inputs = %v, want %v", test.args, captured.Inputs, test.input)
			}
		})
	}
}

func TestServeForwardsOptions(t *testing.T) {
	previous := runServer
	t.Cleanup(func() { runServer = previous })

	var captured server.Options
	runServer = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}
	stdout, stderr, err := runCommand(
		t,
		"guide.md",
		"--host", "0.0.0.0",
		"--port", "9142",
		"--mode", "dark",
	)
	if err != nil || stdout != "" || stderr != "" {
		t.Fatalf("serve result stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	if len(captured.Inputs) != 1 || captured.Inputs[0] != "guide.md" ||
		captured.Host != "0.0.0.0" || captured.Port != 9142 ||
		captured.Mode != markdown.ModeDark || captured.Depth != defaultDepth || captured.DepthSet ||
		!captured.Browser || captured.Log == nil || captured.Version != "dev-20260809-fe65804" ||
		captured.UI == nil {
		t.Fatalf("serve options = %+v", captured)
	}
	// --open and --toc default to true but neither flag is set explicitly.
	if !captured.TOC || captured.TOCSet {
		t.Fatalf("serve toc = %+v, want TOC=true TOCSet=false", captured)
	}
}

func TestServeOpensBrowserByDefault(t *testing.T) {
	previous := runServer
	t.Cleanup(func() { runServer = previous })

	var captured server.Options
	runServer = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}
	if _, _, err := runCommand(t, "guide.md"); err != nil {
		t.Fatalf("serve returned error: %v", err)
	}
	if !captured.Browser {
		t.Fatalf("serve Browser = false, want true by default")
	}
}

func TestServeNoOpen(t *testing.T) {
	previous := runServer
	t.Cleanup(func() { runServer = previous })

	var captured server.Options
	runServer = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}
	if _, _, err := runCommand(t, "guide.md", "--no-open"); err != nil {
		t.Fatalf("serve --no-open returned error: %v", err)
	}
	if captured.Browser {
		t.Fatalf("serve Browser = true, want false with --no-open")
	}
}

func TestServeForwardsTOCFlag(t *testing.T) {
	previous := runServer
	t.Cleanup(func() { runServer = previous })

	var captured server.Options
	runServer = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}
	_, _, err := runCommand(t, "guide.md", "--toc=false")
	if err != nil {
		t.Fatalf("serve --toc=false returned error: %v", err)
	}
	if captured.TOC || !captured.TOCSet {
		t.Fatalf("serve toc = %+v, want TOC=false TOCSet=true", captured)
	}

	_, _, err = runCommand(t, "guide.md", "--toc=true")
	if err != nil {
		t.Fatalf("serve --toc=true returned error: %v", err)
	}
	if !captured.TOC || !captured.TOCSet {
		t.Fatalf("serve toc = %+v, want TOC=true TOCSet=true", captured)
	}
}

func TestServeValidatesArgumentsAndPort(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{","}, want: "Error: requires one or more files or directories"},
		{args: []string{" , , "}, want: "Error: requires one or more files or directories"},
		{args: []string{"missing.md", "--port", "0"}, want: "Error: --port must be between 1 and 65535"},
		{args: []string{"missing.md", "--port", "65536"}, want: "Error: --port must be between 1 and 65535"},
	} {
		_, _, err := runCommand(t, test.args...)
		if err == nil || err.Error() != test.want {
			t.Errorf("m2h %v error = %v, want %q", test.args, err, test.want)
		}
	}
}

func TestServeExpandsMultipleAndCommaSeparatedInputs(t *testing.T) {
	previous := runServer
	t.Cleanup(func() { runServer = previous })

	var captured server.Options
	runServer = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{name: "single input", args: []string{"/a"}, want: []string{"/a"}},
		{name: "space separated", args: []string{"/a", "/b", "/c"}, want: []string{"/a", "/b", "/c"}},
		{name: "comma separated", args: []string{"/a,/b,/c"}, want: []string{"/a", "/b", "/c"}},
		{name: "mixed separators", args: []string{"/a,/b", "/c"}, want: []string{"/a", "/b", "/c"}},
		{name: "segments are trimmed", args: []string{"/a, /b"}, want: []string{"/a", "/b"}},
		{name: "empty segments dropped", args: []string{"/a,,/b"}, want: []string{"/a", "/b"}},
		{name: "exact duplicates removed", args: []string{"/a,/a", "/a"}, want: []string{"/a"}},
	} {
		if _, _, err := runCommand(t, test.args...); err != nil {
			t.Fatalf("%s: serve returned error: %v", test.name, err)
		}
		if got := captured.Inputs; !slices.Equal(got, test.want) {
			t.Errorf("%s: inputs = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestServeRejectsInvalidMultiRootInputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "guide.txt"), []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing root",
			args: []string{filepath.Join(root, "missing"), root},
			want: "inspect input",
		},
		{
			name: "non-markdown file root",
			args: []string{filepath.Join(root, "guide.txt"), root},
			want: "expected a Markdown file",
		},
		{
			name: "duplicate canonical root",
			args: []string{root, root + string(os.PathSeparator)},
			want: "duplicate workspace root",
		},
	} {
		_, _, err := runCommand(t, test.args...)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: m2h %v error = %v, want %q", test.name, test.args, err, test.want)
		}
	}
}

func TestExportCommandWritesHTML(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide\n\n[Next](next.md)\n\n<details>raw HTML</details>"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCommand(t, "export", source, "--output", "index.html", "--mode", "dark", "--width", "wide")
	if err != nil {
		t.Fatalf("export returned error: %v", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "index.html")
	if want := "Wrote " + filepath.Join(resolvedRoot, "index.html") + "\n"; stdout != want || stderr != "" {
		t.Fatalf("export output stdout=%q stderr=%q", stdout, stderr)
	}
	html, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<title>Guide</title>`, `href="next.md"`, `class="m2h-mode-dark"`, `data-width="wide"`, `<details>raw HTML</details>`} {
		if !bytes.Contains(html, []byte(want)) {
			t.Errorf("HTML does not contain %q", want)
		}
	}
}

func TestExportCommandRejectsPathOutput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runCommand(t, "export", source, "--output", "../escape.html")
	if err == nil || !strings.Contains(err.Error(), "must be a plain filename, not a path") {
		t.Fatalf("export error = %v, want plain-filename requirement", err)
	}
}

func TestExportCommandRejectsDirectoryInput(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "guide.md"), []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runCommand(t, "export", source)
	if err == nil || err.Error() != "Error: export requires a Markdown file: \""+source+"\"" {
		t.Fatalf("export directory error = %v, want Markdown-file requirement", err)
	}
	if _, err := os.Stat(filepath.Join(source, "guide.html")); !os.IsNotExist(err) {
		t.Fatal("directory input produced output")
	}
}

func TestExportCommandValidatesArgumentCount(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runCommand(t, "export", source, source)
	if err == nil || err.Error() != "Error: requires exactly one Markdown file" {
		t.Fatalf("export error = %v, want argument count error", err)
	}
}

func TestExportCommandOverwriteRequiresForce(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "guide.html")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runCommand(t, "export", source)
	if err == nil || !strings.Contains(err.Error(), "already exists; rerun with --force") {
		t.Fatalf("export error = %v, want already-exists error", err)
	}
	if stdout != "" {
		t.Fatalf("rejected export wrote stdout %q", stdout)
	}

	stdout, stderr, err := runCommand(t, "export", source, "--force")
	if err != nil {
		t.Fatalf("export --force returned error: %v", err)
	}
	if stderr != "" || !strings.HasPrefix(stdout, "Wrote ") {
		t.Fatalf("export --force stdout=%q stderr=%q", stdout, stderr)
	}
	html, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(html, []byte("<title>Guide</title>")) {
		t.Fatal("--force did not replace the output HTML")
	}
}

func TestModeValidationRunsBeforeFeatureHandlers(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"README.md", "--mode", "sepia"},
		{"export", "README.md", "--mode", "sepia"},
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
		{"export", "README.md", "--width", "narrow"},
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

	_, _, err := runCommand(t, "README.md", "--port", "many")
	if err == nil {
		t.Fatal("serve accepted a non-integer port")
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

func TestCheckCommandReportsScope(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.md"), []byte("# Index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCommand(t, "check", root)
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}
	want := "Checked 2 Markdown files: no issues found\n"
	if stdout != want || stderr != "" {
		t.Fatalf("check stdout=%q stderr=%q, want %q", stdout, stderr, want)
	}

	stdout, _, err = runCommand(t, "check", root, "--glob", "index.md")
	if err != nil {
		t.Fatalf("check --glob returned error: %v", err)
	}
	if want := "Checked 1 Markdown file: no issues found\n"; stdout != want {
		t.Fatalf("check --glob stdout=%q, want %q", stdout, want)
	}

	stdout, _, err = runCommand(t, "check", filepath.Join(root, "guide.md"))
	if err != nil {
		t.Fatalf("check single file returned error: %v", err)
	}
	if want := "Checked 1 Markdown file: no issues found\n"; stdout != want {
		t.Fatalf("check single file stdout=%q, want %q", stdout, want)
	}
}

func TestCheckCommandWritesJSONReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.md"), []byte("# Index"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCommand(t, "check", root, "--format", "json")
	if err != nil {
		t.Fatalf("check --format json returned error: %v", err)
	}
	want := "{\n  \"files\": 1,\n  \"errors\": 0,\n  \"warnings\": 0,\n  \"diagnostics\": []\n}\n"
	if stdout != want || stderr != "" {
		t.Fatalf("check json stdout=%q stderr=%q, want %q", stdout, stderr, want)
	}
}

func TestCheckCommandValidatesArguments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No arguments prints help instead of failing.
	stdout, _, err := runCommand(t, "check")
	if err != nil {
		t.Fatalf("check without arguments returned error: %v", err)
	}
	if !strings.Contains(stdout, "check Markdown documents") {
		t.Fatalf("check without arguments should print help, got %q", stdout)
	}

	_, _, err = runCommand(t, "check", filepath.Join(root, "guide.md"), root)
	if err == nil || err.Error() != "Error: requires exactly one file or directory" {
		t.Fatalf("check error = %v, want argument count error", err)
	}
}

func TestCheckCommandValidatesFlagsBeforeFilesystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"check", "docs", "--format", "table"}, want: "Error: --format must be text or json"},
		{args: []string{"check", "docs", "--depth", "-1"}, want: "Error: --depth must be zero or greater"},
		{args: []string{"check", "docs", "--glob", "["}, want: `Error: validate check options: invalid glob "["`},
		{args: []string{"check", "docs", "--enable", "foo.bar"}, want: `Error: unknown check rule "foo.bar"`},
		{args: []string{"check", "docs", "--disable", "foo.bar"}, want: `Error: unknown check rule "foo.bar"`},
	}
	for _, test := range tests {
		_, _, err := runCommand(t, test.args...)
		if err == nil || err.Error() != test.want {
			t.Errorf("m2h %v error = %v, want %q", test.args, err, test.want)
		}
	}
}

// TestCheckCommandForwardsRuleSelection replaces runCheck and therefore
// cannot run in parallel, the same contract the runServer stub tests follow.
func TestCheckCommandForwardsRuleSelection(t *testing.T) {
	previous := runCheck
	t.Cleanup(func() { runCheck = previous })

	var captured check.Options
	runCheck = func(_ context.Context, options check.Options) (check.Result, error) {
		captured = options
		return check.Result{}, nil
	}
	stdout, stderr, err := runCommand(t,
		"check", "docs",
		"--enable", "section.empty,unicode.mojibake",
		"--disable", "image.alt-empty",
	)
	if err != nil || stderr != "" {
		t.Fatalf("check stderr=%q err=%v", stderr, err)
	}
	if want := "Checked 0 Markdown files: no issues found\n"; stdout != want {
		t.Fatalf("check stdout=%q, want %q", stdout, want)
	}
	if want := []string{"section.empty", "unicode.mojibake"}; !slices.Equal(captured.EnableRules, want) {
		t.Fatalf("EnableRules = %v, want %v", captured.EnableRules, want)
	}
	if want := []string{"image.alt-empty"}; !slices.Equal(captured.DisableRules, want) {
		t.Fatalf("DisableRules = %v, want %v", captured.DisableRules, want)
	}
}

func TestCheckCommandEnableRunsOptInRules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Title\n\n## Empty\n\n## Next\n\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCommand(t, "check", root)
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}
	if strings.Contains(stdout, "section.empty") || stderr != "" {
		t.Fatalf("opt-in rule fired without --enable: stdout=%q stderr=%q", stdout, stderr)
	}

	stdout, _, err = runCommand(t, "check", root, "--enable", "section.empty")
	if err != nil {
		t.Fatalf("check --enable returned error: %v", err)
	}
	// Warnings alone keep the exit code zero; the diagnostic must appear.
	if !strings.Contains(stdout, ":3:1: warning [section.empty]: section \"Empty\" has no content\n") {
		t.Fatalf("check --enable stdout = %q, want the section.empty diagnostic", stdout)
	}
	if !strings.HasSuffix(stdout, "Checked 1 Markdown file: 2 warnings\n") {
		t.Fatalf("check --enable stdout = %q, want two warnings in the summary", stdout)
	}
}

func TestCheckCommandFailsOnDiagnostics(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide\n\n![missing](nope.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCommand(t, "check", root)
	if err == nil || err.Error() != "Error: check found 1 error" {
		t.Fatalf("check error = %v, want a single-error failure", err)
	}
	if !strings.Contains(stdout, "guide.md:3:3: error [local-target.missing]: target \"nope.png\" does not exist\n") ||
		!strings.HasSuffix(stdout, "Checked 1 Markdown file: 1 error\n") {
		t.Fatalf("check stdout = %q, want the diagnostic and summary lines", stdout)
	}
	if stderr != "" {
		t.Fatalf("check wrote stderr %q, want none (the error is the exit signal)", stderr)
	}
}

func TestCheckCommandStrictFailsOnWarnings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide\n\n![](logo.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Warnings alone keep the exit code clean without --strict.
	stdout, _, err := runCommand(t, "check", root)
	if err != nil {
		t.Fatalf("check returned error: %v, want warnings to pass", err)
	}
	if !strings.HasSuffix(stdout, "Checked 1 Markdown file: 1 warning\n") {
		t.Fatalf("check stdout = %q, want the warning summary", stdout)
	}

	_, _, err = runCommand(t, "check", root, "--strict")
	if err == nil || err.Error() != "Error: check found 1 warning" {
		t.Fatalf("check --strict error = %v, want the warning to fail the run", err)
	}
}

func TestCheckCommandRejectsUnusableInput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	notes := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(notes, []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runCommand(t, "check", notes)
	if err == nil || !strings.Contains(err.Error(), "Error: check requires a Markdown file or directory") {
		t.Fatalf("check non-markdown error = %v", err)
	}

	_, _, err = runCommand(t, "check", filepath.Join(root, "missing"))
	if err == nil || !strings.Contains(err.Error(), "Error: inspect input") {
		t.Fatalf("check missing input error = %v", err)
	}
}

// Bare "m2h export" used to print "No help topic for 'export'" and exit 3:
// urfave/cli v3's ShowCommandHelp resolves the name among the *parent's*
// commands, so a command action must pass its root, not itself.
func TestBareSubcommandsShowOwnHelp(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"export", "check"} {
		stdout, stderr, err := runCommand(t, name)
		if err != nil {
			t.Fatalf("m2h %s returned error: %v", name, err)
		}
		if stderr != "" {
			t.Fatalf("m2h %s wrote stderr %q", name, stderr)
		}
		if !strings.Contains(stdout, "USAGE:") || !strings.Contains(stdout, name) {
			t.Fatalf("m2h %s help output does not document the command:\n%s", name, stdout)
		}
	}
}
