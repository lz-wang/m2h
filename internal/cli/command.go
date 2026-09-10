package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"

	urfavecli "github.com/urfave/cli/v3"
)

// Command validates m2h's argument syntax before invoking the CLI parser.
type Command struct {
	root *urfavecli.Command
}

// Run accepts arguments including the executable name, as in os.Args.
func (command *Command) Run(ctx context.Context, args []string) error {
	// urfave normalizes both --toc and --toc=true to the same boolean before
	// flag validators run. Inspect tokens first to keep TOC a valueless switch.
	for index := 1; index < len(args); index++ {
		argument := strings.TrimSpace(args[index])
		if argument == "--" || argument == "-" {
			break
		}
		if !strings.HasPrefix(argument, "-") {
			if command.root.Command(argument) != nil {
				break // Subcommands enforce their own flag contracts.
			}
			continue
		}
		if !strings.HasPrefix(argument, "--") && !unicode.IsLetter(rune(argument[1])) {
			break // Like the parser, treat a negative number as positional.
		}
		name, _, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(argument, "-"), "-"), "=")
		if hasValue && (name == "toc" || name == "no-toc") {
			err := fmt.Errorf("--%s does not accept a value; use --toc or --no-toc", name)
			return normalizeUsageError(ctx, command.root, err, false)
		}
		if hasValue {
			continue
		}
		// A token consumed by another option (for example --glob --toc=true)
		// is that option's value, not a TOC switch. Reuse the declared flag
		// metadata so this check stays aligned with future value-taking flags.
		for _, flag := range command.root.Flags {
			if valueFlag, ok := flag.(urfavecli.DocGenerationFlag); ok &&
				slices.Contains(flag.Names(), name) && valueFlag.TakesValue() {
				index++
				break
			}
		}
	}
	return command.root.Run(ctx, args)
}
