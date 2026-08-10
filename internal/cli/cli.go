// Package cli defines the public m2h command-line contract.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	urfavecli "github.com/urfave/cli/v3"

	"github.com/lz-wang/m2h/internal/convert"
	"github.com/lz-wang/m2h/internal/markdown"
	"github.com/lz-wang/m2h/internal/server"
	"github.com/lz-wang/m2h/internal/version"
	"github.com/lz-wang/m2h/internal/view"
)

const (
	defaultDepth = 2
	defaultHost  = server.DefaultHost
	defaultMode  = "auto"
	defaultWidth = "standard"
	defaultPort  = server.DefaultPort
)

// New constructs the root command after validating the injected build version.
func New(buildVersion string, ui fs.FS, stdout, stderr io.Writer) (*urfavecli.Command, error) {
	info, err := version.Parse(buildVersion)
	if err != nil {
		return nil, fmt.Errorf("configure CLI: %w", err)
	}

	command := &urfavecli.Command{
		Name:        "m2h",
		Usage:       "convert and preview GitHub-flavored Markdown in browsers or terminals",
		UsageText:   "m2h [global options] command [command options]",
		HideVersion: true,
		Writer:      stdout,
		ErrWriter:   stderr,
		Flags: []urfavecli.Flag{
			&urfavecli.BoolFlag{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "print the m2h version",
			},
		},
		Commands: []*urfavecli.Command{
			versionCommand(info),
			convertCommand(),
			previewCommand(ui),
			viewCommand(),
		},
		OnUsageError: normalizeUsageError,
	}
	command.Action = func(_ context.Context, current *urfavecli.Command) error {
		if current.Bool("version") {
			return info.Write(current.Writer)
		}
		return urfavecli.ShowRootCommandHelp(current)
	}

	return command, nil
}

func versionCommand(info version.Info) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "version",
		Usage: "print the m2h version",
		Action: func(_ context.Context, command *urfavecli.Command) error {
			return info.Write(command.Root().Writer)
		},
		OnUsageError: normalizeUsageError,
	}
}

func convertCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "convert",
		Usage:     "convert Markdown to GitHub-style HTML",
		ArgsUsage: "<file|directory>",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "write to an HTML file or output directory"},
			&urfavecli.StringFlag{Name: "glob", Usage: "match Markdown paths with a doublestar glob"},
			&urfavecli.IntFlag{Name: "depth", Aliases: []string{"d"}, Value: defaultDepth, Usage: "maximum directory recursion depth"},
			modeFlag(),
			widthFlag(),
			&urfavecli.BoolFlag{
				Name:        "copy-assets",
				Value:       true,
				DefaultText: "true",
				Usage:       "copy non-Markdown assets in directory mode",
			},
			&urfavecli.BoolFlag{Name: "unsafe-html", Usage: "allow raw HTML in Markdown"},
		},
		Action:       convertAction,
		OnUsageError: normalizeUsageError,
	}
}

func convertAction(ctx context.Context, command *urfavecli.Command) error {
	if command.Args().Len() != 1 {
		return fmt.Errorf("Error: convert requires exactly one file or directory")
	}
	err := convert.Run(ctx, convert.Options{
		Input:         command.Args().First(),
		Output:        command.String("output"),
		Pattern:       command.String("glob"),
		Depth:         command.Int("depth"),
		Mode:          markdown.Mode(command.String("mode")),
		Width:         markdown.Width(command.String("width")),
		CopyAssets:    command.Bool("copy-assets"),
		UnsafeHTML:    command.Bool("unsafe-html"),
		PatternSet:    command.IsSet("glob"),
		DepthSet:      command.IsSet("depth"),
		CopyAssetsSet: command.IsSet("copy-assets"),
		Log:           command.Root().ErrWriter,
	})
	if err == nil || strings.HasPrefix(err.Error(), "Error:") {
		return err
	}
	return fmt.Errorf("Error: %w", err)
}

