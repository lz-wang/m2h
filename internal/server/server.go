// Package server provides the browser document-server HTTP service.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
	appversion "github.com/lz-wang/m2h/internal/version"
)

const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 8793

	shutdownTimeout = 3 * time.Second
)

// Options configures a document server.
type Options struct {
	// Inputs lists one or more files or directories to serve. Each entry
	// keeps its own access boundary; they are never merged into one root.
	Inputs     []string
	Host       string
	Port       int
	Mode       markdown.Mode
	Width      markdown.Width
	Browser    bool
	TOC        bool
	Pattern    string
	Depth      int
	PatternSet bool
	DepthSet   bool
	TOCSet     bool
	Log        io.Writer
	UI         fs.FS
	Version    string

	OnListening func(string)
}

type dependencies struct {
	listen      func(string, string) (net.Listener, error)
	openBrowser func(string) error
}

// Run validates, starts, and gracefully shuts down one document server.
func Run(ctx context.Context, options Options) error {
	return run(ctx, options, dependencies{
		listen:      net.Listen,
		openBrowser: openBrowser,
	})
}

func run(ctx context.Context, options Options, deps dependencies) error {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	inputs := make([]files.Input, 0, len(normalized.Inputs))
	for _, raw := range normalized.Inputs {
		input, err := files.Resolve(raw)
		if err != nil {
			return err
		}
		if input.Kind == files.KindFile && !files.IsMarkdown(input.Path) {
			return fmt.Errorf("serve %q: expected a Markdown file", input.Path)
		}
		inputs = append(inputs, input)
	}
	// --glob and --depth are directory discovery rules. They apply to every
	// directory root in the workspace and are ignored by single-file roots —
	// a workspace mixing both must not be rejected outright — but with no
	// directory root at all there is nothing for them to mean.
	hasDirectoryRoot := false
	for _, input := range inputs {
		if input.Kind == files.KindDirectory {
			hasDirectoryRoot = true
			break
		}
	}
	if normalized.PatternSet && !hasDirectoryRoot {
		return fmt.Errorf("--glob can only be used when serving a directory")
	}
	if normalized.DepthSet && !hasDirectoryRoot {
		return fmt.Errorf("--depth can only be used when serving a directory")
	}

	logger := normalized.Log
	if logger == nil {
		logger = io.Discard
	}
	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	// Web publishing hides dot-prefixed paths: anything under a dot component
	// of a directory root (.git/, .env, foo/.private/) is not publishable
	// content. Static analysis (m2h check) keeps its own discovery without
	// SkipHidden, so this policy never narrows what check can inspect.
	workspace, err := newWorkspace(inputs, files.DiscoverOptions{
		Depth:      normalized.Depth,
		Pattern:    normalized.Pattern,
		SkipHidden: true,
		Log:        logger,
	})
	if err != nil {
		return err
	}
	handler := newDocumentHandlerWithVersion(workspace, logger, options.UI, normalized.Version)
	httpServer := &http.Server{
		Handler: handler,
		BaseContext: func(net.Listener) context.Context {
			return runContext
		},
		ErrorLog: log.New(logger, "m2h: http: ", 0),
	}
	requestedAddress := net.JoinHostPort(normalized.Host, strconv.Itoa(normalized.Port))
	listener, err := deps.listen("tcp", requestedAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", requestedAddress, err)
	}

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- httpServer.Serve(listener)
	}()

	address := serverOptionsURL(serverURL(normalized.Host, listener.Addr()), normalized.Mode, normalized.Width, normalized.TOC)
	_, _ = fmt.Fprintf(logger, "m2h: serving %s at %s\n", strings.Join(workspace.inputPaths(), ", "), address)
	if normalized.OnListening != nil {
		normalized.OnListening(address)
	}
	if normalized.Browser {
		if err := deps.openBrowser(address); err != nil {
			_, _ = fmt.Fprintf(logger, "m2h: open browser: %v\n", err)
		}
	}

	var runErr error
	select {
	case <-ctx.Done():
	case err := <-serveDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("serve documents: %w", err)
		}
	}

	cancel()
	shutdownContext, stopShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer stopShutdown()
	if err := httpServer.Shutdown(shutdownContext); err != nil && runErr == nil {
		runErr = fmt.Errorf("shut down server: %w", err)
	}
	return runErr
}

func normalizeOptions(options Options) (Options, error) {
	if len(options.Inputs) == 0 {
		return Options{}, fmt.Errorf("input path is required")
	}
	for _, input := range options.Inputs {
		if strings.TrimSpace(input) == "" {
			return Options{}, fmt.Errorf("input path is required")
		}
	}
	if options.Version == "" {
		options.Version = appversion.Development
	}
	if _, err := appversion.Parse(options.Version); err != nil {
		return Options{}, fmt.Errorf("invalid server version: %w", err)
	}
	if options.Host == "" {
		options.Host = DefaultHost
	}
	if options.Port == 0 {
		options.Port = DefaultPort
	}
	if options.Width == "" {
		options.Width = markdown.WidthStandard
	}
	if !options.TOCSet {
		options.TOC = true
	}
	if options.Port < 1 || options.Port > 65535 {
		return Options{}, fmt.Errorf("invalid port %d: expected 1 through 65535", options.Port)
	}
	switch options.Mode {
	case markdown.ModeLight, markdown.ModeDark, markdown.ModeAuto:
	default:
		return Options{}, fmt.Errorf("invalid mode %q: expected light, dark, or auto", options.Mode)
	}
	switch options.Width {
	case markdown.WidthStandard, markdown.WidthWide, markdown.WidthFull:
	default:
		return Options{}, fmt.Errorf("invalid width %q: expected standard, wide, or full", options.Width)
	}
	if err := files.ValidateDiscoverOptions(files.DiscoverOptions{Depth: options.Depth, Pattern: options.Pattern}); err != nil {
		return Options{}, err
	}
	return options, nil
}

func serverURL(host string, address net.Addr) string {
	if host == "0.0.0.0" || host == "::" || host == "" {
		host = DefaultHost
	}
	port := DefaultPort
	if tcpAddress, ok := address.(*net.TCPAddr); ok {
		port = tcpAddress.Port
	} else if _, value, err := net.SplitHostPort(address.String()); err == nil {
		if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
			port = parsed
		}
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/"
}

func serverOptionsURL(address string, mode markdown.Mode, width markdown.Width, toc bool) string {
	parameters := url.Values{}
	if mode != markdown.ModeAuto {
		parameters.Set("mode", string(mode))
	}
	if width != markdown.WidthStandard {
		parameters.Set("width", string(width))
	}
	if !toc {
		parameters.Set("toc", "false")
	}
	if query := parameters.Encode(); query != "" {
		return address + "?" + query
	}
	return address
}
