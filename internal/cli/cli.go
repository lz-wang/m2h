// Package cli defines the public m2h command-line contract.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	urfavecli "github.com/urfave/cli/v3"

	"github.com/lz-wang/m2h/internal/convert"
	"github.com/lz-wang/m2h/internal/markdown"
	"github.com/lz-wang/m2h/internal/version"
)

const (
	defaultDepth = 2
	defaultHost  = "127.0.0.1"
	defaultMode  = "auto"
	defaultPort  = 8793
)

// New constructs the root command after validating the injected build version.
func New(buildVersion string, stdout, stderr io.Writer) (*urfavecli.Command, error) {
	info, err := version.Parse(buildVersion)
	if err != nil {
		return nil, fmt.Errorf("configure CLI: %w", err)
	}

	command := &urfavecli.Command{
		Name:        "m2h",
		Usage:       "convert and preview GitHub-flavored Markdown",
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
			previewCommand(),
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

func previewCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "preview",
		Usage:     "preview Markdown in a browser",
		ArgsUsage: "<file|directory>",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{Name: "host", Value: defaultHost, Usage: "listen host"},
			&urfavecli.IntFlag{Name: "port", Aliases: []string{"p"}, Value: defaultPort, Usage: "listen port"},
			&urfavecli.BoolFlag{Name: "browser", Usage: "open the default browser after listening"},
			modeFlag(),
			&urfavecli.BoolFlag{Name: "unsafe-html", Usage: "allow raw HTML in Markdown"},
			&urfavecli.StringFlag{Name: "glob", Usage: "match Markdown paths with a doublestar glob"},
			&urfavecli.IntFlag{Name: "depth", Aliases: []string{"d"}, Value: defaultDepth, Usage: "maximum directory recursion depth"},
		},
		Action:       notImplemented("preview"),
		OnUsageError: normalizeUsageError,
	}
}

func viewCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:         "view",
		Usage:        "render one Markdown file in the terminal",
		ArgsUsage:    "<file>",
		Flags:        []urfavecli.Flag{modeFlag()},
		Action:       notImplemented("view"),
		OnUsageError: normalizeUsageError,
	}
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

func notImplemented(name string) urfavecli.ActionFunc {
	return func(context.Context, *urfavecli.Command) error {
		return fmt.Errorf("Error: %s is not implemented in this release", name)
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
