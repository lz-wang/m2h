package version

import (
	"bytes"
	"testing"
)

func TestParseAcceptsSupportedVersions(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"1.2.3",
		"0.1.0",
		"dev-20260809-fe65804",
		"dev-unknown-unknown",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			info, err := Parse(value)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", value, err)
			}
			if got := info.String(); got != value {
				t.Fatalf("Info.String() = %q, want %q", got, value)
			}
		})
	}
}

func TestParseRejectsUnsupportedVersions(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"",
		"v1.2.3",
		"1.2",
		"1.2.3-beta.1",
		"dev-2026089-fe65804",
		"dev-20260809-FE65804",
		"dev-today-fe65804",
		"dev-20260230-fe65804",
		"dev-20260809-short",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := Parse(value); err == nil {
				t.Fatalf("Parse(%q) succeeded, want error", value)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, bytes.ErrTooLarge
}

func TestInfoWritePreservesWriterError(t *testing.T) {
	t.Parallel()

	info, err := Parse("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if err := info.Write(failingWriter{}); err == nil {
		t.Fatal("Write() succeeded with a failing writer")
	}
}

func TestInfoWrite(t *testing.T) {
	t.Parallel()

	info, err := Parse("1.2.3")
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := info.Write(&output); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if got, want := output.String(), "1.2.3\n"; got != want {
		t.Fatalf("Write() = %q, want %q", got, want)
	}
}
