// Package convert transforms one Markdown file or a discovered directory tree.
package convert

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/lz-wang/m2h/internal/assets"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
)

// Options configures file or directory conversion.
type Options struct {
	Input         string
	Output        string
	Pattern       string
	Depth         int
	Mode          markdown.Mode
	Width         markdown.Width
	CopyAssets    bool
	PatternSet    bool
	DepthSet      bool
	CopyAssetsSet bool
	Log           io.Writer
}

// Result describes the files produced by a successful conversion.
type Result struct {
	HTMLFiles    []string
	CopiedAssets int
}

// WriteSummary writes a concise, human-readable conversion summary.
func (result Result) WriteSummary(writer io.Writer) error {
	if writer == nil {
		return fmt.Errorf("conversion result writer is nil")
	}

	fileLabel := "files"
	if len(result.HTMLFiles) == 1 {
		fileLabel = "file"
	}
	if _, err := fmt.Fprintf(writer, "Converted %d Markdown %s", len(result.HTMLFiles), fileLabel); err != nil {
		return err
	}
	if result.CopiedAssets > 0 {
		assetLabel := "assets"
		if result.CopiedAssets == 1 {
			assetLabel = "asset"
		}
		if _, err := fmt.Fprintf(writer, "; copied %d %s", result.CopiedAssets, assetLabel); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer, "."); err != nil {
		return err
	}
	if len(result.HTMLFiles) == 0 {
		_, err := fmt.Fprintln(writer, "Output HTML files: none.")
		return err
	}
	if _, err := fmt.Fprintln(writer, "Output HTML files:"); err != nil {
		return err
	}
	for _, output := range result.HTMLFiles {
		if _, err := fmt.Fprintf(writer, "- %s\n", output); err != nil {
			return err
		}
	}
	return nil
}

type plannedFile struct {
	source       string
	sourcePath   string
	destination  string
	mode         fs.FileMode
	markdown     bool
	renderedHTML []byte
}

// Run validates options before resolving the input and performs the conversion.
func Run(ctx context.Context, options Options) error {
	_, err := RunWithResult(ctx, options)
	return err
}

// RunWithResult validates options, performs the conversion, and returns the generated HTML files.
func RunWithResult(ctx context.Context, options Options) (Result, error) {
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
	switch input.Kind {
	case files.KindFile:
		return runFile(ctx, input.Path, options)
	case files.KindDirectory:
		return runDirectory(ctx, input.Path, options)
	default:
		return Result{}, fmt.Errorf("convert %q: unsupported input kind", input.Path)
	}
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
	return files.ValidateDiscoverOptions(files.DiscoverOptions{Depth: options.Depth, Pattern: options.Pattern})
}

func runFile(ctx context.Context, source string, options Options) (Result, error) {
	if options.PatternSet {
		return Result{}, fmt.Errorf("--glob can only be used when converting a directory")
	}
	if options.DepthSet {
		return Result{}, fmt.Errorf("--depth can only be used when converting a directory")
	}
	if options.CopyAssetsSet {
		return Result{}, fmt.Errorf("--copy-assets can only be used when converting a directory")
	}
	if !files.IsMarkdown(source) {
		return Result{}, fmt.Errorf("convert %q: expected a Markdown file", source)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("convert %q: %w", source, err)
	}

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
	if info, err := os.Stat(destination); err == nil && info.IsDir() {
		return Result{}, fmt.Errorf("convert %q: output %q is a directory", source, destination)
	} else if err != nil && !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("inspect output %q: %w", destination, err)
	}

	contents, err := os.ReadFile(source)
	if err != nil {
		return Result{}, fmt.Errorf("read Markdown %q: %w", source, err)
	}
	rendered, err := markdown.Render(contents, markdown.RenderOptions{
		Mode:       options.Mode,
		Width:      options.Width,
		Target:     markdown.TargetConvert,
		SourcePath: filepath.Base(source),
		AssetBase:  assets.RichAssetDir + "/",
	})
	if err != nil {
		return Result{}, fmt.Errorf("render Markdown %q: %w", source, err)
	}
	if err := writeAtomic(destination, []byte(rendered.HTML), 0o644); err != nil {
		return Result{}, fmt.Errorf("write HTML %q: %w", destination, err)
	}
	assetDir := filepath.Join(filepath.Dir(destination), assets.RichAssetDir)
	if err := assets.WriteRichAssets(assetDir); err != nil {
		return Result{}, fmt.Errorf("write rich-content assets %q: %w", assetDir, err)
	}
	return Result{HTMLFiles: []string{destination}}, nil
}

