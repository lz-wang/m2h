package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

func TestRunBindsServesAndGracefullyStops(t *testing.T) {
	t.Parallel()

	source := filepath.Join(t.TempDir(), "guide.md")
	writeTestFile(t, source, "# Running")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requested := make(chan string, 1)
	listening := make(chan string, 1)
	var logOutput bytes.Buffer
	deps := testDependencies()
	deps.listen = func(network, address string) (net.Listener, error) {
		requested <- network + " " + address
		return net.Listen("tcp", "127.0.0.1:0")
	}
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, Options{
			Inputs:  []string{source},
			Mode:    markdown.ModeAuto,
			Log:     &logOutput,
			UI:      directoryTestUI(),
			Version: "1.2.3",
			OnListening: func(address string) {
				listening <- address
			},
		}, deps)
	}()

	if got := <-requested; got != "tcp 127.0.0.1:8793" {
		t.Fatalf("listen request = %q", got)
	}
	address := <-listening

	// A single-file preview now serves the unified React WebUI: the shell is
	// the embedded SPA and the document arrives through the JSON API.
	shell, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	shellBody, err := io.ReadAll(shell.Body)
	shell.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if shell.StatusCode != http.StatusOK || !bytes.Contains(shellBody, []byte(`id="root"`)) {
		t.Fatalf("preview shell = %d %q", shell.StatusCode, shellBody)
	}

	listResponse, err := http.Get(address + "api/files")
	if err != nil {
		t.Fatal(err)
	}
	var list fileListResponse
	if err := json.NewDecoder(listResponse.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	listResponse.Body.Close()
	if len(list.Files) != 1 || list.Files[0].Path != "guide.md" || list.DefaultPath != "guide.md" {
		t.Fatalf("single-file list = %+v", list)
	}
	if list.Kind != previewSingle {
		t.Fatalf("single-file kind = %q, want %q", list.Kind, previewSingle)
	}
	if list.Version != "1.2.3" {
		t.Fatalf("single-file version = %q, want 1.2.3", list.Version)
	}

	document, err := http.Get(address + "api/document?path=guide.md")
	if err != nil {
		t.Fatal(err)
	}
	documentBody, err := io.ReadAll(document.Body)
	document.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if document.StatusCode != http.StatusOK || !bytes.Contains(documentBody, []byte("Running")) {
		t.Fatalf("document response = %d %q", document.StatusCode, documentBody)
	}
	if !strings.Contains(logOutput.String(), "m2h: previewing") {
		t.Fatalf("preview log = %q", logOutput.String())
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() after cancellation returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run() did not shut down after cancellation")
	}
}

func TestRunStopsWithActiveEventStream(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "# Events")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	listening := make(chan string, 1)
	deps := testDependencies()
	deps.listen = func(string, string) (net.Listener, error) {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, Options{
			Inputs: []string{root},
			Mode:   markdown.ModeAuto,
			UI:     directoryTestUI(),
			OnListening: func(address string) {
				listening <- address
			},
		}, deps)
	}()

	response, err := http.Get(<-listening + "api/events")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = response.Body.Close()
	})
	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil || line != ": connected\n" {
		t.Fatalf("initial SSE line = %q, %v", line, err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() after cancellation returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run() waited for the active SSE request during shutdown")
	}
}

func TestRunUsesCustomBindAndLogsBrowserFailure(t *testing.T) {
	t.Parallel()

	source := filepath.Join(t.TempDir(), "guide.md")
	writeTestFile(t, source, "# Browser")
	ctx, cancel := context.WithCancel(context.Background())
	requested := make(chan string, 1)
	opened := make(chan string, 1)
	var logOutput bytes.Buffer
	deps := testDependencies()
	deps.listen = func(_ string, address string) (net.Listener, error) {
		requested <- address
		return net.Listen("tcp", "127.0.0.1:0")
	}
	deps.openBrowser = func(address string) error {
		opened <- address
		cancel()
		return errors.New("launcher unavailable")
	}
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, Options{
			Inputs:  []string{source},
			Host:    "0.0.0.0",
			Port:    9142,
			Mode:    markdown.ModeLight,
			Browser: true,
			Log:     &logOutput,
		}, deps)
	}()

	if got := <-requested; got != "0.0.0.0:9142" {
		t.Fatalf("custom listen request = %q", got)
	}
	if address := <-opened; !strings.HasPrefix(address, "http://127.0.0.1:") {
		t.Fatalf("browser address = %q", address)
	}
	if err := <-done; err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
	if !strings.Contains(logOutput.String(), "open browser: launcher unavailable") {
		t.Fatalf("browser failure log = %q", logOutput.String())
	}
}

