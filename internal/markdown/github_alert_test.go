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

	for _, typ := range []AlertType{AlertNote, AlertTip, AlertImportant, AlertWarning, AlertCaution} {
		svg := string(alertIconSVG(typ))
		if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "<path") {
			t.Errorf("alertIconSVG(%s) not a valid inline svg: %s", typ, svg)
		}
		if title := alertTitleText(typ); title == "" {
			t.Errorf("alertTitleText(%s) returned empty title", typ)
		}
	}

	// Defensive: an unknown variant yields no icon and no title rather than
	// emitting partial markup.
	if got := alertIconSVG("bogus"); got != nil {
		t.Errorf("alertIconSVG(unknown) = %q, want nil", got)
	}
	if got := alertTitleText("bogus"); got != "" {
		t.Errorf("alertTitleText(unknown) = %q, want empty", got)
	}
}
