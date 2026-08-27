package markdown

import (
	"bytes"
	stdhtml "html"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// The Keys extension implements PyMdown Extensions' ++key+key++ syntax on top
// of the shared GFM engine, so export output and the WebUI stay identical.
//
// keyMap and keyAliases follow PyMdown Extensions' English US key database.
// Canonical key names are also used as CSS class suffixes:
//
//	control -> key-control
//	page-up -> key-page-up
//
// Keep aliases separate from canonical display labels: keyMap owns the
// canonical names and their display text, keyAliases only normalizes an alias
// to its canonical name, and the rendered class always uses the canonical name
// so ++ctrl++ and ++control++ produce the same DOM.

// KindKeys is the AST node kind for ++key+key++ sequences.
var KindKeys = ast.NewNodeKind("Keys")

// keysNode holds the raw ++...++ content (without the surrounding plus pairs).
// The renderer splits it on '+' and emits one <kbd> per segment.
type keysNode struct {
	ast.BaseInline
	content []byte
}

// Dump implements ast.Node.Dump.
func (n *keysNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// Kind implements ast.Node.Kind.
func (n *keysNode) Kind() ast.NodeKind { return KindKeys }

// keyMap maps a canonical key name to its display label.
var keyMap = map[string]string{
	// Digits
	"0": "0",
	"1": "1",
	"2": "2",
	"3": "3",
	"4": "4",
	"5": "5",
	"6": "6",
	"7": "7",
	"8": "8",
	"9": "9",

	// Letters
	"a": "A",
	"b": "B",
	"c": "C",
	"d": "D",
	"e": "E",
	"f": "F",
	"g": "G",
	"h": "H",
	"i": "I",
	"j": "J",
	"k": "K",
	"l": "L",
	"m": "M",
	"n": "N",
	"o": "O",
	"p": "P",
	"q": "Q",
	"r": "R",
	"s": "S",
	"t": "T",
	"u": "U",
	"v": "V",
	"w": "W",
	"x": "X",
	"y": "Y",
	"z": "Z",

	// Space
	"space": "Space",

	// Punctuation
	"backslash":     `\`,
	"bar":           "|",
	"brace-left":    "{",
	"brace-right":   "}",
	"bracket-left":  "[",
	"bracket-right": "]",
	"colon":         ":",
	"comma":         ",",
	"double-quote":  `"`,
	"equal":         "=",
	"exclam":        "!",
	"grave":         "`",
	"greater":       ">",
	"less":          "<",
	"minus":         "-",
	"period":        ".",
	"plus":          "+",
	"question":      "?",
	"semicolon":     ";",
	"single-quote":  "'",
	"slash":         "/",
	"tilde":         "~",
	"underscore":    "_",

	// Navigation keys
	"arrow-up":    "Up",
	"arrow-down":  "Down",
	"arrow-left":  "Left",
	"arrow-right": "Right",
	"page-up":     "Page Up",
	"page-down":   "Page Down",
	"home":        "Home",
	"end":         "End",

	// Edit keys
	"backspace": "Backspace",
	"delete":    "Del",
	"insert":    "Ins",
	"tab":       "Tab",

	// Action keys
	"break":        "Break",
	"caps-lock":    "Caps Lock",
	"clear":        "Clear",
	"eject":        "Eject",
	"enter":        "Enter",
	"escape":       "Esc",
	"help":         "Help",
	"print-screen": "Print Screen",
	"scroll-lock":  "Scroll Lock",

	// Numeric keypad
	"num0":          "Num 0",
	"num1":          "Num 1",
	"num2":          "Num 2",
	"num3":          "Num 3",
	"num4":          "Num 4",
	"num5":          "Num 5",
	"num6":          "Num 6",
	"num7":          "Num 7",
	"num8":          "Num 8",
	"num9":          "Num 9",
	"num-asterisk":  "Num *",
	"num-clear":     "Num Clear",
	"num-delete":    "Num Del",
	"num-equal":     "Num =",
	"num-lock":      "Num Lock",
	"num-minus":     "Num -",
	"num-plus":      "Num +",
	"num-separator": "Num .",
	"num-slash":     "Num /",
	"num-enter":     "Num Enter",

	// Modifier keys
	"alt":           "Alt",
	"alt-graph":     "AltGr",
	"command":       "Cmd",
	"control":       "Ctrl",
	"function":      "Fn",
	"left-alt":      "Left Alt",
	"left-command":  "Left Command",
	"left-control":  "Left Ctrl",
	"left-meta":     "Left Meta",
	"left-option":   "Left Option",
	"left-shift":    "Left Shift",
	"left-super":    "Left Super",
	"left-windows":  "Left Win",
	"meta":          "Meta",
	"option":        "Option",
	"right-alt":     "Right Alt",
	"right-command": "Right Command",
	"right-control": "Right Ctrl",
	"right-meta":    "Right Meta",
	"right-option":  "Right Option",
	"right-shift":   "Right Shift",
	"right-super":   "Right Super",
	"right-windows": "Right Win",
	"shift":         "Shift",
	"super":         "Super",
	"windows":       "Win",

	// Function keys
	"f1":  "F1",
	"f2":  "F2",
	"f3":  "F3",
	"f4":  "F4",
	"f5":  "F5",
	"f6":  "F6",
	"f7":  "F7",
	"f8":  "F8",
	"f9":  "F9",
	"f10": "F10",
	"f11": "F11",
	"f12": "F12",
	"f13": "F13",
	"f14": "F14",
	"f15": "F15",
	"f16": "F16",
	"f17": "F17",
	"f18": "F18",
	"f19": "F19",
	"f20": "F20",
	"f21": "F21",
	"f22": "F22",
	"f23": "F23",
	"f24": "F24",

	// Extra keys
	"backtab":           "Back Tab",
	"browser-back":      "Browser Back",
	"browser-favorites": "Browser Favorites",
	"browser-forward":   "Browser Forward",
	"browser-home":      "Browser Home",
	"browser-refresh":   "Browser Refresh",
	"browser-search":    "Browser Search",
	"browser-stop":      "Browser Stop",
	"context-menu":      "Menu",
	"copy":              "Copy",
	"mail":              "Mail",
	"media":             "Media",
	"media-next-track":  "Next Track",
	"media-pause":       "Pause",
	"media-play":        "Play",
	"media-play-pause":  "Play/Pause",
	"media-prev-track":  "Previous Track",
	"media-stop":        "Stop",
	"print":             "Print",
	"reset":             "Reset",
	"select":            "Select",
	"sleep":             "Sleep",
	"volume-down":       "Volume Down",
	"volume-mute":       "Mute",
	"volume-up":         "Volume Up",
	"zoom":              "Zoom",
	"power":             "Power",
	"fingerprint":       "Fingerprint",

	// Mouse
	"left-button":   "Left Button",
	"middle-button": "Middle Button",
	"right-button":  "Right Button",
	"x-button1":     "X Button 1",
	"x-button2":     "X Button 2",
}

// keyAliases maps an alias to its canonical key name.
var keyAliases = map[string]string{
	"add":           "num-plus",
	"altgr":         "alt-graph",
	"apps":          "context-menu",
	"back":          "backspace",
	"bksp":          "backspace",
	"bktab":         "backtab",
	"cancel":        "break",
	"capital":       "caps-lock",
	"close-brace":   "brace-right",
	"close-bracket": "bracket-right",
	"clr":           "clear",
	"cmd":           "command",
	"cplk":          "caps-lock",
	"ctrl":          "control",
	"dblquote":      "double-quote",
	"decimal":       "num-separator",
	"del":           "delete",
	"divide":        "num-slash",
	"down":          "arrow-down",
	"esc":           "escape",
	"return":        "enter",
	"exclamation":   "exclam",
	"favorites":     "browser-favorites",
	"fn":            "function",
	"forward":       "browser-forward",
	"grave-accent":  "grave",
	"greater-than":  "greater",
	"gt":            "greater",
	"hyphen":        "minus",
	"ins":           "insert",
	"lalt":          "left-alt",
	"launch-mail":   "mail",
	"launch-media":  "media",
	"lbutton":       "left-button",
	"lcmd":          "left-command",
	"lcommand":      "left-command",
	"lcontrol":      "left-control",
	"lctrl":         "left-control",
	"left":          "arrow-left",
	"left-cmd":      "left-command",
	"left-ctrl":     "left-control",
	"lopt":          "left-option",
	"loption":       "left-option",
	"left-opt":      "left-option",
	"left-win":      "left-windows",
	"less-than":     "less",
	"lmeta":         "left-meta",
	"lshift":        "left-shift",
	"lsuper":        "left-super",
	"lt":            "less",
	"lwin":          "left-windows",
	"lwindows":      "left-windows",
	"mbutton":       "middle-button",
	"menu":          "context-menu",
	"multiply":      "num-asterisk",
	"mute":          "volume-mute",
	"next":          "page-down",
	"next-track":    "media-next-track",
	"num-del":       "num-delete",
	"numlk":         "num-lock",
	"open-brace":    "brace-left",
	"open-bracket":  "bracket-left",
	"opt":           "option",
	"page-dn":       "page-down",
	"page-up":       "page-up",
	"pause":         "media-pause",
	"pg-dn":         "page-down",
	"pg-up":         "page-up",
	"pipe":          "bar",
	"play":          "media-play",
	"play-pause":    "media-play-pause",
	"prev-track":    "media-prev-track",
	"prior":         "page-up",
	"prtsc":         "print-screen",
	"question-mark": "question",
	"ralt":          "right-alt",
	"rbutton":       "right-button",
	"rcontrol":      "right-control",
	"rcmd":          "right-command",
	"rcommand":      "right-command",
	"rctrl":         "right-control",
	"refresh":       "browser-refresh",
	"right":         "arrow-right",
	"right-cmd":     "right-command",
	"right-ctrl":    "right-control",
	"right-meta":    "right-meta",
	"right-opt":     "right-option",
	"right-win":     "right-windows",
	"rmeta":         "right-meta",
	"ropt":          "right-option",
	"roption":       "right-option",
	"rshift":        "right-shift",
	"rsuper":        "right-super",
	"rwin":          "right-windows",
	"rwindows":      "right-windows",
	"scroll":        "scroll-lock",
	"search":        "browser-search",
	"separator":     "num-separator",
	"spc":           "space",
	"stop":          "media-stop",
	"subtract":      "num-minus",
	"tabulator":     "tab",
	"up":            "arrow-up",
	"vol-down":      "volume-down",
	"vol-mute":      "volume-mute",
	"vol-up":        "volume-up",
	"win":           "windows",
	"xbutton1":      "x-button1",
	"xbutton2":      "x-button2",
}

// resolvedKey is the single answer resolveKey gives for one raw segment.
type resolvedKey struct {
	canonical string
	display   string
	known     bool
}

// resolveKey normalizes one raw key segment. Input is matched case-insensitively
// (so ++Ctrl++ and ++CTRL++ behave like ++ctrl++), aliases collapse to their
// canonical key, and an unknown key keeps its original text.
func resolveKey(raw string) resolvedKey {
	normalized := strings.ToLower(raw)

	canonical := normalized
	if alias, ok := keyAliases[normalized]; ok {
		canonical = alias
	}

	if display, ok := keyMap[canonical]; ok {
		return resolvedKey{
			canonical: canonical,
			display:   display,
			known:     true,
		}
	}

	return resolvedKey{
		display: raw,
		known:   false,
	}
}

// keysParser recognizes ++ctrl+alt+del++ on a single line. It only fires on a
// real pair: the opener must be exactly two pluses, the closer must exist on the
// same line, and the content must be non-empty with no whitespace. Those guards
// keep C++, "a + b" and other prose plus signs literal.
type keysParser struct{}

// newKeysParser returns the Keys inline parser.
func newKeysParser() parser.InlineParser { return &keysParser{} }

func (*keysParser) Trigger() []byte { return []byte{'+'} }

func (*keysParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 3 || line[0] != '+' || line[1] != '+' || line[2] == '+' {
		return nil
	}
	rest := line[2:]
	closeIdx := bytes.Index(rest, []byte("++"))
	if closeIdx < 0 {
		return nil
	}
	content := rest[:closeIdx]
	if !validKeysContent(content) {
		return nil
	}
	block.Advance(2 + closeIdx + 2)
	return &keysNode{content: append([]byte(nil), content...)}
}

func (*keysParser) CloseBlock(ast.Node, parser.Context) {}

// validKeysContent rejects empty content, whitespace (so prose like "C++ C++"
// cannot match) and a leading or trailing '+' that would split into empty keys.
func validKeysContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	for _, b := range content {
		switch b {
		case ' ', '\t', '\n', '\r':
			return false
		}
	}
	return !bytes.HasPrefix(content, []byte("+")) && !bytes.HasSuffix(content, []byte("+"))
}

// renderKeys emits <span class="keys">…</span> with one <kbd> per segment joined
// by <span>+</span>. Known keys and aliases get the canonical key-* class and
// canonical label; unknown segments keep their escaped text and no class. Both
// paths escape the display value because punctuation keys render characters
// like <, > and " that would otherwise break the surrounding HTML.
func renderKeys(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*keysNode)

	_, _ = w.WriteString(`<span class="keys">`)

	for i, segment := range bytes.Split(n.content, []byte("+")) {
		if i > 0 {
			_, _ = w.WriteString("<span>+</span>")
		}

		raw := string(segment)
		key := resolveKey(raw)

		if key.known {
			_, _ = w.WriteString(`<kbd class="key-`)
			_, _ = w.WriteString(key.canonical)
			_, _ = w.WriteString(`">`)
			_, _ = w.WriteString(stdhtml.EscapeString(key.display))
			_, _ = w.WriteString("</kbd>")
			continue
		}

		_, _ = w.WriteString("<kbd>")
		_, _ = w.WriteString(stdhtml.EscapeString(raw))
		_, _ = w.WriteString("</kbd>")
	}

	_, _ = w.WriteString("</span>")

	return ast.WalkContinue, nil
}
