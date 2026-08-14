package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmAcceptsExplicitYesOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "lowercase y", input: "y\n", want: true},
		{name: "uppercase y", input: "Y\n", want: true},
		{name: "yes", input: "yes\n", want: true},
		{name: "YES uppercase", input: "YES\n", want: true},
		{name: "padded y", input: "  y  \n", want: true},
		{name: "y without trailing newline", input: "y", want: true},
		{name: "n declines", input: "n\n", want: false},
		{name: "empty line declines", input: "\n", want: false},
		{name: "eof declines", input: "", want: false},
		{name: "garbage declines", input: "maybe\n", want: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var prompt bytes.Buffer
			got := confirm(strings.NewReader(test.input), &prompt, "Proceed? [y/N] ")
			if got != test.want {
				t.Fatalf("confirm(%q) = %v, want %v", test.input, got, test.want)
			}
			if got := prompt.String(); got != "Proceed? [y/N] " {
				t.Fatalf("prompt output = %q, want prompt echoed verbatim", got)
			}
		})
	}
}
