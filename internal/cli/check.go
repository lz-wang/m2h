package cli

import (
	"context"
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
// "m2h docs" and "m2h check docs" always see the same document scope.
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
	}
}

var runCheck = check.Run

func checkAction(ctx context.Context, command *urfavecli.Command) error {
	if command.Args().Len() == 0 {
		return urfavecli.ShowCommandHelp(ctx, command.Root(), command.Name)
	}
	if command.Args().Len() != 1 {
		return fmt.Errorf("Error: requires exactly one file or directory")
	}
	result, err := runCheck(ctx, check.Options{
		Input:   command.Args().First(),
		Pattern: command.String("glob"),
		Depth:   command.Int("depth"),
	})
	if err != nil {
		return fmt.Errorf("Error: %w", err)
	}
	if err := check.WriteReport(command.Root().Writer, result, check.Format(command.String("format"))); err != nil {
		return fmt.Errorf("Error: %w", err)
	}
	if result.Errors > 0 || (command.Bool("strict") && result.Warnings > 0) {
		return fmt.Errorf("Error: check found %s", checkFailureDetails(result))
	}
	return nil
}

// checkFailureDetails names what failed in the exit error, reusing the same
// counting vocabulary as the summary line.
func checkFailureDetails(result check.Result) string {
	switch {
	case result.Warnings == 0:
		return countNoun(result.Errors, "error", "errors")
	case result.Errors == 0:
		return countNoun(result.Warnings, "warning", "warnings")
	default:
		return countNoun(result.Errors, "error", "errors") + " and " + countNoun(result.Warnings, "warning", "warnings")
	}
}

func countNoun(count int, singular string, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
