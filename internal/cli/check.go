package cli

import (
	"context"
	"errors"
	"fmt"

	urfavecli "github.com/urfave/cli/v3"

	"github.com/lz-wang/m2h/internal/check"
)

func checkCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:         "check",
		Usage:        "check Markdown documents for problems",
		ArgsUsage:    "<file|directory>",
		Flags:        checkFlags(),
		Action:       checkAction,
		OnUsageError: normalizeUsageError,
	}
}

// checkFlags returns the document-scope and reporting options for the check
// subcommand. --glob and --depth mirror the root serve command's flags so
// "m2h docs" and "m2h check docs" always see the same document scope, while
// --enable/--disable select which rules run: the defaults, widened by
// --enable and narrowed by --disable (which wins).
func checkFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "glob", Usage: "match Markdown paths with a doublestar glob", Local: true},
		&urfavecli.IntFlag{
			Name:    "depth",
			Aliases: []string{"d"},
			Value:   defaultDepth,
			Usage:   "maximum directory recursion depth",
			Local:   true,
			Validator: func(value int) error {
				if value < 0 {
					return fmt.Errorf("Error: --depth must be zero or greater")
				}
				return nil
			},
		},
		&urfavecli.StringFlag{
			Name:             "format",
			Value:            string(check.FormatText),
			Usage:            "output format: text or json",
			ValidateDefaults: true,
			Local:            true,
			Validator: func(value string) error {
				switch check.Format(value) {
				case check.FormatText, check.FormatJSON:
					return nil
				default:
					return fmt.Errorf("Error: --format must be text or json")
				}
			},
		},
		&urfavecli.BoolFlag{Name: "strict", Usage: "treat warnings as failures", Local: true},
		&urfavecli.StringSliceFlag{
			Name:  "enable",
			Usage: `enable additional check rules (comma-separated; "all" addresses every rule)`,
			Local: true,
		},
		&urfavecli.StringSliceFlag{
			Name:  "disable",
			Usage: `disable check rules, overriding --enable (comma-separated; "all" addresses every rule)`,
			Local: true,
		},
	}
}

var runCheck = check.Run

var errCheckFailure = errors.New("check found issues")

// IsCheckFailure reports whether err represents a completed check whose
// diagnostics require a non-zero process exit. The report already contains
// the details, so the process entrypoint must not print this internal signal.
func IsCheckFailure(err error) bool {
	return errors.Is(err, errCheckFailure)
}

func checkAction(ctx context.Context, command *urfavecli.Command) error {
	if command.Args().Len() == 0 {
		return urfavecli.ShowCommandHelp(ctx, command.Root(), command.Name)
	}
	if command.Args().Len() != 1 {
		return fmt.Errorf("Error: requires exactly one file or directory")
	}
	result, err := runCheck(ctx, check.Options{
		Input:        command.Args().First(),
		Pattern:      command.String("glob"),
		Depth:        command.Int("depth"),
		EnableRules:  command.StringSlice("enable"),
		DisableRules: command.StringSlice("disable"),
	})
	if err != nil {
		return fmt.Errorf("Error: %w", err)
	}
	if err := check.WriteReport(command.Root().Writer, result, check.Format(command.String("format"))); err != nil {
		return fmt.Errorf("Error: %w", err)
	}
	if result.Errors > 0 || (command.Bool("strict") && result.Warnings > 0) {
		return errCheckFailure
	}
	return nil
}
