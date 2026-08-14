package markdown

import (
	"strings"
	"testing"
)

// TestResolveKeyCanonicalNames pins representative canonical keys from every
// category of the PyMdown English US database: modifiers, punctuation,
// navigation, numeric keypad, function, media, browser and mouse keys.
func TestResolveKeyCanonicalNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input     string
		canonical string
		display   string
	}{
		{"control", "control", "Ctrl"},
		{"command", "command", "Cmd"},
		{"delete", "delete", "Del"},
		{"escape", "escape", "Esc"},

		{"backslash", "backslash", `\`},
		{"bar", "bar", "|"},
		{"brace-left", "brace-left", "{"},
		{"brace-right", "brace-right", "}"},
		{"less", "less", "<"},
		{"greater", "greater", ">"},
		{"double-quote", "double-quote", `"`},

		{"page-up", "page-up", "Page Up"},
		{"page-down", "page-down", "Page Down"},

		{"num-plus", "num-plus", "Num +"},
		{"num-enter", "num-enter", "Num Enter"},

		{"f1", "f1", "F1"},
		{"f12", "f12", "F12"},
		{"f24", "f24", "F24"},

		{"media-play-pause", "media-play-pause", "Play/Pause"},
		{"browser-refresh", "browser-refresh", "Browser Refresh"},
		{"left-button", "left-button", "Left Button"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := resolveKey(tt.input)
			if !got.known {
				t.Fatalf("resolveKey(%q) is unknown, want known", tt.input)
			}
			if got.canonical != tt.canonical {
				t.Errorf("resolveKey(%q).canonical = %q, want %q", tt.input, got.canonical, tt.canonical)
			}
			if got.display != tt.display {
				t.Errorf("resolveKey(%q).display = %q, want %q", tt.input, got.display, tt.display)
			}
		})
	}
}

// TestResolveKeyAliases pins the aliases users actually type, including the
// Windows/Qt-style shorthand PyMdown ships.
func TestResolveKeyAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input     string
		canonical string
	}{
		{"ctrl", "control"},
		{"cmd", "command"},
		{"del", "delete"},
		{"esc", "escape"},
		{"return", "enter"},
		{"pg-up", "page-up"},
		{"pg-dn", "page-down"},
		{"pipe", "bar"},
		{"open-brace", "brace-left"},
		{"close-brace", "brace-right"},
		{"open-bracket", "bracket-left"},
		{"close-bracket", "bracket-right"},
		{"left-ctrl", "left-control"},
		{"right-ctrl", "right-control"},
		{"win", "windows"},
		{"vol-up", "volume-up"},
		{"prtsc", "print-screen"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := resolveKey(tt.input)
			if !got.known {
				t.Fatalf("resolveKey(%q) is unknown, want alias to resolve", tt.input)
			}
			if got.canonical != tt.canonical {
				t.Errorf("resolveKey(%q).canonical = %q, want %q", tt.input, got.canonical, tt.canonical)
			}
		})
	}
}

// TestResolveKeyCaseInsensitive keeps m2h's existing tolerance: ++Ctrl++,
// ++CTRL++ and ++ctrl++ all resolve to the same canonical key, including for
// hyphenated aliases and mixed case.
func TestResolveKeyCaseInsensitive(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"Ctrl", "CTRL", "cTrL", "Control"} {
		got := resolveKey(input)
		if !got.known || got.canonical != "control" {
			t.Errorf("resolveKey(%q) = {canonical: %q, known: %v}, want canonical %q", input, got.canonical, got.known, "control")
		}
	}
	for _, input := range []string{"PG-UP", "Pg-Up"} {
		if got := resolveKey(input); !got.known || got.canonical != "page-up" {
			t.Errorf("resolveKey(%q) = {canonical: %q, known: %v}, want canonical %q", input, got.canonical, got.known, "page-up")
		}
	}
}

// TestResolveKeyUnknownKeepsRawText preserves m2h's compatibility contract for
// unknown keys: they stay unknown, keep their original casing and receive no
// canonical class.
func TestResolveKeyUnknownKeepsRawText(t *testing.T) {
	t.Parallel()

	got := resolveKey("FooBar")
	if got.known {
		t.Fatalf("resolveKey(FooBar) is known, want unknown")
	}
	if got.display != "FooBar" {
		t.Errorf("resolveKey(FooBar).display = %q, want the original text", got.display)
	}
	if got.canonical != "" {
		t.Errorf("resolveKey(FooBar).canonical = %q, want empty", got.canonical)
	}
}

