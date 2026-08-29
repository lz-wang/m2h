package files

import (
	"errors"
	"testing"
)

func TestDecodeRelativePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "plain path", value: "guide.md", want: "guide.md"},
		{name: "nested path", value: "design/architecture.md", want: "design/architecture.md"},
		{name: "spaces survive", value: "space name.md", want: "space name.md"},
		{name: "unicode survives", value: "计划.md", want: "计划.md"},
		{name: "encoded separator", value: "design%2Farchitecture.md", want: "design/architecture.md"},
		{name: "encoded backslash becomes slash", value: "sub%5Cdeep.md", want: "sub/deep.md"},
		{name: "literal backslash becomes slash", value: `sub\deep.md`, want: "sub/deep.md"},
		{name: "dot segment is cleaned", value: "images%2F.%2Flogo.png", want: "images/logo.png"},
		{name: "repeated separators are cleaned", value: "images%2F%2Flogo.png", want: "images/logo.png"},
		{name: "multi-layer encoding decodes repeatedly", value: "%2567uide.md", want: "guide.md"},
		{name: "encoded space", value: "my%20logo.png", want: "my logo.png"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodeRelativePath(test.value)
			if err != nil {
				t.Fatalf("DecodeRelativePath(%q) returned error: %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("DecodeRelativePath(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestDecodeRelativePathRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "current directory", value: "."},
		{name: "parent escape", value: "../guide.md"},
		{name: "encoded parent escape", value: "..%2Fguide.md"},
		{name: "encoded mid-path parent", value: "sub%2F..%2Fguide.md"},
		{name: "absolute path", value: "/guide.md"},
		{name: "encoded absolute path", value: "%2Fetc%2Fpasswd"},
		{name: "windows volume", value: `C:\guide.md`},
		{name: "encoded windows volume", value: "C%3Aguide.md"},
		{name: "NUL byte", value: "name\x00.md"},
		{name: "encoded NUL byte", value: "name%00.md"},
		{name: "malformed percent", value: "%"},
		{name: "decoding never stabilizes", value: "%2525252525252525252e"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := DecodeRelativePath(test.value); err == nil {
				t.Fatalf("DecodeRelativePath(%q) succeeded, want rejection", test.value)
			}
		})
	}
}

func TestDecodeRelativePathReturnsValueForDiagnostics(t *testing.T) {
	t.Parallel()

	// On rejection the decoded value comes back alongside the error so
	// diagnostics can still show which path was refused.
	got, err := DecodeRelativePath("..%2Fsecret.md")
	if err == nil {
		t.Fatal("DecodeRelativePath(..%2Fsecret.md) succeeded, want rejection")
	}
	if got != "../secret.md" {
		t.Fatalf("value = %q, want the decoded path for diagnostics", got)
	}
	if !errors.Is(err, err) {
		t.Fatal("error must satisfy errors.Is with itself")
	}
}
