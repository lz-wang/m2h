package markdown

import "fmt"

// OptionError reports a render option rejected before parsing or I/O.
type OptionError struct {
	Name  string
	Value string
}

func (err *OptionError) Error() string {
	return fmt.Sprintf("invalid Markdown %s %q", err.Name, err.Value)
}