// TestResolveKeyCoversWholeDatabase walks both maps so every canonical key
// resolves to itself and every alias points at a real canonical key. This is
// the guard that keeps the database complete when it is edited.
func TestResolveKeyCoversWholeDatabase(t *testing.T) {
	t.Parallel()

	for name, display := range keyMap {
		got := resolveKey(name)
		if !got.known {
			t.Errorf("resolveKey(%q) is unknown, want known", name)
			continue
		}
		if got.canonical != name {
			t.Errorf("resolveKey(%q).canonical = %q, want %q", name, got.canonical, name)
		}
		if got.display != display {
			t.Errorf("resolveKey(%q).display = %q, want %q", name, got.display, display)
		}
	}

	for alias, canonical := range keyAliases {
		display, ok := keyMap[canonical]
		if !ok {
			t.Errorf("alias %q targets unknown canonical key %q", alias, canonical)
			continue
		}
		got := resolveKey(alias)
		if !got.known {
			t.Errorf("resolveKey(%q) is unknown, want alias to resolve", alias)
			continue
		}
		if got.canonical != canonical {
			t.Errorf("resolveKey(%q).canonical = %q, want %q", alias, got.canonical, canonical)
		}
		if got.display != display {
			t.Errorf("resolveKey(%q).display = %q, want %q", alias, got.display, display)
		}
	}
}

