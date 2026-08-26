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
func New(buildVersion string, ui fs.FS, stdin io.Reader, stdout, stderr io.Writer) (*urfavecli.Command, error) {
	info, err := version.Parse(buildVersion)
	if err != nil {
		return nil, fmt.Errorf("configure CLI: %w", err)
	}

	command := &urfavecli.Command{
		Name:        "m2h",
		Usage:       "serve Markdown documents in a browser",
		UsageText:   "m2h [options] <file|directory>...\n   m2h convert [options] <file|directory>",
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
			convertCommand(stdin),
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

func convertCommand(stdin io.Reader) *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "convert",
		Usage:     "convert Markdown to HTML",
		ArgsUsage: "<file|directory>",
		Flags:     conversionFlags(),
		Action: func(ctx context.Context, command *urfavecli.Command) error {
			return convertAction(ctx, command, stdin)
		},
		OnUsageError: normalizeUsageError,
	}
}

// conversionFlags returns the Markdown-to-HTML options for the convert
// subcommand.
func conversionFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "write to an HTML file or output directory", Local: true},
		&urfavecli.StringFlag{Name: "glob", Usage: "match Markdown paths with a doublestar glob", Local: true},
		&urfavecli.IntFlag{Name: "depth", Aliases: []string{"d"}, Value: defaultDepth, Usage: "maximum directory recursion depth", Local: true},
		modeFlag(),
		widthFlag(),
		&urfavecli.BoolFlag{
			Name:  "standalone",
			Usage: "embed runtime assets and local images into each HTML file (directory mode)",
			Local: true,
		},
		&urfavecli.BoolFlag{
			Name:        "copy-assets",
			Value:       true,
			DefaultText: "true",
			Usage:       "copy non-Markdown assets in directory mode",
			Local:       true,
		},
		&urfavecli.BoolFlag{
			Name:    "yes",
			Aliases: []string{"y"},
			Usage:   "skip the conversion confirmation prompt",
			Local:   true,
		},
	}
}

var runConvert = convert.RunWithResult

func convertAction(ctx context.Context, command *urfavecli.Command, stdin io.Reader) error {
	if command.Args().Len() == 0 {
		return urfavecli.ShowCommandHelp(ctx, command, "convert")
	}
	if command.Args().Len() != 1 {
		return fmt.Errorf("Error: requires exactly one file or directory")
	}
	proceed, err := confirmConversion(command, stdin)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}
	result, err := runConvert(ctx, convert.Options{
		Input:         command.Args().First(),
		Output:        command.String("output"),
		Pattern:       command.String("glob"),
		Depth:         command.Int("depth"),
		Mode:          markdown.Mode(command.String("mode")),
		Width:         markdown.Width(command.String("width")),
		CopyAssets:    command.Bool("copy-assets"),
		Standalone:    command.Bool("standalone"),
		PatternSet:    command.IsSet("glob"),
		DepthSet:      command.IsSet("depth"),
		CopyAssetsSet: command.IsSet("copy-assets"),
		StandaloneSet: command.IsSet("standalone"),
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

// confirmConversion asks the user to confirm the conversion unless --yes was
// given. Non-interactive stdin without --yes is an error; a declined answer
// prints "Aborted." and stops the command without failing it.
func confirmConversion(command *urfavecli.Command, stdin io.Reader) (bool, error) {
	if command.Bool("yes") {
		return true, nil
	}
	prompt := convertPrompt(command.Args().First(), command.String("output"))
	if prompt == "" {
		return true, nil
	}
	if !interactiveStdin(stdin) {
		return false, fmt.Errorf("Error: conversion requires confirmation; rerun with --yes")
	}
	if !confirm(stdin, command.Root().ErrWriter, prompt) {
		_, _ = fmt.Fprintln(command.Root().ErrWriter, "Aborted.")
		return false, nil
	}
	return true, nil
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