func TestRunReturnsListenWatcherAndServeErrors(t *testing.T) {
	t.Parallel()

	source := filepath.Join(t.TempDir(), "guide.md")
	writeTestFile(t, source, "# Errors")
	base := Options{Inputs: []string{source}, Host: DefaultHost, Port: 9000, Mode: markdown.ModeAuto}

	listenFailure := testDependencies()
	listenFailure.listen = func(string, string) (net.Listener, error) {
		return nil, errors.New("address unavailable")
	}
	if err := run(context.Background(), base, listenFailure); err == nil || !strings.Contains(err.Error(), "address unavailable") {
		t.Fatalf("listen failure = %v", err)
	}

	watchFailure := testDependencies()
	watchFailure.watch = func(context.Context, string, time.Duration, func(), io.Writer) error {
		return errors.New("watch failed")
	}
	if err := run(context.Background(), base, watchFailure); err == nil || !strings.Contains(err.Error(), "watch failed") {
		t.Fatalf("watch failure = %v", err)
	}

	serveFailure := testDependencies()
	serveFailure.listen = func(string, string) (net.Listener, error) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		if err := listener.Close(); err != nil {
			return nil, err
		}
		return listener, nil
	}
	if err := run(context.Background(), base, serveFailure); err == nil || !strings.Contains(err.Error(), "serve preview") {
		t.Fatalf("serve failure = %v", err)
	}
}

func TestRunRejectsOccupiedPort(t *testing.T) {
	t.Parallel()

	source := filepath.Join(t.TempDir(), "guide.md")
	writeTestFile(t, source, "# Occupied")
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port
	err = Run(context.Background(), Options{Inputs: []string{source}, Host: DefaultHost, Port: port, Mode: markdown.ModeAuto})
	if err == nil || !strings.Contains(err.Error(), "listen on") {
		t.Fatalf("occupied-port error = %v", err)
	}
}

func TestRunValidatesBeforeFilesystemAndNetwork(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing.md")
	tests := []struct {
		name    string
		options Options
		want    string
	}{
		{name: "empty input", options: Options{Mode: markdown.ModeAuto}, want: "input path is required"},
		{name: "port", options: Options{Inputs: []string{missing}, Port: 70000, Mode: markdown.ModeAuto}, want: "invalid port"},
		{name: "mode", options: Options{Inputs: []string{missing}, Mode: "sepia"}, want: "invalid mode"},
		{name: "width", options: Options{Inputs: []string{missing}, Mode: markdown.ModeAuto, Width: "narrow"}, want: "invalid width"},
		{name: "version", options: Options{Inputs: []string{missing}, Mode: markdown.ModeAuto, Version: "latest"}, want: "invalid preview version"},
		{name: "depth", options: Options{Inputs: []string{missing}, Mode: markdown.ModeAuto, Depth: -1}, want: "invalid depth"},
		{name: "glob", options: Options{Inputs: []string{missing}, Mode: markdown.ModeAuto, Pattern: "["}, want: "invalid glob"},
	}
	for _, test := range tests {
		called := false
		deps := testDependencies()
		deps.listen = func(string, string) (net.Listener, error) {
			called = true
			return nil, errors.New("unexpected")
		}
		err := run(context.Background(), test.options, deps)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s error = %v, want %q", test.name, err, test.want)
		}
		if called {
			t.Errorf("%s reached network before validation", test.name)
		}
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	err := run(canceled, Options{Inputs: []string{missing}, Mode: markdown.ModeAuto}, testDependencies())
	if err == nil || !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "missing") {
		t.Fatalf("canceled error = %v", err)
	}
}