// TestRenderKeysHTML pins the exact renderer output: canonical class names,
// canonical display labels, escaped punctuation and the raw fallback for
// unknown keys.
func TestRenderKeysHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "alias uses canonical class",
			source: "++ctrl++",
			want:   `<span class="keys"><kbd class="key-control">Ctrl</kbd></span>`,
		},
		{
			name:   "canonical name renders identically",
			source: "++control++",
			want:   `<span class="keys"><kbd class="key-control">Ctrl</kbd></span>`,
		},
		{
			name:   "mixed case input normalizes",
			source: "++CTRL++",
			want:   `<span class="keys"><kbd class="key-control">Ctrl</kbd></span>`,
		},
		{
			name:   "three key combination",
			source: "++ctrl+alt+del++",
			want: `<span class="keys"><kbd class="key-control">Ctrl</kbd><span>+</span>` +
				`<kbd class="key-alt">Alt</kbd><span>+</span><kbd class="key-delete">Del</kbd></span>`,
		},
		{
			name:   "command palette combination",
			source: "++cmd+shift+p++",
			want: `<span class="keys"><kbd class="key-command">Cmd</kbd><span>+</span>` +
				`<kbd class="key-shift">Shift</kbd><span>+</span><kbd class="key-p">P</kbd></span>`,
		},
		{
			name:   "page-up alias keeps canonical class",
			source: "++pg-up++",
			want:   `<span class="keys"><kbd class="key-page-up">Page Up</kbd></span>`,
		},
		{
			name:   "page-up in combination",
			source: "++ctrl+page-up++",
			want: `<span class="keys"><kbd class="key-control">Ctrl</kbd><span>+</span>` +
				`<kbd class="key-page-up">Page Up</kbd></span>`,
		},
		{
			name:   "plus key is distinct from the separator",
			source: "++ctrl+plus++",
			want: `<span class="keys"><kbd class="key-control">Ctrl</kbd><span>+</span>` +
				`<kbd class="key-plus">+</kbd></span>`,
		},
		{
			name:   "letter key",
			source: "++a++",
			want:   `<span class="keys"><kbd class="key-a">A</kbd></span>`,
		},
		{
			name:   "function key",
			source: "++f12++",
			want:   `<span class="keys"><kbd class="key-f12">F12</kbd></span>`,
		},
		{
			name:   "numeric keypad",
			source: "++num-enter++",
			want:   `<span class="keys"><kbd class="key-num-enter">Num Enter</kbd></span>`,
		},
		{
			name:   "punctuation less is escaped",
			source: "++less++",
			want:   `<span class="keys"><kbd class="key-less">&lt;</kbd></span>`,
		},
		{
			name:   "punctuation greater is escaped",
			source: "++greater++",
			want:   `<span class="keys"><kbd class="key-greater">&gt;</kbd></span>`,
		},
		{
			name:   "punctuation double-quote is escaped",
			source: "++double-quote++",
			want:   `<span class="keys"><kbd class="key-double-quote">&#34;</kbd></span>`,
		},
		{
			name:   "punctuation single-quote is escaped",
			source: "++single-quote++",
			want:   `<span class="keys"><kbd class="key-single-quote">&#39;</kbd></span>`,
		},
		{
			name:   "unknown key keeps raw text without class",
			source: "++foobar++",
			want:   `<span class="keys"><kbd>foobar</kbd></span>`,
		},
		{
			name:   "unknown markup-like key is escaped",
			source: "++<b>++",
			want:   `<span class="keys"><kbd>&lt;b&gt;</kbd></span>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := renderBody(t, tt.source)
			if !strings.Contains(body, tt.want) {
				t.Errorf("body does not contain %q:\n%s", tt.want, body)
			}
		})
	}
}

// TestRenderKeysNoAliasClasses guards the core refactor invariant: a rendered
// alias-only class such as key-ctrl or key-pg-up must never come back, because
// it would give one physical key two different DOM classes.
func TestRenderKeysNoAliasClasses(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "++ctrl+alt+del+pg-up+win++")
	for _, unwanted := range []string{`class="key-ctrl"`, `class="key-del"`, `class="key-pg-up"`, `class="key-win"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("body contains alias class %q:\n%s", unwanted, body)
		}
	}
	for _, want := range []string{`class="key-control"`, `class="key-delete"`, `class="key-page-up"`, `class="key-windows"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain canonical class %q:\n%s", want, body)
		}
	}
}

// TestRenderKeysAliasAndCanonicalIdentical asserts the DOM-equivalence
// contract: ++ctrl++ and ++control++ produce byte-identical markup.
func TestRenderKeysAliasAndCanonicalIdentical(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{
		{"++ctrl++", "++control++"},
		{"++cmd++", "++command++"},
		{"++del++", "++delete++"},
		{"++esc++", "++escape++"},
		{"++pg-up++", "++page-up++"},
		{"++pipe++", "++bar++"},
		{"++open-brace++", "++brace-left++"},
	}
	for _, pair := range pairs {
		alias, canonical := renderBody(t, pair[0]), renderBody(t, pair[1])
		if alias != canonical {
			t.Errorf("%s and %s render differently:\n%s\n%s", pair[0], pair[1], alias, canonical)
		}
	}
}

// TestKeysDoesNotFireInCodeSpan keeps code spans literal.
func TestKeysDoesNotFireInCodeSpan(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "`++ctrl+alt++`")
	if !strings.Contains(body, "<code>++ctrl+alt++</code>") {
		t.Errorf("keys fired inside a code span:\n%s", body)
	}
	if strings.Contains(body, "<kbd") {
		t.Errorf("code span produced kbd:\n%s", body)
	}
}

// TestKeysLiteralProse keeps C++ and prose plus signs out of the Keys parser.
func TestKeysLiteralProse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
	}{
		{name: "C++ stays literal", source: "C++ 语言"},
		{name: "prose plus stays literal", source: "a + b"},
		{name: "unterminated opener stays literal", source: "++oops"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := renderBody(t, tt.source)
			if strings.Contains(body, `<span class="keys">`) {
				t.Errorf("body unexpectedly produced keys:\n%s", body)
			}
		})
	}
}

func TestValidKeysContent(t *testing.T) {
	t.Parallel()

	valid := []string{"ctrl", "ctrl+alt+del", "f1", "a-b", "backslash"}
	invalid := []string{"", "a b", "a\tb", "+ctrl", "ctrl+", "+", "a\nb"}
	for _, in := range valid {
		if !validKeysContent([]byte(in)) {
			t.Errorf("validKeysContent(%q) = false, want true", in)
		}
	}
	for _, in := range invalid {
		if validKeysContent([]byte(in)) {
			t.Errorf("validKeysContent(%q) = true, want false", in)
		}
	}
}
