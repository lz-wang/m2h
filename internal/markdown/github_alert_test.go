package markdown

import (
	"strings"
	"testing"
)

func TestParseAlertMarker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		line  string
		want  AlertType
		found bool
	}{
		{name: "note upper", line: "[!NOTE]", want: AlertNote, found: true},
		{name: "tip lower", line: "[!tip]", want: AlertTip, found: true},
		{name: "important mixed case", line: "[!Important]", want: AlertImportant, found: true},
		{name: "warning", line: "[!WARNING]", want: AlertWarning, found: true},
		{name: "caution", line: "[!Caution]", want: AlertCaution, found: true},
		{name: "padded marker", line: "  [!NOTE]  ", want: AlertNote, found: true},
		{name: "unknown variant", line: "[!UNKNOWN]", found: false},
		{name: "plain text", line: "Just text", found: false},
		{name: "trailing content", line: "[!NOTE] more", found: false},
		{name: "too short", line: "[!]", found: false},
		{name: "missing bang", line: "[NOTE]", found: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseAlertMarker([]byte(tc.line))
			if ok != tc.found || (ok && got != tc.want) {
				t.Fatalf("parseAlertMarker(%q) = (%q, %v), want (%q, %v)", tc.line, got, ok, tc.want, tc.found)
			}
		})
	}
}

func TestAlertIconsAndTitles(t *testing.T) {
	t.Parallel()

	titles := map[AlertType]string{
		AlertNote:      "NOTE",
		AlertTip:       "TIP",
		AlertImportant: "IMPORTANT",
		AlertWarning:   "WARNING",
		AlertCaution:   "CAUTION",
	}
	for typ, want := range titles {
		svg := string(alertIconSVG(typ))
		if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "<path") {
			t.Errorf("alertIconSVG(%s) not a valid inline svg: %s", typ, svg)
		}
		if title := alertTitleText(typ); title != want {
			t.Errorf("alertTitleText(%s) = %q, want %q", typ, title, want)
		}
	}

	// Defensive: an unknown variant yields no icon rather than emitting a
	// broken one. Its title is a deterministic uppercase passthrough — and the
	// parser only produces the five known variants, so it never renders.
	if got := alertIconSVG("bogus"); got != nil {
		t.Errorf("alertIconSVG(unknown) = %q, want nil", got)
	}
	if got := alertTitleText("bogus"); got != "BOGUS" {
		t.Errorf("alertTitleText(unknown) = %q, want %q", got, "BOGUS")
	}
}
