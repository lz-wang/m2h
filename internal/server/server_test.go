package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
			Input: source,
			Mode:  markdown.ModeAuto,
			Log:   &logOutput,
			OnListening: func(address string) {
				listening <- address
			},
		}, deps)
	}()

	if got := <-requested; got != "tcp 127.0.0.1:8793" {
		t.Fatalf("listen request = %q", got)
	}
	address := <-listening
	response, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("<title>Running</title>")) {
		t.Fatalf("preview response = %d %q", response.StatusCode, body)
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
			Input:   source,
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
	base := Options{Input: source, Host: DefaultHost, Port: 9000, Mode: markdown.ModeAuto}

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
	err = Run(context.Background(), Options{Input: source, Host: DefaultHost, Port: port, Mode: markdown.ModeAuto})
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
		{name: "port", options: Options{Input: missing, Port: 70000, Mode: markdown.ModeAuto}, want: "invalid port"},
		{name: "mode", options: Options{Input: missing, Mode: "sepia"}, want: "invalid mode"},
		{name: "depth", options: Options{Input: missing, Mode: markdown.ModeAuto, Depth: -1}, want: "invalid depth"},
		{name: "glob", options: Options{Input: missing, Mode: markdown.ModeAuto, Pattern: "["}, want: "invalid glob"},
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
	err := run(canceled, Options{Input: missing, Mode: markdown.ModeAuto}, testDependencies())
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
		{options: Options{Input: root, Mode: markdown.ModeAuto}, want: "directory preview is not implemented"},
		{options: Options{Input: textFile, Mode: markdown.ModeAuto}, want: "expected a Markdown file"},
		{options: Options{Input: markdownFile, Mode: markdown.ModeAuto, PatternSet: true}, want: "--glob can only be used"},
		{options: Options{Input: markdownFile, Mode: markdown.ModeAuto, DepthSet: true}, want: "--depth can only be used"},
	}
	for _, test := range tests {
		err := run(context.Background(), test.options, testDependencies())
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("run(%+v) error = %v, want %q", test.options, err, test.want)
		}
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
