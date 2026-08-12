// Package server provides the browser preview HTTP service.
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
	"github.com/lz-wang/m2h/internal/watcher"
)

const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 8793

	defaultKeepAlive = 15 * time.Second
	shutdownTimeout  = 3 * time.Second
)

// Options configures a preview service.
type Options struct {
	Input      string
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

	OnListening func(string)
}

type dependencies struct {
	listen      func(string, string) (net.Listener, error)
	watch       func(context.Context, string, time.Duration, func(), io.Writer) error
	openBrowser func(string) error
	keepAlive   time.Duration
	debounce    time.Duration
}

// Run validates, starts, and gracefully shuts down one preview service.
func Run(ctx context.Context, options Options) error {
	return run(ctx, options, dependencies{
		listen:      net.Listen,
		watch:       watcher.Watch,
		openBrowser: openBrowser,
		keepAlive:   defaultKeepAlive,
		debounce:    watcher.DefaultDebounce,
	})
}

func run(ctx context.Context, options Options, deps dependencies) error {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("preview: %w", err)
	}

	input, err := files.Resolve(normalized.Input)
	if err != nil {
		return err
	}
	if input.Kind == files.KindFile && normalized.PatternSet {
		return fmt.Errorf("--glob can only be used when previewing a directory")
	}
	if input.Kind == files.KindFile && normalized.DepthSet {
		return fmt.Errorf("--depth can only be used when previewing a directory")
	}
	if input.Kind == files.KindFile && !files.IsMarkdown(input.Path) {
		return fmt.Errorf("preview %q: expected a Markdown file", input.Path)
	}

	logger := normalized.Log
	if logger == nil {
		logger = io.Discard
	}
	hub := newEventHub(deps.keepAlive)
	var handler http.Handler
	if input.Kind == files.KindDirectory {
		handler = newDirectoryHandlerWithWidth(input.Path, normalized.Mode, normalized.Width, files.DiscoverOptions{
			Depth:   normalized.Depth,
			Pattern: normalized.Pattern,
			Log:     logger,
		}, hub, logger, options.UI)
	} else {
		handler = newSingleFileHandlerWithWidth(input.Path, normalized.Mode, normalized.Width, hub)
	}
	httpServer := &http.Server{
		Handler:  handler,
		ErrorLog: log.New(logger, "m2h: http: ", 0),
	}
	requestedAddress := net.JoinHostPort(normalized.Host, strconv.Itoa(normalized.Port))
	listener, err := deps.listen("tcp", requestedAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", requestedAddress, err)
	}

	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	serveDone := make(chan error, 1)
	var watchDone <-chan error
	go func() {
		serveDone <- httpServer.Serve(listener)
	}()
	if input.Kind == files.KindFile {
		watchResults := make(chan error, 1)
		watchDone = watchResults
		go func() {
			watchResults <- deps.watch(runContext, input.Path, deps.debounce, func() {
				hub.publish(documentChanged)
			}, logger)
		}()
	}

	address := previewURL(normalized.Host, listener.Addr())
	if input.Kind == files.KindDirectory {
		address = directoryPreviewURL(address, normalized.Mode, normalized.Width, normalized.TOC)
	}
	_, _ = fmt.Fprintf(logger, "m2h: previewing %s at %s\n", input.Path, address)
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
	case err := <-watchDone:
		if err != nil {
			runErr = err
		}
	case err := <-serveDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("serve preview: %w", err)
		}
	}

	cancel()
	shutdownContext, stopShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer stopShutdown()
	if err := httpServer.Shutdown(shutdownContext); err != nil && runErr == nil {
		runErr = fmt.Errorf("shut down preview: %w", err)
	}
	return runErr
}

func normalizeOptions(options Options) (Options, error) {
	if strings.TrimSpace(options.Input) == "" {
		return Options{}, fmt.Errorf("input path is required")
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

func previewURL(host string, address net.Addr) string {
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

func directoryPreviewURL(address string, mode markdown.Mode, width markdown.Width, toc bool) string {
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