func TestRunValidatesInputKindsAndDirectoryFlags(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	markdownFile := filepath.Join(root, "guide.md")
	textFile := filepath.Join(root, "guide.txt")
	writeTestFile(t, markdownFile, "# Guide")
	writeTestFile(t, textFile, "text")
	tests := []struct {
		options Options
		want    string
	}{
		{options: Options{Inputs: []string{textFile}, Mode: markdown.ModeAuto}, want: "expected a Markdown file"},
		{options: Options{Inputs: []string{markdownFile}, Mode: markdown.ModeAuto, PatternSet: true}, want: "--glob can only be used"},
		{options: Options{Inputs: []string{markdownFile}, Mode: markdown.ModeAuto, DepthSet: true}, want: "--depth can only be used"},
	}
	for _, test := range tests {
		err := run(context.Background(), test.options, testDependencies())
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("run(%+v) error = %v, want %q", test.options, err, test.want)
		}
	}
}

func TestRunAcceptsMultipleInputsAndWatchesEachSingleFile(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	first := filepath.Join(base, "one.md")
	second := filepath.Join(base, "two.md")
	writeTestFile(t, first, "# One")
	writeTestFile(t, second, "# Two")
	watched := make(chan string, 2)
	deps := testDependencies()
	deps.listen = func(string, string) (net.Listener, error) {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	deps.watch = func(ctx context.Context, target string, _ time.Duration, _ func(), _ io.Writer) error {
		watched <- target
		<-ctx.Done()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, Options{
			Inputs: []string{first, second},
			Mode:   markdown.ModeAuto,
		}, deps)
	}()

	seen := map[string]bool{}
	for range 2 {
		select {
		case target := <-watched:
			seen[target] = true
		case <-time.After(2 * time.Second):
			t.Fatal("a single-file root did not get its own watcher")
		}
	}
	// files.Resolve canonicalizes inputs (macOS /var aliases to /private/var),
	// so the watcher targets are compared through the same normalization.
	firstCanonical, err := files.CanonicalPath(first)
	if err != nil {
		t.Fatal(err)
	}
	secondCanonical, err := files.CanonicalPath(second)
	if err != nil {
		t.Fatal(err)
	}
	if !seen[firstCanonical] || !seen[secondCanonical] {
		t.Fatalf("watched targets = %v, want both %q and %q", seen, firstCanonical, secondCanonical)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("multi-input run error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("multi-input run did not shut down after cancellation")
	}
}

func TestRunAllowsDepthAndGlobWithMixedFileAndDirectoryRoots(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	document := filepath.Join(base, "note.md")
	writeTestFile(t, document, "# Note")
	docs := filepath.Join(base, "docs")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(docs, "README.md"), "# Docs")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps := testDependencies()
	deps.listen = func(string, string) (net.Listener, error) {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	err := run(ctx, Options{
		Inputs:     []string{document, docs},
		Mode:       markdown.ModeAuto,
		Depth:      2,
		DepthSet:   true,
		Pattern:    "**/*.md",
		PatternSet: true,
		OnListening: func(string) {
			cancel()
		},
	}, deps)
	if err != nil {
		t.Fatalf("mixed roots with directory flags error = %v", err)
	}
}

func TestRunRejectsDuplicateInputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "# Duplicate")
	err := run(context.Background(), Options{
		Inputs: []string{root, root},
		Mode:   markdown.ModeAuto,
	}, testDependencies())
	if err == nil || !strings.Contains(err.Error(), "duplicate preview root") {
		t.Fatalf("duplicate input error = %v", err)
	}
}

func TestRunDirectoryDoesNotCreateWatcher(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "# Directory")
	ctx, cancel := context.WithCancel(context.Background())
	watchCalled := false
	listeningAddress := ""
	deps := testDependencies()
	deps.listen = func(string, string) (net.Listener, error) {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	deps.watch = func(context.Context, string, time.Duration, func(), io.Writer) error {
		watchCalled = true
		return errors.New("directory watcher must not run")
	}
	err := run(ctx, Options{
		Inputs: []string{root + string(os.PathSeparator)},
		Mode:   markdown.ModeDark,
		OnListening: func(address string) {
			listeningAddress = address
			cancel()
		},
	}, deps)
	if err != nil {
		t.Fatalf("directory run error = %v", err)
	}
	if watchCalled {
		t.Fatal("directory preview created a watcher")
	}
	if !strings.HasSuffix(listeningAddress, "/?mode=dark") {
		t.Fatalf("directory preview address = %q, want non-default mode query", listeningAddress)
	}
}

