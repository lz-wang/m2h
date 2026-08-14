package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
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
