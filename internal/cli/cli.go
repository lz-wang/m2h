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

	"github.com/lz-wang/m2h/internal/export"
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
		Usage:       "serve Markdown documents in a browser",
		UsageText:   "m2h [options] <file|directory>...\n   m2h convert [options] <file>",
		HideVersion: true,
		Writer:      stdout,
		ErrWriter:   stderr,
		Flags: append(
			serverFlags(),
			&urfavecli.BoolFlag{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "print the m2h version",
				Local:   true,
			},
		),
		Commands: []*urfavecli.Command{
			convertCommand(),
		},
		OnUsageError: normalizeUsageError,
	}
	command.Action = func(ctx context.Context, current *urfavecli.Command) error {
		if current.Bool("version") {
			return info.Write(current.Writer)
		}
		return serveAction(ctx, current, ui, info.String())
	}

	return command, nil
}

// serverFlags returns the document-server options for the root command.
func serverFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
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
	}
}

var runServer = server.Run

// serverInputs expands the root command's arguments into individual server
// inputs. Each argument may itself carry a comma-separated list — a
// convenience for shell histories — so "a,b c" and "a b c" are equivalent.
// Segments are trimmed, empty segments dropped, order preserved, and exact
// textual duplicates removed; inputs that only collide after resolution
// (trailing separators, symlink aliases) are rejected by the server.
func serverInputs(args urfavecli.Args) ([]string, error) {
	inputs := []string{}
	seen := make(map[string]bool, args.Len())
	for index := range args.Len() {
		for segment := range strings.SplitSeq(args.Get(index), ",") {
			trimmed := strings.TrimSpace(segment)
			if trimmed == "" || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			inputs = append(inputs, trimmed)
		}
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("Error: requires one or more files or directories")
	}
	return inputs, nil
}

func serveAction(ctx context.Context, command *urfavecli.Command, ui fs.FS, buildVersion string) error {
	if command.Args().Len() == 0 {
		return urfavecli.ShowRootCommandHelp(command)
	}
	// "m2h web" served documents before the root command became the server
	// itself. Callers migrating from that CLI get a pointed error instead of a
	// confusing "input not found" failure.
	if removed := command.Args().First(); removed == "web" {
		return fmt.Errorf("Error: unknown command %q", removed)
	}
	inputs, err := serverInputs(command.Args())
	if err != nil {
		return err
	}
	err = runServer(ctx, server.Options{
		Inputs:     inputs,
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
		Version:    buildVersion,
	})
	if err == nil || strings.HasPrefix(err.Error(), "Error:") {
		return err
	}
	return fmt.Errorf("Error: %w", err)
}

func convertCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:         "convert",
		Usage:        "convert a Markdown file to a self-contained HTML page",
		ArgsUsage:    "<file>",
		Flags:        conversionFlags(),
		Action:       convertAction,
		OnUsageError: normalizeUsageError,
	}
}

// conversionFlags returns the Markdown-to-HTML options for the convert
// subcommand.
func conversionFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "write to an HTML file", Local: true},
		modeFlag(),
		widthFlag(),
		&urfavecli.BoolFlag{
			Name:  "force",
			Usage: "overwrite the output file if it already exists",
			Local: true,
		},
	}
}

var runExport = export.Run

func convertAction(ctx context.Context, command *urfavecli.Command) error {
	if command.Args().Len() == 0 {
		return urfavecli.ShowCommandHelp(ctx, command, "convert")
	}
	if command.Args().Len() != 1 {
		return fmt.Errorf("Error: requires exactly one Markdown file")
	}
	result, err := runExport(ctx, export.Options{
		Input:  command.Args().First(),
		Output: command.String("output"),
		Mode:   markdown.Mode(command.String("mode")),
		Width:  markdown.Width(command.String("width")),
		Force:  command.Bool("force"),
	})
	if err == nil || strings.HasPrefix(err.Error(), "Error:") {
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(command.Root().Writer, "Wrote %s\n", result.Output); err != nil {
			return fmt.Errorf("Error: write conversion result: %w", err)
		}
		return nil
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
