package check

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const (
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiReset  = "\x1b[0m"
)

// Format selects the diagnostic output format.
type Format string

const (
	// FormatText prints one path:line:column line per diagnostic plus a
	// summary line — readable in a terminal and linkable in IDEs.
	FormatText Format = "text"
	// FormatJSON prints a stable object for CI and tooling consumption.
	FormatJSON Format = "json"
)

// WriteReport writes a completed result in the requested format.
func WriteReport(writer io.Writer, result Result, format Format) error {
	switch format {
	case FormatText:
		return writeTextReport(writer, result, terminalColorsEnabled(writer))
	case FormatJSON:
		return writeJSONReport(writer, result)
	default:
		return fmt.Errorf("write check report: unknown format %q", string(format))
	}
}

func writeTextReport(writer io.Writer, result Result, colorsEnabled bool) error {
	for _, diagnostic := range result.Diagnostics {
		_, err := fmt.Fprintf(
			writer,
			"%s:%d:%d: %s [%s]: %s\n",
			diagnostic.Path,
			diagnostic.Line,
			diagnostic.Column,
			coloredSeverity(diagnostic.Severity, colorsEnabled),
			diagnostic.Rule,
			diagnostic.Message,
		)
		if err != nil {
			return fmt.Errorf("write check report: %w", err)
		}
	}
	_, err := fmt.Fprintf(writer, "Checked %d Markdown %s: %s\n", result.Files, noun(result.Files, "file", "files"), textSummary(result, colorsEnabled))
	if err != nil {
		return fmt.Errorf("write check report: %w", err)
	}
	return nil
}

func textSummary(result Result, colorsEnabled bool) string {
	switch {
	case result.Errors == 0 && result.Warnings == 0:
		return colorize("no issues found", ansiGreen, colorsEnabled)
	case result.Warnings == 0:
		return colorize(countNoun(result.Errors, "error", "errors"), ansiRed, colorsEnabled)
	case result.Errors == 0:
		return colorize(countNoun(result.Warnings, "warning", "warnings"), ansiYellow, colorsEnabled)
	default:
		return colorize(countNoun(result.Errors, "error", "errors"), ansiRed, colorsEnabled) +
			", " + colorize(countNoun(result.Warnings, "warning", "warnings"), ansiYellow, colorsEnabled)
	}
}

func coloredSeverity(severity Severity, colorsEnabled bool) string {
	switch severity {
	case SeverityError:
		return colorize(string(severity), ansiRed, colorsEnabled)
	case SeverityWarning:
		return colorize(string(severity), ansiYellow, colorsEnabled)
	default:
		return string(severity)
	}
}

func colorize(value string, color string, enabled bool) string {
	if !enabled {
		return value
	}
	return color + value + ansiReset
}

// terminalColorsEnabled keeps redirected text reports machine-readable while
// making interactive diagnostics easier to scan. NO_COLOR and TERM=dumb are
// conventional opt-outs even when stdout is attached to a terminal.
func terminalColorsEnabled(writer io.Writer) bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled || os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// jsonReport is the stable wire contract of the JSON report: field order and
// names must not change once published, because CI jobs and editor tooling
// consume them directly.
type jsonReport struct {
	Files       int              `json:"files"`
	Errors      int              `json:"errors"`
	Warnings    int              `json:"warnings"`
	Diagnostics []jsonDiagnostic `json:"diagnostics"`
}

type jsonDiagnostic struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

func writeJSONReport(writer io.Writer, result Result) error {
	diagnostics := make([]jsonDiagnostic, 0, len(result.Diagnostics))
	for _, diagnostic := range result.Diagnostics {
		diagnostics = append(diagnostics, jsonDiagnostic{
			Path:     diagnostic.Path,
			Line:     diagnostic.Line,
			Column:   diagnostic.Column,
			Severity: string(diagnostic.Severity),
			Rule:     diagnostic.Rule,
			Message:  diagnostic.Message,
		})
	}
	encoded, err := json.MarshalIndent(jsonReport{
		Files:       result.Files,
		Errors:      result.Errors,
		Warnings:    result.Warnings,
		Diagnostics: diagnostics,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode check report: %w", err)
	}
	if _, err := writer.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write check report: %w", err)
	}
	return nil
}

// countNoun renders a count with its singular or plural noun, e.g. "1 error"
// or "3 errors", for human-facing summaries.
func countNoun(count int, singular string, plural string) string {
	return fmt.Sprintf("%d %s", count, noun(count, singular, plural))
}

// noun picks the singular or plural form for a count.
func noun(count int, singular string, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
