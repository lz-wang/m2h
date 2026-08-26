// Package convert exports a single Markdown file as a self-contained HTML page.
package convert

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

// Options configures a single-file Markdown export.
type Options struct {
	Input  string
	Output string
	Mode   markdown.Mode
	Width  markdown.Width
	// Force replaces an existing output file; without it an existing output is
	// an error.
	Force bool
}

// Result reports the HTML file produced by a successful export.
type Result struct {
	Output string
}

// Run validates options, renders one Markdown file, and writes the HTML page.
func Run(ctx context.Context, options Options) (Result, error) {
	if options.Width == "" {
		options.Width = markdown.WidthStandard
	}
	if err := validateOptions(options); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("convert: %w", err)
	}

	input, err := files.Resolve(options.Input)
	if err != nil {
		return Result{}, err
	}
	if input.Kind != files.KindFile || !files.IsMarkdown(input.Path) {
		return Result{}, fmt.Errorf("convert requires a Markdown file: %q", options.Input)
	}
	source := input.Path

	destination := options.Output
	if destination == "" {
		destination = strings.TrimSuffix(source, filepath.Ext(source)) + ".html"
	} else {
		absolute, err := files.CanonicalPath(destination)
		if err != nil {
			return Result{}, fmt.Errorf("resolve output %q: %w", destination, err)
		}
		destination = absolute
	}
	if samePath(source, destination) {
		return Result{}, fmt.Errorf("convert %q: output conflicts with input", source)
	}
	if info, err := os.Stat(destination); err == nil {
		if info.IsDir() {
			return Result{}, fmt.Errorf("convert %q: output %q is a directory", source, destination)
		}
		if !options.Force {
			return Result{}, fmt.Errorf("convert %q: output %q already exists; rerun with --force to overwrite", source, destination)
		}
	} else if !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("inspect output %q: %w", destination, err)
	}

	contents, err := os.ReadFile(source)
	if err != nil {
		return Result{}, fmt.Errorf("read Markdown %q: %w", source, err)
	}
	// The page inlines m2h's own Markdown CSS; large third-party runtimes
	// (KaTeX, Mermaid, Tablesort) load from a pinned CDN only when the
	// document actually uses them.
	rendered, err := markdown.Render(contents, markdown.RenderOptions{
		Mode:       options.Mode,
		Width:      options.Width,
		Target:     markdown.TargetConvert,
		SourcePath: filepath.Base(source),
	})
	if err != nil {
		return Result{}, fmt.Errorf("render Markdown %q: %w", source, err)
	}
	if err := writeAtomic(destination, []byte(rendered.HTML), 0o644); err != nil {
		return Result{}, fmt.Errorf("write HTML %q: %w", destination, err)
	}
	return Result{Output: destination}, nil
}

func validateOptions(options Options) error {
	if strings.TrimSpace(options.Input) == "" {
		return fmt.Errorf("input path is required")
	}
	switch options.Mode {
	case markdown.ModeLight, markdown.ModeDark, markdown.ModeAuto:
	default:
		return fmt.Errorf("invalid mode %q: expected light, dark, or auto", options.Mode)
	}
	width := options.Width
	if width == "" {
		width = markdown.WidthStandard
	}
	switch width {
	case markdown.WidthStandard, markdown.WidthWide, markdown.WidthFull:
	default:
		return fmt.Errorf("invalid width %q: expected standard, wide, or full", options.Width)
	}
	return nil
}

func destinationKey(value string) string {
	value = filepath.Clean(value)
	if runtime.GOOS == "windows" {
		return strings.ToLower(value)
	}
	return value
}

func samePath(left, right string) bool {
	leftAbsolute, leftErr := files.CanonicalPath(left)
	rightAbsolute, rightErr := files.CanonicalPath(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return destinationKey(leftAbsolute) == destinationKey(rightAbsolute)
}

func writeAtomic(destination string, contents []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".m2h-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(mode.Perm()); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary file mode: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("rename temporary file: %w", err)
	}
	return nil
}
