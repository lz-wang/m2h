// Package cli defines the public m2h command-line contract.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	urfavecli "github.com/urfave/cli/v3"

	"github.com/lz-wang/m2h/internal/convert"
	"github.com/lz-wang/m2h/internal/markdown"
	"github.com/lz-wang/m2h/internal/server"
	"github.com/lz-wang/m2h/internal/version"
)

const (
	defaultDepth = 4
	defaultHost  = server.DefaultHost
	defaultMode  = "auto"
	defaultWidth = "standard"
	defaultPort  = server.DefaultPort
	defaultTOC   = true
)

// New constructs the root command after validating the injected build version.
func New(buildVersion string, ui fs.FS, stdout, stderr io.Writer) (*urfavecli.Command, error) {
	info, err := version.Parse(buildVersion)
	if err != nil {
		return nil, fmt.Errorf("configure CLI: %w", err)
	}

	command := &urfavecli.Command{
		Name:        "m2h",
		Usage:       "convert GitHub-flavored Markdown to HTML",
		UsageText:   "m2h [options] <file|directory>\n   m2h web [options] <file|directory>",
		HideVersion: true,
		Writer:      stdout,
		ErrWriter:   stderr,
		Flags: append(
			conversionFlags(),
			&urfavecli.BoolFlag{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "print the m2h version",
				Local:   true,
			},
		),
		Commands: []*urfavecli.Command{
			webCommand(ui),
		},
		OnUsageError: normalizeUsageError,
	}
	command.Action = func(ctx context.Context, current *urfavecli.Command) error {
		if current.Bool("version") {
			return info.Write(current.Writer)
		}
		return convertAction(ctx, current)
	}

	return command, nil
}

// conversionFlags returns the Markdown-to-HTML options for the default convert
// action on the root command.
func conversionFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "write to an HTML file or output directory", Local: true},
		&urfavecli.StringFlag{Name: "glob", Usage: "match Markdown paths with a doublestar glob", Local: true},
		&urfavecli.IntFlag{Name: "depth", Aliases: []string{"d"}, Value: defaultDepth, Usage: "maximum directory recursion depth", Local: true},
		modeFlag(),
		widthFlag(),
		&urfavecli.BoolFlag{
			Name:        "copy-assets",
			Value:       true,
			DefaultText: "true",
			Usage:       "copy non-Markdown assets in directory mode",
			Local:       true,
		},
	}
}

var runConvert = convert.RunWithResult

func convertAction(ctx context.Context, command *urfavecli.Command) error {
	if command.Args().Len() == 0 {
		return urfavecli.ShowRootCommandHelp(command)
	}
	if command.Args().Len() != 1 {
		return fmt.Errorf("Error: requires exactly one file or directory")
	}
	result, err := runConvert(ctx, convert.Options{
		Input:         command.Args().First(),
		Output:        command.String("output"),
		Pattern:       command.String("glob"),
		Depth:         command.Int("depth"),
		Mode:          markdown.Mode(command.String("mode")),
		Width:         markdown.Width(command.String("width")),
		CopyAssets:    command.Bool("copy-assets"),
		PatternSet:    command.IsSet("glob"),
		DepthSet:      command.IsSet("depth"),
		CopyAssetsSet: command.IsSet("copy-assets"),
		Log:           command.Root().ErrWriter,
	})
	if err == nil || strings.HasPrefix(err.Error(), "Error:") {
		if err != nil {
			return err
		}
		if err := result.WriteSummary(command.Root().Writer); err != nil {
			return fmt.Errorf("Error: write conversion result: %w", err)
		}
		return nil
	}
	return fmt.Errorf("Error: %w", err)
}

func webCommand(ui fs.FS) *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "web",
		Usage:     "view Markdown in a browser",
		ArgsUsage: "<file|directory>",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{Name: "host", Value: defaultHost, Usage: "listen host", Local: true},
			&urfavecli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   defaultPort,
				Usage:   "listen port",
				Local:   true,
				Validator: func(value int) error {
					if value < 1 || value > 65535 {
						return fmt.Errorf("Error: --port must be between 1 and 65535")
					}
					return nil
				},
			},
			&urfavecli.BoolWithInverseFlag{Name: "open", Value: true, Usage: "open the default browser after listening", Local: true},
			modeFlag(),
			widthFlag(),
			&urfavecli.BoolFlag{
				Name:        "toc",
				Value:       defaultTOC,
				DefaultText: "true",
				Usage:       "show the document table of contents",
				Local:       true,
			},
			&urfavecli.StringFlag{Name: "glob", Usage: "match Markdown paths with a doublestar glob", Local: true},
			&urfavecli.IntFlag{Name: "depth", Aliases: []string{"d"}, Value: defaultDepth, Usage: "maximum directory recursion depth", Local: true},
		},
		Action: func(ctx context.Context, command *urfavecli.Command) error {
			return webAction(ctx, command, ui)
		},
		OnUsageError: normalizeUsageError,
	}
}

var runPreview = server.Run

func webAction(ctx context.Context, command *urfavecli.Command, ui fs.FS) error {
	if command.Args().Len() != 1 {
		return fmt.Errorf("Error: web requires exactly one file or directory")
	}
	err := runPreview(ctx, server.Options{
		Input:      command.Args().First(),
		Host:       command.String("host"),
		Port:       command.Int("port"),
		Mode:       markdown.Mode(command.String("mode")),
		Width:      markdown.Width(command.String("width")),
		Browser:    command.Bool("open"),
		TOC:        command.Bool("toc"),
		Pattern:    command.String("glob"),
		Depth:      command.Int("depth"),
		PatternSet: command.IsSet("glob"),
		DepthSet:   command.IsSet("depth"),
		TOCSet:     command.IsSet("toc"),
		Log:        command.Root().ErrWriter,
		UI:         ui,
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
		Local:            true,
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
		Local:            true,
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
