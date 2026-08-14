package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// confirm prints prompt to writer and reads one line from reader. It reports
// true only for an explicit y/yes answer (case-insensitive, surrounding
// whitespace ignored); an empty answer or read failure declines.
func confirm(reader io.Reader, writer io.Writer, prompt string) bool {
	if _, err := fmt.Fprint(writer, prompt); err != nil {
		return false
	}
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// interactiveStdin reports whether reader is an interactive terminal. It is a
// package variable so tests can force the interactive path without a TTY.
var interactiveStdin = func(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// convertPrompt renders the confirmation prompt shown before a conversion.
// Paths are echoed exactly as the user typed them. A missing input returns an
// empty prompt: nothing exists to overwrite yet, so conversion should run and
// report its own error.
func convertPrompt(input, output string) string {
	info, err := os.Stat(input)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		if output == "" {
			return fmt.Sprintf("Convert %s in place (may overwrite existing HTML)? [y/N] ", input)
		}
		return fmt.Sprintf("Convert %s into %s? [y/N] ", input, output)
	}
	target := output
	if target == "" {
		target = strings.TrimSuffix(input, filepath.Ext(input)) + ".html"
	}
	return fmt.Sprintf("Convert %s to %s? [y/N] ", input, target)
}
