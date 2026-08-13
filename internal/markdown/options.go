package markdown

import "fmt"

// OptionError reports a render option rejected before parsing or I/O.
type OptionError struct {
	Name  string
	Value string
	// Reason optionally explains the constraint the value violated.
	Reason string
}

func (err *OptionError) Error() string {
	if err.Reason != "" {
		return fmt.Sprintf("invalid Markdown %s %q: %s", err.Name, err.Value, err.Reason)
	}
	return fmt.Sprintf("invalid Markdown %s %q", err.Name, err.Value)
}
