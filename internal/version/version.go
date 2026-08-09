// Package version validates and prints m2h build versions.
package version

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const Development = "dev-unknown-unknown"

var (
	releasePattern    = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	developmentCommit = regexp.MustCompile(`^[0-9a-f]{7}$`)
)

// Info is a validated build version.
type Info struct {
	value string
}

// Parse validates a release version or a development build version.
func Parse(value string) (Info, error) {
	if releasePattern.MatchString(value) {
		return Info{value: value}, nil
	}

	if value == Development {
		return Info{value: value}, nil
	}

	const prefix = "dev-"
	if strings.HasPrefix(value, prefix) {
		parts := strings.Split(strings.TrimPrefix(value, prefix), "-")
		if len(parts) == 2 && validDate(parts[0]) && developmentCommit.MatchString(parts[1]) {
			return Info{value: value}, nil
		}
	}

	return Info{}, fmt.Errorf("invalid m2h version %q", value)
}

func validDate(value string) bool {
	if len(value) != len("20060102") {
		return false
	}
	_, err := time.Parse("20060102", value)
	return err == nil
}

// String returns the validated version without a leading v.
func (info Info) String() string {
	return info.value
}

// Write prints the version followed by a newline.
func (info Info) Write(writer io.Writer) error {
	if _, err := fmt.Fprintln(writer, info.value); err != nil {
		return fmt.Errorf("write version: %w", err)
	}
	return nil
}