func TestRunDirectoryPreviewURLRespectsTOCFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "# Directory")
	ctx, cancel := context.WithCancel(context.Background())
	listeningAddress := ""
	deps := testDependencies()
	deps.listen = func(string, string) (net.Listener, error) {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	deps.watch = func(context.Context, string, time.Duration, func(), io.Writer) error {
		return errors.New("directory watcher must not run")
	}
	err := run(ctx, Options{
		Inputs: []string{root + string(os.PathSeparator)},
		Mode:   markdown.ModeDark,
		TOC:    false,
		TOCSet: true,
		OnListening: func(address string) {
			listeningAddress = address
			cancel()
		},
	}, deps)
	if err != nil {
		t.Fatalf("directory run error = %v", err)
	}
	if !strings.HasSuffix(listeningAddress, "/?mode=dark&toc=false") {
		t.Fatalf("directory preview address = %q, want toc=false query", listeningAddress)
	}
}

func TestDirectoryPreviewURLOmitsDefaultParameters(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		mode  markdown.Mode
		width markdown.Width
		toc   bool
		want  string
	}{
		{name: "all defaults", mode: markdown.ModeAuto, width: markdown.WidthStandard, toc: true, want: "http://127.0.0.1:8793/"},
		{name: "non-default mode", mode: markdown.ModeDark, width: markdown.WidthStandard, toc: true, want: "http://127.0.0.1:8793/?mode=dark"},
		{name: "non-default width", mode: markdown.ModeAuto, width: markdown.WidthWide, toc: true, want: "http://127.0.0.1:8793/?width=wide"},
		{name: "both non-default", mode: markdown.ModeLight, width: markdown.WidthFull, toc: true, want: "http://127.0.0.1:8793/?mode=light&width=full"},
		{name: "toc disabled", mode: markdown.ModeAuto, width: markdown.WidthStandard, toc: false, want: "http://127.0.0.1:8793/?toc=false"},
		{name: "toc disabled with mode", mode: markdown.ModeDark, width: markdown.WidthStandard, toc: false, want: "http://127.0.0.1:8793/?mode=dark&toc=false"},
		{name: "toc disabled with width", mode: markdown.ModeAuto, width: markdown.WidthWide, toc: false, want: "http://127.0.0.1:8793/?toc=false&width=wide"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := previewOptionsURL("http://127.0.0.1:8793/", test.mode, test.width, test.toc); got != test.want {
				t.Fatalf("previewOptionsURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPreviewURLNormalizesWildcardAndAddressTypes(t *testing.T) {
	t.Parallel()

	tcpAddress := &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 4321}
	if got, want := previewURL("0.0.0.0", tcpAddress), "http://127.0.0.1:4321/"; got != want {
		t.Fatalf("previewURL wildcard = %q, want %q", got, want)
	}
	if got, want := previewURL("::1", stringAddress("[::1]:5432")), "http://[::1]:5432/"; got != want {
		t.Fatalf("previewURL IPv6 = %q, want %q", got, want)
	}
	if got, want := previewURL("localhost", stringAddress("invalid")), "http://localhost:8793/"; got != want {
		t.Fatalf("previewURL fallback = %q, want %q", got, want)
	}
}

type stringAddress string

func (address stringAddress) Network() string { return "tcp" }

func (address stringAddress) String() string { return string(address) }

func testDependencies() dependencies {
	return dependencies{
		listen: net.Listen,
		watch: func(ctx context.Context, _ string, _ time.Duration, _ func(), _ io.Writer) error {
			<-ctx.Done()
			return nil
		},
		openBrowser: func(string) error { return nil },
		keepAlive:   time.Second,
		debounce:    time.Millisecond,
	}
}