func runDirectory(ctx context.Context, sourceRoot string, options Options) (Result, error) {
	outputRoot, err := directoryOutputRoot(sourceRoot, options.Output)
	if err != nil {
		return Result{}, err
	}
	excludeRoot := ""
	if !samePath(sourceRoot, outputRoot) && files.IsWithin(sourceRoot, outputRoot) {
		excludeRoot = outputRoot
	}
	discovered, err := files.Discover(ctx, sourceRoot, files.DiscoverOptions{
		Depth:       options.Depth,
		Pattern:     options.Pattern,
		ExcludeRoot: excludeRoot,
		Excludes:    []string{filepath.Join(outputRoot, assets.RichAssetDir)},
		Log:         options.Log,
	})
	if err != nil {
		return Result{}, err
	}

	plans := make([]plannedFile, 0, len(discovered.Markdown)+len(discovered.Assets))
	for _, entry := range discovered.Markdown {
		plans = append(plans, plannedFile{
			source:      entry.AbsolutePath,
			sourcePath:  entry.RelativePath,
			destination: filepath.Join(outputRoot, filepath.FromSlash(changeExtension(entry.RelativePath))),
			mode:        0o644,
			markdown:    true,
		})
	}
	if options.CopyAssets && !samePath(sourceRoot, outputRoot) {
		for _, entry := range discovered.Assets {
			plans = append(plans, plannedFile{
				source:      entry.AbsolutePath,
				sourcePath:  entry.RelativePath,
				destination: filepath.Join(outputRoot, filepath.FromSlash(entry.RelativePath)),
				mode:        entry.Mode.Perm(),
			})
		}
	}
	if err := detectConflicts(plans); err != nil {
		return Result{}, err
	}
	sortPlans(plans)

	for index := range plans {
		if !plans[index].markdown {
			continue
		}
		if err := ctx.Err(); err != nil {
			return Result{}, fmt.Errorf("convert directory %q: %w", sourceRoot, err)
		}
		contents, err := os.ReadFile(plans[index].source)
		if err != nil {
			return Result{}, fmt.Errorf("read Markdown %q: %w", plans[index].source, err)
		}
		rendered, err := markdown.Render(contents, markdown.RenderOptions{
			Mode:       options.Mode,
			Width:      options.Width,
			Target:     markdown.TargetConvert,
			SourcePath: plans[index].sourcePath,
			AssetBase:  richAssetBase(plans[index].sourcePath),
		})
		if err != nil {
			return Result{}, fmt.Errorf("render Markdown %q: %w", plans[index].source, err)
		}
		plans[index].renderedHTML = []byte(rendered.HTML)
	}

	result := Result{HTMLFiles: make([]string, 0, len(discovered.Markdown))}
	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return Result{}, fmt.Errorf("convert directory %q: %w", sourceRoot, err)
		}
		if plan.markdown {
			err = writeAtomic(plan.destination, plan.renderedHTML, plan.mode)
		} else {
			err = copyAtomic(plan.source, plan.destination, plan.mode)
		}
		if err != nil {
			return Result{}, fmt.Errorf("write %q from %q: %w", plan.destination, plan.source, err)
		}
		if plan.markdown {
			result.HTMLFiles = append(result.HTMLFiles, plan.destination)
		} else {
			result.CopiedAssets++
		}
	}

	// Only emit the shared rich-content runtime when at least one HTML file was
	// generated, so an empty glob match leaves the output directory untouched.
	if len(discovered.Markdown) > 0 {
		assetDir := filepath.Join(outputRoot, assets.RichAssetDir)
		if err := assets.WriteRichAssets(assetDir); err != nil {
			return Result{}, fmt.Errorf("write rich-content assets %q: %w", assetDir, err)
		}
	}
	return result, nil
}

func directoryOutputRoot(sourceRoot, output string) (string, error) {
	if output == "" {
		return sourceRoot, nil
	}
	absolute, err := files.CanonicalPath(output)
	if err != nil {
		return "", fmt.Errorf("resolve output %q: %w", output, err)
	}
	info, err := os.Stat(absolute)
	if err == nil && !info.IsDir() {
		return "", fmt.Errorf("convert directory %q: output %q is not a directory", sourceRoot, absolute)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect output %q: %w", absolute, err)
	}
	return absolute, nil
}

func detectConflicts(plans []plannedFile) error {
	destinations := make(map[string]plannedFile, len(plans))
	for _, plan := range plans {
		key := destinationKey(plan.destination)
		if previous, exists := destinations[key]; exists {
			return fmt.Errorf(
				"output conflict %q: both %q and %q map to the same destination",
				plan.destination,
				previous.source,
				plan.source,
			)
		}
		destinations[key] = plan
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

// richAssetBase returns the relative URL prefix a generated HTML file must use
// to reach the shared rich-content asset directory at the output root. A file
// nested one directory deeper needs one extra "../" segment.
func richAssetBase(relativePath string) string {
	directory := path.Dir(files.NormalizeRelativePath(relativePath))
	levels := 0
	if directory != "." && directory != "" {
		levels = strings.Count(directory, "/") + 1
	}
	return strings.Repeat("../", levels) + assets.RichAssetDir + "/"
}

func changeExtension(relative string) string {
	relative = files.NormalizeRelativePath(relative)
	extension := path.Ext(relative)
	return strings.TrimSuffix(relative, extension) + ".html"
}

func sortPlans(plans []plannedFile) {
	sort.Slice(plans, func(left, right int) bool {
		return destinationKey(plans[left].destination) < destinationKey(plans[right].destination)
	})
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

func copyAtomic(source, destination string, mode fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source asset: %w", err)
	}
	defer func() {
		_ = input.Close()
	}()

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".m2h-*")
	if err != nil {
		return fmt.Errorf("create temporary asset: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(mode.Perm()); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary asset mode: %w", err)
	}
	if _, err := io.Copy(temporary, input); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("copy asset data: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary asset: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("rename temporary asset: %w", err)
	}
	return nil
}