func previewCommand(ui fs.FS) *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "preview",
		Usage:     "preview Markdown in a browser",
		ArgsUsage: "<file|directory>",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{Name: "host", Value: defaultHost, Usage: "listen host"},
			&urfavecli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   defaultPort,
				Usage:   "listen port",
				Validator: func(value int) error {
					if value < 1 || value > 65535 {
						return fmt.Errorf("Error: --port must be between 1 and 65535")
					}
					return nil
				},
			},
			&urfavecli.BoolFlag{Name: "browser", Usage: "open the default browser after listening"},
			modeFlag(),
			widthFlag(),
			&urfavecli.BoolFlag{Name: "unsafe-html", Usage: "allow raw HTML in Markdown"},
			&urfavecli.StringFlag{Name: "glob", Usage: "match Markdown paths with a doublestar glob"},
			&urfavecli.IntFlag{Name: "depth", Aliases: []string{"d"}, Value: defaultDepth, Usage: "maximum directory recursion depth"},
		},
		Action: func(ctx context.Context, command *urfavecli.Command) error {
			return previewAction(ctx, command, ui)
		},
		OnUsageError: normalizeUsageError,
	}
}

var runPreview = server.Run

func previewAction(ctx context.Context, command *urfavecli.Command, ui fs.FS) error {
	if command.Args().Len() != 1 {
		return fmt.Errorf("Error: preview requires exactly one file or directory")
	}
	err := runPreview(ctx, server.Options{
		Input:      command.Args().First(),
		Host:       command.String("host"),
		Port:       command.Int("port"),
		Mode:       markdown.Mode(command.String("mode")),
		Width:      markdown.Width(command.String("width")),
		Browser:    command.Bool("browser"),
		UnsafeHTML: command.Bool("unsafe-html"),
		Pattern:    command.String("glob"),
		Depth:      command.Int("depth"),
		PatternSet: command.IsSet("glob"),
		DepthSet:   command.IsSet("depth"),
		Log:        command.Root().ErrWriter,
		UI:         ui,
	})
	if err == nil || strings.HasPrefix(err.Error(), "Error:") {
		return err
	}
	return fmt.Errorf("Error: %w", err)
}

func viewCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:         "view",
		Usage:        "render one Markdown file in the terminal",
		ArgsUsage:    "<file>",
		Flags:        []urfavecli.Flag{modeFlag()},
		Action:       viewAction,
		OnUsageError: normalizeUsageError,
	}
}

var runView = view.Run

func viewAction(ctx context.Context, command *urfavecli.Command) error {
	if command.Args().Len() != 1 {
		return fmt.Errorf("Error: view requires exactly one Markdown file")
	}
	err := runView(ctx, view.Options{
		Input:  command.Args().First(),
		Mode:   markdown.Mode(command.String("mode")),
		Stdin:  os.Stdin,
		Output: command.Root().Writer,
	})
	if err == nil || strings.HasPrefix(err.Error(), "Error:") {
		return err
	}
	return fmt.Errorf("Error: %w", err)
}

func modeFlag() *urfavecli.StringFlag {
	return &urfavecli.StringFlag{
		Name:             "mode",
		Value:            defaultMode,
		Usage:            "color mode: light, dark, or auto",
		ValidateDefaults: true,
		Validator: func(value string) error {
			switch value {
			case "light", "dark", "auto":
				return nil
			default:
				return fmt.Errorf("Error: --mode must be one of light, dark, or auto")
			}
		},
	}
}

func widthFlag() *urfavecli.StringFlag {
	return &urfavecli.StringFlag{
		Name:             "width",
		Value:            defaultWidth,
		Usage:            "document width: standard, wide, or full",
		ValidateDefaults: true,
		Validator: func(value string) error {
			switch value {
			case "standard", "wide", "full":
				return nil
			default:
				return fmt.Errorf("Error: --width must be one of standard, wide, or full")
			}
		},
	}
}

func normalizeUsageError(_ context.Context, _ *urfavecli.Command, err error, _ bool) error {
	message := err.Error()
	if strings.Contains(message, "flag provided but not defined") {
		return fmt.Errorf("Error: unknown option")
	}
	if strings.HasPrefix(message, "Error:") {
		return err
	}
	if index := strings.Index(message, "Error: "); index >= 0 {
		return errors.New(message[index:])
	}
	return fmt.Errorf("Error: %w", err)
}
