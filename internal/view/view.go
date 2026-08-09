// Package view renders one Markdown file for a terminal.
package view

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	goldmarkrenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

const terminalWidth = 80

// Options configures terminal rendering for one Markdown file.
type Options struct {
	Input  string
	Mode   markdown.Mode
	Stdin  *os.File
	Output io.Writer
}

type dependencies struct {
	read       func(context.Context, string) ([]byte, error)
	render     func([]byte, string) ([]byte, error)
	detectDark func(*os.File, *os.File) bool
	noColor    func() bool
	write      func(io.Writer, []byte) error
}

type renderResult struct {
	contents []byte
	err      error
}

// Run validates and completely renders the input before writing any output.
func Run(ctx context.Context, options Options) error {
	return run(ctx, options, dependencies{
		read:   readSource,
		render: renderMarkdown,
		detectDark: func(input, output *os.File) bool {
			return lipgloss.HasDarkBackground(input, output)
		},
		noColor: func() bool {
			return os.Getenv("NO_COLOR") != ""
		},
		write: writeOutput,
	})
}

func run(ctx context.Context, options Options, deps dependencies) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("view Markdown: %w", err)
	}
	if err := validateMode(options.Mode); err != nil {
		return err
	}

	input, err := files.Resolve(options.Input)
	if err != nil {
		return fmt.Errorf("resolve input %q: %w", options.Input, err)
	}
	if input.Kind != files.KindFile || !files.IsMarkdown(input.Path) {
		return fmt.Errorf("view input %q: expected a Markdown file", input.Path)
	}

	source, err := deps.read(ctx, input.Path)
	if err != nil {
		return fmt.Errorf("read Markdown %q: %w", input.Path, err)
	}

	noColor := deps.noColor()
	style := selectStyle(options, noColor, deps.detectDark)
	result := make(chan renderResult, 1)
	go func() {
		contents, renderErr := deps.render(source, style)
		result <- renderResult{contents: contents, err: renderErr}
	}()

	var rendered []byte
	select {
	case <-ctx.Done():
		return fmt.Errorf("render Markdown %q: %w", input.Path, ctx.Err())
	case completed := <-result:
		if completed.err != nil {
			return fmt.Errorf("render Markdown %q: %w", input.Path, completed.err)
		}
		rendered = completed.contents
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("render Markdown %q: %w", input.Path, err)
	}

	if noColor {
		rendered = []byte(xansi.Strip(string(rendered)))
	}
	output := options.Output
	if output == nil {
		output = os.Stdout
	}
	if err := deps.write(output, rendered); err != nil {
		return fmt.Errorf("write terminal output for %q: %w", input.Path, err)
	}
	return nil
}

func validateMode(mode markdown.Mode) error {
	switch mode {
	case markdown.ModeLight, markdown.ModeDark, markdown.ModeAuto:
		return nil
	default:
		return fmt.Errorf("invalid mode %q: expected light, dark, or auto", mode)
	}
}

func selectStyle(options Options, noColor bool, detectDark func(*os.File, *os.File) bool) string {
	switch options.Mode {
	case markdown.ModeLight:
		return styles.LightStyle
	case markdown.ModeDark:
		return styles.DarkStyle
	}
	if noColor {
		return styles.DarkStyle
	}
	stdin := options.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	output, ok := options.Output.(*os.File)
	if options.Output == nil {
		output = os.Stdout
		ok = true
	}
	if ok && detectDark(stdin, output) {
		return styles.DarkStyle
	}
	if ok {
		return styles.LightStyle
	}
	return styles.DarkStyle
}

func readSource(ctx context.Context, path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var source bytes.Buffer
	buffer := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count, readErr := file.Read(buffer)
		if count > 0 {
			_, _ = source.Write(buffer[:count])
		}
		if readErr == io.EOF {
			return source.Bytes(), nil
		}
		if readErr != nil {
			return nil, readErr
		}
	}
}

func renderMarkdown(source []byte, style string) ([]byte, error) {
	styleConfig := styles.DarkStyleConfig
	if style == styles.LightStyle {
		styleConfig = styles.LightStyleConfig
	}
	terminalRenderer := goldmarkrenderer.NewRenderer(
		goldmarkrenderer.WithNodeRenderers(
			util.Prioritized(ansi.NewRenderer(ansi.Options{
				WordWrap: terminalWidth,
				Styles:   styleConfig,
			}), 1000),
		),
	)
	engine := markdown.NewGFM()
	engine.SetRenderer(terminalRenderer)
	document := engine.Parser().Parse(text.NewReader(source))
	if err := markdown.SanitizeDangerousURLs(document); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := engine.Renderer().Render(&output, source, document); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func writeOutput(output io.Writer, contents []byte) error {
	_, err := lipgloss.Fprint(output, string(contents))
	return err
}
